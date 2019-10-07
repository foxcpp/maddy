/*
Package queue implements module which keeps messages on disk and tries delivery
to the configured target (usually remote) multiple times until all recipients
are succeeded.

Interfaces implemented:
- module.DeliveryTarget

Implementation summary follows.

All scheduled deliveries are attempted to the configured DeliveryTarget.
All metadata is preserved on disk so

Failure status is determined on per-recipient basis:
- Delivery.Start fail handled as a failure for all recipients.
- Delivery.AddRcpt fail handled as a failure for corresponding recipient.
- Delivery.Body fail handled as a failure for all recipients.
- If Delivery implements PartialDelivery, then
  PartialDelivery.BodyNonAtomic is used instead. Failures are determined based
  on StatusCollector.SetStatus calls done by target in this case.

For each failure check is done to see if it is a permanent failure
or a temporary one. This is done using exterrors.IsTemporaryOrUnspec.
That is, errors are assumed to be temporary by default.
All errors are converted to SMTPError then due to a storage limitations.

If there are any *temporary* failed recipients, delivery will be retried
after delay *only for these* recipients.

Last error for each recipient is saved for reporting in NDN. A NDN is generated
if there are any failed recipients left after
last attempt to deliver the message.

Amount of attempts for each message is limited to a certain configured number.
After last attempt, all recipients that are still temporary failing are assumed
to be permanently failed.
*/
package queue

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	modconfig "github.com/foxcpp/maddy/config/module"
	"github.com/foxcpp/maddy/dispatcher"
	"github.com/foxcpp/maddy/dsn"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/target"
)

// partialError describes state of partially successful message delivery.
type partialError struct {
	// Recipients for which delivery permanently failed.
	Failed []string
	// Recipients for which delivery temporary failed.
	TemporaryFailed []string

	// Underlying error objects.
	Errs map[string]error

	// Fields can be accessed without holding this lock, but only after
	// target.BodyNonAtomic/Body returns.
	statusLock sync.Mutex
}

// SetStatus implements module.StatusCollector so partialError can be
// passed directly to PartialDelivery.BodyNonAtomic.
func (pe *partialError) SetStatus(rcptTo string, err error) {
	log.Debugf("PartialError.SetStatus(%s, %v)", rcptTo, err)
	if err == nil {
		return
	}
	pe.statusLock.Lock()
	defer pe.statusLock.Unlock()
	if exterrors.IsTemporaryOrUnspec(err) {
		pe.TemporaryFailed = append(pe.TemporaryFailed, rcptTo)
	} else {
		pe.Failed = append(pe.Failed, rcptTo)
	}
	pe.Errs[rcptTo] = err
}

func (pe partialError) Error() string {
	return fmt.Sprintf("delivery failed for some recipients (permanently: %v, temporary: %v): %v", pe.Failed, pe.TemporaryFailed, pe.Errs)
}

type Queue struct {
	name             string
	location         string
	hostname         string
	autogenMsgDomain string
	wheel            *TimeWheel

	dsnDispatcher *dispatcher.Dispatcher

	// Retry delay is calculated using the following formula:
	// initialRetryTime * retryTimeScale ^ (TriesCount - 1)

	initialRetryTime time.Duration
	retryTimeScale   float64
	maxTries         int

	// If any delivery is scheduled in less than postInitDelay
	// after Init, its delay will be increased by postInitDelay.
	//
	// Say, if postInitDelay is 10 secs.
	// Then if some message is scheduled to delivered 5 seconds
	// after init, it will be actually delivered 15 seconds
	// after start-up.
	//
	// This delay is added to make that if maddy is killed shortly
	// after start-up for whatever reason it will not affect the queue.
	postInitDelay time.Duration

	Log    log.Logger
	Target module.DeliveryTarget

	deliveryWg sync.WaitGroup
	// Buffered channel used to restrict count of deliveries attempted
	// in parallel.
	deliverySemaphore chan struct{}
}

type QueueMetadata struct {
	MsgMeta *module.MsgMetadata
	From    string

	// Recipients that should be tried next.
	// May or may not be equal to partialError.TemporaryFailed.
	To []string

	// Information about previous failures.
	// Preserved to be included in a bounce message.
	FailedRcpts          []string
	TemporaryFailedRcpts []string
	// All errors are converted to SMTPError we can serialize and
	// also it is directly usable for bounce messages.
	RcptErrs map[string]*smtp.SMTPError

	// Amount of times delivery *already tried*.
	TriesCount int

	FirstAttempt time.Time
	LastAttempt  time.Time

	// Whether this is a delivery notification.
	DSN bool
}

func NewQueue(_, instName string, _, inlineArgs []string) (module.Module, error) {
	q := &Queue{
		name:             instName,
		initialRetryTime: 15 * time.Minute,
		retryTimeScale:   2,
		postInitDelay:    10 * time.Second,
		Log:              log.Logger{Name: "queue"},
	}
	switch len(inlineArgs) {
	case 0:
		// Not inline definition.
	case 1:
		q.location = inlineArgs[0]
	default:
		return nil, errors.New("queue: wrong amount of inline arguments")
	}
	return q, nil
}

func (q *Queue) Init(cfg *config.Map) error {
	var maxParallelism int
	cfg.Bool("debug", true, false, &q.Log.Debug)
	cfg.Int("max_tries", false, false, 8, &q.maxTries)
	cfg.Int("max_parallelism", false, false, 16, &maxParallelism)
	cfg.String("location", false, false, q.location, &q.location)
	cfg.Custom("target", false, true, nil, modconfig.DeliveryDirective, &q.Target)
	cfg.String("hostname", true, true, "", &q.hostname)
	cfg.String("autogenerated_msg_domain", true, false, "", &q.autogenMsgDomain)
	cfg.Custom("bounce", false, false, nil, func(m *config.Map, node *config.Node) (interface{}, error) {
		return dispatcher.NewDispatcher(m.Globals, node.Children)
	}, &q.dsnDispatcher)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if q.dsnDispatcher != nil {
		if q.autogenMsgDomain == "" {
			return errors.New("queue: autogenerated_msg_domain is required if bounce {} is specified")
		}

		q.dsnDispatcher.Hostname = q.hostname
		q.dsnDispatcher.Log = log.Logger{Name: "queue/dispatcher", Debug: q.Log.Debug}
	}
	if q.location == "" && q.name == "" {
		return errors.New("queue: need explicit location directive or inline argument if defined inline")
	}
	if q.location == "" {
		q.location = filepath.Join(config.StateDirectory, q.name)
	}

	// TODO: Check location write permissions.
	if err := os.MkdirAll(q.location, os.ModePerm); err != nil {
		return err
	}

	return q.start(maxParallelism)
}

func (q *Queue) start(maxParallelism int) error {
	q.wheel = NewTimeWheel()
	q.deliverySemaphore = make(chan struct{}, maxParallelism)

	if err := q.readDiskQueue(); err != nil {
		return err
	}

	q.Log.Debugf("delivery target: %T", q.Target)

	go q.dispatch()
	return nil
}

func (q *Queue) Close() error {
	q.deliveryWg.Wait()
	q.wheel.Close()
	return nil
}

func (q *Queue) dispatch() {
	for slot := range q.wheel.Dispatch() {
		q.Log.Debugln("starting delivery for", slot.Value)
		id := slot.Value.(string)

		q.deliveryWg.Add(1)
		go func() {
			q.Log.Debugln("waiting on delivery semaphore for", slot.Value)
			q.deliverySemaphore <- struct{}{}
			defer func() {
				<-q.deliverySemaphore
				q.deliveryWg.Done()
			}()

			q.Log.Debugln("delivery semaphore acquired for", slot.Value)
			meta, header, body, err := q.openMessage(id)
			if err != nil {
				q.Log.Printf("failed to read message: %v", err)
				return
			}

			q.tryDelivery(meta, header, body)
		}()
	}
}

func (q *Queue) tryDelivery(meta *QueueMetadata, header textproto.Header, body buffer.Buffer) {
	dl := target.DeliveryLogger(q.Log, meta.MsgMeta)
	dl.Debugf("delivery attempt #%d", meta.TriesCount+1)

	partialErr := q.deliver(meta, header, body)
	dl.Debugf("failures: permanently: %v, temporary: %v, errors: %v",
		partialErr.Failed, partialErr.TemporaryFailed, partialErr.Errs)

	for _, rcpt := range meta.To {
		if _, ok := partialErr.Errs[rcpt]; ok {
			continue
		}

		dl.Printf("delivered to %s after %d attempt(s)", rcpt, meta.TriesCount+1)
	}

	// Save errors information for reporting in bounce message.
	meta.FailedRcpts = append(meta.FailedRcpts, partialErr.Failed...)
	for rcpt, rcptErr := range partialErr.Errs {
		var smtpErr *smtp.SMTPError
		var ok bool
		if smtpErr, ok = rcptErr.(*smtp.SMTPError); !ok {
			smtpErr = &smtp.SMTPError{
				Code:         554,
				EnhancedCode: smtp.EnhancedCode{5, 0, 0},
				Message:      rcptErr.Error(),
			}
			if exterrors.IsTemporaryOrUnspec(rcptErr) {
				smtpErr.Code = 451
				smtpErr.EnhancedCode = smtp.EnhancedCode{4, 0, 0}
			}
		}

		meta.RcptErrs[rcpt] = smtpErr
	}
	meta.To = partialErr.TemporaryFailed

	meta.LastAttempt = time.Now()
	if meta.TriesCount == q.maxTries || len(partialErr.TemporaryFailed) == 0 {
		// Attempt either fully succeeded or completely failed.
		if meta.TriesCount == q.maxTries {
			for _, rcpt := range meta.TemporaryFailedRcpts {
				dl.Printf("gave up trying to deliver to %s", rcpt)
			}
		}
		for _, rcpt := range meta.FailedRcpts {
			dl.Printf("failed to deliver to %s", rcpt)
		}
		if (len(meta.FailedRcpts) != 0 || meta.TriesCount == q.maxTries) && !meta.DSN {
			q.emitDSN(meta, header)
		}
		q.removeFromDisk(meta.MsgMeta)
		return
	}

	meta.TriesCount++

	if err := q.updateMetadataOnDisk(meta); err != nil {
		dl.Printf("failed to update meta-data: %v", err)
	}

	nextTryTime := time.Now()
	nextTryTime = nextTryTime.Add(q.initialRetryTime * time.Duration(math.Pow(q.retryTimeScale, float64(meta.TriesCount-1))))
	dl.Printf("%d attempt failed, will retry in %v (at %v) for %v",
		meta.TriesCount, time.Until(nextTryTime), nextTryTime.Truncate(time.Second), meta.To)

	q.wheel.Add(nextTryTime, meta.MsgMeta.ID)
}

func (q *Queue) deliver(meta *QueueMetadata, header textproto.Header, body buffer.Buffer) partialError {
	dl := target.DeliveryLogger(q.Log, meta.MsgMeta)
	perr := partialError{
		Errs: map[string]error{},
	}

	deliveryTarget := q.Target
	if meta.DSN {
		deliveryTarget = q.dsnDispatcher
	}

	msgMeta := meta.MsgMeta.DeepCopy()
	msgMeta.ID = msgMeta.ID + "-" + strconv.Itoa(meta.TriesCount+1)
	dl.Debugf("using message ID = %s", msgMeta.ID)

	delivery, err := deliveryTarget.Start(meta.MsgMeta, meta.From)
	if err != nil {
		dl.Debugf("target.Start failed: %v", err)
		perr.Failed = append(perr.Failed, meta.To...)
		for _, rcpt := range meta.To {
			perr.Errs[rcpt] = err
		}
		return perr
	}
	dl.Debugf("target.Start OK")

	var acceptedRcpts []string
	for _, rcpt := range meta.To {
		if err := delivery.AddRcpt(rcpt); err != nil {
			if exterrors.IsTemporaryOrUnspec(err) {
				perr.TemporaryFailed = append(perr.TemporaryFailed, rcpt)
			} else {
				perr.Failed = append(perr.Failed, rcpt)
			}
			dl.Debugf("delivery.AddRcpt %s failed: %v", rcpt, err)
			perr.Errs[rcpt] = err
		} else {
			dl.Debugf("delivery.AddRcpt %s OK", rcpt)
			acceptedRcpts = append(acceptedRcpts, rcpt)
		}
	}

	if len(acceptedRcpts) == 0 {
		dl.Debugf("delivery.Abort (no accepted receipients)")
		if err := delivery.Abort(); err != nil {
			dl.Printf("delivery.Abort failed: %v", err)
		}
		return perr
	}

	expandToPartialErr := func(err error) {
		if exterrors.IsTemporaryOrUnspec(err) {
			perr.TemporaryFailed = append(perr.TemporaryFailed, acceptedRcpts...)
		} else {
			perr.Failed = append(perr.Failed, acceptedRcpts...)
		}
		for _, rcpt := range acceptedRcpts {
			perr.Errs[rcpt] = err
		}
	}

	partDelivery, ok := delivery.(module.PartialDelivery)
	if ok {
		dl.Debugf("using delivery.BodyNonAtomic")
		partDelivery.BodyNonAtomic(&perr, header, body)
	} else {
		if err := delivery.Body(header, body); err != nil {
			dl.Debugf("delivery.Body failed: %v", err)
			expandToPartialErr(err)
		}
		dl.Debugf("delivery.Body OK")
	}

	allFailed := true
	for _, rcpt := range acceptedRcpts {
		if perr.Errs[rcpt] == nil {
			allFailed = false
		}
	}
	if allFailed {
		// No recipients succeeded.
		dl.Debugf("delivery.Abort (all recipients failed)")
		if err := delivery.Abort(); err != nil {
			dl.Printf("delivery.Abort failed: %v", err)
		}
		return perr
	}

	if err := delivery.Commit(); err != nil {
		dl.Debugf("delivery.Commit failed: %v", err)
		expandToPartialErr(err)
	}
	dl.Debugf("delivery.Commit OK")

	return perr
}

type queueDelivery struct {
	q    *Queue
	meta *QueueMetadata

	header textproto.Header
	body   buffer.Buffer
}

func (qd *queueDelivery) AddRcpt(rcptTo string) error {
	qd.meta.To = append(qd.meta.To, rcptTo)
	return nil
}

func (qd *queueDelivery) Body(header textproto.Header, body buffer.Buffer) error {
	// Body buffer initially passed to us may not be valid after "delivery" to queue completes.
	// storeNewMessage returns a new buffer object created from message blob stored on disk.
	storedBody, err := qd.q.storeNewMessage(qd.meta, header, body)
	if err != nil {
		return err
	}

	qd.body = storedBody
	qd.header = header
	return nil
}

func (qd *queueDelivery) Abort() error {
	if qd.body != nil {
		qd.q.removeFromDisk(qd.meta.MsgMeta)
	}
	return nil
}

func (qd *queueDelivery) Commit() error {
	qd.q.deliveryWg.Add(1)
	go func() {
		qd.q.deliverySemaphore <- struct{}{}
		defer func() {
			<-qd.q.deliverySemaphore
			qd.q.deliveryWg.Done()
		}()

		qd.q.tryDelivery(qd.meta, qd.header, qd.body)
	}()
	return nil
}

func (q *Queue) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	meta := &QueueMetadata{
		MsgMeta:      msgMeta,
		From:         mailFrom,
		RcptErrs:     map[string]*smtp.SMTPError{},
		FirstAttempt: time.Now(),
		LastAttempt:  time.Now(),
	}
	return &queueDelivery{q: q, meta: meta}, nil
}

func (q *Queue) StartDSN(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	meta := &QueueMetadata{
		DSN:          true,
		MsgMeta:      msgMeta,
		From:         mailFrom,
		RcptErrs:     map[string]*smtp.SMTPError{},
		FirstAttempt: time.Now(),
		LastAttempt:  time.Now(),
	}
	return &queueDelivery{q: q, meta: meta}, nil
}

func (q *Queue) removeFromDisk(msgMeta *module.MsgMetadata) {
	id := msgMeta.ID
	dl := target.DeliveryLogger(q.Log, msgMeta)

	// Order is important.
	// If we remove header and body but can't remove meta now - readDiskQueue
	// will detect and report it.
	headerPath := filepath.Join(q.location, id+".header")
	if err := os.Remove(headerPath); err != nil {
		dl.Printf("failed to remove header from disk: %v", err)
	}
	bodyPath := filepath.Join(q.location, id+".body")
	if err := os.Remove(bodyPath); err != nil {
		dl.Printf("failed to remove body from disk: %v", err)
	}
	metaPath := filepath.Join(q.location, id+".meta")
	if err := os.Remove(metaPath); err != nil {
		dl.Printf("failed to remove meta-data from disk: %v", err)
	}
	dl.Debugf("removed message from disk")
}

func (q *Queue) readDiskQueue() error {
	dirInfo, err := ioutil.ReadDir(q.location)
	if err != nil {
		return err
	}

	// TODO: Rewrite this function to pass all sub-tests in TestQueueDelivery_DeserializationCleanUp/NoMeta.

	loadedCount := 0
	for _, entry := range dirInfo {
		// We start loading from meta-data files and then check whether ID.header and ID.body exist.
		// This allows us to properly detect dangling body files.
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta") {
			continue
		}
		id := entry.Name()[:len(entry.Name())-5]

		meta, err := q.readMessageMeta(id)
		if err != nil {
			q.Log.Printf("failed to read meta-data, skipping: %v (msg ID = %s)", err, id)
			continue
		}

		// Check header file existence.
		if _, err := os.Stat(filepath.Join(q.location, id+".header")); err != nil {
			if os.IsNotExist(err) {
				q.Log.Printf("header file doesn't exist for msg ID = %s", id)
				q.tryRemoveDanglingFile(id + ".meta")
				q.tryRemoveDanglingFile(id + ".body")
			} else {
				q.Log.Printf("skipping nonstat'able header file: %v (msg ID = %s)", err, id)
			}
			continue
		}

		// Check body file existence.
		if _, err := os.Stat(filepath.Join(q.location, id+".body")); err != nil {
			if os.IsNotExist(err) {
				q.Log.Printf("body file doesn't exist for msg ID = %s", id)
				q.tryRemoveDanglingFile(id + ".meta")
				q.tryRemoveDanglingFile(id + ".header")
			} else {
				q.Log.Printf("skipping nonstat'able body file: %v (msg ID = %s)", err, id)
			}
			continue
		}

		nextTryTime := meta.LastAttempt
		nextTryTime = nextTryTime.Add(q.initialRetryTime * time.Duration(math.Pow(q.retryTimeScale, float64(meta.TriesCount-1))))

		if time.Until(nextTryTime) < q.postInitDelay {
			nextTryTime = time.Now().Add(q.postInitDelay)
		}

		q.Log.Debugf("will try to deliver (msg ID = %s) in %v (%v)", id, time.Until(nextTryTime), nextTryTime)
		q.wheel.Add(nextTryTime, id)
		loadedCount++
	}

	if loadedCount != 0 {
		q.Log.Printf("loaded %d saved queue entries", loadedCount)
	}

	return nil
}

func (q *Queue) storeNewMessage(meta *QueueMetadata, header textproto.Header, body buffer.Buffer) (buffer.Buffer, error) {
	id := meta.MsgMeta.ID

	headerPath := filepath.Join(q.location, id+".header")
	headerFile, err := os.Create(headerPath)
	if err != nil {
		return nil, err
	}
	defer headerFile.Close()

	if err := textproto.WriteHeader(headerFile, header); err != nil {
		q.tryRemoveDanglingFile(id + ".header")
		return nil, err
	}

	bodyReader, err := body.Open()
	if err != nil {
		q.tryRemoveDanglingFile(id + ".header")
		return nil, err
	}
	defer bodyReader.Close()

	bodyPath := filepath.Join(q.location, id+".body")
	bodyFile, err := os.Create(bodyPath)
	if err != nil {
		return nil, err
	}
	defer bodyFile.Close()

	if _, err := io.Copy(bodyFile, bodyReader); err != nil {
		q.tryRemoveDanglingFile(id + ".body")
		q.tryRemoveDanglingFile(id + ".header")
		return nil, err
	}

	if err := q.updateMetadataOnDisk(meta); err != nil {
		q.tryRemoveDanglingFile(id + ".body")
		q.tryRemoveDanglingFile(id + ".header")
		return nil, err
	}

	return buffer.FileBuffer{Path: bodyPath}, nil
}

func (q *Queue) updateMetadataOnDisk(meta *QueueMetadata) error {
	metaPath := filepath.Join(q.location, meta.MsgMeta.ID+".meta")
	file, err := os.Create(metaPath)
	if err != nil {
		return err
	}
	defer file.Close()

	metaCopy := *meta
	metaCopy.MsgMeta = meta.MsgMeta.DeepCopy()

	if _, ok := metaCopy.MsgMeta.SrcAddr.(*net.TCPAddr); !ok {
		meta.MsgMeta.SrcAddr = nil
	}

	if err := json.NewEncoder(file).Encode(metaCopy); err != nil {
		return err
	}
	return nil
}

func (q *Queue) readMessageMeta(id string) (*QueueMetadata, error) {
	metaPath := filepath.Join(q.location, id+".meta")
	file, err := os.Open(metaPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	meta := &QueueMetadata{}

	// net.Addr can't be deserialized because we don't know concrete type. For
	// this reason we assume that SrcAddr is TCPAddr, if it is not - we drop it
	// during serialization (see updateMetadataOnDisk).
	meta.MsgMeta = &module.MsgMetadata{}
	meta.MsgMeta.SrcAddr = &net.TCPAddr{}

	if err := json.NewDecoder(file).Decode(meta); err != nil {
		return nil, err
	}

	return meta, nil
}

type BufferedReadCloser struct {
	*bufio.Reader
	io.Closer
}

func (q *Queue) tryRemoveDanglingFile(name string) {
	if err := os.Remove(filepath.Join(q.location, name)); err != nil {
		q.Log.Println(err)
		return
	}
	q.Log.Printf("removed dangling file %s", name)
}

func (q *Queue) openMessage(id string) (*QueueMetadata, textproto.Header, buffer.Buffer, error) {
	meta, err := q.readMessageMeta(id)
	if err != nil {
		return nil, textproto.Header{}, nil, err
	}

	bodyPath := filepath.Join(q.location, id+".body")
	_, err = os.Stat(bodyPath)
	if err != nil {
		if os.IsNotExist(err) {
			q.tryRemoveDanglingFile(id + ".meta")
		}
		return nil, textproto.Header{}, nil, nil
	}
	body := buffer.FileBuffer{Path: bodyPath}

	headerPath := filepath.Join(q.location, id+".header")
	headerFile, err := os.Open(headerPath)
	if err != nil {
		if os.IsNotExist(err) {
			q.tryRemoveDanglingFile(id + ".meta")
			q.tryRemoveDanglingFile(id + ".body")
		}
		return nil, textproto.Header{}, nil, nil
	}

	bufferedHeader := bufio.NewReader(headerFile)
	header, err := textproto.ReadHeader(bufferedHeader)
	if err != nil {
		return nil, textproto.Header{}, nil, nil
	}

	return meta, header, body, nil
}

func (q *Queue) InstanceName() string {
	return q.name
}

func (q *Queue) Name() string {
	return "queue"
}

func (q *Queue) emitDSN(meta *QueueMetadata, header textproto.Header) {
	// If, apparently, we have no DSN dispatcher configured - do nothing.
	if q.dsnDispatcher == nil {
		return
	}

	dsnID, err := dispatcher.GenerateMsgID()
	if err != nil {
		q.Log.Printf("rand.Rand error: %v", err)
		return
	}

	dsnEnvelope := dsn.Envelope{
		MsgID: "<" + dsnID + "@" + q.autogenMsgDomain + ">",
		From:  "MAILER-DAEMON@" + q.autogenMsgDomain,
		To:    meta.From,
	}
	mtaInfo := dsn.ReportingMTAInfo{
		ReportingMTA:    q.hostname,
		XSender:         meta.From,
		XMessageID:      meta.MsgMeta.ID,
		ArrivalDate:     meta.FirstAttempt,
		LastAttemptDate: meta.LastAttempt,
	}
	if !meta.MsgMeta.DontTraceSender {
		mtaInfo.ReceivedFromMTA = meta.MsgMeta.SrcHostname
	}

	rcptInfo := make([]dsn.RecipientInfo, 0, len(meta.RcptErrs))
	for rcpt, err := range meta.RcptErrs {
		if meta.MsgMeta.OriginalRcpts != nil {
			originalRcpt := meta.MsgMeta.OriginalRcpts[rcpt]
			if originalRcpt != "" {
				rcpt = originalRcpt
			}
		}

		rcptInfo = append(rcptInfo, dsn.RecipientInfo{
			FinalRecipient: rcpt,
			Action:         dsn.ActionFailed,
			Status:         err.EnhancedCode,
			DiagnosticCode: err,
		})
	}

	var dsnBodyBlob bytes.Buffer
	dl := target.DeliveryLogger(q.Log, meta.MsgMeta)
	dsnHeader, err := dsn.GenerateDSN(dsnEnvelope, mtaInfo, rcptInfo, header, &dsnBodyBlob)
	if err != nil {
		dl.Printf("failed to generate fail DSN: %v", err)
		return
	}
	dsnBody := buffer.MemoryBuffer{Slice: dsnBodyBlob.Bytes()}

	dsnMeta := &module.MsgMetadata{
		ID:          dsnID,
		SrcProto:    "",
		SrcHostname: q.hostname,
	}
	dl.Printf("generated failed DSN, DSN ID = %s", dsnID)

	dsnDelivery, err := q.StartDSN(dsnMeta, "MAILER-DAEMON@"+q.autogenMsgDomain)
	if err != nil {
		dl.Printf("failed to enqueue DSN: %v", err)
		return
	}
	defer func() {
		if err != nil {
			dl.Printf("failed to enqueue DSN: %v", err)
			dsnDelivery.Abort()
		}
	}()

	if err = dsnDelivery.AddRcpt(meta.From); err != nil {
		return
	}
	if err = dsnDelivery.Body(dsnHeader, dsnBody); err != nil {
		return
	}
	if err = dsnDelivery.Commit(); err != nil {
		return
	}
}

func init() {
	module.Register("queue", NewQueue)
}
