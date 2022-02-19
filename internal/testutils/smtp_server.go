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
	"github.com/foxcpp/maddy/framework/exterrors"
)

type SMTPMessage struct {
	From     string
	Opts     smtp.MailOptions
	To       []string
	Data     []byte
	State    *smtp.ConnectionState
	AuthUser string
	AuthPass string
}

type SMTPBackend struct {
	Messages        []*SMTPMessage
	MailFromCounter int
	SessionCounter  int
	SourceEndpoints map[string]struct{}

	AuthErr     error
	MailErr     error
	RcptErr     map[string]error
	DataErr     error
	LMTPDataErr []error
}

func (be *SMTPBackend) NewSession(state smtp.ConnectionState, _ string) (smtp.Session, error) {
	be.SessionCounter++
	if be.SourceEndpoints == nil {
		be.SourceEndpoints = make(map[string]struct{})
	}
	be.SourceEndpoints[state.RemoteAddr.String()] = struct{}{}
	return &session{
		backend: be,
		state:   &state,
	}, nil
}

func (be *SMTPBackend) CheckMsg(t *testing.T, indx int, from string, rcptTo []string) {
	t.Helper()

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
		t.Errorf("Wrong DATA payload: %v (%v)", string(msg.Data), msg.Data)
	}
}

type session struct {
	backend  *SMTPBackend
	user     string
	password string
	state    *smtp.ConnectionState
	msg      *SMTPMessage
}

func (s *session) Reset() {
	s.msg = &SMTPMessage{}
}

func (s *session) Logout() error {
	return nil
}

func (s *session) AuthPlain(username, password string) error {
	if s.backend.AuthErr != nil {
		return s.backend.AuthErr
	}
	s.user = username
	s.password = password
	return nil
}

func (s *session) Mail(from string, opts *smtp.MailOptions) error {
	s.backend.MailFromCounter++

	if s.backend.MailErr != nil {
		return s.backend.MailErr
	}

	s.Reset()
	s.msg.From = from
	s.msg.Opts = *opts
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
	s.msg.State = s.state
	s.msg.AuthUser = s.user
	s.msg.AuthPass = s.password
	s.backend.Messages = append(s.backend.Messages, s.msg)
	return nil
}

func (s *session) LMTPData(r io.Reader, status smtp.StatusCollector) error {
	if s.backend.DataErr != nil {
		return s.backend.DataErr
	}

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	s.msg.Data = b
	s.msg.State = s.state
	s.msg.AuthUser = s.user
	s.msg.AuthPass = s.password
	s.backend.Messages = append(s.backend.Messages, s.msg)

	for i, rcpt := range s.msg.To {
		status.SetStatus(rcpt, s.backend.LMTPDataErr[i])
	}

	return nil
}

type SMTPServerConfigureFunc func(*smtp.Server)

var AuthDisabled = func(s *smtp.Server) {
	s.AuthDisabled = true
}

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

// RSA 1024, valid for *.example.invalid, 127.0.0.1, 127.0.0.2,, 127.0.0.3
// until Nov 18 17:13:45 2029 GMT.
const testServerCert = `-----BEGIN CERTIFICATE-----
MIICDzCCAXigAwIBAgIRAJ1x+qCW7L+Hs6sRU8BHmWkwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xOTExMTgxNzEzNDVaFw0yOTExMTUxNzEz
NDVaMBIxEDAOBgNVBAoTB0FjbWUgQ28wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJ
AoGBAPINKMyuu3AvzndLDS2/BroA+DRUcAhWPBxMxG1b1BkkHisAZWteKajKmwdO
O13N8HHBRPPOD56AAPLZGNxYLHn6nel7AiH8k40/xC5tDOthqA82+00fwJHDFCnW
oDLOLcO17HulPvfCSWfefc+uee4kajPa+47hutqZH2bGMTXhAgMBAAGjZTBjMA4G
A1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAA
MC4GA1UdEQQnMCWCESouZXhhbXBsZS5pbnZhbGlkhwR/AAABhwR/AAAChwR/AAAD
MA0GCSqGSIb3DQEBCwUAA4GBAGRn3C2NbwR4cyQmTRm5jcaqi1kAYyEu6U8Q9PJW
Q15BXMKUTx2lw//QScK9MH2JpKxDuzWDSvaxZMnTxgri2uiplqpe8ydsWj6Wl0q9
2XMGJ9LIxTZk5+cyZP2uOolvmSP/q8VFTyk9Udl6KUZPQyoiiDq4rBFUIxUyb+bX
pHkR
-----END CERTIFICATE-----`

const testServerKey = `-----BEGIN PRIVATE KEY-----
MIICeAIBADANBgkqhkiG9w0BAQEFAASCAmIwggJeAgEAAoGBAPINKMyuu3AvzndL
DS2/BroA+DRUcAhWPBxMxG1b1BkkHisAZWteKajKmwdOO13N8HHBRPPOD56AAPLZ
GNxYLHn6nel7AiH8k40/xC5tDOthqA82+00fwJHDFCnWoDLOLcO17HulPvfCSWfe
fc+uee4kajPa+47hutqZH2bGMTXhAgMBAAECgYEAgPjSDH3uEdDnSlkLJJzskJ+D
oR58s3R/gvTElSCg2uSLzo3ffF4oBHAwOqxMpabdvz8j5mSdne7Gkp9qx72TtEG2
wt6uX1tZhm2UTAkInH8IQDthj98P8vAWQsS6HHEIMErsrW2CyUrAt/+o1BRg/hWW
zixA3CLTthhZTJkaUCECQQD5EM16UcTAKfhr3IZppgq+ZsAOMkeCl3XVV9gHo32i
DL6UFAb27BAYyjfcZB1fPou4RszX0Ryu9yU0P5qm6N47AkEA+MpdAPkaPziY0ok4
e9Tcee6P0mIR+/AHk9GliVX2P74DDoOHyMXOSRBwdb+z2tYjrdjkNEL1Txe+sHny
k/EukwJBAOBqlmqPwNNRPeiaRHZvSSD0XjqsbSirJl48D4gadPoNt66fOQNGAt8D
Xj/z6U9HgQdiq/IOFmVEhT5FzSh1jL8CQQD3Myth8iGQO84tM0c6U3CWfuHMqsEv
0XnV+HNAmHdLMqOa4joi1dh4ZKs5dDdi828UJ/PnsbhI1FEWzLSpJvWdAkAkVWqf
AC/TvWvEZLA6Z5CllyNzZJ7XvtIaNOosxHDolyZ1HMWMlfEb2K2ZXWLy5foKPeoY
Xi3olS9rB0J+Rvjz
-----END PRIVATE KEY-----`

// SMTPServerSTARTTLS starts a server listening on the specified addr with the
// STARTTLS extension supported.
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
			return time.Date(2019, time.November, 18, 17, 59, 41, 0, time.UTC)
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

// SMTPServerTLS starts a SMTP server listening on the specified addr with
// Implicit TLS.
func SMTPServerTLS(t *testing.T, addr string, fn ...SMTPServerConfigureFunc) (*tls.Config, *SMTPBackend, *smtp.Server) {
	t.Helper()

	cert, err := tls.X509KeyPair([]byte(testServerCert), []byte(testServerKey))
	if err != nil {
		panic(err)
	}

	l, err := tls.Listen("tcp", addr, &tls.Config{
		Certificates: []tls.Certificate{cert},
	})
	if err != nil {
		t.Fatal(err)
	}

	be := new(SMTPBackend)
	s := smtp.NewServer(be)
	s.Domain = "localhost"
	for _, f := range fn {
		f(s)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(testServerCert))

	clientCfg := &tls.Config{
		ServerName: "127.0.0.1",
		Time: func() time.Time {
			return time.Date(2019, time.November, 18, 17, 59, 41, 0, time.UTC)
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
	t.Helper()

	// Connection closure is handled asynchronously, so before failing
	// wait a bit for handleQuit in go-smtp to do its work.
	for i := 0; i < 10; i++ {
		found := false
		srv.ForEachConn(func(_ *smtp.Conn) {
			found = true
		})
		if !found {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Error("Non-closed connections present after test completion")
}

func WaitForConnsClose(t *testing.T, srv *smtp.Server) {
	t.Helper()
	CheckSMTPConnLeak(t, srv)
}

// FailOnConn fails the test if attempt is made to connect the
// specified endpoint.
func FailOnConn(t *testing.T, addr string) net.Listener {
	t.Helper()

	tarpit, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		t.Helper()

		_, err := tarpit.Accept()
		if err == nil {
			t.Error("No connection expected")
		}
	}()
	return tarpit
}

func CheckSMTPErr(t *testing.T, err error, code int, enchCode exterrors.EnhancedCode, msg string) {
	t.Helper()

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
