// Package smtp_downstream provides smtp_downstream module that implements
// transparent forwarding or messages to configured list of SMTP servers.
//
// Like remote module, this implementation doesn't handle atomic
// delivery properly since it is impossible to do with SMTP protocol
//
// Interfaces implemented:
// - module.DeliveryTarget
package smtp_downstream

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/address"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/target"
	"golang.org/x/net/idna"
)

type Downstream struct {
	instName   string
	targetsArg []string

	requireTLS      bool
	attemptStartTLS bool
	hostname        string
	endpoints       []config.Endpoint
	saslFactory     saslClientFactory
	tlsConfig       tls.Config

	log log.Logger
}

func NewDownstream(_, instName string, _, inlineArgs []string) (module.Module, error) {
	return &Downstream{
		instName:   instName,
		targetsArg: inlineArgs,
		log:        log.Logger{Name: "smtp_downstream"},
	}, nil
}

func (u *Downstream) Init(cfg *config.Map) error {
	var targetsArg []string
	cfg.Bool("debug", true, false, &u.log.Debug)
	cfg.Bool("require_tls", false, false, &u.requireTLS)
	cfg.Bool("attempt_starttls", false, true, &u.attemptStartTLS)
	cfg.String("hostname", true, true, "", &u.hostname)
	cfg.StringList("targets", false, false, nil, &targetsArg)
	// TODO: Support basic load-balancing.
	// - ordered
	//   Current behavior
	// - roundrobin
	//   Pick next server from list each time.
	cfg.Custom("auth", false, false, func() (interface{}, error) {
		return nil, nil
	}, saslAuthDirective, &u.saslFactory)
	cfg.Custom("tls_client", true, false, func() (interface{}, error) {
		return tls.Config{}, nil
	}, config.TLSClientBlock, &u.tlsConfig)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	// INTERNATIONALIZATION: See RFC 6531 Section 3.7.1.
	var err error
	u.hostname, err = idna.ToASCII(u.hostname)
	if err != nil {
		return fmt.Errorf("smtp_downstream: cannot represent the hostname as an A-label name: %w", err)
	}

	u.targetsArg = append(u.targetsArg, targetsArg...)
	for _, tgt := range u.targetsArg {
		endp, err := config.ParseEndpoint(tgt)
		if err != nil {
			return err
		}

		u.endpoints = append(u.endpoints, endp)
	}

	if len(u.endpoints) == 0 {
		return fmt.Errorf("smtp_downstream: at least one target endpoint is required")
	}

	return nil
}

func (u *Downstream) Name() string {
	return "smtp_downstream"
}

func (u *Downstream) InstanceName() string {
	return u.instName
}

type delivery struct {
	u   *Downstream
	log log.Logger

	msgMeta  *module.MsgMetadata
	mailFrom string
	body     io.ReadCloser
	hdr      textproto.Header

	downstreamAddr string
	client         *smtp.Client
}

func (u *Downstream) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	d := &delivery{
		u:        u,
		log:      target.DeliveryLogger(u.log, msgMeta),
		msgMeta:  msgMeta,
		mailFrom: mailFrom,
	}
	if err := d.connect(); err != nil {
		return nil, err
	}

	// INTERNATIONALIZATION: Use SMTPUTF8 is possible, attempt to convert addresses otherwise.
	opts := &smtp.MailOptions{
		// Future extensions may add additional fields that should not be
		// copied blindly. So we copy only fields we know should be handled
		// this way.

		Size:       msgMeta.SMTPOpts.Size,
		RequireTLS: msgMeta.SMTPOpts.RequireTLS,
	}

	// There is no way we can accept a message with non-ASCII addresses without SMTPUTF8
	// this is enforced by endpoint/smtp.
	if msgMeta.SMTPOpts.UTF8 {
		if ok, _ := d.client.Extension("SMTPUTF8"); ok {
			opts.UTF8 = true
		} else {
			var err error
			mailFrom, err = address.ToASCII(mailFrom)
			if err != nil {
				if err := d.client.Quit(); err != nil {
					d.log.Error("QUIT error", d.wrapClientErr(err))
				}
				return nil, &exterrors.SMTPError{
					Code:         550,
					EnhancedCode: exterrors.EnhancedCode{5, 6, 7},
					Message:      "Downstream does not support SMTPUTF8, can not convert sender address",
					TargetName:   "smtp_downstream",
					Misc: map[string]interface{}{
						"downstream_server": d.downstreamAddr,
					},
					Err: err,
				}
			}
		}
	}

	if err := d.client.Mail(mailFrom, opts); err != nil {
		if err := d.client.Quit(); err != nil {
			d.log.Error("QUIT error", d.wrapClientErr(err))
		}
		return nil, d.wrapClientErr(err)
	}
	return d, nil
}

func (d *delivery) connect() error {
	// TODO: Review possibility of connection pooling here.
	var (
		lastErr        error
		lastDownstream string
	)
	for _, endp := range d.u.endpoints {
		addr := net.JoinHostPort(endp.Host, endp.Port)
		cl, err := d.attemptConnect(endp, d.u.attemptStartTLS)
		if err == nil {
			d.log.DebugMsg("connected", "downstream_server", addr)
			lastErr = nil
			d.downstreamAddr = addr
			d.client = cl
			break
		}

		if len(d.u.endpoints) != 1 {
			d.log.Msg("connect error", err, "downstream_server", addr)
		}
		lastErr = err
		lastDownstream = addr
	}
	if lastErr != nil {
		return d.wrapClientErrAddr(lastErr, lastDownstream)
	}

	if d.u.saslFactory != nil {
		saslClient, err := d.u.saslFactory(d.msgMeta)
		if err != nil {
			if err := d.client.Quit(); err != nil {
				d.client.Close()
			}
			return err
		}

		if err := d.client.Auth(saslClient); err != nil {
			if err := d.client.Quit(); err != nil {
				d.client.Close()
			}
			return err
		}
	}

	return nil
}

func (d *delivery) attemptConnect(endp config.Endpoint, attemptStartTLS bool) (*smtp.Client, error) {
	var conn net.Conn
	conn, err := net.Dial(endp.Network(), endp.Address())
	if err != nil {
		return nil, err
	}

	if endp.IsTLS() {
		cfg := d.u.tlsConfig.Clone()
		cfg.ServerName = endp.Host
		conn = tls.Client(conn, cfg)
	}

	cl, err := smtp.NewClient(conn, endp.Host)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// INTERNATIONALIZATION: hostname is converted to A-label in Init.
	if err := cl.Hello(d.u.hostname); err != nil {
		cl.Close()
		return nil, err
	}

	if attemptStartTLS && !endp.IsTLS() {
		cfg := d.u.tlsConfig.Clone()
		cfg.ServerName = endp.Host
		if err := cl.StartTLS(cfg); err != nil {
			cl.Close()

			if d.u.requireTLS {
				return nil, err
			}

			// Re-attempt without STARTTLS. It is not possible to reuse connection
			// since it is probably in a bad state.
			return d.attemptConnect(endp, false)
		}
	}

	return cl, nil
}

func (d *delivery) wrapClientErr(err error) error {
	return d.wrapClientErrAddr(err, d.downstreamAddr)
}

func (d *delivery) wrapClientErrAddr(err error, addr string) error {
	if err == nil {
		return nil
	}
	switch err := err.(type) {
	case *smtp.SMTPError:
		return &exterrors.SMTPError{
			Code:         err.Code,
			EnhancedCode: exterrors.EnhancedCode(err.EnhancedCode),
			Message:      err.Message,
			TargetName:   "smtp_downstream",
			Err:          err,
			Misc: map[string]interface{}{
				"downstream_server": addr,
			},
		}
	case *net.DNSError:
		reason, misc := exterrors.UnwrapDNSErr(err)
		misc["downstream_server"] = addr
		return &exterrors.SMTPError{
			Code:         exterrors.SMTPCode(err, 450, 550),
			EnhancedCode: exterrors.SMTPEnchCode(err, exterrors.EnhancedCode{0, 4, 4}),
			Message:      "DNS error",
			TargetName:   "smtp_downstream",
			Err:          err,
			Reason:       reason,
			Misc:         misc,
		}
	case *net.OpError:
		return &exterrors.SMTPError{
			Code:         450,
			EnhancedCode: exterrors.EnhancedCode{4, 4, 2},
			Message:      "Network I/O error",
			TargetName:   "smtp_downstream",
			Err:          err,
			Misc: map[string]interface{}{
				"remote_addr": err.Addr,
				"io_op":       err.Op,
			},
		}
	default:
		return exterrors.WithFields(err, map[string]interface{}{
			"downstream_server": addr,
			"target":            "smtp_downstream",
		})
	}
}

func (d *delivery) AddRcpt(rcptTo string) error {
	// If necessary, the extension flag is enabled in Start.
	if ok, _ := d.client.Extension("SMTPUTF8"); !address.IsASCII(rcptTo) && !ok {
		var err error
		rcptTo, err = address.ToASCII(rcptTo)
		if err != nil {
			return &exterrors.SMTPError{
				Code:         553,
				EnhancedCode: exterrors.EnhancedCode{5, 6, 7},
				Message:      "Downstream does not support SMTPUTF8, can not convert recipient address",
				TargetName:   "smtp_downstream",
				Misc: map[string]interface{}{
					"downstream_server": d.downstreamAddr,
				},
				Err: err,
			}
		}
	}

	return d.wrapClientErr(d.client.Rcpt(rcptTo))
}

func (d *delivery) Body(header textproto.Header, body buffer.Buffer) error {
	r, err := body.Open()
	if err != nil {
		return exterrors.WithFields(err, map[string]interface{}{"target": "smtp_downstream"})
	}

	d.body = r
	d.hdr = header
	return nil
}

func (d *delivery) Abort() error {
	if d.body != nil {
		d.body.Close()
	}
	if err := d.client.Quit(); err != nil {
		d.client.Close()
	}
	return nil
}

func (d *delivery) Commit() error {
	defer d.client.Close()
	defer d.body.Close()

	wc, err := d.client.Data()
	if err != nil {
		return d.wrapClientErr(err)
	}

	if err := textproto.WriteHeader(wc, d.hdr); err != nil {
		return d.wrapClientErr(err)
	}

	if _, err := io.Copy(wc, d.body); err != nil {
		return d.wrapClientErr(err)
	}

	if err := wc.Close(); err != nil {
		return d.wrapClientErr(err)
	}

	if err := d.client.Quit(); err != nil {
		d.log.Error("QUIT error", d.wrapClientErr(err))
	}
	return nil
}

func init() {
	module.Register("smtp_downstream", NewDownstream)
}
