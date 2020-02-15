package smtp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime/trace"
	"strings"
	"sync"
	"time"

	"math/rand"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/internal/address"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/config"
	modconfig "github.com/foxcpp/maddy/internal/config/module"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/future"
	"github.com/foxcpp/maddy/internal/limits"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/msgpipeline"
	"github.com/foxcpp/maddy/internal/target"
	"golang.org/x/net/idna"
)

type Session struct {
	endp *Endpoint

	// Specific for this session.
	// sessionCtx is not used for cancellation or timeouts, only for tracing.
	sessionCtx       context.Context
	cancelRDNS       func()
	connState        module.ConnState
	repeatedMailErrs int
	loggedRcptErrors int

	// Specific for the currently handled message.
	// msgCtx is not used for cancellation or timeouts, only for tracing.
	// It is the subcontext of sessionCtx.
	msgCtx      context.Context
	msgTask     *trace.Task
	mailFrom    string
	opts        smtp.MailOptions
	msgMeta     *module.MsgMetadata
	delivery    module.Delivery
	deliveryErr error

	log log.Logger
}

func (s *Session) Reset() {
	if s.delivery != nil {
		s.abort(s.msgCtx)
	}
	s.endp.Log.DebugMsg("reset")
}

func (s *Session) releaseLimits() {
	_, domain, err := address.Split(s.mailFrom)
	if err != nil {
		return
	}
	addr, ok := s.msgMeta.Conn.RemoteAddr.(*net.TCPAddr)
	if !ok {
		addr = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}
	}
	s.endp.limits.ReleaseMsg(addr.IP, domain)
}

func (s *Session) abort(ctx context.Context) {
	if err := s.delivery.Abort(ctx); err != nil {
		s.endp.Log.Error("delivery abort failed", err)
	}
	s.log.Msg("aborted", "msg_id", s.msgMeta.ID)

	s.mailFrom = ""
	s.opts = smtp.MailOptions{}
	s.msgMeta = nil
	s.delivery = nil
	s.deliveryErr = nil
	s.msgCtx = nil
	s.msgTask.End()
}

func (s *Session) startDelivery(ctx context.Context, from string, opts smtp.MailOptions) (string, error) {
	var err error
	msgMeta := &module.MsgMetadata{
		Conn:     &s.connState,
		SMTPOpts: opts,
	}

	// INTERNATIONALIZATION: Do not permit non-ASCII addresses unless SMTPUTF8 is
	// used.
	for _, ch := range from {
		if ch > 128 && !opts.UTF8 {
			return "", &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 6, 7},
				Message:      "SMTPUTF8 is required for non-ASCII senders",
			}
		}
	}

	// Decode punycode, normalize to NFC and case-fold address.
	cleanFrom, err := address.CleanDomain(from)
	if err != nil {
		return "", &exterrors.SMTPError{
			Code:         553,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 7},
			Message:      "Unable to normalize the sender address",
		}
	}

	msgMeta.ID, err = msgpipeline.GenerateMsgID()
	if err != nil {
		return "", err
	}
	msgMeta.OriginalFrom = from

	_, domain, err := address.Split(cleanFrom)
	if err != nil {
		return "", err
	}
	remoteIP, ok := msgMeta.Conn.RemoteAddr.(*net.TCPAddr)
	if !ok {
		remoteIP = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}
	}
	if err := s.endp.limits.TakeMsg(context.Background(), remoteIP.IP, domain); err != nil {
		return "", err
	}

	if s.connState.AuthUser != "" {
		s.log.Msg("incoming message",
			"src_host", msgMeta.Conn.Hostname,
			"src_ip", msgMeta.Conn.RemoteAddr.String(),
			"sender", from,
			"msg_id", msgMeta.ID,
			"username", s.connState.AuthUser,
		)
	} else {
		s.log.Msg("incoming message",
			"src_host", msgMeta.Conn.Hostname,
			"src_ip", msgMeta.Conn.RemoteAddr.String(),
			"sender", from,
			"msg_id", msgMeta.ID,
		)
	}

	s.msgCtx, s.msgTask = trace.NewTask(ctx, "Incoming Message")

	mailCtx, mailTask := trace.NewTask(s.msgCtx, "MAIL FROM")
	defer mailTask.End()

	delivery, err := s.endp.pipeline.Start(mailCtx, msgMeta, cleanFrom)
	if err != nil {
		s.msgCtx = nil
		s.msgTask.End()
		return msgMeta.ID, err
	}

	s.msgMeta = msgMeta
	s.mailFrom = cleanFrom
	s.delivery = delivery

	return msgMeta.ID, nil
}

func (s *Session) Mail(from string, opts smtp.MailOptions) error {
	if !s.endp.deferServerReject {
		// Will initialize s.msgCtx.
		msgID, err := s.startDelivery(s.sessionCtx, from, opts)
		if err != nil {
			if err != context.DeadlineExceeded {
				s.log.Error("MAIL FROM error", err, "msg_id", msgID)
			}
			return s.endp.wrapErr(msgID, !opts.UTF8, err)
		}
	}

	// Keep the MAIL FROM argument for deferred startDelivery.
	s.mailFrom = from
	s.opts = opts

	return nil
}

func (s *Session) fetchRDNSName(ctx context.Context) {
	defer trace.StartRegion(ctx, "rDNS fetch").End()

	tcpAddr, ok := s.connState.RemoteAddr.(*net.TCPAddr)
	if !ok {
		s.connState.RDNSName.Set(nil, nil)
		return
	}

	name, err := dns.LookupAddr(ctx, s.endp.resolver, tcpAddr.IP)
	if err != nil {
		dnsErr, ok := err.(*net.DNSError)
		if ok && dnsErr.IsNotFound {
			s.connState.RDNSName.Set(nil, nil)
			return
		}

		reason, misc := exterrors.UnwrapDNSErr(err)
		misc["reason"] = reason
		s.log.Error("rDNS error", exterrors.WithFields(err, misc), "src_ip", s.connState.RemoteAddr)
		s.connState.RDNSName.Set(nil, err)
		return
	}

	s.connState.RDNSName.Set(name, nil)
}

func (s *Session) Rcpt(to string) error {
	// deferServerReject = true and this is the first RCPT TO command.
	if s.delivery == nil {
		// If we already attempted to initialize the delivery -
		// fail again.
		if s.deliveryErr != nil {
			s.repeatedMailErrs++
			// The deliveryErr is already wrapped.
			return s.deliveryErr
		}

		// It will initialize s.msgCtx.
		msgID, err := s.startDelivery(s.sessionCtx, s.mailFrom, s.opts)
		if err != nil {
			if err != context.DeadlineExceeded {
				s.log.Error("MAIL FROM error (deferred)", err, "rcpt", to, "msg_id", msgID)
			}
			s.deliveryErr = s.endp.wrapErr(msgID, !s.opts.UTF8, err)
			return s.deliveryErr
		}
	}

	rcptCtx, rcptTask := trace.NewTask(s.msgCtx, "RCPT TO")
	defer rcptTask.End()

	if err := s.rcpt(rcptCtx, to); err != nil {
		if s.loggedRcptErrors < s.endp.maxLoggedRcptErrors {
			s.log.Error("RCPT error", err, "rcpt", to)
			s.loggedRcptErrors++
			if s.loggedRcptErrors == s.endp.maxLoggedRcptErrors {
				s.log.Msg("too many RCPT errors, possible dictonary attack", "src_ip", s.connState.RemoteAddr, "msg_id", s.msgMeta.ID)
			}
		}
		return s.endp.wrapErr(s.msgMeta.ID, !s.opts.UTF8, err)
	}
	s.endp.Log.Msg("RCPT ok", "rcpt", to, "msg_id", s.msgMeta.ID)
	return nil
}

func (s *Session) rcpt(ctx context.Context, to string) error {
	// INTERNATIONALIZATION: Do not permit non-ASCII addresses unless SMTPUTF8 is
	// used.
	if !address.IsASCII(to) && !s.opts.UTF8 {
		return &exterrors.SMTPError{
			Code:         553,
			EnhancedCode: exterrors.EnhancedCode{5, 6, 7},
			Message:      "SMTPUTF8 is required for non-ASCII recipients",
		}
	}
	cleanTo, err := address.CleanDomain(to)
	if err != nil {
		return &exterrors.SMTPError{
			Code:         501,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
			Message:      "Unable to normalize the recipient address",
		}
	}

	return s.delivery.AddRcpt(ctx, cleanTo)
}

func (s *Session) Logout() error {
	if s.delivery != nil {
		s.abort(s.msgCtx)

		if s.repeatedMailErrs > s.endp.maxLoggedRcptErrors {
			s.log.Msg("MAIL FROM repeated error a lot of times, possible dictonary attack", "count", s.repeatedMailErrs, "src_ip", s.connState.RemoteAddr)
		}
	}
	if s.cancelRDNS != nil {
		s.cancelRDNS()
	}
	return nil
}

func (s *Session) prepareBody(ctx context.Context, r io.Reader) (textproto.Header, buffer.Buffer, error) {
	bufr := bufio.NewReader(r)
	header, err := textproto.ReadHeader(bufr)
	if err != nil {
		return textproto.Header{}, nil, err
	}

	if s.endp.submission {
		// The MsgMetadata is passed by pointer all the way down.
		if err := s.submissionPrepare(s.msgMeta, &header); err != nil {
			return textproto.Header{}, nil, err
		}
	}

	buf, err := s.endp.buffer(bufr)
	if err != nil {
		return textproto.Header{}, nil, err
	}

	received, err := target.GenerateReceived(ctx, s.msgMeta, s.endp.hostname, s.msgMeta.OriginalFrom)
	if err != nil {
		return textproto.Header{}, nil, err
	}
	header.Add("Received", received)

	return header, buf, nil
}

func (s *Session) Data(r io.Reader) error {
	bodyCtx, bodyTask := trace.NewTask(s.msgCtx, "DATA")
	defer bodyTask.End()

	wrapErr := func(err error) error {
		s.log.Error("DATA error", err, "msg_id", s.msgMeta.ID)
		return s.endp.wrapErr(s.msgMeta.ID, !s.opts.UTF8, err)
	}

	header, buf, err := s.prepareBody(bodyCtx, r)
	if err != nil {
		return wrapErr(err)
	}
	defer func() {
		if err := buf.Remove(); err != nil {
			s.log.Error("failed to remove buffered body", err)
		}

		// go-smtp will call Reset, but it will call Abort if delivery is non-nil.
		s.delivery = nil
		s.msgCtx = nil
		s.msgTask.End()
		s.msgTask = nil
		s.releaseLimits()
	}()

	if err := s.delivery.Body(bodyCtx, header, buf); err != nil {
		return wrapErr(err)
	}

	if err := s.delivery.Commit(bodyCtx); err != nil {
		return wrapErr(err)
	}

	s.log.Msg("accepted", "msg_id", s.msgMeta.ID)

	return nil
}

type statusWrapper struct {
	sc smtp.StatusCollector
	s  *Session
}

func (sw statusWrapper) SetStatus(rcpt string, err error) {
	sw.sc.SetStatus(rcpt, sw.s.endp.wrapErr(sw.s.msgMeta.ID, !sw.s.opts.UTF8, err))
}

func (s *Session) LMTPData(r io.Reader, sc smtp.StatusCollector) error {
	bodyCtx, bodyTask := trace.NewTask(s.msgCtx, "DATA")
	defer bodyTask.End()

	wrapErr := func(err error) error {
		s.log.Error("DATA error", err, "msg_id", s.msgMeta.ID)
		return s.endp.wrapErr(s.msgMeta.ID, !s.opts.UTF8, err)
	}

	header, buf, err := s.prepareBody(bodyCtx, r)
	if err != nil {
		return wrapErr(err)
	}
	defer func() {
		if err := buf.Remove(); err != nil {
			s.log.Error("failed to remove buffered body", err)
		}

		// go-smtp will call Reset, but it will call Abort if delivery is non-nil.
		s.delivery = nil
		s.msgCtx = nil
		s.msgTask.End()
		s.msgTask = nil
		s.releaseLimits()
	}()

	s.delivery.(module.PartialDelivery).BodyNonAtomic(bodyCtx, statusWrapper{sc, s}, header, buf)

	// We can't really tell whether it is failed completely or succeeded
	// so always commit. Should be harmless, anyway.
	if err := s.delivery.Commit(bodyCtx); err != nil {
		return wrapErr(err)
	}

	s.log.Msg("accepted", "msg_id", s.msgMeta.ID)

	return nil
}

func (endp *Endpoint) wrapErr(msgId string, mangleUTF8 bool, err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return &smtp.SMTPError{
			Code:         451,
			EnhancedCode: smtp.EnhancedCode{4, 4, 5},
			Message:      "High load, try again later",
		}
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

	// INTERNATIONALIZATION: See RFC 6531 Section 3.7.4.1.
	if mangleUTF8 {
		b := strings.Builder{}
		b.Grow(len(res.Message))
		for _, ch := range res.Message {
			if ch > 128 {
				b.WriteRune('?')
			} else {
				b.WriteRune(ch)
			}
		}
		res.Message = b.String()
	}

	return res
}

type Endpoint struct {
	hostname  string
	Auth      module.AuthProvider
	serv      *smtp.Server
	name      string
	addrs     []string
	listeners []net.Listener
	pipeline  *msgpipeline.MsgPipeline
	resolver  dns.Resolver
	limits    *limits.Group

	buffer func(r io.Reader) (buffer.Buffer, error)

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
		resolver:   dns.DefaultResolver(),
		buffer:     buffer.BufferInMemory,
		Log:        log.Logger{Name: modName},
	}
	return endp, nil
}

func (endp *Endpoint) Init(cfg *config.Map) error {
	endp.serv = smtp.NewServer(endp)
	endp.serv.ErrorLog = endp.Log
	endp.serv.LMTP = endp.lmtp
	endp.serv.EnableSMTPUTF8 = true
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

func autoBufferMode(maxSize int, dir string) func(io.Reader) (buffer.Buffer, error) {
	return func(r io.Reader) (buffer.Buffer, error) {
		// First try to read up to N bytes.
		initial := make([]byte, maxSize)
		actualSize, err := io.ReadFull(r, initial)
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				// Ok, the message is smaller than N. Make a MemoryBuffer and
				// handle it in RAM.
				log.Debugln("autobuffer: keeping the message in RAM")
				return buffer.MemoryBuffer{Slice: initial[:actualSize]}, nil
			}
			// Some I/O error happened, bail out.
			return nil, err
		}

		log.Debugln("autobuffer: spilling the message to the FS")
		// The message is big. Dump what we got to the disk and continue writing it there.
		return buffer.BufferInFile(
			io.MultiReader(bytes.NewReader(initial[:actualSize]), r),
			dir)
	}
}

func bufferModeDirective(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) < 1 {
		return nil, m.MatchErr("at least one argument required")
	}
	switch node.Args[0] {
	case "ram":
		if len(node.Args) > 1 {
			return nil, m.MatchErr("no additional arguments for 'ram' mode")
		}
		return buffer.BufferInMemory, nil
	case "fs":
		path := filepath.Join(config.StateDirectory, "buffer")
		switch len(node.Args) {
		case 2:
			path = node.Args[1]
			fallthrough
		case 1:
			return func(r io.Reader) (buffer.Buffer, error) {
				return buffer.BufferInFile(r, path)
			}, nil
		default:
			return nil, m.MatchErr("too many arguments for 'fs' mode")
		}
	case "auto":
		path := filepath.Join(config.StateDirectory, "buffer")
		if err := os.MkdirAll(path, 0700); err != nil {
			return nil, err
		}

		maxSize := 1 * 1024 * 1024 // 1 MiB
		switch len(node.Args) {
		case 3:
			path = node.Args[2]
			fallthrough
		case 2:
			var err error
			maxSize, err = config.ParseDataSize(node.Args[1])
			if err != nil {
				return nil, m.MatchErr("%v", err)
			}
			fallthrough
		case 1:
			return autoBufferMode(maxSize, path), nil
		default:
			return nil, m.MatchErr("too many arguments for 'auto' mode")
		}
	default:
		return nil, m.MatchErr("unknown buffer mode: %v", node.Args[0])
	}
}

func (endp *Endpoint) setConfig(cfg *config.Map) error {
	var (
		err     error
		ioDebug bool
	)

	cfg.Custom("auth", false, false, nil, modconfig.AuthDirective, &endp.Auth)
	cfg.String("hostname", true, true, "", &endp.hostname)
	cfg.Duration("write_timeout", false, false, 1*time.Minute, &endp.serv.WriteTimeout)
	cfg.Duration("read_timeout", false, false, 10*time.Minute, &endp.serv.ReadTimeout)
	cfg.DataSize("max_message_size", false, false, 32*1024*1024, &endp.serv.MaxMessageBytes)
	cfg.Int("max_recipients", false, false, 20000, &endp.serv.MaxRecipients)
	cfg.Custom("buffer", false, false, func() (interface{}, error) {
		path := filepath.Join(config.StateDirectory, "buffer")
		if err := os.MkdirAll(path, 0700); err != nil {
			return nil, err
		}
		return autoBufferMode(1*1024*1024 /* 1 MiB */, path), nil
	}, bufferModeDirective, &endp.buffer)
	cfg.Custom("tls", true, true, nil, config.TLSDirective, &endp.serv.TLSConfig)
	cfg.Bool("insecure_auth", false, false, &endp.serv.AllowInsecureAuth)
	cfg.Bool("io_debug", false, false, &ioDebug)
	cfg.Bool("debug", true, false, &endp.Log.Debug)
	cfg.Bool("defer_sender_reject", false, true, &endp.deferServerReject)
	cfg.Int("max_logged_rcpt_errors", false, false, 5, &endp.maxLoggedRcptErrors)
	cfg.Custom("limits", false, false, func() (interface{}, error) {
		return &limits.Group{}, nil
	}, func(cfg *config.Map, n *config.Node) (interface{}, error) {
		var g *limits.Group
		if err := modconfig.GroupFromNode("limits", n.Args, n, cfg.Globals, &g); err != nil {
			return nil, err
		}
		return g, nil
	}, &endp.limits)
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

	// INTERNATIONALIZATION: See RFC 6531 Section 3.3.
	endp.serv.Domain, err = idna.ToASCII(endp.hostname)
	if err != nil {
		return fmt.Errorf("%s: cannot represent the hostname as an A-label name: %w", endp.name, err)
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
			if err := endp.serv.Serve(l); err != nil {
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
	if err := endp.pipeline.RunEarlyChecks(context.TODO(), state); err != nil {
		return nil, endp.wrapErr("", true, err)
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
	if err := endp.pipeline.RunEarlyChecks(context.TODO(), state); err != nil {
		return nil, endp.wrapErr("", true, err)
	}

	return endp.newSession(true, "", "", state), nil
}

func (endp *Endpoint) newSession(anonymous bool, username, password string, state *smtp.ConnectionState) smtp.Session {
	s := &Session{
		endp: endp,
		log:  endp.Log,
		connState: module.ConnState{
			ConnectionState: *state,
			AuthUser:        username,
			AuthPassword:    password,
		},
		sessionCtx: context.Background(),
	}

	if endp.serv.LMTP {
		s.connState.Proto = "LMTP"
	} else {
		// Check if TLS connection state struct is poplated.
		// If it is - we are ssing TLS.
		if state.TLS.HandshakeComplete {
			s.connState.Proto = "ESMTPS"
		} else {
			s.connState.Proto = "ESMTP"
		}
	}

	if endp.resolver != nil {
		rdnsCtx, cancelRDNS := context.WithCancel(s.sessionCtx)
		s.connState.RDNSName = future.New()
		s.cancelRDNS = cancelRDNS
		go s.fetchRDNSName(rdnsCtx)
	}

	return s
}

func (endp *Endpoint) Close() error {
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
