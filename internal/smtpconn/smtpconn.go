// The package smtpconn contains the code shared between target.smtp and
// remote modules.
//
// It implements the wrapper over the SMTP connection (go-smtp.Client) object
// with the following features added:
// - Logging of certain errors (e.g. QUIT command errors)
// - Wrapping of returned errors using the exterrors package.
// - SMTPUTF8/IDNA support.
// - TLS support mode (don't use, attempt, require).
package smtpconn

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"runtime/trace"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
)

// The C object represents the SMTP connection and is a wrapper around
// go-smtp.Client with additional maddy-specific logic.
//
// Currently, the C object represents one session and cannot be reused.
type C struct {
	// Dialer to use to estabilish new network connections. Set to net.Dialer
	// DialContext by New.
	Dialer func(ctx context.Context, network, addr string) (net.Conn, error)

	// Hostname to sent in the EHLO/HELO command. Set to
	// 'localhost.localdomain' by New. Expected to be encoded in ACE form.
	Hostname string

	// tls.Config to use. Can be nil if no special changes are required.
	TLSConfig *tls.Config

	// Logger to use for debug log and certain errors.
	Log log.Logger

	// Include the remote server address in SMTP status messages in the form
	// "ADDRESS said: ..."
	AddrInSMTPMsg bool

	serverName string
	cl         *smtp.Client
	rcpts      []string
}

// New creates the new instance of the C object, populating the required fields
// with resonable default values.
func New() *C {
	return &C{
		Dialer:    (&net.Dialer{}).DialContext,
		TLSConfig: &tls.Config{},
		Hostname:  "localhost.localdomain",
	}
}

func (c *C) wrapClientErr(err error, serverName string) error {
	if err == nil {
		return nil
	}

	switch err := err.(type) {
	case TLSError:
		return err
	case *exterrors.SMTPError:
		return err
	case *smtp.SMTPError:
		msg := err.Message
		if c.AddrInSMTPMsg {
			msg = serverName + " said: " + err.Message
		}

		if err.Code == 552 {
			err.Code = 452
			err.EnhancedCode[0] = 4
			c.Log.Msg("SMTP code 552 rewritten to 452 per RFC 5321 Section 4.5.3.1.10")
		}

		return &exterrors.SMTPError{
			Code:         err.Code,
			EnhancedCode: exterrors.EnhancedCode(err.EnhancedCode),
			Message:      msg,
			Misc: map[string]interface{}{
				"remote_server": serverName,
			},
			Err: err,
		}
	case *net.OpError:
		if _, ok := err.Err.(*net.DNSError); ok {
			reason, misc := exterrors.UnwrapDNSErr(err)
			misc["remote_server"] = err.Addr
			misc["io_op"] = err.Op
			return &exterrors.SMTPError{
				Code:         exterrors.SMTPCode(err, 450, 550),
				EnhancedCode: exterrors.SMTPEnchCode(err, exterrors.EnhancedCode{0, 4, 4}),
				Message:      "DNS error",
				Err:          err,
				Reason:       reason,
				Misc:         misc,
			}
		}
		return &exterrors.SMTPError{
			Code:         450,
			EnhancedCode: exterrors.EnhancedCode{4, 4, 2},
			Message:      "Network I/O error",
			Err:          err,
			Misc: map[string]interface{}{
				"remote_addr": err.Addr,
				"io_op":       err.Op,
			},
		}
	default:
		return exterrors.WithFields(err, map[string]interface{}{
			"remote_server": serverName,
		})
	}
}

// Connect actually estabilishes the network connection with the remote host,
// executes HELO/EHLO and optionally STARTTLS command.
func (c *C) Connect(ctx context.Context, endp config.Endpoint, starttls bool, tlsConfig *tls.Config) (didTLS bool, err error) {
	didTLS, cl, err := c.attemptConnect(ctx, false, endp, starttls, tlsConfig)
	if err != nil {
		return false, c.wrapClientErr(err, endp.Host)
	}

	c.serverName = endp.Host
	c.cl = cl
	return didTLS, nil
}

// ConnectLMTP estabilishes the network connection with the remote host and
// sends LHLO command, negotiating LMTP use.
func (c *C) ConnectLMTP(ctx context.Context, endp config.Endpoint, starttls bool, tlsConfig *tls.Config) (didTLS bool, err error) {
	didTLS, cl, err := c.attemptConnect(ctx, true, endp, starttls, tlsConfig)
	if err != nil {
		return false, c.wrapClientErr(err, endp.Host)
	}

	c.serverName = endp.Host
	c.cl = cl
	return didTLS, nil
}

// TLSError is returned by Connect to indicate the error during STARTTLS
// command execution.
//
// If the endpoint uses Implicit TLS, TLS errors are threated as connection
// errors and thus are not returned as TLSError.
type TLSError struct {
	Err error
}

func (err TLSError) Error() string {
	return "smtpconn: " + err.Err.Error()
}

func (err TLSError) Unwrap() error {
	return err.Err
}

func (c *C) attemptConnect(ctx context.Context, lmtp bool, endp config.Endpoint, starttls bool, tlsConfig *tls.Config) (didTLS bool, cl *smtp.Client, err error) {
	var conn net.Conn
	conn, err = c.Dialer(ctx, endp.Network(), endp.Address())
	if err != nil {
		return false, nil, err
	}

	if endp.IsTLS() {
		cfg := tlsConfig.Clone()
		cfg.ServerName = endp.Host
		conn = tls.Client(conn, cfg)
	}

	if lmtp {
		cl, err = smtp.NewClientLMTP(conn, endp.Host)
	} else {
		cl, err = smtp.NewClient(conn, endp.Host)
	}
	if err != nil {
		conn.Close()
		return false, nil, err
	}

	// i18n: hostname is already expected to be in A-labels form.
	if err := cl.Hello(c.Hostname); err != nil {
		cl.Close()
		return false, nil, err
	}

	if endp.IsTLS() || !starttls {
		return endp.IsTLS(), cl, nil
	}

	if ok, _ := cl.Extension("STARTTLS"); !ok {
		return false, cl, nil
	}

	cfg := tlsConfig.Clone()
	cfg.ServerName = endp.Host
	if err := cl.StartTLS(cfg); err != nil {
		// After the handshake failure, the connection may be in a bad state.
		// We attempt to send the proper QUIT command though, in case the error happened
		// *after* the handshake (e.g. PKI verification fail), we don't log the error in
		// this case though.
		if err := cl.Quit(); err != nil {
			cl.Close()
		}

		return false, nil, TLSError{err}
	}

	return true, cl, nil
}

// Mail sends the MAIL FROM command to the remote server.
//
// SIZE and REQUIRETLS options are forwarded to the remote server as-is.
// SMTPUTF8 is forwarded if supported by the remote server, if it is not
// supported - attempt will be done to convert addresses to the ASCII form, if
// this is not possible, the corresponding method (Mail or Rcpt) will fail.
func (c *C) Mail(ctx context.Context, from string, opts smtp.MailOptions) error {
	defer trace.StartRegion(ctx, "smtpconn/MAIL FROM").End()

	outOpts := smtp.MailOptions{
		// Future extensions may add additional fields that should not be
		// copied blindly. So we copy only fields we know should be handled
		// this way.

		Size:       opts.Size,
		RequireTLS: opts.RequireTLS,
	}

	// INTERNATIONALIZATION: Use SMTPUTF8 is possible, attempt to convert addresses otherwise.

	// There is no way we can accept a message with non-ASCII addresses without SMTPUTF8
	// this is enforced by endpoint/smtp.
	if opts.UTF8 {
		if ok, _ := c.cl.Extension("SMTPUTF8"); ok {
			outOpts.UTF8 = true
		} else {
			var err error
			from, err = address.ToASCII(from)
			if err != nil {
				return &exterrors.SMTPError{
					Code:         550,
					EnhancedCode: exterrors.EnhancedCode{5, 6, 7},
					Message:      "SMTPUTF8 is unsupported, cannot convert sender address",
					Misc: map[string]interface{}{
						"remote_server": c.serverName,
					},
					Err: err,
				}
			}
		}
	}

	if err := c.cl.Mail(from, &outOpts); err != nil {
		return c.wrapClientErr(err, c.serverName)
	}

	c.Log.DebugMsg("connected", "remote_server", c.serverName)
	return nil
}

// Rcpts returns the list of recipients that were accepted by the remote server.
func (c *C) Rcpts() []string {
	return c.rcpts
}

func (c *C) ServerName() string {
	return c.serverName
}

func (c *C) Client() *smtp.Client {
	return c.cl
}

// Rcpt sends the RCPT TO command to the remote server.
//
// If the address is non-ASCII and cannot be converted to ASCII and the remote
// server does not support SMTPUTF8, error will be returned.
func (c *C) Rcpt(ctx context.Context, to string) error {
	defer trace.StartRegion(ctx, "smtpconn/RCPT TO").End()

	// If necessary, the extension flag is enabled in Start.
	if ok, _ := c.cl.Extension("SMTPUTF8"); !address.IsASCII(to) && !ok {
		var err error
		to, err = address.ToASCII(to)
		if err != nil {
			return &exterrors.SMTPError{
				Code:         553,
				EnhancedCode: exterrors.EnhancedCode{5, 6, 7},
				Message:      "SMTPUTF8 is unsupported, cannot convert recipient address",
				Misc: map[string]interface{}{
					"remote_server": c.serverName,
				},
				Err: err,
			}
		}
	}

	if err := c.cl.Rcpt(to); err != nil {
		return c.wrapClientErr(err, c.serverName)
	}

	c.rcpts = append(c.rcpts, to)

	return nil
}

// Data sends the DATA command to the remote server and then sends the message header
// and body.
//
// If the Data command fails, the connection may be in a unclean state (e.g. in
// the middle of message data stream). It is not safe to continue using it.
func (c *C) Data(ctx context.Context, hdr textproto.Header, body io.Reader) error {
	defer trace.StartRegion(ctx, "smtpconn/DATA").End()

	wc, err := c.cl.Data()
	if err != nil {
		return c.wrapClientErr(err, c.serverName)
	}

	if err := textproto.WriteHeader(wc, hdr); err != nil {
		return c.wrapClientErr(err, c.serverName)
	}

	if _, err := io.Copy(wc, body); err != nil {
		return c.wrapClientErr(err, c.serverName)
	}

	if err := wc.Close(); err != nil {
		return c.wrapClientErr(err, c.serverName)
	}

	return nil
}

func (c *C) LMTPData(ctx context.Context, hdr textproto.Header, body io.Reader, statusCb func(string, *smtp.SMTPError)) error {
	defer trace.StartRegion(ctx, "smtpconn/LMTPDATA").End()

	wc, err := c.cl.LMTPData(statusCb)
	if err != nil {
		return c.wrapClientErr(err, c.serverName)
	}

	if err := textproto.WriteHeader(wc, hdr); err != nil {
		return c.wrapClientErr(err, c.serverName)
	}

	if _, err := io.Copy(wc, body); err != nil {
		return c.wrapClientErr(err, c.serverName)
	}

	if err := wc.Close(); err != nil {
		return c.wrapClientErr(err, c.serverName)
	}

	return nil
}

func (c *C) Noop() error {
	if c.cl == nil {
		return errors.New("smtpconn: not connected")
	}

	return c.cl.Noop()
}

// Close sends the QUIT command, if it fail - it directly closes the
// connection.
func (c *C) Close() error {
	if err := c.cl.Quit(); err != nil {
		c.Log.Error("QUIT error", c.wrapClientErr(err, c.serverName))
		return c.cl.Close()
	}

	c.cl = nil
	c.serverName = ""

	return nil
}

// DirectClose closes the underlying connection without sending the QUIT
// command.
func (c *C) DirectClose() error {
	c.cl.Close()
	c.cl = nil
	c.serverName = ""
	return nil
}
