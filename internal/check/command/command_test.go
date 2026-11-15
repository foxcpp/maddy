package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

func TestCommandHelperProcess(t *testing.T) {
	switch os.Getenv("MADDY_COMMAND_HELPER") {
	case "wait":
		if err := os.WriteFile(os.Getenv("MADDY_COMMAND_READY"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(2)
		}
		select {}
	case "body":
		_, _ = io.Copy(io.Discard, os.Stdin)
		_, _ = fmt.Fprint(os.Stdout, "\r\n")
		os.Exit(0)
	}
}

func TestCheckConnectionCancelsCommand(t *testing.T) {
	ready := t.TempDir() + "/ready"
	t.Setenv("MADDY_COMMAND_HELPER", "wait")
	t.Setenv("MADDY_COMMAND_READY", ready)

	check := &Check{
		stage:            StageConnection,
		cmd:              os.Args[0],
		cmdArgs:          []string{"-test.run=^TestCommandHelperProcess$"},
		timeout:          time.Second,
		interruptTimeout: time.Second,
	}
	s := &state{c: check, log: &log.NopLogger, msgMeta: &module.MsgMetadata{}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.CheckConnection(ctx)
		close(done)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		pidBytes, err := os.ReadFile(ready)
		if err == nil {
			if pid, err := strconv.Atoi(string(pidBytes)); err == nil {
				if proc, err := os.FindProcess(pid); err == nil {
					_ = proc.Kill()
				}
			}
		}
		<-done
		t.Fatal("command did not stop after context cancellation")
	}
}

type trackedBody struct {
	closed bool
}

func (b *trackedBody) Open() (io.ReadCloser, error) {
	return trackedReadCloser{Reader: io.NopCloser(nilReader{}), body: b}, nil
}

func (b *trackedBody) Len() int { return 0 }

func (b *trackedBody) Remove() error { return nil }

type nilReader struct{}

func (nilReader) Read([]byte) (int, error) { return 0, io.EOF }

type trackedReadCloser struct {
	io.Reader
	body *trackedBody
}

func (r trackedReadCloser) Close() error {
	r.body.closed = true
	return nil
}

var _ buffer.Buffer = (*trackedBody)(nil)

func TestCheckBodyClosesBodyReader(t *testing.T) {
	t.Setenv("MADDY_COMMAND_HELPER", "body")
	body := &trackedBody{}
	check := &Check{
		stage:            StageBody,
		cmd:              os.Args[0],
		cmdArgs:          []string{"-test.run=^TestCommandHelperProcess$"},
		timeout:          time.Second,
		interruptTimeout: time.Second,
	}
	s := &state{c: check, log: &log.NopLogger, msgMeta: &module.MsgMetadata{}}

	res := s.CheckBody(context.Background(), textproto.Header{}, body)
	if res.Reject {
		t.Fatalf("check rejected body: %v", res.Reason)
	}
	if !body.closed {
		t.Fatal("body reader was not closed")
	}
}
