package queue

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/testutils"
)

// newTestQueue returns properly initialized Queue object usable for testing.
//
// See newTestQueueDir to create testing queue from an existing directory.
// It is called responsibility to remove queue directory created by this function.
func newTestQueue(t *testing.T, target module.DeliveryTarget) *Queue {
	dir, err := ioutil.TempDir("", "maddy-tests-queue")
	if err != nil {
		t.Fatal("failed to create temporary directory for queue:", err)
	}
	return newTestQueueDir(t, target, dir)
}

func cleanQueue(t *testing.T, q *Queue) {
	if err := q.Close(); err != nil {
		t.Fatal("queue.Close:", err)
	}
	if err := os.RemoveAll(q.location); err != nil {
		t.Fatal("os.RemoveAll", err)
	}
}

func newTestQueueDir(t *testing.T, target module.DeliveryTarget, dir string) *Queue {
	mod, _ := NewQueue("", "queue", nil, nil)
	q := mod.(*Queue)
	q.Log = testutils.Logger(t, "queue")
	q.initialRetryTime = 0
	q.retryTimeScale = 1
	q.postInitDelay = 0
	q.maxTries = 5
	q.location = dir
	q.Target = target

	if !testing.Verbose() {
		q.Log = testutils.Logger(t, "queue")
	}

	q.start(1)

	return q
}

// unreliableTarget is a module.DeliveryTarget implementation that stores
// messages to a slice and sometimes fails with the specified error.
type unreliableTarget struct {
	committed chan testutils.Msg
	aborted   chan testutils.Msg

	// Amount of completed deliveries (both failed and succeeded)
	passedMessages int

	// To make unreliableTarget fail Commit for N-th delivery, set N-1-th
	// element of this slice to wanted error object. If slice is
	// nil/empty or N is bigger than its size - delivery will succeed.
	bodyFailures        []error
	bodyFailuresPartial []map[string]error
	rcptFailures        []map[string]error
}

type unreliableTargetDelivery struct {
	ut  *unreliableTarget
	msg testutils.Msg
}

type unreliableTargetDeliveryPartial struct {
	*unreliableTargetDelivery
}

func (utd *unreliableTargetDelivery) AddRcpt(rcptTo string) error {
	if len(utd.ut.rcptFailures) > utd.ut.passedMessages {
		rcptErrs := utd.ut.rcptFailures[utd.ut.passedMessages]
		if err := rcptErrs[rcptTo]; err != nil {
			return err
		}
	}

	utd.msg.RcptTo = append(utd.msg.RcptTo, rcptTo)
	return nil
}

func (utd *unreliableTargetDelivery) Body(header textproto.Header, body buffer.Buffer) error {
	if utd.ut.bodyFailuresPartial != nil {
		return errors.New("partial failure occurred, no additional information available")
	}

	r, _ := body.Open()
	utd.msg.Body, _ = ioutil.ReadAll(r)

	if len(utd.ut.bodyFailures) > utd.ut.passedMessages {
		return utd.ut.bodyFailures[utd.ut.passedMessages]
	}

	return nil
}

func (utd *unreliableTargetDeliveryPartial) BodyNonAtomic(c module.StatusCollector, header textproto.Header, body buffer.Buffer) {
	r, _ := body.Open()
	utd.msg.Body, _ = ioutil.ReadAll(r)

	if len(utd.ut.bodyFailuresPartial) > utd.ut.passedMessages {
		for rcpt, err := range utd.ut.bodyFailuresPartial[utd.ut.passedMessages] {
			c.SetStatus(rcpt, err)
		}
	}
}

func (utd *unreliableTargetDelivery) Abort() error {
	utd.ut.passedMessages++
	if utd.ut.aborted != nil {
		utd.ut.aborted <- utd.msg
	}
	return nil
}

func (utd *unreliableTargetDelivery) Commit() error {
	utd.ut.passedMessages++
	if utd.ut.committed != nil {
		utd.ut.committed <- utd.msg
	}
	return nil
}

func (ut *unreliableTarget) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	if ut.bodyFailuresPartial != nil {
		return &unreliableTargetDeliveryPartial{
			&unreliableTargetDelivery{
				ut: ut,
				msg: testutils.Msg{
					MsgMeta:  msgMeta,
					MailFrom: mailFrom,
				},
			},
		}, nil
	}
	return &unreliableTargetDelivery{
		ut: ut,
		msg: testutils.Msg{
			MsgMeta:  msgMeta,
			MailFrom: mailFrom,
		},
	}, nil
}

func readMsgChanTimeout(t *testing.T, ch <-chan testutils.Msg, timeout time.Duration) *testutils.Msg {
	t.Helper()
	timer := time.NewTimer(timeout)
	select {
	case msg := <-ch:
		return &msg
	case <-timer.C:
		t.Fatal("chan read timed out")
		return nil
	}
}

func checkQueueDir(t *testing.T, q *Queue, expectedIDs []string) {
	t.Helper()
	// We use the map to lookups and also to mark messages we found
	// we can report missing entries.
	expectedMap := make(map[string]bool, len(expectedIDs))
	for _, id := range expectedIDs {
		expectedMap[id] = false
	}

	dir, err := ioutil.ReadDir(q.location)
	if err != nil {
		t.Fatalf("failed to read queue directory: %v", err)
	}

	// Queue implementation uses file names in the following format:
	// DELIVERY_ID.SOMETHING
	for _, file := range dir {
		if file.IsDir() {
			t.Fatalf("queue should not create subdirectories in the store, but there is %s dir in it", file.Name())
		}

		nameParts := strings.Split(file.Name(), ".")
		if len(nameParts) != 2 {
			t.Fatalf("did the queue files name format changed? got %s", file.Name())
		}

		_, ok := expectedMap[nameParts[0]]
		if !ok {
			t.Errorf("message with unexpected Msg ID %s is stored in queue store", nameParts[0])
			continue
		}

		expectedMap[nameParts[0]] = true
	}

	for id, found := range expectedMap {
		if !found {
			t.Errorf("expected message with Msg ID %s is missing from queue store", id)
		}
	}
}

func TestQueueDelivery(t *testing.T) {
	t.Parallel()

	dt := unreliableTarget{committed: make(chan testutils.Msg, 10)}
	q := newTestQueue(t, &dt)
	defer cleanQueue(t, q)

	testutils.DoTestDelivery(t, q, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

	// This is far from being a proper blackbox testing.
	// But I can't come up with a better way to inspect the Queue state.
	// This probably will be improved when bounce messages will be implemented.
	// For now, this is a dirty hack. Close the Queue and inspect serialized state.
	// FIXME.

	// Wait for the delivery to complete and stop processing.
	msg := readMsgChanTimeout(t, dt.committed, 5*time.Second)
	q.Close()

	testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

	// There should be no queued messages.
	checkQueueDir(t, q, []string{})
}

func TestQueueDelivery_PermanentFail_NonPartial(t *testing.T) {
	t.Parallel()

	dt := unreliableTarget{
		bodyFailures: []error{
			exterrors.WithTemporary(errors.New("you shall not pass"), false),
		},
		aborted: make(chan testutils.Msg, 10),
	}
	q := newTestQueue(t, &dt)
	defer cleanQueue(t, q)

	testutils.DoTestDelivery(t, q, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

	// Queue will abort a delivery if it fails for all recipients.
	readMsgChanTimeout(t, dt.aborted, 5*time.Second)
	q.Close()

	// Delivery is failed permanently, hence no retry should be rescheduled.
	checkQueueDir(t, q, []string{})
}

func TestQueueDelivery_PermanentFail_Partial(t *testing.T) {
	t.Parallel()

	dt := unreliableTarget{
		bodyFailuresPartial: []map[string]error{
			{
				"tester1@example.org": exterrors.WithTemporary(errors.New("you shall not pass"), false),
				"tester2@example.org": exterrors.WithTemporary(errors.New("you shall not pass"), false),
			},
		},
		aborted: make(chan testutils.Msg, 10),
	}
	q := newTestQueue(t, &dt)
	defer cleanQueue(t, q)

	testutils.DoTestDelivery(t, q, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

	// This this is similar to the previous test, but checks PartialDelivery processing logic.
	// Here delivery fails for recipients too, but this is reported using PartialDelivery.

	readMsgChanTimeout(t, dt.aborted, 5*time.Second)
	q.Close()
	checkQueueDir(t, q, []string{})
}

func TestQueueDelivery_TemporaryFail(t *testing.T) {
	t.Parallel()

	dt := unreliableTarget{
		bodyFailures: []error{
			exterrors.WithTemporary(errors.New("you shall not pass"), true),
		},
		aborted:   make(chan testutils.Msg, 10),
		committed: make(chan testutils.Msg, 10),
	}
	q := newTestQueue(t, &dt)
	defer cleanQueue(t, q)

	testutils.DoTestDelivery(t, q, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

	// Delivery should be aborted, because it failed for all recipients.
	readMsgChanTimeout(t, dt.aborted, 5*time.Second)

	// Second retry, should work fine.
	msg := readMsgChanTimeout(t, dt.committed, 5*time.Second)
	testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

	q.Close()
	// No more retries scheduled, queue storage is clear.
	defer checkQueueDir(t, q, []string{})
}

func TestQueueDelivery_TemporaryFail_Partial(t *testing.T) {
	t.Parallel()

	dt := unreliableTarget{
		bodyFailuresPartial: []map[string]error{
			{
				"tester2@example.org": exterrors.WithTemporary(errors.New("go away"), true),
			},
		},
		aborted:   make(chan testutils.Msg, 10),
		committed: make(chan testutils.Msg, 10),
	}
	q := newTestQueue(t, &dt)
	defer cleanQueue(t, q)

	testutils.DoTestDelivery(t, q, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

	// Committed, tester1@example.org - ok.
	msg := readMsgChanTimeout(t, dt.committed, 5000*time.Second)
	// Side note: unreliableTarget adds recipients to the msg object even if they were rejected
	// later using a partial error. So slice below is all recipients that were submitted by
	// the queue.
	testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

	// committed #2, tester2@example.org - ok
	msg = readMsgChanTimeout(t, dt.committed, 5000*time.Second)
	testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester2@example.org"})

	q.Close()
	// No more retries scheduled, queue storage is clear.
	checkQueueDir(t, q, []string{})
}

func TestQueueDelivery_MultipleAttempts(t *testing.T) {
	t.Parallel()

	dt := unreliableTarget{
		bodyFailuresPartial: []map[string]error{
			{
				"tester1@example.org": exterrors.WithTemporary(errors.New("you shall not pass"), false),
				"tester2@example.org": exterrors.WithTemporary(errors.New("you shall not pass"), true),
			},
			{
				"tester2@example.org": exterrors.WithTemporary(errors.New("you shall not pass"), true),
			},
		},
		committed: make(chan testutils.Msg, 10),
	}
	q := newTestQueue(t, &dt)
	defer cleanQueue(t, q)

	testutils.DoTestDelivery(t, q, "tester@example.com", []string{"tester1@example.org", "tester2@example.org", "tester3@example.org"})

	// Committed because delivery to tester3@example.org is succeeded.
	msg := readMsgChanTimeout(t, dt.committed, 5*time.Second)
	// Side note: This slice contains all recipients submitted by the queue, even if
	// they were rejected later using partialError.
	testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester1@example.org", "tester2@example.org", "tester3@example.org"})

	// tester1 is failed permanently, should not be retried.
	// tester2 is failed temporary, should be retried.
	msg = readMsgChanTimeout(t, dt.committed, 5*time.Second)
	testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester2@example.org"})

	q.Close()
	// No more retries should be scheduled.
	checkQueueDir(t, q, []string{})
}

func TestQueueDelivery_PermanentRcptReject(t *testing.T) {
	t.Parallel()

	dt := unreliableTarget{
		rcptFailures: []map[string]error{
			{
				"tester1@example.org": exterrors.WithTemporary(errors.New("go away"), false),
			},
		},
		committed: make(chan testutils.Msg, 10),
	}
	q := newTestQueue(t, &dt)
	defer cleanQueue(t, q)

	testutils.DoTestDelivery(t, q, "tester@example.org", []string{"tester1@example.org", "tester2@example.org"})

	// Committed, tester2@example.org succeeded.
	msg := readMsgChanTimeout(t, dt.committed, 5*time.Second)
	testutils.CheckMsg(t, msg, "tester@example.org", []string{"tester2@example.org"})

	q.Close()
	// No more retries should be scheduled.
	checkQueueDir(t, q, []string{})
}

func TestQueueDelivery_TemporaryRcptReject(t *testing.T) {
	t.Parallel()

	dt := unreliableTarget{
		rcptFailures: []map[string]error{
			{
				"tester1@example.org": &smtp.SMTPError{
					Code:    400,
					Message: "go away",
				},
			},
		},
		committed: make(chan testutils.Msg, 10),
	}
	q := newTestQueue(t, &dt)
	defer cleanQueue(t, q)

	// First attempt:
	//  tester1 - temp. fail
	//  tester2 - ok
	// Second attempt:
	//  tester1 - ok
	testutils.DoTestDelivery(t, q, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

	msg := readMsgChanTimeout(t, dt.committed, 5*time.Second)
	// Unlike previous tests where unreliableTarget rejected recipients by partialError, here they are rejected
	// by AddRcpt directly, so they are NOT saved by the target.
	testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester2@example.org"})

	msg = readMsgChanTimeout(t, dt.committed, 5*time.Second)
	testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester1@example.org"})

	q.Close()
	// No more retries should be scheduled.
	checkQueueDir(t, q, []string{})
}

func TestQueueDelivery_SerializationRoundtrip(t *testing.T) {
	t.Parallel()

	dt := unreliableTarget{
		rcptFailures: []map[string]error{
			{
				"tester1@example.org": &smtp.SMTPError{
					Code:    400,
					Message: "go away",
				},
			},
		},
		committed: make(chan testutils.Msg, 10),
	}
	q := newTestQueue(t, &dt)
	defer cleanQueue(t, q)

	// This is the most tricky test because it is racy and I have no idea what can be done to avoid it.
	// It relies on us calling Close before queue dispatcher decides to retry delivery.
	// Hence retry delay is increased from 0ms used in other tests to make it reliable.
	q.initialRetryTime = 1 * time.Second

	// To make sure we will not time out due to post-init delay.
	q.postInitDelay = 0

	deliveryID := testutils.DoTestDelivery(t, q, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

	// Standard partial delivery, retry will be scheduled for tester1@example.org.
	msg := readMsgChanTimeout(t, dt.committed, 5*time.Second)
	testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester2@example.org"})

	// Then stop it.
	q.Close()

	// Make sure it is saved.
	checkQueueDir(t, q, []string{deliveryID})

	// Then reinit it.
	q = newTestQueueDir(t, &dt, q.location)

	// Wait for retry and check it.
	msg = readMsgChanTimeout(t, dt.committed, 5*time.Second)
	testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester1@example.org"})

	// Close it again.
	q.Close()
	// No more retries should be scheduled.
	checkQueueDir(t, q, []string{})
}

func TestQueueDelivery_DeserlizationCleanUp(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, fileSuffix string) {
		dt := unreliableTarget{
			rcptFailures: []map[string]error{
				{
					"tester1@example.org": &smtp.SMTPError{
						Code:    400,
						Message: "go away",
					},
				},
			},
			committed: make(chan testutils.Msg, 10),
		}
		q := newTestQueue(t, &dt)
		defer cleanQueue(t, q)

		// This is the most tricky test because it is racy and I have no idea what can be done to avoid it.
		// It relies on us calling Close before queue dispatcher decides to retry delivery.
		// Hence retry delay is increased from 0ms used in other tests to make it reliable.
		q.initialRetryTime = 1 * time.Second

		// To make sure we will not time out due to post-init delay.
		q.postInitDelay = 0

		deliveryID := testutils.DoTestDelivery(t, q, "tester@example.com", []string{"tester1@example.org", "tester2@example.org"})

		// Standard partial delivery, retry will be scheduled for tester1@example.org.
		msg := readMsgChanTimeout(t, dt.committed, 5*time.Second)
		testutils.CheckMsg(t, msg, "tester@example.com", []string{"tester2@example.org"})

		q.Close()

		if err := os.Remove(filepath.Join(q.location, deliveryID+fileSuffix)); err != nil {
			t.Fatal(err)
		}

		// Dangling files should be removing during load.
		q = newTestQueueDir(t, &dt, q.location)
		q.Close()

		// Nothing should be left.
		checkQueueDir(t, q, []string{})
	}

	t.Run("NoMeta", func(t *testing.T) {
		t.Skip("Not implemented")
		test(t, ".meta")
	})
	t.Run("NoBody", func(t *testing.T) {
		test(t, ".body")
	})
	t.Run("NoHeader", func(t *testing.T) {
		test(t, ".header")
	})
}
