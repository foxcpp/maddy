package dkim

import (
	"reflect"
	"sort"
	"testing"

	"github.com/emersion/go-message/textproto"
)

// TODO: Add tests that check actual signing once
// we have LookupTXT hook for go-msgauth/dkim.

func TestFieldsToSign(t *testing.T) {
	h := textproto.Header{}
	h.Add("A", "1")
	h.Add("c", "2")
	h.Add("C", "3")
	h.Add("a", "4")
	h.Add("b", "5")
	h.Add("unrelated", "6")

	m := Modifier{
		oversignHeader: []string{"A", "B"},
		signHeader:     []string{"C"},
	}
	fields := m.fieldsToSign(h)
	sort.Strings(fields)
	expected := []string{"A", "A", "A", "B", "B", "C", "C"}

	if !reflect.DeepEqual(fields, expected) {
		t.Errorf("incorrect set of fields to sign\nwant: %v\ngot:  %v", expected, fields)
	}
}
