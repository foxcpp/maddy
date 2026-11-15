package command

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

func TestIMAPCommandHelperProcess(t *testing.T) {
	if os.Getenv("MADDY_IMAP_COMMAND_HELPER") != "1" {
		return
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	_ = os.WriteFile(os.Getenv("MADDY_IMAP_COMMAND_READY"), []byte("done"), 0o600)
	_, _ = io.WriteString(os.Stdout, "INBOX\n")
	os.Exit(0)
}

type trackedBody struct {
	closed bool
}

func (b *trackedBody) Open() (io.ReadCloser, error) {
	return trackedReadCloser{Reader: &emptyReader{}, body: b}, nil
}

func (b *trackedBody) Len() int { return 0 }

func (b *trackedBody) Remove() error { return nil }

type emptyReader struct{}

func (emptyReader) Read([]byte) (int, error) { return 0, io.EOF }

type trackedReadCloser struct {
	io.Reader
	body *trackedBody
}

func (r trackedReadCloser) Close() error {
	r.body.closed = true
	return nil
}

var _ buffer.Buffer = (*trackedBody)(nil)

func TestIMAPFilterClosesBodyReader(t *testing.T) {
	ready := t.TempDir() + "/ready"
	t.Setenv("MADDY_IMAP_COMMAND_HELPER", "1")
	t.Setenv("MADDY_IMAP_COMMAND_READY", ready)
	check := &Check{
		log:              &log.NopLogger,
		cmd:              os.Args[0],
		cmdArgs:          []string{"-test.run=^TestIMAPCommandHelperProcess$"},
		timeout:          5 * time.Second,
		interruptTimeout: time.Second,
	}
	body := &trackedBody{}
	folder, flags, err := check.IMAPFilter("account", "recipient@example.com", &module.MsgMetadata{}, textproto.Header{}, body)
	if err != nil {
		t.Fatalf("IMAPFilter failed: %v", err)
	}
	if folder != "INBOX" || len(flags) != 0 {
		t.Errorf("filter result = (%q, %v), want (INBOX, [])", folder, flags)
	}
	if !body.closed {
		t.Fatal("body reader was not closed")
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatalf("helper did not consume body: %v", err)
	}
}
