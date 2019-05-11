package maddy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"encoding/hex"
	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
	"math/rand"
)

type SMTPSession struct {
	endp  *SMTPEndpoint
	state *smtp.ConnectionState
	ctx   *module.DeliveryContext
}

func (s *SMTPSession) Reset() {
	s.ctx.From = ""
	s.ctx.To = nil
}

func (s *SMTPSession) Mail(from string) error {
	s.ctx.From = from
	return nil
}

func (s *SMTPSession) Rcpt(to string) error {
	s.ctx.To = append(s.ctx.To, to)
	return nil
}

func (s *SMTPSession) Logout() error {
	return nil
}

func (s *SMTPSession) Data(r io.Reader) error {
	// TODO: Execute pipeline steps in parallel.
	// https://github.com/emersion/maddy/pull/17#discussion_r267573580

	rawID := make([]byte, 32)
	_, err := rand.Read(rawID)
	if err != nil {
		return err
	}
	s.ctx.DeliveryID = hex.EncodeToString(rawID)

	s.endp.Log.Printf("incoming message from %s (%s), delivery ID = %s", s.ctx.SrcHostname, s.ctx.SrcAddr, s.ctx.DeliveryID)
	_, _, err = passThroughPipeline(s.endp.pipeline, s.ctx, r)
	if err != nil {
		return err
	}
	s.endp.Log.Printf("accepted message from %s (%s), delivery ID = %s", s.ctx.SrcHostname, s.ctx.SrcAddr, s.ctx.DeliveryID)

	return nil
}

type SMTPEndpoint struct {
	Auth      module.AuthProvider
	serv      *smtp.Server
	name      string
	listeners []net.Listener
	pipeline  []SMTPPipelineStep

	authAlwaysRequired bool

	submission bool

	listenersWg sync.WaitGroup

	Log log.Logger
}

func (endp *SMTPEndpoint) Name() string {
	return "smtp"
}

func (endp *SMTPEndpoint) InstanceName() string {
	return endp.name
}

func NewSMTPEndpoint(modName, instName string) (module.Module, error) {
	endp := &SMTPEndpoint{
		name:       instName,
		submission: modName == "submission",
		Log:        log.Logger{Name: "smtp"},
	}
	return endp, nil
}

func (endp *SMTPEndpoint) Init(cfg *config.Map) error {
	endp.serv = smtp.NewServer(endp)
	if err := endp.setConfig(cfg); err != nil {
		return err
	}

	if endp.Auth != nil {
		endp.Log.Debugf("authentication provider: %s %s", endp.Auth.(module.Module).Name(), endp.Auth.(module.Module).InstanceName())
	}
	endp.Log.Debugf("pipeline: %#v", endp.pipeline)

	addresses := make([]Address, 0, len(cfg.Block.Args))
	for _, addr := range cfg.Block.Args {
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

func (endp *SMTPEndpoint) setConfig(cfg *config.Map) error {
	var (
		err     error
		ioDebug bool

		writeTimeoutSecs uint
		readTimeoutSecs  uint
	)

	cfg.Custom("auth", false, false, nil, authDirective, &endp.Auth)
	cfg.String("hostname", true, false, "", &endp.serv.Domain)
	// TODO: Parse human-readable duration values.
	cfg.UInt("write_timeout", false, false, 60, &writeTimeoutSecs)
	cfg.UInt("read_timeout", false, false, 600, &readTimeoutSecs)
	cfg.Int("max_message_size", false, false, 32*1024*1024, &endp.serv.MaxMessageBytes)
	cfg.Int("max_recipients", false, false, 255, &endp.serv.MaxRecipients)
	cfg.Custom("tls", true, true, nil, tlsDirective, &endp.serv.TLSConfig)
	cfg.Bool("insecure_auth", false, &endp.serv.AllowInsecureAuth)
	cfg.Bool("io_debug", false, &ioDebug)
	cfg.Bool("debug", true, &endp.Log.Debug)
	cfg.Bool("submission", false, &endp.submission)
	cfg.AllowUnknown()

	remainingDirs, err := cfg.Process()
	if err != nil {
		return err
	}

	endp.serv.WriteTimeout = time.Duration(writeTimeoutSecs) * time.Second
	endp.serv.ReadTimeout = time.Duration(readTimeoutSecs) * time.Second

	for _, entry := range remainingDirs {
		step, err := StepFromCfg(entry)
		if err != nil {
			return err
		}

		// This will not trigger for nested blocks ('match', 'destination').
		if entry.Name == "require_auth" {
			endp.authAlwaysRequired = true
		}

		endp.pipeline = append(endp.pipeline, step)
	}

	if endp.submission {
		endp.pipeline = append([]SMTPPipelineStep{submissionPrepareStep{}, requireAuthStep{}}, endp.pipeline...)
		endp.authAlwaysRequired = true

		if endp.Auth == nil {
			return fmt.Errorf("smtp: auth. provider must be set for submission endpoint")
		}
	}

	if ioDebug {
		endp.serv.Debug = endp.Log.DebugWriter()
		endp.Log.Println("I/O debugging is on! It may leak passwords in logs, be careful!")
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

func (endp *SMTPEndpoint) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	if endp.Auth == nil {
		return nil, smtp.ErrAuthUnsupported
	}

	if !endp.Auth.CheckPlain(username, password) {
		endp.Log.Printf("authentication failed for %s (from %v)", username, state.RemoteAddr)
		return nil, errors.New("Invalid credentials")
	}

	return endp.newSession(false, username, state), nil
}

func (endp *SMTPEndpoint) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	if endp.authAlwaysRequired {
		return nil, smtp.ErrAuthRequired
	}

	return endp.newSession(true, "", state), nil
}

func (endp *SMTPEndpoint) newSession(anonymous bool, username string, state *smtp.ConnectionState) smtp.Session {
	ctx := module.DeliveryContext{
		Anonymous:   anonymous,
		AuthUser:    username,
		SrcTLSState: state.TLS,
		SrcHostname: state.Hostname,
		SrcAddr:     state.RemoteAddr,
		OurHostname: endp.serv.Domain,
		Ctx:         make(map[string]interface{}),
	}

	if endp.serv.LMTP {
		ctx.SrcProto = "LMTP"
	} else {
		// Check if TLS connection state struct is poplated.
		// If it is - we are ssing TLS.
		if state.TLS.Version != 0 {
			ctx.SrcProto = "ESMTPS"
		} else {
			ctx.SrcProto = "ESMTP"
		}
	}

	return &SMTPSession{
		endp: endp,
		ctx:  &ctx,
	}
}

func sanitizeString(raw string) string {
	return strings.Replace(raw, "\n", "", -1)
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
