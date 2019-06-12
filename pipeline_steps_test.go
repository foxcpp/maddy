package maddy

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/emersion/maddy/module"
)

type msg struct {
	Ctx  *module.DeliveryContext
	Body []byte
}

var testMsgBody = `hello!`
var testCtx = module.DeliveryContext{
	// Otherwise it will do external networking, which is undesirable in tests.
	DontTraceSender: true,

	SrcProto:    "TEST",
	SrcHostname: "tester",
	OurHostname: "testable",
	DeliveryID:  "testing",
	From:        "tester@example.org",
	To:          []string{"tester2@example.org"},
	Ctx:         make(map[string]interface{}),
}

// dummyStep is a pipeline step that just appends all received messages to
// the slice.
type dummyStep struct {
	Messages []msg
}

func (ds *dummyStep) Pass(ctx *module.DeliveryContext, body io.Reader) (newBody io.Reader, continueProcessing bool, err error) {
	buf, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, false, err
	}

	ds.Messages = append(ds.Messages, msg{ctx.DeepCopy(), buf})
	return nil, true, nil
}

// markStep is a pipeline step that just adds "X-Marked: 1" header to the header
// marked=true to Ctx ma
type markStep struct{}

func (ms *markStep) Pass(ctx *module.DeliveryContext, body io.Reader) (newBody io.Reader, continueProcessing bool, err error) {
	ctx.Header.Add("X-Marked", "1")
	ctx.Ctx["marked"] = true
	return nil, true, nil
}

// bodyReplStep is a pipeline step that replaces message body.
type bodyReplStep struct {
	Body []byte
}

func (brs *bodyReplStep) Pass(_ *module.DeliveryContext, _ io.Reader) (newBody io.Reader, continueProcessing bool, err error) {
	return bytes.NewReader(brs.Body), true, nil
}

// failingStep makes test fail if it is called.
type failingStep struct {
	t *testing.T
}

func (fs *failingStep) Pass(_ *module.DeliveryContext, _ io.Reader) (newBody io.Reader, continueProcessing bool, err error) {
	fs.t.Error("failingStep called")
	return nil, true, nil
}

func TestPipelineBasicPass(t *testing.T) {
	ds := dummyStep{}
	steps := []PipelineStep{&ds}

	ctx := testCtx.DeepCopy()
	body := strings.NewReader(testMsgBody)

	_, cont, err := passThroughPipeline(steps, ctx, body)
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if !cont {
		t.Error("passThroughPipeline should return continueProcessing=true")
	}

	if len(ds.Messages) != 1 {
		t.Errorf("Unexpected amount of messages received by step (%d != 1)", len(ds.Messages))
	}

	if ds.Messages[0].Ctx.SrcProto != "TEST" {
		t.Errorf("Missing or wrong Ctx.SrcProto (%s != %s)", ds.Messages[0].Ctx.SrcProto, "TEST")
	}
	if string(ds.Messages[0].Body) != testMsgBody {
		t.Errorf("Wrong message body:\nexpected: %s\ngot: %s\n", testMsgBody, ds.Messages[0].Body)
	}
}

func TestPipelinePassTwice(t *testing.T) {
	ds := dummyStep{}
	steps := []PipelineStep{&ds, &ds}

	ctx := testCtx.DeepCopy()
	body := strings.NewReader(testMsgBody)

	_, cont, err := passThroughPipeline(steps, ctx, body)
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if !cont {
		t.Error("passThroughPipeline should return continueProcessing=true")
	}

	if len(ds.Messages) != 2 {
		t.Errorf("Unexpected amount of messages received by step (%d != 2)", len(ds.Messages))
	}

	if ds.Messages[0].Ctx.SrcProto != "TEST" {
		t.Errorf("Missing or wrong Ctx.SrcProto (%s != %s)", ds.Messages[0].Ctx.SrcProto, "TEST")
	}
	if string(ds.Messages[0].Body) != testMsgBody {
		t.Errorf("Wrong message body:\nexpected: %s\ngot: %s\n", testMsgBody, ds.Messages[0].Body)
	}

	if !reflect.DeepEqual(ds.Messages[0], ds.Messages[1]) {
		t.Errorf("Second message is not equal to the first: %+v != %+v", ds.Messages[0], ds.Messages[1])
	}
}

func TestPipelineContextMutation(t *testing.T) {
	ds := dummyStep{}
	steps := []PipelineStep{&ds, &markStep{}, &ds}

	ctx := testCtx.DeepCopy()
	body := strings.NewReader(testMsgBody)

	_, cont, err := passThroughPipeline(steps, ctx, body)
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if !cont {
		t.Error("passThroughPipeline should return continueProcessing=true")
	}

	if len(ds.Messages) != 2 {
		t.Errorf("Unexpected amount of messages received by step (%d != 2)", len(ds.Messages))
	}

	if ds.Messages[0].Ctx.Ctx["marked"] != nil {
		t.Errorf("First message should not be marked")
	}
	if ds.Messages[1].Ctx.Ctx["marked"] == nil {
		t.Errorf("First message should be marked")
	}
}

func TestPipelineBodyMutation(t *testing.T) {
	ds := dummyStep{}
	steps := []PipelineStep{&ds, &bodyReplStep{Body: []byte("goodbye!")}, &ds}

	ctx := testCtx.DeepCopy()
	body := strings.NewReader(testMsgBody)

	_, cont, err := passThroughPipeline(steps, ctx, body)
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if !cont {
		t.Error("passThroughPipeline should return continueProcessing=true")
	}

	if len(ds.Messages) != 2 {
		t.Errorf("Unexpected amount of messages received by step (%d != 2)", len(ds.Messages))
	}

	if string(ds.Messages[0].Body) != "hello!" {
		t.Errorf("First message should not be changed")
	}
	if string(ds.Messages[1].Body) != "goodbye!" {
		t.Errorf("First message should be changed")
	}
}

func TestPipelineStop(t *testing.T) {
	ds := dummyStep{}
	steps := []PipelineStep{&ds, &stopStep{}, &failingStep{t}}

	ctx := testCtx.DeepCopy()
	body := strings.NewReader(testMsgBody)

	_, cont, err := passThroughPipeline(steps, ctx, body)
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if cont {
		t.Error("passThroughPipeline should return continueProcessing=false")
	}
	if len(ds.Messages) != 1 {
		t.Errorf("Unexpected amount of messages received by step (%d != 2)", len(ds.Messages))
	}
}

func TestPipelineStopWithError(t *testing.T) {
	ds := dummyStep{}
	steps := []PipelineStep{&ds, &stopStep{errCode: 500, errMsg: "foo"}, &failingStep{t: t}}

	ctx := testCtx.DeepCopy()
	body := strings.NewReader(testMsgBody)

	_, cont, err := passThroughPipeline(steps, ctx, body)
	if err == nil {
		t.Error("Expected error, got none")
	}
	if cont {
		t.Error("passThroughPipeline should return continueProcessing=false")
	}
}

func TestPipelineRequireAuthPass(t *testing.T) {
	ds := dummyStep{}
	steps := []PipelineStep{&ds, &requireAuthStep{}, &failingStep{t}}

	ctx := testCtx.DeepCopy()
	body := strings.NewReader(testMsgBody)

	_, cont, err := passThroughPipeline(steps, ctx, body)
	if err == nil {
		t.Error("Expected error, got none")
	}
	if cont {
		t.Error("passThroughPipeline should return continueProcessing=false")
	}
}

type deliveryTarget struct {
	Messages []msg
}

func (dt *deliveryTarget) Deliver(ctx module.DeliveryContext, body io.Reader) error {
	buf, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}

	dt.Messages = append(dt.Messages, msg{ctx.DeepCopy(), buf})
	return nil
}

func TestPipelineDeliverPass(t *testing.T) {
	dt := deliveryTarget{}
	steps := []PipelineStep{
		&deliverStep{t: &dt, resolver: net.DefaultResolver},
		&deliverStep{t: &dt, resolver: net.DefaultResolver},
	}

	ctx := testCtx.DeepCopy()
	body := strings.NewReader(testMsgBody)

	_, cont, err := passThroughPipeline(steps, ctx, body)
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if !cont {
		t.Error("passThroughPipeline should return continueProcessing=true")
	}

	if dt.Messages[0].Ctx.SrcProto != "TEST" {
		t.Errorf("Missing or wrong Ctx.SrcProto (%s != %s)", dt.Messages[0].Ctx.SrcProto, "TEST")
	}
	if string(dt.Messages[0].Body) != testMsgBody {
		t.Errorf("Wrong message body:\nexpected: %s\ngot: %s\n", testMsgBody, dt.Messages[0].Body)
	}
	if !reflect.DeepEqual(dt.Messages[0], dt.Messages[1]) {
		t.Errorf("Second message is not equal to the first: %+v != %+v", dt.Messages[0], dt.Messages[1])
	}

	fields := dt.Messages[1].Ctx.Header.FieldsByKey("Received")
	count := 0
	for fields.Next() {
		count += 1
	}
	if count != 1 {
		t.Errorf("Second message should have only one Received header, got %d", count)
	}
}

func TestPipelineDestinationSplit(t *testing.T) {
	cases := []struct {
		conds     []destinationCond
		to        []string
		matchedTo []string
		missedTo  []string
	}{
		{
			conds: []destinationCond{
				{
					value: "example.org",
				},
			},
			to:        []string{"tester@example.org"},
			matchedTo: []string{"tester@example.org"},
			missedTo:  []string{},
		},
		{
			conds: []destinationCond{
				{
					value: "example.com",
				},
			},
			to:        []string{"tester@example.org"},
			matchedTo: []string{},
			missedTo:  []string{"tester@example.org"},
		},
		{
			conds: []destinationCond{
				{
					value: "tester@example.com",
				},
			},
			to:        []string{"tester@example.org"},
			matchedTo: []string{},
			missedTo:  []string{"tester@example.org"},
		},
		{
			conds: []destinationCond{
				{
					value: "tester@example.org",
				},
			},
			to:        []string{"tester@example.org", "tester2@example.org"},
			matchedTo: []string{"tester@example.org"},
			missedTo:  []string{},
		},
		{
			conds: []destinationCond{
				{
					value: "tester@example.org",
				},
			},
			to:        []string{"tester@example.org", "tester2@example.org"},
			matchedTo: []string{"tester@example.org"},
			missedTo:  []string{"tester2@example.org"},
		},
		{
			conds: []destinationCond{
				{
					value: "example.com",
				},
			},
			to:        []string{"tester@example.org", "tester@example.com"},
			matchedTo: []string{"tester@example.com"},
			missedTo:  []string{"tester@example.org"},
		},
		{
			conds: []destinationCond{
				{
					value: "example.org",
				},
				{
					value: "example.com",
				},
			},
			to:        []string{"tester@example.org", "tester@example.com"},
			matchedTo: []string{"tester@example.org", "tester@example.com"},
			missedTo:  []string{},
		},
		{
			conds: []destinationCond{
				{
					value: "example.org",
				},
				{
					value: "tester@example.com",
				},
			},
			to:        []string{"tester@example.org", "tester2@example.com"},
			matchedTo: []string{"tester@example.org"},
			missedTo:  []string{"tester2@example.com"},
		},
		{
			conds: []destinationCond{
				{
					value: "example.org",
				},
				{
					value: "tester@example.com",
				},
			},
			to:        []string{"tester@example.org", "tester2@example.com"},
			matchedTo: []string{"tester@example.org"},
			missedTo:  []string{"tester2@example.com"},
		},
		{
			conds: []destinationCond{
				{
					value: "example.org",
				},
				{
					value: "tester@example.com",
				},
			},
			to:        []string{"tester@example.org", "tester2@example.com"},
			matchedTo: []string{"tester@example.org"},
			missedTo:  []string{"tester2@example.com"},
		},
		{
			conds: []destinationCond{
				{
					regexp: regexp.MustCompile(`tester.?@example\.(org|com)`),
				},
			},
			to:        []string{"tester@example.org", "tester2@example.com"},
			matchedTo: []string{"tester@example.org", "tester2@example.com"},
			missedTo:  []string{},
		},
		{
			conds: []destinationCond{
				{
					value: "tester@example.org",
				},
			},
			to:        []string{"tester@EXAMPLE.ORG"},
			matchedTo: []string{"tester@EXAMPLE.ORG"},
			missedTo:  []string{},
		},
		{
			conds: []destinationCond{
				{
					value: "example.org",
				},
			},
			to:        []string{"tester@EXAMPLE.ORG"},
			matchedTo: []string{"tester@EXAMPLE.ORG"},
			missedTo:  []string{},
		},
		{
			conds: []destinationCond{
				{
					value: "tester2@example.org",
				},
			},
			to:        []string{"TeStEr2@EXAMPLE.ORG"},
			matchedTo: []string{"TeStEr2@EXAMPLE.ORG"},
			missedTo:  []string{},
		},
	}

	for _, case_ := range cases {
		case_ := case_
		t.Run(fmt.Sprintf("%+v", case_), func(t *testing.T) {
			matched := dummyStep{}
			missed := dummyStep{}
			steps := []PipelineStep{
				&destinationStep{
					conds:    case_.conds,
					substeps: []PipelineStep{&matched},
				},
				&missed,
			}

			ctx := testCtx.DeepCopy()
			ctx.To = case_.to
			body := strings.NewReader(testMsgBody)

			_, cont, err := passThroughPipeline(steps, ctx, body)
			if err != nil {
				t.Error("Unexpected error:", err)
			}
			if cont != (len(missed.Messages) != 0) {
				t.Errorf("passThroughPipeline should return continueProcessing=false if no recipients missed")
			}

			if len(case_.matchedTo) != 0 {
				if len(matched.Messages) != 1 {
					t.Fatalf("Unexpected number of messages matched (%d) (to = %v, matchedTo = %v)", len(matched.Messages), case_.to, case_.matchedTo)
				}
				msg := matched.Messages[0]
				if !reflect.DeepEqual(case_.matchedTo, msg.Ctx.To) {
					t.Errorf("Incorrect receipients for matched message, wanted %v, got %v", case_.matchedTo, msg.Ctx.To)
				}
			}
			if len(case_.missedTo) != 0 {
				if len(missed.Messages) != 1 {
					t.Fatalf("Unexpected number of messages missed (%d) (to = %v, matchedTo = %v)", len(missed.Messages), case_.to, case_.matchedTo)
				}
				msg := missed.Messages[0]
				if !reflect.DeepEqual(case_.missedTo, msg.Ctx.To) {
					t.Errorf("Incorrect receipients for missed message, wanted %v, got %v", case_.missedTo, msg.Ctx.To)
				}
			}
		})
	}
}
