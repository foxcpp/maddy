package dns

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/foxcpp/maddy/framework/log"
	mdns "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func writeResolverConfig(t *testing.T, path, contents string, modTime time.Time) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
	require.NoError(t, os.Chtimes(path, modTime, modTime))
}

func forceCheckDue(r *ExtResolver) {
	r.mu.Lock()
	r.lastConfigCheck = time.Time{}
	r.mu.Unlock()
}

func TestResolverReloadOnlyWhenModificationTimeChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resolv.conf")
	mtime := time.Unix(1600000000, 0)
	writeResolverConfig(t, path, "nameserver 127.0.0.1\noptions timeout:2\n", mtime)
	optionCalls := 0
	r, err := NewFromConfigFile(
		path,
		func(*ExtResolver) { optionCalls++ },
		WithLogger(&log.NopLogger),
	)
	require.NoError(t, err)
	require.Equal(t, 1, optionCalls)

	// Changes made without an mtime change must not be reloaded, even when due.
	writeResolverConfig(t, path, "nameserver 127.0.0.2\noptions timeout:3\n", mtime)
	forceCheckDue(r)
	r.ensureActualConfig()
	require.Equal(t, []string{"127.0.0.1"}, r.cfg.Servers)
	require.Equal(t, 1, optionCalls)

	// A changed mtime is not observed until the next check is due.
	writeResolverConfig(t, path, "nameserver 127.0.0.2\noptions timeout:3\n", mtime.Add(time.Second))
	r.ensureActualConfig()
	require.Equal(t, []string{"127.0.0.1"}, r.cfg.Servers)
	forceCheckDue(r)
	r.ensureActualConfig()
	require.Equal(t, []string{"127.0.0.2"}, r.cfg.Servers)
	require.Equal(t, 3*time.Second, r.cl.Dialer.Timeout)
	require.Equal(t, 2, optionCalls)
}

func TestResolverReloadRetainsConfigOnStatFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resolv.conf")
	mtime := time.Unix(1600000000, 0)
	writeResolverConfig(t, path, "nameserver 127.0.0.1\n", mtime)

	r, err := NewFromConfigFile(path, WithLogger(&log.NopLogger))
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))
	forceCheckDue(r)
	r.ensureActualConfig()
	require.Equal(t, []string{"127.0.0.1"}, r.cfg.Servers)

	r.ensureActualConfig()
	writeResolverConfig(t, path, "nameserver 127.0.0.2\n", mtime.Add(time.Second))
	forceCheckDue(r)
	r.ensureActualConfig()
	require.Equal(t, []string{"127.0.0.2"}, r.cfg.Servers)
}

func TestResolverReloadRetainsConfigWhenPathBecomesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resolv.conf")
	mtime := time.Unix(1600000000, 0)
	writeResolverConfig(t, path, "nameserver 127.0.0.2\n", mtime)

	r, err := NewFromConfigFile(path, WithLogger(&log.NopLogger))
	require.NoError(t, err)

	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Mkdir(path, 0700))
	require.NoError(t, os.Chtimes(path, mtime.Add(time.Second), mtime.Add(time.Second)))
	forceCheckDue(r)
	r.ensureActualConfig()

	require.Equal(t, []string{"127.0.0.2"}, r.cfg.Servers)
}

func TestResolverReloadRetainsConfigOnOverlongLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resolv.conf")
	mtime := time.Unix(1600000000, 0)
	writeResolverConfig(t, path, "nameserver 127.0.0.2\n", mtime)

	r, err := NewFromConfigFile(path, WithLogger(&log.NopLogger))
	require.NoError(t, err)

	writeResolverConfig(t, path, strings.Repeat("x", 65536)+"\nnameserver 127.0.0.3\n", mtime.Add(time.Second))
	forceCheckDue(r)
	r.ensureActualConfig()
	require.Equal(t, []string{"127.0.0.2"}, r.cfg.Servers)
}

func TestResolverReloadRejectsFIFO(t *testing.T) {
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifo is not available")
	}
	path := filepath.Join(t.TempDir(), "resolv.conf")
	mtime := time.Unix(1600000000, 0)
	writeResolverConfig(t, path, "nameserver 127.0.0.2\n", mtime)

	r, err := NewFromConfigFile(path, WithLogger(&log.NopLogger))
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))
	require.NoError(t, exec.Command(mkfifo, path).Run())
	forceCheckDue(r)

	done := make(chan struct{})
	go func() {
		r.ensureActualConfig()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		// Unblock the buggy implementation before failing, so it leaves no goroutine behind.
		f, openErr := os.OpenFile(path, os.O_WRONLY, 0)
		if openErr == nil {
			_ = f.Close()
		}
		<-done
		t.Fatal("checking a FIFO blocked waiting for a writer")
	}
	require.Equal(t, []string{"127.0.0.2"}, r.cfg.Servers)
}

func TestResolverConfigIsIndependentOfCaller(t *testing.T) {
	cfg := &ClientConfig{Servers: []string{"127.0.0.1"}, Port: "53", Timeout: 1}
	r, err := NewFromConfig(cfg)
	require.NoError(t, err)

	cfg.Servers[0] = "127.0.0.2"
	require.Equal(t, []string{"127.0.0.1"}, r.cfg.Servers)

	r.ensureActualConfig()
	require.Equal(t, []string{"127.0.0.1"}, r.cfg.Servers)
}

func TestResolverOptionsObserveInitializedDefaults(t *testing.T) {
	called := false
	_, err := NewFromConfig(&ClientConfig{
		Servers: []string{"127.0.0.1"},
		Port:    "53",
		Timeout: 1,
	}, func(r *ExtResolver) {
		called = true
		require.NotNil(t, r.cfg)
		require.NotNil(t, r.cl)
		require.NotNil(t, r.log)
	})
	require.NoError(t, err)
	require.True(t, called)
}

func TestResolverReloadPreservesDNSOverride(t *testing.T) {
	previous := overrideServ
	overrideServ = "127.0.0.9:5353"
	t.Cleanup(func() { overrideServ = previous })

	path := filepath.Join(t.TempDir(), "resolv.conf")
	mtime := time.Unix(1600000000, 0)
	writeResolverConfig(t, path, "nameserver 127.0.0.1\n", mtime)

	r, err := NewFromConfigFile(path)
	require.NoError(t, err)
	require.Equal(t, []string{"127.0.0.9"}, r.cfg.Servers)
	require.Equal(t, "5353", r.cfg.Port)

	writeResolverConfig(t, path, "nameserver 127.0.0.2\n", mtime.Add(time.Second))
	forceCheckDue(r)
	r.ensureActualConfig()

	require.Equal(t, []string{"127.0.0.9"}, r.cfg.Servers)
	require.Equal(t, "5353", r.cfg.Port)
}

func TestResolverConcurrentChecks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resolv.conf")
	mtime := time.Unix(1600000000, 0)
	writeResolverConfig(t, path, "nameserver 127.0.0.1\n", mtime)

	r, err := NewFromConfigFile(path, WithLogger(&log.NopLogger))
	require.NoError(t, err)

	writeResolverConfig(t, path, "nameserver 127.0.0.2\n", mtime.Add(time.Second))
	forceCheckDue(r)
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			r.ensureActualConfig()
		})
	}
	wg.Wait()
	require.Equal(t, []string{"127.0.0.2"}, r.cfg.Servers)
}

func TestResolverLookupsUseReloadedServer(t *testing.T) {
	first, err := net.ListenPacket("udp4", "127.0.0.1:0")
	require.NoError(t, err)
	port := first.LocalAddr().(*net.UDPAddr).Port
	second, err := net.ListenPacket("udp4", net.JoinHostPort("127.0.0.2", strconv.Itoa(port)))
	require.NoError(t, err)
	for i, conn := range []net.PacketConn{first, second} {
		answer := fmt.Sprintf("server-%d", i+1)
		server := &mdns.Server{PacketConn: conn, Handler: mdns.HandlerFunc(func(w mdns.ResponseWriter, req *mdns.Msg) {
			resp := new(mdns.Msg)
			resp.SetReply(req)
			resp.Answer = []mdns.RR{&mdns.TXT{Hdr: mdns.RR_Header{Name: req.Question[0].Name, Rrtype: mdns.TypeTXT, Class: mdns.ClassINET}, Txt: []string{answer}}}
			_ = w.WriteMsg(resp)
		})}
		go func() { _ = server.ActivateAndServe() }()
		t.Cleanup(func() { _ = server.Shutdown() })
	}

	path := filepath.Join(t.TempDir(), "resolv.conf")
	mtime := time.Unix(1600000000, 0)
	writeResolverConfig(t, path, "nameserver 127.0.0.1\n", mtime)
	r, err := NewFromConfigFile(path, func(r *ExtResolver) { r.cfg.Port = strconv.Itoa(port) })
	require.NoError(t, err)
	_, records, err := r.AuthLookupTXT(context.Background(), "example.test")
	require.NoError(t, err)
	require.Equal(t, []string{"server-1"}, records)
	writeResolverConfig(t, path, "nameserver 127.0.0.2\n", mtime.Add(time.Second))
	forceCheckDue(r)
	_, records, err = r.AuthLookupTXT(context.Background(), "example.test")
	require.NoError(t, err)
	require.Equal(t, []string{"server-2"}, records)

	// Other lookups may use the previous snapshot while a reload is underway.
	writeResolverConfig(t, path, "nameserver 127.0.0.1\n", mtime.Add(2*time.Second))
	forceCheckDue(r)
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			_, answers, lookupErr := r.AuthLookupTXT(context.Background(), "example.test")
			if lookupErr != nil {
				t.Errorf("lookup during reload: %v", lookupErr)
			} else if len(answers) != 1 || (answers[0] != "server-1" && answers[0] != "server-2") {
				t.Errorf("unexpected answer during reload: %v", answers)
			}
		})
	}
	wg.Wait()
	_, records, err = r.AuthLookupTXT(context.Background(), "example.test")
	require.NoError(t, err)
	require.Equal(t, []string{"server-1"}, records)
}
