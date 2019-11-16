package testutils

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/exterrors"
)

// Test server runner, based on emersion/go-smtp tests.

type SMTPMessage struct {
	From string
	To   []string
	Data []byte
}

type SMTPBackend struct {
	Messages []*SMTPMessage

	MailErr error
	RcptErr map[string]error
	DataErr error
}

func (be *SMTPBackend) Login(_ *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	return &session{backend: be}, nil
}

func (be *SMTPBackend) AnonymousLogin(_ *smtp.ConnectionState) (smtp.Session, error) {
	return &session{backend: be, anonymous: true}, nil
}

func (be *SMTPBackend) CheckMsg(t *testing.T, indx int, from string, rcptTo []string) {
	if len(be.Messages) <= indx {
		t.Errorf("Expected at least %d messages in mailbox, got %d", indx+1, len(be.Messages))
		return
	}

	msg := be.Messages[indx]
	if msg.From != from {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}

	sort.Strings(msg.To)
	sort.Strings(rcptTo)

	if !reflect.DeepEqual(msg.To, rcptTo) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}
}

type session struct {
	backend   *SMTPBackend
	anonymous bool

	msg *SMTPMessage
}

func (s *session) Reset() {
	s.msg = &SMTPMessage{}
}

func (s *session) Logout() error {
	return nil
}

func (s *session) Mail(from string) error {
	if s.backend.MailErr != nil {
		return s.backend.MailErr
	}

	s.Reset()
	s.msg.From = from
	return nil
}

func (s *session) Rcpt(to string) error {
	if err := s.backend.RcptErr[to]; err != nil {
		return err
	}

	s.msg.To = append(s.msg.To, to)
	return nil
}

func (s *session) Data(r io.Reader) error {
	if s.backend.DataErr != nil {
		return s.backend.DataErr
	}

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	s.msg.Data = b
	s.backend.Messages = append(s.backend.Messages, s.msg)
	return nil
}

type SMTPServerConfigureFunc func(*smtp.Server)

var (
	AuthDisabled = func(s *smtp.Server) {
		s.AuthDisabled = true
	}
)

func SMTPServer(t *testing.T, addr string, fn ...SMTPServerConfigureFunc) (*SMTPBackend, *smtp.Server) {
	t.Helper()

	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}

	be := new(SMTPBackend)
	s := smtp.NewServer(be)
	s.Domain = "localhost"
	s.AllowInsecureAuth = true
	for _, f := range fn {
		f(s)
	}

	go func() {
		if err := s.Serve(l); err != nil {
			t.Error(err)
		}
	}()

	// Dial it once it make sure Server completes its initialization before
	// we try to use it. Notably, if test fails before connecting to the server,
	// it will call Server.Close which will call Server.listener.Close with a
	// nil Server.listener (Serve sets it to a non-nil value, so it is racy and
	// happens only sometimes).
	testConn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	testConn.Close()

	return be, s
}

// RSA 1024, valid for *.example.invalid
// until Nov 13 16:59:41 2029 GMT.
const testServerCert = `-----BEGIN CERTIFICATE-----
MIIB/TCCAWagAwIBAgIRAJPNJW5c73AaVF29mMs6UjowDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xOTExMTYxNjU5NDFaFw0yOTExMTMxNjU5
NDFaMBIxEDAOBgNVBAoTB0FjbWUgQ28wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJ
AoGBAMbIZsgWGfbSzolrL+pfG7JejKzTUpJRZ1y5mr0tqo3RIdEVz56SacfjXVUb
33u2+wbJr6GgGlt910sdwHh/exl0WpBpXC/4Wcz3eK5VC+HcaiMRnlchG7hOazFH
wsxUEcQtVXhFreotoUQrjrKB4n6qMhGif7Iy45oWJLkbNI1rAgMBAAGjUzBRMA4G
A1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAA
MBwGA1UdEQQVMBOCESouZXhhbXBsZS5pbnZhbGlkMA0GCSqGSIb3DQEBCwUAA4GB
AMMVhLupUgeewIQ1UFDtMIj3GaKFdb6WVnptLLS55ZWtUPNuJH5BzPaFHTqcHW8p
h0awjmghQoQY0ECBvbEmjlnyRL64FUTpibMvv/K6QKz1XuRWN4S9RQX3++X5I0vR
R2+SAXHyOiKIa0M9jdP+6DscciM2lk32xCcr6WhNcD2q
-----END CERTIFICATE-----`

const testServerKey = `-----BEGIN PRIVATE KEY-----
MIICdwIBADANBgkqhkiG9w0BAQEFAASCAmEwggJdAgEAAoGBAMbIZsgWGfbSzolr
L+pfG7JejKzTUpJRZ1y5mr0tqo3RIdEVz56SacfjXVUb33u2+wbJr6GgGlt910sd
wHh/exl0WpBpXC/4Wcz3eK5VC+HcaiMRnlchG7hOazFHwsxUEcQtVXhFreotoUQr
jrKB4n6qMhGif7Iy45oWJLkbNI1rAgMBAAECgYEAsuT/uupJC5zES1+vi5l0b54v
tAmqsguYnhZbcA19BIxFhsm+Q9M4Z6/y+vlOsyQF3iH8cdSIY/Zony1zXf48ZSCt
ujIWpJnpEicQkMDKkP6eCcxEDU4OfykGYW8MAcmu3DCKVV4goJZoB3wzUTGwnGBb
n/euDGe8c9fp/qssHaECQQDPy+pBoNON6O/bsET+xMzhD4jbwMlsjYr9V3f7CPFl
9FgTMw3DNg7oA6yKi4u6K44f/0z/2mxQNOy3PB7trd3nAkEA9OUz3PffZBRF2g58
DBOCddiCK/Gd8mG1R3giR8n7Dz0MIOl2BL4bp1wlvAPf56KZlnA6P09U5Uzc0amI
bdJ73QJAAhbn1R8b4XptJwVfvDwYX077rlIC9H973U5K25BcdQz+8bp6sfLSNY0L
6By9G/MiK7oyeQQmQKw3kSQen383EwJBAN6isK+mONSHCanfmS5xXh08o7rHgcwk
v+UldiTFnxSPb0NMexp8qi9QOo3fB+NRk0eM56c+u/NqGSYSdhFBVZECQH4A14fb
DDiDLYv/gUliRlSs9Ua3Ez7kmvZ710TclMgbMRhq5mqZTvC2J4OGp9wehQd5yeAk
jcHIGpj/jkvmxck=
-----END PRIVATE KEY-----`

// SMTPServerSTARTTLS starts a server listening on the specified addr with STARTTLS
// extension supported.
//
// Returned *tls.Config is for the client and is set to trust the server
// certificate.
func SMTPServerSTARTTLS(t *testing.T, addr string, fn ...SMTPServerConfigureFunc) (*tls.Config, *SMTPBackend, *smtp.Server) {
	t.Helper()

	cert, err := tls.X509KeyPair([]byte(testServerCert), []byte(testServerKey))
	if err != nil {
		panic(err)
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}

	be := new(SMTPBackend)
	s := smtp.NewServer(be)
	s.Domain = "localhost"
	s.AllowInsecureAuth = true
	s.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	for _, f := range fn {
		f(s)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(testServerCert))

	clientCfg := &tls.Config{
		ServerName: "127.0.0.1",
		Time: func() time.Time {
			return time.Date(2019, time.November, 16, 16, 59, 41, 0, time.UTC)
		},
		RootCAs: pool,
	}

	go func() {
		if err := s.Serve(l); err != nil {
			t.Error(err)
		}
	}()

	// Dial it once it make sure Server completes its initialization before
	// we try to use it. Notably, if test fails before connecting to the server,
	// it will call Server.Close which will call Server.listener.Close with a
	// nil Server.listener (Serve sets it to a non-nil value, so it is racy and
	// happens only sometimes).
	testConn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	testConn.Close()

	return clientCfg, be, s
}

func CheckSMTPConnLeak(t *testing.T, srv *smtp.Server) {
	// Connection closure is handled asynchronously, so before failing
	// wait a bit for handleQuit in go-smtp to do its work.
	for i := 0; i < 10; i++ {
		found := false
		srv.ForEachConn(func(c *smtp.Conn) {
			found = true
		})
		if !found {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("Non-closed connections present after test completion")
}

// FailOnConn fails the test if attempt is made to connect the
// specified endpoint.
func FailOnConn(t *testing.T, addr string) net.Listener {
	tarpit, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, err := tarpit.Accept()
		if err == nil {
			t.Error("No connection expected")
		}
	}()
	return tarpit
}

func CheckSMTPErr(t *testing.T, err error, code int, enchCode exterrors.EnhancedCode, msg string) {
	if err == nil {
		t.Error("Expected an error, got none")
		return
	}

	fields := exterrors.Fields(err)
	if val, _ := fields["smtp_code"].(int); val != code {
		t.Errorf("Wrong smtp_code: %v", val)
	}
	if val, _ := fields["smtp_enchcode"].(exterrors.EnhancedCode); val != enchCode {
		t.Errorf("Wrong smtp_enchcode: %v", val)
	}
	if val, _ := fields["smtp_msg"].(string); val != msg {
		t.Errorf("Wrong smtp_msg: %v", val)
	}
}
