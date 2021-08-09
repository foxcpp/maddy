/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

/*
Package queue implements module which keeps messages on disk and tries delivery
to the configured target (usually remote) multiple times until all recipients
are succeeded.

Interfaces implemented:
- module.DeliveryTarget

Implementation summary follows.

All scheduled deliveries are attempted to the configured DeliveryTarget.
All metadata is preserved on disk.

Failure status is determined on per-recipient basis:
- Delivery.Start fail handled as a failure for all recipients.
- Delivery.AddRcpt fail handled as a failure for the corresponding recipient.
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/trace"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/dsn"
	"github.com/foxcpp/maddy/internal/msgpipeline"
	"github.com/foxcpp/maddy/internal/target"
)

// partialError describes state of partially successful message delivery.
type partialError struct {

	// Underlying error objects for each recipient.
	Errs map[string]error

	// Fields can be accessed without holding this lock, but only after
	// target.BodyNonAtomic/Body returns.
	statusLock *sync.Mutex
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
	pe.Errs[rcptTo] = err
}

func (pe partialError) Error() string {
	return fmt.Sprintf("delivery failed for some recipients: %v", pe.Errs)
}

// dontRecover controls the behavior of panic handlers, if it is set to true -
// they are disabled and so tests will panic to avoid masking bugs.
var dontRecover = false

type Queue struct {
	name             string
	location         string
	hostname         string
	autogenMsgDomain string
	wheel            *TimeWheel

	dsnPipeline module.DeliveryTarget

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
	TriesCount map[string]int

	FirstAttempt time.Time
	LastAttempt  time.Time
}

type queueSlot struct {
	ID string

	// If nil - Hdr and Body are invalid, all values should be read from
	// disk.
	Meta *QueueMetadata
	Hdr  *textproto.Header
	Body buffer.Buffer
}

func NewQueue(_, instName string, _, inlineArgs []string) (module.Module, error) {
	q := &Queue{
		name:             instName,
		initialRetryTime: 15 * time.Minute,
		retryTimeScale:   1.25,
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
	cfg.Int("max_tries", false, false, 20, &q.maxTries)
	cfg.Int("max_parallelism", false, false, 16, &maxParallelism)
	cfg.String("location", false, false, q.location, &q.location)
	cfg.Custom("target", false, true, nil, modconfig.DeliveryDirective, &q.Target)
	cfg.String("hostname", true, true, "", &q.hostname)
	cfg.String("autogenerated_msg_domain", true, false, "", &q.autogenMsgDomain)
	cfg.Custom("bounce", false, false, nil, func(m *config.Map, node config.Node) (interface{}, error) {
		return msgpipeline.New(m.Globals, node.Children)
	}, &q.dsnPipeline)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if q.dsnPipeline != nil {
		if q.autogenMsgDomain == "" {
			return errors.New("queue: autogenerated_msg_domain is required if bounce {} is specified")
		}

		q.dsnPipeline.(*msgpipeline.MsgPipeline).Hostname = q.hostname
		q.dsnPipeline.(*msgpipeline.MsgPipeline).Log = log.Logger{Name: "queue/pipeline", Debug: q.Log.Debug}
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
	q.wheel = NewTimeWheel(q.dispatch)
	q.deliverySemaphore = make(chan struct{}, maxParallelism)

	if err := q.readDiskQueue(); err != nil {
		return err
	}

	q.Log.Debugf("delivery target: %T", q.Target)

	return nil
}

func (q *Queue) Close() error {
	q.wheel.Close()
	q.deliveryWg.Wait()

	return nil
}

// discardBroken changes the name of metadata file to have .meta_broken
// extension.
//
// Further attempts to deliver (due to a timewheel) it will fail due to
// non-existent meta-data file.
//
// No error handling is done since this function is called from panic handler.
func (q *Queue) discardBroken(id string) {
	err := os.Rename(filepath.Join(q.location, id+".meta"), filepath.Join(q.location, id+".meta_broken"))
	if err != nil {
		// Note: Global logger is used in case there is something wrong with Queue.Log.
		log.Printf("can't mark the queue message as broken: %v", err)
	}
}

func (q *Queue) dispatch(value TimeSlot) {
	slot := value.Value.(queueSlot)

	q.Log.Debugln("starting delivery for", slot.ID)

	q.deliveryWg.Add(1)
	go func() {
		q.Log.Debugln("waiting on delivery semaphore for", slot.ID)
		q.deliverySemaphore <- struct{}{}
		defer func() {
			<-q.deliverySemaphore
			q.deliveryWg.Done()

			if dontRecover {
				return
			}

			if err := recover(); err != nil {
				stack := debug.Stack()
				log.Printf("panic during queue dispatch %s: %v\n%s", slot.ID, err, stack)
				q.discardBroken(slot.ID)
			}
		}()

		q.Log.Debugln("delivery semaphore acquired for", slot.ID)
		var (
			meta *QueueMetadata
			hdr  textproto.Header
			body buffer.Buffer
		)
		if slot.Meta == nil {
			var err error
			meta, hdr, body, err = q.openMessage(slot.ID)
			if err != nil {
				q.Log.Error("read message", err, slot.ID)
				return
			}
			if meta == nil {
				panic("wtf")
			}
		} else {
			meta = slot.Meta
			hdr = *slot.Hdr
			body = slot.Body
		}

		q.tryDelivery(meta, hdr, body)
	}()
}

func toSMTPErr(err error) *smtp.SMTPError {
	if err == nil {
		return nil
	}

	res := &smtp.SMTPError{
		Code:         554,
		EnhancedCode: smtp.EnhancedCode{5, 0, 0},
		Message:      "Internal server error",
	}

	if exterrors.IsTemporaryOrUnspec(err) {
		res.Code = 451
		res.EnhancedCode = smtp.EnhancedCode{4, 0, 0}
	}

	ctxInfo := exterrors.Fields(err)
	ctxCode, ok := ctxInfo["smtp_code"].(int)
	if ok {
		res.Code = ctxCode
	}
	ctxEnchCode, ok := ctxInfo["smtp_enchcode"].(smtp.EnhancedCode)
	if ok {
		res.EnhancedCode = ctxEnchCode
	}
	ctxMsg, ok := ctxInfo["smtp_msg"].(string)
	if ok {
		res.Message = ctxMsg
	}

	if smtpErr, ok := err.(*smtp.SMTPError); ok {
		log.Printf("plain SMTP error returned, this is deprecated")
		res.Code = smtpErr.Code
		res.EnhancedCode = smtpErr.EnhancedCode
		res.Message = smtpErr.Message
	}

	return res
}

func (q *Queue) tryDelivery(meta *QueueMetadata, header textproto.Header, body buffer.Buffer) {
	dl := target.DeliveryLogger(q.Log, meta.MsgMeta)

	partialErr := q.deliver(meta, header, body)
	dl.Debugf("errors: %v", partialErr.Errs)

	// While iterating the list of recipients we also pick the smallest tries count
	// and use it to calculate the delay for the next attempt.
	smallestTriesCount := 999999

	if meta.TriesCount == nil {
		meta.TriesCount = make(map[string]int)
	}

	// Check attempted recipients and corresponding errors.
	// Split list into two parts: recipients that should be retried (newRcpts)
	// and recipients DSN will be generated for.
	newRcpts := make([]string, 0, len(partialErr.Errs))
	failedRcpts := make([]string, 0, len(partialErr.Errs))
	for _, rcpt := range meta.To {
		rcptErr, ok := partialErr.Errs[rcpt]
		if !ok {
			dl.Msg("delivered", "rcpt", rcpt, "attempt", meta.TriesCount[rcpt]+1)
			continue
		}

		// Save last error (either temporary or permanent) for reporting in the DSN.
		dl.Error("delivery attempt failed", rcptErr, "rcpt", rcpt)
		meta.RcptErrs[rcpt] = toSMTPErr(rcptErr)

		temporary := exterrors.IsTemporaryOrUnspec(rcptErr)
		if !temporary || meta.TriesCount[rcpt]+1 == q.maxTries {
			delete(meta.TriesCount, rcpt)
			dl.Msg("not delivered, permanent error", "rcpt", rcpt)
			failedRcpts = append(failedRcpts, rcpt)
			continue
		}

		// Temporary error, increase tries counter and requeue.
		meta.TriesCount[rcpt]++
		newRcpts = append(newRcpts, rcpt)

		// See smallestTriesCount comment.
		if count := meta.TriesCount[rcpt]; count < smallestTriesCount {
			smallestTriesCount = count
		}
	}

	// Generate DSN for recipients that failed permanently this time.
	if len(failedRcpts) != 0 {
		q.emitDSN(meta, header, failedRcpts)
	}
	// No recipients to try, either all failed or all succeeded.
	if len(newRcpts) == 0 {
		q.removeFromDisk(meta.MsgMeta)
		return
	}

	meta.To = newRcpts
	meta.LastAttempt = time.Now()

	if err := q.updateMetadataOnDisk(meta); err != nil {
		dl.Error("meta-data update", err)
	}

	nextTryTime := time.Now()
	// Delay between retries grows exponentally, the formula is:
	// initialRetryTime * retryTimeScale ^ (smallestTriesCount - 1)
	dl.Debugf("delay: %v * %v ^ (%v - 1)", q.initialRetryTime, q.retryTimeScale, smallestTriesCount)
	scaleFactor := time.Duration(math.Pow(q.retryTimeScale, float64(smallestTriesCount-1)))
	nextTryTime = nextTryTime.Add(q.initialRetryTime * scaleFactor)
	dl.Msg("will retry",
		"attempts_count", meta.TriesCount,
		"next_try_delay", time.Until(nextTryTime),
		"rcpts", meta.To)

	q.wheel.Add(nextTryTime, queueSlot{
		ID: meta.MsgMeta.ID,

		// Do not keep (meta-)data in memory to reduce usage.  At this point,
		// it is safe on disk and next try will reread it.
		Meta: nil,
		Hdr:  nil,
		Body: nil,
	})
}

func (q *Queue) deliver(meta *QueueMetadata, header textproto.Header, body buffer.Buffer) partialError {
	dl := target.DeliveryLogger(q.Log, meta.MsgMeta)
	perr := partialError{
		Errs:       map[string]error{},
		statusLock: new(sync.Mutex),
	}

	msgMeta := meta.MsgMeta.DeepCopy()
	msgMeta.ID = msgMeta.ID + "-" + strconv.FormatInt(time.Now().Unix(), 16)
	dl.Debugf("using message ID = %s", msgMeta.ID)

	msgCtx, msgTask := trace.NewTask(context.Background(), "Queue delivery")
	defer msgTask.End()

	mailCtx, mailTask := trace.NewTask(msgCtx, "MAIL FROM")
	delivery, err := q.Target.Start(mailCtx, msgMeta, meta.From)
	mailTask.End()
	if err != nil {
		dl.Debugf("target.Start failed: %v", err)
		for _, rcpt := range meta.To {
			perr.Errs[rcpt] = err
		}
		return perr
	}
	dl.Debugf("target.Start OK")

	var acceptedRcpts []string
	for _, rcpt := range meta.To {
		rcptCtx, rcptTask := trace.NewTask(msgCtx, "RCPT TO")
		if err := delivery.AddRcpt(rcptCtx, rcpt); err != nil {
			dl.Debugf("delivery.AddRcpt %s failed: %v", rcpt, err)
			perr.Errs[rcpt] = err
		} else {
			dl.Debugf("delivery.AddRcpt %s OK", rcpt)
			acceptedRcpts = append(acceptedRcpts, rcpt)
		}
		rcptTask.End()
	}

	if len(acceptedRcpts) == 0 {
		dl.Debugf("delivery.Abort (no accepted receipients)")
		if err := delivery.Abort(msgCtx); err != nil {
			dl.Error("delivery.Abort failed", err)
		}
		return perr
	}

	expandToPartialErr := func(err error) {
		for _, rcpt := range acceptedRcpts {
			perr.Errs[rcpt] = err
		}
	}

	bodyCtx, bodyTask := trace.NewTask(msgCtx, "DATA")
	defer bodyTask.End()

	partDelivery, ok := delivery.(module.PartialDelivery)
	if ok {
		dl.Debugf("using delivery.BodyNonAtomic")
		partDelivery.BodyNonAtomic(bodyCtx, &perr, header, body)
	} else {
		if err := delivery.Body(bodyCtx, header, body); err != nil {
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
		if err := delivery.Abort(bodyCtx); err != nil {
			dl.Msg("delivery.Abort failed", err)
		}
		return perr
	}

	if err := delivery.Commit(bodyCtx); err != nil {
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

func (qd *queueDelivery) AddRcpt(ctx context.Context, rcptTo string) error {
	qd.meta.To = append(qd.meta.To, rcptTo)
	return nil
}

func (qd *queueDelivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	defer trace.StartRegion(ctx, "queue/Body").End()

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

func (qd *queueDelivery) Abort(ctx context.Context) error {
	defer trace.StartRegion(ctx, "queue/Abort").End()

	if qd.body != nil {
		qd.q.removeFromDisk(qd.meta.MsgMeta)
	}
	return nil
}

func (qd *queueDelivery) Commit(ctx context.Context) error {
	defer trace.StartRegion(ctx, "queue/Commit").End()

	if qd.meta == nil {
		panic("queue: double Commit")
	}

	qd.q.wheel.Add(time.Time{}, queueSlot{
		ID:   qd.meta.MsgMeta.ID,
		Meta: qd.meta,
		Hdr:  &qd.header,
		Body: qd.body,
	})
	qd.meta = nil
	qd.body = nil
	return nil
}

func (q *Queue) Start(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	meta := &QueueMetadata{
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
		dl.Error("failed to remove header from disk", err)
	}
	bodyPath := filepath.Join(q.location, id+".body")
	if err := os.Remove(bodyPath); err != nil {
		dl.Error("failed to remove body from disk", err)
	}
	metaPath := filepath.Join(q.location, id+".meta")
	if err := os.Remove(metaPath); err != nil {
		dl.Error("failed to remove meta-data from disk", err)
	}
	dl.Debugf("removed message from disk")
}

func (q *Queue) readDiskQueue() error {
	dirInfo, err := ioutil.ReadDir(q.location)
	if err != nil {
		return err
	}

	// TODO(GH #209): Rewrite this function to pass all sub-tests in TestQueueDelivery_DeserializationCleanUp/NoMeta.

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

		smallestTriesCount := 999999
		for _, count := range meta.TriesCount {
			if smallestTriesCount > count {
				smallestTriesCount = count
			}
		}
		nextTryTime := meta.LastAttempt
		scaleFactor := time.Duration(math.Pow(q.retryTimeScale, float64(smallestTriesCount-1)))
		nextTryTime = nextTryTime.Add(q.initialRetryTime * scaleFactor)

		if time.Until(nextTryTime) < q.postInitDelay {
			nextTryTime = time.Now().Add(q.postInitDelay)
		}

		q.Log.Debugf("will try to deliver (msg ID = %s) in %v (%v)", id, time.Until(nextTryTime), nextTryTime)
		q.wheel.Add(nextTryTime, queueSlot{
			ID: id,
		})
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

	if err := headerFile.Sync(); err != nil {
		return nil, err
	}

	if err := bodyFile.Sync(); err != nil {
		return nil, err
	}

	return buffer.FileBuffer{Path: bodyPath, LenHint: body.Len()}, nil
}

func (q *Queue) updateMetadataOnDisk(meta *QueueMetadata) error {
	metaPath := filepath.Join(q.location, meta.MsgMeta.ID+".meta")

	var file *os.File
	var err error
	if runtime.GOOS == "windows" {
		file, err = os.Create(metaPath)
		if err != nil {
			return err
		}
	} else {
		file, err = os.Create(metaPath + ".new")
		if err != nil {
			return err
		}
	}
	defer file.Close()

	metaCopy := *meta
	metaCopy.MsgMeta = meta.MsgMeta.DeepCopy()
	metaCopy.MsgMeta.Conn = nil

	if err := json.NewEncoder(file).Encode(metaCopy); err != nil {
		return err
	}

	if err := file.Sync(); err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		if err := os.Rename(metaPath+".new", metaPath); err != nil {
			return err
		}
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

	meta.MsgMeta = &module.MsgMetadata{}

	// There is a couple of problems we have to solve before we would be able to
	// serialize ConnState.
	// 1. future.Future can't be serialized.
	// 2. net.Addr can't be deserialized because we don't know the concrete type.

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
		q.Log.Error("dangling file remove failed", err)
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
		return nil, textproto.Header{}, nil, err
	}
	body := buffer.FileBuffer{Path: bodyPath}

	headerPath := filepath.Join(q.location, id+".header")
	headerFile, err := os.Open(headerPath)
	if err != nil {
		if os.IsNotExist(err) {
			q.tryRemoveDanglingFile(id + ".meta")
			q.tryRemoveDanglingFile(id + ".body")
		}
		return nil, textproto.Header{}, nil, err
	}

	bufferedHeader := bufio.NewReader(headerFile)
	header, err := textproto.ReadHeader(bufferedHeader)
	if err != nil {
		return nil, textproto.Header{}, nil, err
	}

	return meta, header, body, nil
}

func (q *Queue) InstanceName() string {
	return q.name
}

func (q *Queue) Name() string {
	return "queue"
}

func (q *Queue) emitDSN(meta *QueueMetadata, header textproto.Header, failedRcpts []string) {
	// If, apparently, we have no DSN msgpipeline configured - do nothing.
	if q.dsnPipeline == nil {
		return
	}

	// Null return-path, used in DSNs.
	if meta.MsgMeta.OriginalFrom == "" {
		return
	}

	dsnID, err := module.GenerateMsgID()
	if err != nil {
		q.Log.Error("rand.Rand error", err)
		return
	}

	dsnEnvelope := dsn.Envelope{
		MsgID: "<" + dsnID + "@" + q.autogenMsgDomain + ">",
		From:  "MAILER-DAEMON@" + q.autogenMsgDomain,
		To:    meta.MsgMeta.OriginalFrom,
	}
	mtaInfo := dsn.ReportingMTAInfo{
		ReportingMTA:    q.hostname,
		XSender:         meta.From,
		XMessageID:      meta.MsgMeta.ID,
		ArrivalDate:     meta.FirstAttempt,
		LastAttemptDate: meta.LastAttempt,
	}
	if !meta.MsgMeta.DontTraceSender && meta.MsgMeta.Conn != nil {
		mtaInfo.ReceivedFromMTA = meta.MsgMeta.Conn.Hostname
	}

	rcptInfo := make([]dsn.RecipientInfo, 0, len(meta.RcptErrs))
	for _, rcpt := range failedRcpts {
		rcptErr := meta.RcptErrs[rcpt]
		// rcptErr is stored in RcptErrs using the effective recipient address,
		// not the original one.

		originalRcpt := meta.MsgMeta.OriginalRcpts[rcpt]
		if originalRcpt != "" {
			rcpt = originalRcpt
		}

		rcptInfo = append(rcptInfo, dsn.RecipientInfo{
			FinalRecipient: rcpt,
			Action:         dsn.ActionFailed,
			Status:         rcptErr.EnhancedCode,
			DiagnosticCode: rcptErr,
		})
	}

	var dsnBodyBlob bytes.Buffer
	dl := target.DeliveryLogger(q.Log, meta.MsgMeta)
	dsnHeader, err := dsn.GenerateDSN(meta.MsgMeta.SMTPOpts.UTF8, dsnEnvelope, mtaInfo, rcptInfo, header, &dsnBodyBlob)
	if err != nil {
		dl.Error("failed to generate fail DSN", err)
		return
	}
	dsnBody := buffer.MemoryBuffer{Slice: dsnBodyBlob.Bytes()}

	dsnMeta := &module.MsgMetadata{
		ID: dsnID,
		SMTPOpts: smtp.MailOptions{
			UTF8:       meta.MsgMeta.SMTPOpts.UTF8,
			RequireTLS: meta.MsgMeta.SMTPOpts.RequireTLS,
		},
	}
	dl.Msg("generated failed DSN", "dsn_id", dsnID)

	msgCtx, msgTask := trace.NewTask(context.Background(), "DSN Delivery")
	defer msgTask.End()

	mailCtx, mailTask := trace.NewTask(msgCtx, "MAIL FROM")
	dsnDelivery, err := q.dsnPipeline.Start(mailCtx, dsnMeta, "")
	mailTask.End()
	if err != nil {
		dl.Error("failed to enqueue DSN", err, "dsn_id", dsnID)
		return
	}

	defer func() {
		if err != nil {
			dl.Error("failed to enqueue DSN", err, "dsn_id", dsnID)
			if err := dsnDelivery.Abort(msgCtx); err != nil {
				dl.Error("failed to abort DSN delivery", err, "dsn_id", dsnID)
			}
		}
	}()

	rcptCtx, rcptTask := trace.NewTask(msgCtx, "RCPT TO")
	if err = dsnDelivery.AddRcpt(rcptCtx, meta.From); err != nil {
		rcptTask.End()
		return
	}
	rcptTask.End()

	bodyCtx, bodyTask := trace.NewTask(msgCtx, "DATA")
	if err = dsnDelivery.Body(bodyCtx, dsnHeader, dsnBody); err != nil {
		bodyTask.End()
		return
	}
	if err = dsnDelivery.Commit(bodyCtx); err != nil {
		bodyTask.End()
		return
	}
	bodyTask.End()
}

func init() {
	module.Register("target.queue", NewQueue)
}
