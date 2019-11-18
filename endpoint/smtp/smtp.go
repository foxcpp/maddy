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
	"github.com/foxcpp/maddy/dns"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/future"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/msgpipeline"
	"github.com/foxcpp/maddy/target"
)

type Session struct {
	endp        *Endpoint
	delivery    module.Delivery
	deliveryErr error
	msgMeta     *module.MsgMetadata
	log         log.Logger
	cancelRDNS  func()

	repeatedMailErrs int
	loggedRcptErrors int
}

func (s *Session) Reset() {
	if s.delivery != nil {
		if err := s.delivery.Abort(); err != nil {
			s.endp.Log.Error("delivery abort failed", err)
		}
		s.log.Msg("aborted")
		s.delivery = nil
		s.log = s.endp.Log
	}
	s.deliveryErr = nil
	s.repeatedMailErrs = 0
}

func (s *Session) Mail(from string) error {
	var err error
	s.msgMeta.ID, err = msgpipeline.GenerateMsgID()
	if err != nil {
		s.log.Msg("rand.Rand fail", "err", err)
		return s.endp.wrapErr(s.msgMeta.ID, err)
	}
	s.msgMeta.OriginalFrom = from

	if s.endp.resolver != nil && s.msgMeta.SrcAddr != nil {
		rdnsCtx, cancelRDNS := context.WithCancel(context.TODO())
		s.msgMeta.SrcRDNSName = future.New()
		s.cancelRDNS = cancelRDNS
		go s.fetchRDNSName(rdnsCtx)
	}

	if !s.endp.deferServerReject {
		s.log = target.DeliveryLogger(s.log, s.msgMeta)
		s.log.Msg("incoming message",
			"src_host", s.msgMeta.SrcHostname,
			"src_ip", s.msgMeta.SrcAddr.String(),
			"sender", s.msgMeta.OriginalFrom,
		)

		s.delivery, err = s.endp.pipeline.Start(s.msgMeta, s.msgMeta.OriginalFrom)
		if err != nil {
			s.log.Error("MAIL FROM error", err)
			return s.endp.wrapErr(s.msgMeta.ID, err)
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
		reason, misc := exterrors.UnwrapDNSErr(err)
		misc["reason"] = reason
		s.log.Error("rDNS error", exterrors.WithFields(err, misc))
		s.msgMeta.SrcRDNSName.Set(nil)
		return
	}

	s.msgMeta.SrcRDNSName.Set(name)
}

func (s *Session) Rcpt(to string) error {
	if s.delivery == nil {
		s.log = target.DeliveryLogger(s.log, s.msgMeta)

		if s.deliveryErr == nil {
			s.log.Msg("incoming message",
				"src_host", s.msgMeta.SrcHostname,
				"src_ip", s.msgMeta.SrcAddr.String(),
				"sender", s.msgMeta.OriginalFrom,
			)
		} else {
			s.repeatedMailErrs++
			return s.endp.wrapErr(s.msgMeta.ID, s.deliveryErr)
		}

		var err error
		s.delivery, err = s.endp.pipeline.Start(s.msgMeta, s.msgMeta.OriginalFrom)
		if err != nil {
			s.log.Error("MAIL FROM error (deferred)", err, "rcpt", to)
			s.deliveryErr = err
			return s.endp.wrapErr(s.msgMeta.ID, err)
		}
	}

	err := s.delivery.AddRcpt(to)
	if err != nil {
		if s.loggedRcptErrors < s.endp.maxLoggedRcptErrors {
			s.log.Error("RCPT error", err, "rcpt", to)
			s.loggedRcptErrors++
			if s.loggedRcptErrors == s.endp.maxLoggedRcptErrors {
				s.log.Msg("further RCPT TO errors will not be logged")
			}
		}
		return s.endp.wrapErr(s.msgMeta.ID, err)
	}
	s.log.Msg("RCPT ok", "rcpt", to)
	return nil
}

func (s *Session) Logout() error {
	if s.delivery != nil {
		if err := s.delivery.Abort(); err != nil {
			s.endp.Log.Printf("failed to abort delivery: %v", err)
		}
		s.log.Msg("aborted")
		s.delivery = nil
	}
	if s.cancelRDNS != nil {
		s.cancelRDNS()
	}
	if s.repeatedMailErrs != 0 {
		s.log.Msg("MAIL FROM error repeated multiple times", "count", s.repeatedMailErrs)

		// XXX: Workaround for go-smtp calling Logout twice.
		s.repeatedMailErrs = 0
	}
	return nil
}

func (s *Session) Data(r io.Reader) error {
	bufr := bufio.NewReader(r)
	header, err := textproto.ReadHeader(bufr)
	if err != nil {
		s.log.Error("DATA error", err)
		return s.endp.wrapErr(s.msgMeta.ID, err)
	}

	if s.endp.submission {
		if err := s.submissionPrepare(header); err != nil {
			s.log.Error("DATA error", err)
			return s.endp.wrapErr(s.msgMeta.ID, err)
		}
	}

	// TODO: Disk buffering.
	buf, err := buffer.BufferInMemory(bufr)
	if err != nil {
		s.log.Error("DATA error", err)
		return s.endp.wrapErr(s.msgMeta.ID, err)
	}
	s.msgMeta.BodyLength = len(buf.(buffer.MemoryBuffer).Slice)

	received, err := target.GenerateReceived(context.TODO(), s.msgMeta, s.endp.serv.Domain, s.msgMeta.OriginalFrom)
	if err != nil {
		s.log.Error("DATA error", err)
		return err
	}
	header.Add("Received", received)

	if err := s.delivery.Body(header, buf); err != nil {
		s.log.Error("DATA error", err)
		return s.endp.wrapErr(s.msgMeta.ID, err)
	}

	if err := s.delivery.Commit(); err != nil {
		s.log.Error("DATA error", err)
		return s.endp.wrapErr(s.msgMeta.ID, err)
	}

	s.log.Msg("accepted")
	s.delivery = nil

	return nil
}

func (endp *Endpoint) wrapErr(msgId string, err error) error {
	if err == nil {
		return nil
	}

	res := &smtp.SMTPError{
		Code:         554,
		EnhancedCode: smtp.EnhancedCodeNotSet,
		// Err on the side of caution if the error lacks SMTP annotations. If
		// we just pass the error text through, we might accidenetally disclose
		// details of server configuration.
		Message: "Internal server error",
	}

	if exterrors.IsTemporary(err) {
		res.Code = 451
	}

	ctxInfo := exterrors.Fields(err)
	ctxCode, ok := ctxInfo["smtp_code"].(int)
	if ok {
		res.Code = ctxCode
	}
	ctxEnchCode, ok := ctxInfo["smtp_enchcode"].(exterrors.EnhancedCode)
	if ok {
		res.EnhancedCode = smtp.EnhancedCode(ctxEnchCode)
	}
	ctxMsg, ok := ctxInfo["smtp_msg"].(string)
	if ok {
		res.Message = ctxMsg
	}

	if smtpErr, ok := err.(*smtp.SMTPError); ok {
		endp.Log.Printf("plain SMTP error returned, this is deprecated")
		res.Code = smtpErr.Code
		res.EnhancedCode = smtpErr.EnhancedCode
		res.Message = smtpErr.Message
	}

	if msgId != "" {
		res.Message += " (msg ID = " + msgId + ")"
	}
	return res
}

type Endpoint struct {
	Auth      module.AuthProvider
	serv      *smtp.Server
	name      string
	addrs     []string
	listeners []net.Listener
	pipeline  *msgpipeline.MsgPipeline
	resolver  dns.Resolver

	authAlwaysRequired  bool
	submission          bool
	lmtp                bool
	deferServerReject   bool
	maxLoggedRcptErrors int

	listenersWg sync.WaitGroup

	Log log.Logger
}

func (endp *Endpoint) Name() string {
	return endp.name
}

func (endp *Endpoint) InstanceName() string {
	return endp.name
}

func New(modName string, addrs []string) (module.Module, error) {
	endp := &Endpoint{
		name:       modName,
		addrs:      addrs,
		submission: modName == "submission",
		lmtp:       modName == "lmtp",
		resolver:   net.DefaultResolver,
		Log:        log.Logger{Name: modName},
	}
	return endp, nil
}

func (endp *Endpoint) Init(cfg *config.Map) error {
	endp.serv = smtp.NewServer(endp)
	endp.serv.ErrorLog = endp.Log
	if err := endp.setConfig(cfg); err != nil {
		return err
	}

	if endp.Auth != nil {
		endp.Log.Debugf("authentication provider: %s %s", endp.Auth.(module.Module).Name(), endp.Auth.(module.Module).InstanceName())
	}

	addresses := make([]config.Endpoint, 0, len(endp.addrs))
	for _, addr := range endp.addrs {
		saddr, err := config.ParseEndpoint(addr)
		if err != nil {
			return fmt.Errorf("%s: invalid address: %s", addr, endp.name)
		}

		addresses = append(addresses, saddr)
	}

	if err := endp.setupListeners(addresses); err != nil {
		for _, l := range endp.listeners {
			l.Close()
		}
		return err
	}

	allLocal := true
	for _, addr := range addresses {
		if addr.Scheme != "unix" && !strings.HasPrefix(addr.Host, "127.0.0.") {
			allLocal = false
		}
	}

	if endp.serv.AllowInsecureAuth && !allLocal {
		endp.Log.Println("authentication over unencrypted connections is allowed, this is insecure configuration and should be used only for testing!")
	}
	if endp.serv.TLSConfig == nil {
		if !allLocal {
			endp.Log.Println("TLS is disabled, this is insecure configuration and should be used only for testing!")
		}

		endp.serv.AllowInsecureAuth = true
	}

	return nil
}

func (endp *Endpoint) setConfig(cfg *config.Map) error {
	var (
		err     error
		ioDebug bool
	)

	cfg.Custom("auth", false, false, nil, modconfig.AuthDirective, &endp.Auth)
	cfg.String("hostname", true, true, "", &endp.serv.Domain)
	cfg.Duration("write_timeout", false, false, 1*time.Minute, &endp.serv.WriteTimeout)
	cfg.Duration("read_timeout", false, false, 10*time.Minute, &endp.serv.ReadTimeout)
	cfg.DataSize("max_message_size", false, false, 32*1024*1024, &endp.serv.MaxMessageBytes)
	cfg.Int("max_recipients", false, false, 20000, &endp.serv.MaxRecipients)
	cfg.Custom("tls", true, true, nil, config.TLSDirective, &endp.serv.TLSConfig)
	cfg.Bool("insecure_auth", false, false, &endp.serv.AllowInsecureAuth)
	cfg.Bool("io_debug", false, false, &ioDebug)
	cfg.Bool("debug", true, false, &endp.Log.Debug)
	cfg.Bool("defer_sender_reject", false, true, &endp.deferServerReject)
	cfg.Int("max_logged_rcpt_errors", false, false, 5, &endp.maxLoggedRcptErrors)
	cfg.AllowUnknown()
	unknown, err := cfg.Process()
	if err != nil {
		return err
	}
	endp.pipeline, err = msgpipeline.New(cfg.Globals, unknown)
	if err != nil {
		return err
	}
	endp.pipeline.Hostname = endp.serv.Domain
	endp.pipeline.Resolver = endp.resolver
	endp.pipeline.Log = log.Logger{Name: "smtp/pipeline", Debug: endp.Log.Debug}

	endp.serv.AuthDisabled = endp.Auth == nil
	if endp.submission {
		endp.authAlwaysRequired = true

		if endp.Auth == nil {
			return fmt.Errorf("%s: auth. provider must be set for submission endpoint", endp.name)
		}
	}

	if ioDebug {
		endp.serv.Debug = endp.Log.DebugWriter()
		endp.Log.Println("I/O debugging is on! It may leak passwords in logs, be careful!")
	}

	return nil
}

func (endp *Endpoint) setupListeners(addresses []config.Endpoint) error {
	for _, addr := range addresses {
		var l net.Listener
		var err error
		l, err = net.Listen(addr.Network(), addr.Address())
		if err != nil {
			return fmt.Errorf("%s: %w", endp.name, err)
		}
		endp.Log.Printf("listening on %v", addr)

		if addr.IsTLS() {
			if endp.serv.TLSConfig == nil {
				return fmt.Errorf("%s: can't bind on SMTPS endpoint without TLS configuration", endp.name)
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

	return nil
}

func (endp *Endpoint) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	if endp.Auth == nil {
		return nil, smtp.ErrAuthUnsupported
	}

	// Executed before authentication and session initialization.
	if err := endp.pipeline.RunEarlyChecks(state); err != nil {
		return nil, endp.wrapErr("", err)
	}

	if !endp.Auth.CheckPlain(username, password) {
		endp.Log.Msg("authentication failed", "username", username, "src_ip", state.RemoteAddr)
		return nil, errors.New("Invalid credentials")
	}

	return endp.newSession(false, username, password, state), nil
}

func (endp *Endpoint) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	if endp.authAlwaysRequired {
		return nil, smtp.ErrAuthRequired
	}

	// Executed before authentication and session initialization.
	if err := endp.pipeline.RunEarlyChecks(state); err != nil {
		return nil, endp.wrapErr("", err)
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
		log:     endp.Log,
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
	module.RegisterEndpoint("smtp", New)
	module.RegisterEndpoint("submission", New)
	module.RegisterEndpoint("lmtp", New)

	rand.Seed(time.Now().UnixNano())
}
