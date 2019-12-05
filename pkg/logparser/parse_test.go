package parser

import (
	"reflect"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	test := func(line string, msg Msg, errDesc string) {
		t.Helper()

		parsed, err := Parse(line)
		if errDesc != "" {
			if err == nil {
				t.Errorf("Expected an error, got none")
				return
			}
			if err.(MalformedMsg).Desc != errDesc {
				t.Errorf("Wrong error desc returned: %v", err.(MalformedMsg).Desc)
				return
			}
		}
		if errDesc == "" && err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if !reflect.DeepEqual(parsed, msg) {
			t.Errorf("Wrong Parse result,\n got  %#+v\n want %#+v", parsed, msg)
		}
	}

	test("2006-01-02T15:04:05.000Z module: hello\t", Msg{
		Stamp:   time.Date(2006, time.January, 02, 15, 04, 05, 0, time.UTC),
		Module:  "module",
		Message: "hello",
		Context: map[string]interface{}{},
	}, "")
	test("2006-01-02T15:04:05.000Z module: hello: whatever\t", Msg{
		Stamp:   time.Date(2006, time.January, 02, 15, 04, 05, 0, time.UTC),
		Module:  "module",
		Message: "hello: whatever",
		Context: map[string]interface{}{},
	}, "")
	test("2006-01-02T15:04:05.000Z module: hello: whatever\t{\"a\":1}", Msg{
		Stamp:   time.Date(2006, time.January, 02, 15, 04, 05, 0, time.UTC),
		Module:  "module",
		Message: "hello: whatever",
		Context: map[string]interface{}{
			"a": float64(1),
		},
	}, "")
	test("2006-01-02T15:04:05.000Z module: hello: whatever\t{\"a\":1,\"b\":\"bbb\"}", Msg{
		Stamp:   time.Date(2006, time.January, 02, 15, 04, 05, 0, time.UTC),
		Module:  "module",
		Message: "hello: whatever",
		Context: map[string]interface{}{
			"a": float64(1),
			"b": "bbb",
		},
	}, "")
	test("2006-01-02T15:04:05.000Z [debug] module: hello: whatever\t{\"a\":1,\"b\":\"bbb\"}", Msg{
		Stamp:   time.Date(2006, time.January, 02, 15, 04, 05, 0, time.UTC),
		Debug:   true,
		Module:  "module",
		Message: "hello: whatever",
		Context: map[string]interface{}{
			"a": float64(1),
			"b": "bbb",
		},
	}, "")
	test("2006-01-02T15:04:05.000Z [debug] oink oink: hello: whatever\t{\"a\":1,\"b\":\"bbb\"}", Msg{
		Stamp:   time.Date(2006, time.January, 02, 15, 04, 05, 0, time.UTC),
		Debug:   true,
		Message: "oink oink: hello: whatever",
		Context: map[string]interface{}{
			"a": float64(1),
			"b": "bbb",
		},
	}, "")
	test("2006-01-02T15:04:05.000Z [debug] whatever\t{\"a\":1,\"b\":\"bbb\"}", Msg{
		Stamp:   time.Date(2006, time.January, 02, 15, 04, 05, 0, time.UTC),
		Debug:   true,
		Message: "whatever",
		Context: map[string]interface{}{
			"a": float64(1),
			"b": "bbb",
		},
	}, "")
	test("module: hello\t", Msg{}, "timestamp parse")
	test("hello\t", Msg{}, "missing a timestamp")
	test("2006-01-02T15:04:05.000Z module: hello", Msg{}, "missing a tab separator")
	test("2006-01-02T15:04:05.000Z [BROKEN FORMATTING: json: wtf lol omg]: hello map[stringasdasd]", Msg{}, "missing a tab separator")
}
