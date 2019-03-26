package maddy

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"os"
	"strings"
	"sync"

	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

type SMTPUser struct {
	endp      *SMTPEndpoint
	anonymous bool
	username  string
	state     *smtp.ConnectionState
}

func (u SMTPUser) Send(from string, to []string, r io.Reader) error {
	ctx := module.DeliveryContext{
		From:        from,
		To:          to,
		Anonymous:   u.anonymous,
		AuthUser:    u.username,
		SrcTLSState: u.state.TLS,
		SrcHostname: u.state.Hostname,
		SrcAddr:     u.state.RemoteAddr,
		OurHostname: u.endp.serv.Domain,
		Ctx:         make(map[string]interface{}),
	}

	// Check if TLS connection state struct is poplated.
	// If it is - we are using TLS.
	if u.state.TLS.Version != 0 {
		ctx.SrcProto = "ESMTPS"
	} else {
		ctx.SrcProto = "ESMTP"
	}

	// TODO: Execute pipeline steps in parallel.
	// https://github.com/emersion/maddy/pull/17#discussion_r267573580

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return err
	}
	currentMsg := bytes.NewReader(buf.Bytes())

	for _, step := range u.endp.pipeline {
		r, cont, err := step.Pass(&ctx, currentMsg)
		if !cont {
			if err == module.ErrSilentDrop {
				return nil
			}
			if err != nil {
				u.endp.Log.Printf("%T failed: %v", step, err)
				return err
			}
		}

		if r != nil && r != io.Reader(currentMsg) {
			buf.Reset()
			if _, err := io.Copy(&buf, r); err != nil {
				u.endp.Log.Printf("failed to buffer message: %v", err)
				return err
			}
			currentMsg.Reset(buf.Bytes())
		}
		currentMsg.Seek(0, io.SeekStart)
	}

	u.endp.Log.Printf("accepted incoming message from %s (%s)", ctx.SrcHostname, ctx.SrcAddr)

	return nil
}

func (u SMTPUser) Logout() error {
	return nil
}

type SMTPEndpoint struct {
	Auth      module.AuthProvider
	serv      *smtp.Server
	name      string
	listeners []net.Listener
	pipeline  []SMTPPipelineStep

	submission bool

	listenersWg sync.WaitGroup

	debug bool
	Log   log.Logger
}

func (endp *SMTPEndpoint) Name() string {
	return "smtp"
}

func (endp *SMTPEndpoint) InstanceName() string {
	return endp.name
}

func (endp *SMTPEndpoint) Version() string {
	return VersionStr
}

func NewSMTPEndpoint(modName, instName string) (module.Module, error) {
	endp := &SMTPEndpoint{
		name:       instName,
		submission: modName == "submission",
		Log:        log.Logger{Out: log.StderrLog, Name: "smtp"},
	}
	return endp, nil
}

func (endp *SMTPEndpoint) Init(globalCfg map[string]config.Node, cfg config.Node) error {
	endp.serv = smtp.NewServer(endp)
	if err := endp.setConfig(globalCfg, cfg); err != nil {
		return err
	}

	endp.Log.Debugf("authentication provider: %s %s", endp.Auth.(module.Module).Name(), endp.Auth.(module.Module).InstanceName())
	endp.Log.Debugf("pipeline: %#v", endp.pipeline)

	addresses := make([]Address, 0, len(cfg.Args))
	for _, addr := range cfg.Args {
		saddr, err := standardizeAddress(addr)
		if err != nil {
			return fmt.Errorf("smtp: invalid address: %s", addr)
		}

		addresses = append(addresses, saddr)
	}

	if err := endp.setupListeners(addresses); err != nil {
		for _, l := range endp.listeners {
			l.Close()
		}
		return err
	}

	if endp.serv.AllowInsecureAuth {
		endp.Log.Println("authentication over unencrypted connections is allowed, this is insecure configuration and should be used only for testing!")
	}
	if endp.serv.TLSConfig == nil && !endp.serv.LMTP {
		endp.Log.Println("TLS is disabled, this is insecure configuration and should be used only for testing!")
		endp.serv.AllowInsecureAuth = true
	}

	return nil
}

func (endp *SMTPEndpoint) setConfig(globalCfg map[string]config.Node, rawCfg config.Node) error {
	var (
		err        error
		ioDebug    bool
		submission bool

		localDeliveryDefault  string
		localDeliveryOpts     map[string]string
		remoteDeliveryDefault string
		remoteDeliveryOpts    map[string]string
	)

	cfg := config.Map{}
	cfg.Custom("auth", false, true, defaultAuthProvider, authDirective, &endp.Auth)
	cfg.String("hostname", true, true, "", &endp.serv.Domain)
	cfg.Int("max_idle", false, false, 60, &endp.serv.MaxIdleSeconds)
	cfg.Int("max_message_size", false, false, 32*1024*1024, &endp.serv.MaxMessageBytes)
	cfg.Int("max_recipients", false, false, 255, &endp.serv.MaxRecipients)
	cfg.Custom("tls", true, true, nil, tlsDirective, &endp.serv.TLSConfig)
	cfg.Bool("insecure_auth", false, &endp.serv.AllowInsecureAuth)
	cfg.Bool("io_debug", false, &ioDebug)
	cfg.Bool("debug", true, &endp.Log.Debug)
	cfg.Bool("submission", false, &submission)
	cfg.AllowUnknown()

	remainingDirs, err := cfg.Process(globalCfg, &rawCfg)
	if err != nil {
		return err
	}

	// If endp.submission is set by NewSMTPEndpoint because module name is "submission"
	// - don't use value from configuration.
	if !endp.submission {
		endp.submission = submission
	} else if submission {
		endp.Log.Println("redundant submission statement")
	}

	for _, entry := range remainingDirs {
		switch entry.Name {
		case "local_delivery":
			if len(entry.Args) == 0 {
				return errors.New("smtp: local_delivery: expected at least 1 argument")
			}
			if len(endp.pipeline) != 0 {
				return errors.New("smtp: can't use custom pipeline with local_delivery or remote_delivery")
			}

			localDeliveryDefault = entry.Args[0]
			localDeliveryOpts = readOpts(entry.Args[1:])
		case "remote_delivery":
			if len(entry.Args) == 0 {
				return errors.New("smtp: remote_delivery: expected at least 1 argument")
			}
			if len(endp.pipeline) != 0 {
				return errors.New("smtp: can't use custom pipeline with local_delivery or remote_delivery")
			}

			remoteDeliveryDefault = entry.Args[0]
			remoteDeliveryOpts = readOpts(entry.Args[1:])
		case "filter", "delivery", "match", "stop", "require_auth":
			if localDeliveryDefault != "" || remoteDeliveryDefault != "" {
				return errors.New("smtp: can't use custom pipeline with local_delivery or remote_delivery")
			}

			step, err := StepFromCfg(entry)
			if err != nil {
				return err
			}
			endp.pipeline = append(endp.pipeline, step)
		default:
			return fmt.Errorf("smtp: unknown config directive: %v", entry.Name)
		}
	}

	if len(endp.pipeline) == 0 {
		err := endp.setDefaultPipeline(localDeliveryDefault, remoteDeliveryDefault, localDeliveryOpts, remoteDeliveryOpts)
		if err != nil {
			return err
		}
	}
	if endp.submission {
		endp.pipeline = append([]SMTPPipelineStep{submissionPrepareStep{}, requireAuthStep{}}, endp.pipeline...)
	}

	if ioDebug {
		endp.serv.Debug = os.Stderr
	}

	return nil
}

func (endp *SMTPEndpoint) setupListeners(addresses []Address) error {
	var smtpUsed, lmtpUsed bool
	for _, addr := range addresses {
		if addr.Scheme == "smtp" || addr.Scheme == "smtps" {
			if lmtpUsed {
				return errors.New("smtp: can't mix LMTP with SMTP in one endpoint block")
			}
			smtpUsed = true
		}
		if addr.Scheme == "lmtp+unix" || addr.Scheme == "lmtp" {
			if smtpUsed {
				return errors.New("smtp: can't mix LMTP with SMTP in one endpoint block")
			}
			lmtpUsed = true
		}

		var l net.Listener
		var err error
		l, err = net.Listen(addr.Network(), addr.Address())
		if err != nil {
			return fmt.Errorf("failed to bind on %v: %v", addr, err)
		}
		endp.Log.Printf("listening on %v", addr)

		if addr.IsTLS() {
			if endp.serv.TLSConfig == nil {
				return errors.New("smtp: can't bind on SMTPS endpoint without TLS configuration")
			}
			l = tls.NewListener(l, endp.serv.TLSConfig)
		}

		endp.listeners = append(endp.listeners, l)

		endp.listenersWg.Add(1)
		addr := addr
		go func() {
			if err := endp.serv.Serve(l); err != nil && !strings.HasSuffix(err.Error(), "use of closed network connection") {
				endp.Log.Printf("failed to serve %s: %s", addr, err)
			}
			endp.listenersWg.Done()
		}()
	}

	if lmtpUsed {
		endp.serv.LMTP = true
	}

	return nil
}

func (endp *SMTPEndpoint) setDefaultPipeline(localDeliveryName, remoteDeliveryName string, localOpts, remoteOpts map[string]string) error {
	var err error
	var localDelivery module.DeliveryTarget
	if localDeliveryName == "" {
		localDelivery, err = deliveryTarget("default_local_delivery")
		if err != nil {
			localDelivery, err = deliveryTarget("default")
			if err != nil {
				return err
			}
		}
		localOpts = map[string]string{"local_only": ""}
	} else {
		localDelivery, err = deliveryTarget(localDeliveryName)
		if err != nil {
			return err
		}
	}

	if endp.submission {
		if remoteDeliveryName == "" {
			remoteDeliveryName = "default_remote_delivery"
			remoteOpts = map[string]string{"remote_only": ""}
		}
		remoteDelivery, err := deliveryTarget(remoteDeliveryName)
		if err != nil {
			return err
		}

		endp.pipeline = append(endp.pipeline,
			// require_auth and submission_check are always prepended to pipeline
			//TODO: DKIM sign
			deliveryStep{t: localDelivery, opts: localOpts},
			deliveryStep{t: remoteDelivery, opts: remoteOpts},
		)
	} else {
		endp.pipeline = append(endp.pipeline,
			//TODO: DKIM verify
			deliveryStep{t: localDelivery, opts: localOpts},
		)
	}

	return nil
}

func (endp *SMTPEndpoint) Login(state *smtp.ConnectionState, username, password string) (smtp.User, error) {
	if !endp.Auth.CheckPlain(username, password) {
		return nil, errors.New("Invalid credentials")
	}

	return SMTPUser{
		endp:     endp,
		username: username,
		state:    state,
	}, nil
}

func (endp *SMTPEndpoint) AnonymousLogin(state *smtp.ConnectionState) (smtp.User, error) {
	return SMTPUser{
		endp:      endp,
		anonymous: true,
		state:     state,
	}, nil
}

func formatAddr(raw string) string {
	addr := mail.Address{Address: raw}
	return addr.String()
}

func addReturnPath(ctx module.DeliveryContext, msg io.Reader) io.Reader {
	hdr := "Return-Path: " + formatAddr(ctx.From) + "\r\n"
	return io.MultiReader(strings.NewReader(hdr), msg)
}

func (endp *SMTPEndpoint) Close() error {
	for _, l := range endp.listeners {
		l.Close()
	}
	endp.serv.Close()
	endp.listenersWg.Wait()
	return nil
}

func init() {
	module.Register("smtp", NewSMTPEndpoint)
	module.Register("submission", NewSMTPEndpoint)
}
