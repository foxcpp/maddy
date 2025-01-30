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

// Package tests provides the framework for integration testing of maddy.
//
// The packages core object is tests.T object that encapsulates all test
// state. It runs the server using test-provided configuration file and acts as
// a proxy for all interactions with the server.
package tests

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/foxcpp/go-mockdns"
)

var (
	TestBinary  = "./maddy"
	CoverageOut string
	DebugLog    bool
)

type T struct {
	*testing.T

	testDir string
	cfg     string

	dnsServ  *mockdns.Server
	env      []string
	ports    map[string]uint16
	portsRev map[uint16]string

	servProc *exec.Cmd
}

func NewT(t *testing.T) *T {
	return &T{
		T:        t,
		ports:    map[string]uint16{},
		portsRev: map[uint16]string{},
	}
}

// Config sets the configuration to use for the server. It must be called
// before Run.
func (t *T) Config(cfg string) {
	t.Helper()

	if t.servProc != nil {
		panic("tests: Config called after Run")
	}

	t.cfg = cfg
}

// DNS sets the DNS zones to emulate for the tested server instance.
//
// If it is not called before Run, DNS(nil) call is assumed which makes the
// mockdns server respond with NXDOMAIN to all queries.
func (t *T) DNS(zones map[string]mockdns.Zone) {
	t.Helper()

	if zones == nil {
		zones = map[string]mockdns.Zone{}
	}
	if _, ok := zones["100.97.109.127.in-addr.arpa."]; !ok {
		zones["100.97.109.127.in-addr.arpa."] = mockdns.Zone{PTR: []string{"client.maddy.test."}}
	}

	if t.dnsServ != nil {
		t.Log("NOTE: Multiple DNS calls, replacing the server instance...")
		t.dnsServ.Close()
	}

	dnsServ, err := mockdns.NewServer(zones, false)
	if err != nil {
		t.Fatal("Test configuration failed:", err)
	}
	dnsServ.Log = t
	t.dnsServ = dnsServ
}

// Port allocates the random TCP port for use by test. It will made accessible
// in the configuration via environment variables with name in the form
// TEST_PORT_name.
//
// If there is a port with name remote_smtp, it will be passed as the value for
// the -debug.smtpport parameter.
func (t *T) Port(name string) uint16 {
	if port := t.ports[name]; port != 0 {
		return port
	}

	// TODO: Try to bind on port to test its usability.
	port := rand.Int31n(45536) + 20000
	t.ports[name] = uint16(port)
	t.portsRev[uint16(port)] = name
	return uint16(port)
}

func (t *T) Env(kv string) {
	t.env = append(t.env, kv)
}

func (t *T) ensureCanRun() {
	if t.cfg == "" {
		panic("tests: Run called without configuration set")
	}
	if t.dnsServ == nil {
		// If there is no DNS zones set in test - start a server that will
		// respond with NXDOMAIN to all queries to avoid accidentally leaking
		// any DNS queries to the real world.
		t.Log("NOTE: Explicit DNS(nil) is recommended.")
		t.DNS(nil)

		t.Cleanup(func() {
			// Shutdown the DNS server after maddy to make sure it will not spend time
			// timing out queries.
			if err := t.dnsServ.Close(); err != nil {
				t.Log("Unable to stop the DNS server:", err)
			}
			t.dnsServ = nil
		})
	}

	// Setup file system, create statedir, runtimedir, write out config.
	if t.testDir == "" {
		testDir, err := os.MkdirTemp("", "maddy-tests-")
		if err != nil {
			t.Fatal("Test configuration failed:", err)
		}
		t.testDir = testDir
		t.Log("using", t.testDir)

		if err := os.MkdirAll(filepath.Join(t.testDir, "statedir"), os.ModePerm); err != nil {
			t.Fatal("Test configuration failed:", err)
		}
		if err := os.MkdirAll(filepath.Join(t.testDir, "runtimedir"), os.ModePerm); err != nil {
			t.Fatal("Test configuration failed:", err)
		}

		t.Cleanup(func() {
			if !t.Failed() {
				return
			}

			t.Log("removing", t.testDir)
			os.RemoveAll(t.testDir)
			t.testDir = ""
		})
	}

	configPreable := "state_dir " + filepath.Join(t.testDir, "statedir") + "\n" +
		"runtime_dir " + filepath.Join(t.testDir, "runtimedir") + "\n\n"

	err := os.WriteFile(filepath.Join(t.testDir, "maddy.conf"), []byte(configPreable+t.cfg), os.ModePerm)
	if err != nil {
		t.Fatal("Test configuration failed:", err)
	}
}

func (t *T) buildCmd(additionalArgs ...string) *exec.Cmd {
	// Assigning 0 by default will make outbound SMTP unusable.
	remoteSmtp := "0"
	if port := t.ports["remote_smtp"]; port != 0 {
		remoteSmtp = strconv.Itoa(int(port))
	}

	args := []string{"-config", filepath.Join(t.testDir, "maddy.conf"),
		"-debug.smtpport", remoteSmtp,
		"-debug.dnsoverride", t.dnsServ.LocalAddr().String(),
		"-log", "/tmp/test.log"}

	if CoverageOut != "" {
		args = append(args, "-test.coverprofile", CoverageOut+"."+strconv.FormatInt(time.Now().UnixNano(), 16))
	}
	if DebugLog {
		args = append(args, "-debug")
	}

	args = append(args, additionalArgs...)

	cmd := exec.Command(TestBinary, args...)

	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal("Test configuration failed:", err)
	}

	// Set environment variables.
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		"TEST_PWD="+pwd,
		"TEST_STATE_DIR="+filepath.Join(t.testDir, "statedir"),
		"TEST_RUNTIME_DIR="+filepath.Join(t.testDir, "runtimedir"),
	)
	for name, port := range t.ports {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TEST_PORT_%s=%d", name, port))
	}
	cmd.Env = append(cmd.Env, t.env...)

	return cmd
}

func (t *T) MustRunCLIGroup(args ...[]string) {
	t.ensureCanRun()

	wg := sync.WaitGroup{}
	for _, arg := range args {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := t.RunCLI(arg...)
			if err != nil {
				t.Printf("maddy %v: %v", arg, err)
				t.Fail()
			}
		}()
	}
	wg.Wait()
}

func (t *T) MustRunCLI(args ...string) string {
	s, err := t.RunCLI(args...)
	if err != nil {
		t.Fatalf("maddy %v: %v", args, err)
	}
	return s
}

func (t *T) RunCLI(args ...string) (string, error) {
	t.ensureCanRun()
	cmd := t.buildCmd(args...)

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	t.Log("launching maddy", cmd.Args)
	if err := cmd.Run(); err != nil {
		t.Log("Stderr:", stderr.String())
		t.Fatal("Test configuration failed:", err)
	}

	t.Log("Stderr:", stderr.String())

	return stdout.String(), nil
}

// Run completes the configuration of test environment and starts the test server.
//
// T.Close should be called by the end of test to release any resources and
// shutdown the server.
//
// The parameter waitListeners specifies the amount of listeners the server is
// supposed to configure. Run() will block before all of them are up.
func (t *T) Run(waitListeners int) {
	t.ensureCanRun()
	cmd := t.buildCmd("run")

	// Capture maddy log and redirect it.
	logOut, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal("Test configuration failed:", err)
	}

	t.Log("launching maddy", cmd.Args)
	if err := cmd.Start(); err != nil {
		t.Fatal("Test configuration failed:", err)
	}

	// Log scanning goroutine checks for the "listening" messages and sends 'true'
	// on the channel each time.
	listeningMsg := make(chan bool)

	go func() {
		defer logOut.Close()
		defer close(listeningMsg)
		scnr := bufio.NewScanner(logOut)
		for scnr.Scan() {
			line := scnr.Text()

			if strings.Contains(line, "listening on") {
				listeningMsg <- true
				line += " (test runner>listener wait trigger<)"
			}

			t.Log("maddy:", line)
		}
		if err := scnr.Err(); err != nil {
			t.Log("stderr I/O error:", err)
		}
	}()

	for i := 0; i < waitListeners; i++ {
		if !<-listeningMsg {
			t.Fatal("Log ended before all expected listeners are up. Start-up error?")
		}
	}

	t.servProc = cmd

	t.Cleanup(t.killServer)
}

func (t *T) StateDir() string {
	return filepath.Join(t.testDir, "statedir")
}

func (t *T) RuntimeDir() string {
	return filepath.Join(t.testDir, "statedir")
}

func (t *T) killServer() {
	if err := t.servProc.Process.Signal(os.Interrupt); err != nil {
		t.Log("Unable to kill the server process:", err)
		os.RemoveAll(t.testDir)
		return // Return, as now it is pointless to wait for it.
	}

	go func() {
		time.Sleep(5 * time.Second)
		if t.servProc != nil {
			t.Log("Killing possibly hung server process")
			t.servProc.Process.Kill() //nolint:errcheck
		}
	}()

	if err := t.servProc.Wait(); err != nil {
		t.Error("The server did not stop cleanly, deadlock?")
	}

	t.servProc = nil

	if err := os.RemoveAll(t.testDir); err != nil {
		t.Log("Failed to remove test directory:", err)
	}
	t.testDir = ""
}

func (t *T) Close() {
	t.Log("close is no-op")
}

// Printf implements Logger interfaces used by some libraries.
func (t *T) Printf(f string, a ...interface{}) {
	t.Logf(f, a...)
}

// Conn6 connects to the server listener at the specified named port using IPv6 loopback.
func (t *T) Conn6(portName string) Conn {
	port := t.ports[portName]
	if port == 0 {
		panic("tests: connection for the unused port name is requested")
	}

	conn, err := net.Dial("tcp6", "[::1]:"+strconv.Itoa(int(port)))
	if err != nil {
		t.Fatal("Could not connect, is server listening?", err)
	}

	return Conn{
		T:            t,
		WriteTimeout: 1 * time.Second,
		ReadTimeout:  15 * time.Second,
		Conn:         conn,
		Scanner:      bufio.NewScanner(conn),
	}
}

// Conn4 connects to the server listener at the specified named port using one
// of 127.0.0.0/8 addresses as a source.
func (t *T) Conn4(sourceIP, portName string) Conn {
	port := t.ports[portName]
	if port == 0 {
		panic("tests: connection for the unused port name is requested")
	}

	localIP := net.ParseIP(sourceIP)
	if localIP == nil {
		panic("tests: invalid localIP argument")
	}
	if localIP.To4() == nil {
		panic("tests: only IPv4 addresses are allowed")
	}

	conn, err := net.DialTCP("tcp4", &net.TCPAddr{
		IP:   localIP,
		Port: 0,
	}, &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: int(port),
	})
	if err != nil {
		t.Fatal("Could not connect, is server listening?", err)
	}

	return Conn{
		T:            t,
		WriteTimeout: 1 * time.Second,
		ReadTimeout:  15 * time.Second,
		Conn:         conn,
		Scanner:      bufio.NewScanner(conn),
	}
}

var (
	DefaultSourceIP    = net.IPv4(127, 109, 97, 100)
	DefaultSourceIPRev = "100.97.109.127"
)

func (t *T) ConnUnnamed(port uint16) Conn {
	conn, err := net.DialTCP("tcp4", &net.TCPAddr{
		IP:   DefaultSourceIP,
		Port: 0,
	}, &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: int(port),
	})
	if err != nil {
		t.Fatal("Could not connect, is server listening?", err)
	}

	return Conn{
		T:            t,
		WriteTimeout: 1 * time.Second,
		ReadTimeout:  15 * time.Second,
		Conn:         conn,
		Scanner:      bufio.NewScanner(conn),
	}
}

func (t *T) Conn(portName string) Conn {
	port := t.ports[portName]
	if port == 0 {
		panic("tests: connection for the unused port name is requested")
	}

	return t.ConnUnnamed(port)
}

func (t *T) Subtest(name string, f func(t *T)) {
	t.T.Run(name, func(subTT *testing.T) {
		subT := *t
		subT.T = subTT
		f(&subT)
	})
}

func init() {
	flag.StringVar(&TestBinary, "integration.executable", "./maddy", "executable to test")
	flag.StringVar(&CoverageOut, "integration.coverprofile", "", "write coverage stats to file (requires special maddy executable)")
	flag.BoolVar(&DebugLog, "integration.debug", false, "pass -debug to maddy executable")
}
