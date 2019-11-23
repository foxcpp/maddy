package smtp

import (
	"reflect"
	"testing"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/module"
)

func init() {
	msgIDField = func() (string, error) {
		return "A", nil
	}

	now = func() time.Time {
		return time.Unix(0, 0)
	}
}

func TestSubmissionPrepare(t *testing.T) {
	test := func(hdrMap map[string][]string, expectedMap map[string][]string) {
		t.Helper()

		hdr := textproto.Header{}
		for k, v := range hdrMap {
			for _, field := range v {
				hdr.Add(k, field)
			}
		}

		endp := testEndpoint(t, "submission", &module.Dummy{}, &module.Dummy{}, nil, nil)
		defer func() {
			// Synchronize the endpoint initialization.
			// Otherwise Close will race with Serve called by setupListeners.
			cl, _ := smtp.Dial("127.0.0.1:" + testPort)
			cl.Close()

			endp.Close()
		}()

		session, err := endp.Login(&smtp.ConnectionState{}, "u", "p")
		if err != nil {
			t.Fatal(err)
		}

		err = session.(*Session).submissionPrepare(&hdr)
		if expectedMap == nil {
			if err == nil {
				t.Error("Expected an error, got none")
			}
			t.Log(err)
			return
		}
		if expectedMap != nil && err != nil {
			t.Error("Unexpected error:", err)
			return
		}

		resMap := make(map[string][]string)
		for field := hdr.Fields(); field.Next(); {
			resMap[field.Key()] = append(resMap[field.Key()], field.Value())
		}

		if !reflect.DeepEqual(expectedMap, resMap) {
			t.Errorf("wrong header result\nwant %#+v\ngot  %#+v", expectedMap, resMap)
		}
	}

	// No From field.
	test(map[string][]string{}, nil)

	// Malformed From field.
	test(map[string][]string{
		"From": []string{"<hello@example.org>, \"\""},
	}, nil)
	test(map[string][]string{
		"From": []string{" adasda"},
	}, nil)

	// Malformed Reply-To.
	test(map[string][]string{
		"From":     []string{"<hello@example.org>"},
		"Reply-To": []string{"<hello@example.org>, \"\""},
	}, nil)

	// Malformed CC.
	test(map[string][]string{
		"From":     []string{"<hello@example.org>"},
		"Reply-To": []string{"<hello@example.org>"},
		"Cc":       []string{"<hello@example.org>, \"\""},
	}, nil)

	// Malformed Sender.
	test(map[string][]string{
		"From":     []string{"<hello@example.org>"},
		"Reply-To": []string{"<hello@example.org>"},
		"Cc":       []string{"<hello@example.org>"},
		"Sender":   []string{"<hello@example.org> asd"},
	}, nil)

	// Multiple From + no Sender.
	test(map[string][]string{
		"From": []string{"<hello@example.org>, <hello2@example.org>"},
	}, nil)

	// Multiple From + valid Sender.
	test(map[string][]string{
		"From":       []string{"<hello@example.org>, <hello2@example.org>"},
		"Sender":     []string{"<hello@example.org>"},
		"Date":       []string{"Fri, 22 Nov 2019 20:51:31 +0800"},
		"Message-Id": []string{"<foobar@example.org>"},
	}, map[string][]string{
		"From":       []string{"<hello@example.org>, <hello2@example.org>"},
		"Sender":     []string{"<hello@example.org>"},
		"Date":       []string{"Fri, 22 Nov 2019 20:51:31 +0800"},
		"Message-Id": []string{"<foobar@example.org>"},
	})

	// Add missing Message-Id.
	test(map[string][]string{
		"From": []string{"<hello@example.org>"},
		"Date": []string{"Fri, 22 Nov 2019 20:51:31 +0800"},
	}, map[string][]string{
		"From":       []string{"<hello@example.org>"},
		"Date":       []string{"Fri, 22 Nov 2019 20:51:31 +0800"},
		"Message-Id": []string{"<A@mx.example.com>"},
	})

	// Malformed Date.
	test(map[string][]string{
		"From": []string{"<hello@example.org>"},
		"Date": []string{"not a date"},
	}, nil)

	// Add missing Date.
	test(map[string][]string{
		"From":       []string{"<hello@example.org>"},
		"Message-Id": []string{"<A@mx.example.org>"},
	}, map[string][]string{
		"From":       []string{"<hello@example.org>"},
		"Message-Id": []string{"<A@mx.example.org>"},
		"Date":       []string{"Thu, 1 Jan 1970 00:00:00 +0000"},
	})
}
