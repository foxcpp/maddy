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
	"io"
	"net"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/target"
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

	u.targetsArg = append(u.targetsArg, targetsArg...)
	for _, tgt := range u.targetsArg {
		endp, err := config.ParseEndpoint(tgt)
		if err != nil {
			return err
		}

		u.endpoints = append(u.endpoints, endp)
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
	if err := d.client.Mail(mailFrom); err != nil {
		d.client.Quit()
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
		return exterrors.WithFields(lastErr, map[string]interface{}{
			"target":            "smtp_downstream",
			"downstream_server": lastDownstream,
		})
	}

	if d.u.saslFactory != nil {
		saslClient, err := d.u.saslFactory(d.msgMeta.AuthUser, d.msgMeta.AuthPassword)
		if err != nil {
			d.client.Quit()
			return err
		}

		if err := d.client.Auth(saslClient); err != nil {
			d.client.Quit()
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
			Misc: map[string]interface{}{
				"downstream_server": d.downstreamAddr,
			},
		}
	default:
		return exterrors.WithFields(err, map[string]interface{}{
			"downstream_server": d.downstreamAddr,
			"target":            "smtp_downstream",
		})
	}
}

func (d *delivery) AddRcpt(rcptTo string) error {
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
	d.body.Close()
	return d.client.Close()
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

	return d.wrapClientErr(wc.Close())
}

func init() {
	module.Register("smtp_downstream", NewDownstream)
}
