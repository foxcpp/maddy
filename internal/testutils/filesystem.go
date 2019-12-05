package testutils

import (
	"io/ioutil"
	"testing"
)

// Dir is a wrapper for ioutil.TempDir that
// fails the test on errors.
func Dir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "maddy-tests-")
	if err != nil {
		t.Fatalf("can't create test dir: %v", err)
	}
	return dir
}
