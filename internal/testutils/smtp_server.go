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
	Conn     *smtp.Conn
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

func (be *SMTPBackend) NewSession(conn *smtp.Conn) (smtp.Session, error) {
	be.SessionCounter++
	if be.SourceEndpoints == nil {
		be.SourceEndpoints = make(map[string]struct{})
	}
	be.SourceEndpoints[conn.Conn().RemoteAddr().String()] = struct{}{}
	return &session{
		backend: be,
		conn:    conn,
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
	conn     *smtp.Conn
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
	s.msg.Conn = s.conn
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
	s.msg.Conn = s.conn
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
MIIFYDCCA0igAwIBAgIBATANBgkqhkiG9w0BAQsFADAuMRAwDgYDVQQKEwdBY21l
IENvMRowGAYDVQQDDBEqLmV4YW1wbGUuaW52YWxpZDAeFw0yMzA2MTkyMTA3MjNa
Fw0yOTExMTgxNzEzNDVaMC4xEDAOBgNVBAoTB0FjbWUgQ28xGjAYBgNVBAMMESou
ZXhhbXBsZS5pbnZhbGlkMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA
pDHOPeO79b47C3VUhA7ERTT6O1LMfGqcuj+Y14YpiO1J7rui3AQ3O34WgHQrXDt0
Zsypf+1VJTJeNxMvYkQ/++LyP3PrdA3ksomAY4nKOSAQhrXpvagPps13+AUkE23J
47zU1VFtnHI5mj6/StgjWTKJes+PPrEKgm6XqqeOfzanzQTodXAmc6qwLF/kzISh
Fe+ZFPwoK2hduJRqRWAwlpoGPkubJCNrCFAJXe8kYl5JUtDi9ju8YfqgC+pqyfpK
cEQS7YSghsMMJsI6enXQkrfjq5spClqz/Jjc12Krytne5lO8MhO7gkERsU6W5jxH
O86ytU5Oi4peSL3s/G2iD7BqgYq6DVVhH/KgXqCYNdehaeHu/ow0eC5fNx1QxE5g
n3MkN6uwOztYeKmMeURUimeHkI/55BOYhfnribKp3YVEPigsok2FzOZDlmK6HlTR
5ptOoDyxJBNrhYiofSbfQ6Fmo+uze9wCBTSjNd1kqJe2IpCKGl/Q5cOSGAoVfURk
UWP2yxkICIPyhKUPY0koDmlKZDviWz/d8evjVUWFTpaWzyhy7hTawE833dX5onpS
hMQtg4AT542wGRsKlWAy/Uwk37ieZOL55MBGucSUgLODQdaYAR7cZ3/zQ3cMZMUM
Vapdrj1RCZpFZwWU+D0B9DY4c70L6ZE+c5cT1aA/mb0CAwEAAaOBiDCBhTAOBgNV
HQ8BAf8EBAMCAqQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB
/zAdBgNVHQ4EFgQUQT6qal8RpaElQizilqD80eQs18wwLgYDVR0RBCcwJYIRKi5l
eGFtcGxlLmludmFsaWSHBH8AAAGHBH8AAAKHBH8AAAMwDQYJKoZIhvcNAQELBQAD
ggIBAF+h+t3Qj71cWMPnCQXvBD4iyBbvDGgm7rzzmaSf8rfcutxGboeJ+Ob196rN
O9Q7yQQg0fJ4TNiCU4wNqqp+6wtU4SLtcdniw6rVacWcgek0BWTaT2+vv0ydD/xt
eM5fHl2dFW3M92ON7liwKdWgvEkUjHGc1kG6CFYTvAiHFHLsR+MS8CyXp+g0tH8t
w2OHtgw4ZrUZQOH3TftEYODTm+Nghkm+GizKFKAMqR16mdcOen+Mr18W7KNwRy/P
/As8dY4eHJd3OtwoTuCEepdkJQH/Xj0G/Cqk036tlmw6pR8SIk8dJE602Xt9vp1E
1kALRjkHE6ndyFnAn6xo1mqBSyZqn4Btv5xtIeVXiZwJ9b/Z09RXaC/TrBV5n5a6
tJmJGOLyIJlVUYqpHufVqHM1Ld60F3e2WwWmmGu5IStqqnuLQy6H5pKQPh/by46t
DU/XJPjbKaw3jWO/Rm5DEcYDphn/qOUOA7Ff7isNxXUdv+gMMz05pEwbhve5DJ42
RYaaoEDxBNKcQvQFoLz+/RQL1gLvMQZkTdSdCVzExLzM5n7lmPIGJ9QGg5fBAjKI
cezhCg2yEy+oOq4SG26WfuQ461dNy+Lu2qlH02hYssnNAvJsGqu8VKtIUxT9tqmv
RuGxIC+e3FIYU8xKZt6HsZrDNSVjws1cWHo+NnhLzlZvkHB8
-----END CERTIFICATE-----`

const testServerKey = `-----BEGIN PRIVATE KEY-----
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
tVROBLY0Fyjn59wNO+QzDnDBSy7k830=
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
