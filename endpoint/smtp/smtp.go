package smtp

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"math/rand"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	modconfig "github.com/foxcpp/maddy/config/module"
	"github.com/foxcpp/maddy/dispatcher"
	"github.com/foxcpp/maddy/dns"
	"github.com/foxcpp/maddy/endpoint"
	"github.com/foxcpp/maddy/future"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/target"
)

func MsgMetaLog(l log.Logger, msgMeta *module.MsgMetadata) log.Logger {
	out := l.Out
	if out == nil {
		out = log.DefaultLogger.Out
	}

	return log.Logger{
		Out: func(t time.Time, debug bool, str string) {
			ctxInfo := fmt.Sprintf(", HELO = %s, IP = %s, MAIL FROM = %s, msg ID = %s", msgMeta.SrcHostname, msgMeta.SrcAddr, msgMeta.OriginalFrom, msgMeta.ID)
			out(t, debug, str+ctxInfo)
		},
		Debug: l.Debug,
		Name:  l.Name,
	}
}

type Session struct {
	endp        *Endpoint
	delivery    module.Delivery
	deliveryErr error
	msgMeta     *module.MsgMetadata
	log         log.Logger
	cancelRDNS  func()
}

var errInternal = &smtp.SMTPError{
	Code:         451,
	EnhancedCode: smtp.EnhancedCode{4, 0, 0},
	Message:      "Internal server error",
}

func (s *Session) Reset() {
	if s.delivery != nil {
		if err := s.delivery.Abort(); err != nil {
			s.endp.Log.Printf("failed to abort delivery: %v", err)
		}
		s.delivery = nil
	}
}

func (s *Session) Mail(from string) error {
	var err error
	s.msgMeta.ID, err = dispatcher.GenerateMsgID()
	if err != nil {
		s.endp.Log.Printf("rand.Rand error: %v", err)
		return s.wrapErr(errInternal)
	}
	s.msgMeta.OriginalFrom = from

	s.log.Printf("incoming message")

	// Left here for future use.
	mailCtx := context.TODO()

	if s.endp.resolver != nil && s.msgMeta.SrcAddr != nil {
		rdnsCtx, cancelRDNS := context.WithCancel(mailCtx)
		s.msgMeta.SrcRDNSName = future.New()
		s.cancelRDNS = cancelRDNS
		go s.fetchRDNSName(rdnsCtx)
	}

	if !s.endp.deferServerReject {
		s.delivery, err = s.endp.dispatcher.Start(s.msgMeta, s.msgMeta.OriginalFrom)
		if err != nil {
			s.log.Printf("sender rejected: %v", err)
			return s.wrapErr(err)
		}
	}

	return nil
}

func (s *Session) fetchRDNSName(ctx context.Context) {
	tcpAddr, ok := s.msgMeta.SrcAddr.(*net.TCPAddr)
	if !ok {
		s.msgMeta.SrcRDNSName.Set(nil)
		return
	}

	name, err := dns.LookupAddr(ctx, s.endp.resolver, tcpAddr.IP)
	if err != nil {
		s.log.Printf("failed to do RDNS lookup for %v: %v", tcpAddr.IP, err)
		s.msgMeta.SrcRDNSName.Set(nil)
		return
	}

	s.msgMeta.SrcRDNSName.Set(name)
}

func (s *Session) Rcpt(to string) error {
	if s.delivery == nil {
		if s.deliveryErr != nil {
			s.log.Printf("sender rejected (repeated): %v, RCPT TO = %s", s.deliveryErr, to)
			return s.wrapErr(s.deliveryErr)
		}

		var err error
		s.delivery, err = s.endp.dispatcher.Start(s.msgMeta, s.msgMeta.OriginalFrom)
		if err != nil {
			s.log.Printf("sender rejected (deferred): %v, RCPT TO = %s", err, to)
			s.deliveryErr = err
			return s.wrapErr(err)
		}
	}

	err := s.delivery.AddRcpt(to)
	if err != nil {
		s.log.Printf("recipient rejected: %v, RCPT TO = %s", err, to)
	}
	return s.wrapErr(err)
}

func (s *Session) Logout() error {
	if s.delivery != nil {
		if err := s.delivery.Abort(); err != nil {
			s.endp.Log.Printf("failed to abort delivery: %v", err)
		}
		s.delivery = nil
	}
	if s.cancelRDNS != nil {
		s.cancelRDNS()
	}
	return nil
}

func (s *Session) Data(r io.Reader) error {
	bufr := bufio.NewReader(r)
	header, err := textproto.ReadHeader(bufr)
	if err != nil {
		s.log.Printf("malformed header or I/O error: %v", err)
		return s.wrapErr(err)
	}

	if s.endp.submission {
		if err := SubmissionPrepare(s.msgMeta, header, s.endp.serv.Domain); err != nil {
			s.log.Printf("malformed header or I/O error: %v", err)
			return s.wrapErr(err)
		}
	}

	// TODO: Disk buffering.
	buf, err := buffer.BufferInMemory(bufr)
	if err != nil {
		s.log.Printf("I/O error: %v", err)
		return s.wrapErr(errInternal)
	}
	s.msgMeta.BodyLength = len(buf.(buffer.MemoryBuffer).Slice)

	received, err := target.GenerateReceived(context.TODO(), s.msgMeta, s.endp.serv.Domain, s.msgMeta.OriginalFrom)
	if err != nil {
		return err
	}
	header.Add("Received", received)

	if err := s.delivery.Body(header, buf); err != nil {
		s.log.Printf("I/O error: %v", err)
		return s.wrapErr(err)
	}

	if err := s.delivery.Commit(); err != nil {
		s.log.Printf("I/O error: %v", err)
		return s.wrapErr(err)
	}

	s.log.Printf("message accepted")
	s.delivery = nil

	return nil
}

func (s *Session) wrapErr(err error) error {
	if err == nil {
		return nil
	}

	if smtpErr, ok := err.(*smtp.SMTPError); ok {
		return &smtp.SMTPError{
			Code:         smtpErr.Code,
			EnhancedCode: smtpErr.EnhancedCode,
			Message:      smtpErr.Message + " (msg ID = " + s.msgMeta.ID + ")",
		}
	}
	return fmt.Errorf("%v (msg ID = %s)", err, s.msgMeta.ID)
}

type Endpoint struct {
	Auth       module.AuthProvider
	serv       *smtp.Server
	name       string
	aliases    []string
	listeners  []net.Listener
	dispatcher *dispatcher.Dispatcher
	resolver   dns.Resolver

	authAlwaysRequired bool
	submission         bool
	deferServerReject  bool

	listenersWg sync.WaitGroup

	Log log.Logger
}

func (endp *Endpoint) Name() string {
	return "smtp"
}

func (endp *Endpoint) InstanceName() string {
	return endp.name
}

func New(modName, instName string, aliases []string) (module.Module, error) {
	endp := &Endpoint{
		name:       instName,
		aliases:    aliases,
		submission: modName == "submission",
		resolver:   net.DefaultResolver,
		Log:        log.Logger{Name: "smtp"},
	}
	return endp, nil
}

func (endp *Endpoint) Init(cfg *config.Map) error {
	endp.serv = smtp.NewServer(endp)
	if err := endp.setConfig(cfg); err != nil {
		return err
	}

	if endp.Auth != nil {
		endp.Log.Debugf("authentication provider: %s %s", endp.Auth.(module.Module).Name(), endp.Auth.(module.Module).InstanceName())
	}

	args := append([]string{endp.name}, endp.aliases...)
	addresses := make([]config.Address, 0, len(args))
	for _, addr := range args {
		saddr, err := config.StandardizeAddress(addr)
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

func (endp *Endpoint) setConfig(cfg *config.Map) error {
	var (
		err        error
		ioDebug    bool
		submission bool
	)

	cfg.Custom("auth", false, false, nil, modconfig.AuthDirective, &endp.Auth)
	cfg.String("hostname", true, true, "", &endp.serv.Domain)
	cfg.Duration("write_timeout", false, false, 1*time.Minute, &endp.serv.WriteTimeout)
	cfg.Duration("read_timeout", false, false, 10*time.Minute, &endp.serv.ReadTimeout)
	cfg.DataSize("max_message_size", false, false, 32*1024*1024, &endp.serv.MaxMessageBytes)
	cfg.Int("max_recipients", false, false, 20000, &endp.serv.MaxRecipients)
	cfg.Custom("tls", true, true, nil, endpoint.TLSDirective, &endp.serv.TLSConfig)
	cfg.Bool("insecure_auth", false, false, &endp.serv.AllowInsecureAuth)
	cfg.Bool("io_debug", false, false, &ioDebug)
	cfg.Bool("debug", true, false, &endp.Log.Debug)
	cfg.Bool("submission", false, false, &submission)
	cfg.Bool("defer_sender_reject", false, true, &endp.deferServerReject)
	cfg.AllowUnknown()
	unmatched, err := cfg.Process()
	if err != nil {
		return err
	}
	endp.dispatcher, err = dispatcher.NewDispatcher(cfg.Globals, unmatched)
	if err != nil {
		return err
	}
	endp.dispatcher.Hostname = endp.serv.Domain
	endp.dispatcher.Log = log.Logger{Name: "smtp/dispatcher", Debug: endp.Log.Debug}

	// endp.submission can be set to true by New, leave it on
	// even if directive is missing.
	if submission {
		endp.submission = true
	}

	if endp.submission {
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

func (endp *Endpoint) setupListeners(addresses []config.Address) error {
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

func (endp *Endpoint) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	if endp.Auth == nil {
		return nil, smtp.ErrAuthUnsupported
	}

	if !endp.Auth.CheckPlain(username, password) {
		endp.Log.Printf("authentication failed for %s (from %v)", username, state.RemoteAddr)
		return nil, errors.New("Invalid credentials")
	}

	return endp.newSession(false, username, password, state), nil
}

func (endp *Endpoint) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	if endp.authAlwaysRequired {
		return nil, smtp.ErrAuthRequired
	}

	return endp.newSession(true, "", "", state), nil
}

func (endp *Endpoint) newSession(anonymous bool, username, password string, state *smtp.ConnectionState) smtp.Session {
	ctx := &module.MsgMetadata{
		Anonymous:    anonymous,
		AuthUser:     username,
		AuthPassword: password,
		SrcTLSState:  state.TLS,
		SrcHostname:  state.Hostname,
		SrcAddr:      state.RemoteAddr,
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

	return &Session{
		endp:    endp,
		msgMeta: ctx,
		log:     MsgMetaLog(endp.Log, ctx),
	}
}

func (endp *Endpoint) Close() error {
	for _, l := range endp.listeners {
		l.Close()
	}
	endp.serv.Close()
	endp.listenersWg.Wait()
	return nil
}

func init() {
	module.Register("smtp", New)
	module.Register("submission", New)

	rand.Seed(time.Now().UnixNano())
}
