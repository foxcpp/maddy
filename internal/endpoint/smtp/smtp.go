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

package smtp

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/future"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/auth"
	"github.com/foxcpp/maddy/internal/authz"
	"github.com/foxcpp/maddy/internal/limits"
	"github.com/foxcpp/maddy/internal/msgpipeline"
	"golang.org/x/net/idna"
)

type Endpoint struct {
	saslAuth  auth.SASLAuth
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
	maxReceived         int
	maxHeaderBytes      int

	authNormalize authz.NormalizeFunc
	authMap       module.Table

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
		saslAuth: auth.SASLAuth{
			Log: log.Logger{Name: modName + "/sasl"},
		},
	}
	return endp, nil
}

func (endp *Endpoint) Init(cfg *config.Map) error {
	endp.serv = smtp.NewServer(endp)
	endp.serv.ErrorLog = endp.Log
	endp.serv.LMTP = endp.lmtp
	endp.serv.EnableSMTPUTF8 = true
	endp.serv.EnableREQUIRETLS = true
	if err := endp.setConfig(cfg); err != nil {
		return err
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
				log.Debugln("autobuffer: keeping the message in RAM (read", actualSize, "bytes, got EOF)")
				return buffer.MemoryBuffer{Slice: initial[:actualSize]}, nil
			}
			if err == io.EOF {
				// Special case: message with empty body.
				return buffer.MemoryBuffer{}, nil
			}
			// Some I/O error happened, bail out.
			return nil, err
		}
		if actualSize < maxSize {
			// Ok, the message is smaller than N. Make a MemoryBuffer and
			// handle it in RAM.
			log.Debugln("autobuffer: keeping the message in RAM (read", actualSize, "bytes, got short read)")
			return buffer.MemoryBuffer{Slice: initial[:actualSize]}, nil
		}

		log.Debugln("autobuffer: spilling the message to the FS")
		// The message is big. Dump what we got to the disk and continue writing it there.
		return buffer.BufferInFile(
			io.MultiReader(bytes.NewReader(initial[:actualSize]), r),
			dir)
	}
}

func bufferModeDirective(_ *config.Map, node config.Node) (interface{}, error) {
	if len(node.Args) < 1 {
		return nil, config.NodeErr(node, "at least one argument required")
	}
	switch node.Args[0] {
	case "ram":
		if len(node.Args) > 1 {
			return nil, config.NodeErr(node, "no additional arguments for 'ram' mode")
		}
		return buffer.BufferInMemory, nil
	case "fs":
		path := filepath.Join(config.StateDirectory, "buffer")
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, err
		}
		switch len(node.Args) {
		case 2:
			path = node.Args[1]
			fallthrough
		case 1:
			return func(r io.Reader) (buffer.Buffer, error) {
				return buffer.BufferInFile(r, path)
			}, nil
		default:
			return nil, config.NodeErr(node, "too many arguments for 'fs' mode")
		}
	case "auto":
		path := filepath.Join(config.StateDirectory, "buffer")
		if err := os.MkdirAll(path, 0o700); err != nil {
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
				return nil, config.NodeErr(node, "%v", err)
			}
			fallthrough
		case 1:
			return autoBufferMode(maxSize, path), nil
		default:
			return nil, config.NodeErr(node, "too many arguments for 'auto' mode")
		}
	default:
		return nil, config.NodeErr(node, "unknown buffer mode: %v", node.Args[0])
	}
}

func (endp *Endpoint) setConfig(cfg *config.Map) error {
	var (
		hostname string
		err      error
		ioDebug  bool
	)

	cfg.Callback("auth", func(m *config.Map, node config.Node) error {
		return endp.saslAuth.AddProvider(m, node)
	})
	cfg.String("hostname", true, true, "", &hostname)
	config.EnumMapped(cfg, "auth_map_normalize", true, false, authz.NormalizeFuncs, authz.NormalizeAuto,
		&endp.authNormalize)
	modconfig.Table(cfg, "auth_map", true, false, nil, &endp.authMap)
	cfg.Duration("write_timeout", false, false, 1*time.Minute, &endp.serv.WriteTimeout)
	cfg.Duration("read_timeout", false, false, 10*time.Minute, &endp.serv.ReadTimeout)
	cfg.DataSize("max_message_size", false, false, 32*1024*1024, &endp.serv.MaxMessageBytes)
	cfg.DataSize("max_header_size", false, false, 1*1024*1024, &endp.maxHeaderBytes)
	cfg.Int("max_recipients", false, false, 20000, &endp.serv.MaxRecipients)
	cfg.Int("max_received", false, false, 50, &endp.maxReceived)
	cfg.Custom("buffer", false, false, func() (interface{}, error) {
		path := filepath.Join(config.StateDirectory, "buffer")
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, err
		}
		return autoBufferMode(1*1024*1024 /* 1 MiB */, path), nil
	}, bufferModeDirective, &endp.buffer)
	cfg.Custom("tls", true, endp.name != "lmtp", nil, tls2.TLSDirective, &endp.serv.TLSConfig)
	cfg.Bool("insecure_auth", endp.name == "lmtp", false, &endp.serv.AllowInsecureAuth)
	cfg.Bool("reverse_proxy_mode", true, false, &endp.serv.ReverseProxyMode)
	cfg.Int("smtp_max_line_length", false, false, 4000, &endp.serv.MaxLineLength)
	cfg.Bool("io_debug", false, false, &ioDebug)
	cfg.Bool("debug", true, false, &endp.Log.Debug)
	cfg.Bool("defer_sender_reject", false, true, &endp.deferServerReject)
	cfg.Int("max_logged_rcpt_errors", false, false, 5, &endp.maxLoggedRcptErrors)
	cfg.Custom("limits", false, false, func() (interface{}, error) {
		return &limits.Group{}, nil
	}, func(cfg *config.Map, n config.Node) (interface{}, error) {
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

	// INTERNATIONALIZATION: See RFC 6531 Section 3.3.
	endp.serv.Domain, err = idna.ToASCII(hostname)
	if err != nil {
		return fmt.Errorf("%s: cannot represent the hostname as an A-label name: %w", endp.name, err)
	}

	endp.pipeline, err = msgpipeline.New(cfg.Globals, unknown)
	if err != nil {
		return err
	}
	endp.pipeline.Hostname = endp.serv.Domain
	endp.pipeline.Resolver = endp.resolver
	endp.pipeline.Log = log.Logger{Name: "smtp/pipeline", Debug: endp.Log.Debug}
	endp.pipeline.FirstPipeline = true

	endp.serv.AuthDisabled = len(endp.saslAuth.SASLMechanisms()) == 0
	if endp.submission {
		endp.authAlwaysRequired = true
		if len(endp.saslAuth.SASLMechanisms()) == 0 {
			return fmt.Errorf("%s: auth. provider must be set for submission endpoint", endp.name)
		}
	}
	endp.saslAuth.AuthNormalize = endp.authNormalize
	endp.saslAuth.AuthMap = endp.authMap
	for _, mech := range endp.saslAuth.SASLMechanisms() {
		// The code below lacks handling to set AuthPassword. Don't
		// override sasl.Plain handler so Login() will be called as usual.
		if mech == sasl.Plain {
			continue
		}

		mech := mech

		endp.serv.EnableAuth(mech, func(c *smtp.Conn) sasl.Server {
			return endp.saslAuth.CreateSASL(mech, c.Conn().RemoteAddr(), func(id string) error {
				c.Session().(*Session).connState.AuthUser = id
				return nil
			})
		})
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

func (endp *Endpoint) usernameForAuth(ctx context.Context, saslUsername string) (string, error) {
	saslUsername, err := endp.authNormalize(saslUsername)
	if err != nil {
		return "", err
	}

	if endp.authMap == nil {
		return saslUsername, nil
	}

	mapped, ok, err := endp.authMap.Lookup(ctx, saslUsername)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", &smtp.SMTPError{
			Code:         535,
			EnhancedCode: smtp.EnhancedCode{5, 7, 8},
			Message:      "Invalid credentials",
		}
	}

	return mapped, nil
}

func (endp *Endpoint) NewSession(conn *smtp.Conn) (smtp.Session, error) {
	sess := endp.newSession(conn)

	// Executed before authentication and session initialization.
	if err := endp.pipeline.RunEarlyChecks(context.TODO(), &sess.connState); err != nil {
		if err := sess.Logout(); err != nil {
			endp.Log.Error("early checks logout failed", err)
		}
		return nil, endp.wrapErr("", true, "EHLO", err)
	}

	return sess, nil
}

func (endp *Endpoint) newSession(conn *smtp.Conn) *Session {
	s := &Session{
		endp:       endp,
		log:        endp.Log,
		sessionCtx: context.Background(),
	}

	// Used in tests.
	if conn == nil {
		return s
	}

	s.connState = module.ConnState{
		Hostname:   conn.Hostname(),
		LocalAddr:  conn.Conn().LocalAddr(),
		RemoteAddr: conn.Conn().RemoteAddr(),
	}
	if tlsState, ok := conn.TLSConnectionState(); ok {
		s.connState.TLS = tlsState
	}

	if endp.serv.LMTP {
		s.connState.Proto = "LMTP"
	} else {
		// Check if TLS connection conn struct is poplated.
		// If it is - we are ssing TLS.
		if s.connState.TLS.HandshakeComplete {
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
