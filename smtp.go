package maddy

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/config"
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
		SrcHostname: u.state.Hostname,
		SrcAddr:     u.state.RemoteAddr,
		OurHostname: u.endp.domain,
		Ctx:         make(map[string]interface{}),
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
				log.Printf("smtp %s: step failed: %v", u.endp.name, err)
				return err
			}
		}

		if r != nil && r != io.Reader(currentMsg) {
			buf.Reset()
			if _, err := io.Copy(&buf, r); err != nil {
				log.Printf("smtp %s: failed to buffer message: %v", u.endp.name, err)
				return err
			}
			currentMsg.Reset(buf.Bytes())
		}
		currentMsg.Seek(0, io.SeekStart)
	}
	return nil
}

func (u SMTPUser) Logout() error {
	return nil
}

type SMTPEndpoint struct {
	Auth      module.AuthProvider
	serv      *smtp.Server
	name      string
	domain    string
	listeners []net.Listener
	pipeline  []SMTPPipelineStep

	listenersWg sync.WaitGroup
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

func NewSMTPEndpoint(instName string, cfg config.Node) (module.Module, error) {
	endp := new(SMTPEndpoint)
	endp.name = instName
	var (
		err     error
		tlsConf *tls.Config
		tlsSet  bool

		localDeliveryDefault  string
		localDeliveryOpts     map[string]string
		remoteDeliveryDefault string
		remoteDeliveryOpts    map[string]string
	)
	maxIdle := -1
	maxMsgBytes := -1

	insecureAuth := false
	ioDebug := false
	for _, entry := range cfg.Children {
		switch entry.Name {
		case "auth":
			endp.Auth, err = authProvider(entry.Args)
			if err != nil {
				return nil, err
			}
		case "hostname":
			if len(entry.Args) != 1 {
				return nil, errors.New("hostname: expected 1 argument")
			}
			endp.domain = entry.Args[0]
		case "max_idle":
			if len(entry.Args) != 1 {
				return nil, errors.New("max_idle: expected 1 argument")
			}
			maxIdle, err = strconv.Atoi(entry.Args[0])
			if err != nil {
				return nil, errors.New("max_idle: invalid integer value")
			}
		case "max_message_size":
			if len(entry.Args) != 1 {
				return nil, errors.New("max_message_size: expected 1 argument")
			}
			maxMsgBytes, err = strconv.Atoi(entry.Args[0])
			if err != nil {
				return nil, errors.New("max_message_size: invalid integer value")
			}
		case "tls":
			tlsConf = new(tls.Config)
			if err := setTLS(entry.Args, &tlsConf); err != nil {
				return nil, err
			}
			tlsSet = true
			if tlsConf == nil {
				log.Printf("smtp %s: TLS is disabled, this is insecure configuration and should be used only for testing!", instName)
				insecureAuth = true
			}
		case "insecure_auth":
			log.Printf("smtp %s: authentication over unencrypted connections is allowed, this is insecure configuration and should be used only for testing!", instName)
			insecureAuth = true
		case "io_debug":
			log.Printf("smtp %v: I/O debugging is on!\n", instName)
			ioDebug = true
		case "local_delivery":
			if len(entry.Args) == 0 {
				return nil, errors.New("local_delivery: expected at least 1 argument")
			}
			if len(endp.pipeline) != 0 {
				return nil, errors.New("can't use custom pipeline with local_delivery or remote_delivery")
			}

			localDeliveryDefault = entry.Args[0]
			localDeliveryOpts = readOpts(entry.Args[1:])
		case "remote_delivery":
			if len(entry.Args) == 0 {
				return nil, errors.New("remote_delivery: expected at least 1 argument")
			}
			if len(endp.pipeline) != 0 {
				return nil, errors.New("can't use custom pipeline with local_delivery or remote_delivery")
			}

			remoteDeliveryDefault = entry.Args[0]
			remoteDeliveryOpts = readOpts(entry.Args[1:])
		case "filter", "delivery", "match", "stop", "require_auth":
			if localDeliveryDefault != "" || remoteDeliveryDefault != "" {
				return nil, errors.New("can't use custom pipeline with local_delivery or remote_delivery")
			}

			step, err := StepFromCfg(entry)
			if err != nil {
				return nil, err
			}
			endp.pipeline = append(endp.pipeline, step)
		default:
			return nil, fmt.Errorf("unknown config directive: %v", entry.Name)
		}
	}

	if !tlsSet {
		return nil, errors.New("TLS is not configured")
	}

	if endp.domain == "" {
		return nil, fmt.Errorf("hostname is not set")
	}

	if len(endp.pipeline) == 0 {
		err := endp.setDefaultPipeline(localDeliveryDefault, remoteDeliveryDefault, localDeliveryOpts, remoteDeliveryOpts)
		if err != nil {
			return nil, err
		}
	}

	if endp.Auth == nil {
		endp.Auth, err = authProvider([]string{"default_auth"})
		if err != nil {
			endp.Auth, err = authProvider([]string{"default"})
			if err != nil {
				return nil, errors.New("missing default auth. provider, must set custom")
			}
		}
		log.Printf("smtp %s: using %s auth. provider (%s %s)",
			instName, endp.Auth.(module.Module).InstanceName(),
			endp.Auth.(module.Module).Name(), endp.Auth.(module.Module).Version(),
		)
	}

	addresses := make([]Address, 0, len(cfg.Args))
	for _, addr := range cfg.Args {
		saddr, err := standardizeAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address: %s", instName)
		}
		addresses = append(addresses, saddr)
	}

	endp.serv = smtp.NewServer(endp)
	endp.serv.AllowInsecureAuth = insecureAuth
	endp.serv.Domain = endp.domain
	if maxMsgBytes != -1 {
		endp.serv.MaxMessageBytes = maxMsgBytes
	}
	if maxIdle != -1 {
		endp.serv.MaxIdleSeconds = maxIdle
	}
	if ioDebug {
		endp.serv.Debug = os.Stderr
	}

	if err := endp.setupListeners(addresses, tlsConf); err != nil {
		return nil, err
	}
	return endp, nil
}

func (endp *SMTPEndpoint) setupListeners(addresses []Address, tlsConf *tls.Config) error {
	for _, addr := range addresses {
		var l net.Listener
		var err error
		l, err = net.Listen(addr.Network(), addr.Address())
		if err != nil {
			return fmt.Errorf("failed to bind on %v: %v", addr, err)
		}
		log.Printf("smtp: listening on %v\n", addr)

		if addr.IsTLS() {
			l = tls.NewListener(l, tlsConf)
		}

		endp.listeners = append(endp.listeners, l)

		endp.listenersWg.Add(1)
		addr := addr
		go func() {
			if err := endp.serv.Serve(l); err != nil && !strings.HasSuffix(err.Error(), "use of closed network connection") {
				log.Printf("smtp: failed to serve %s: %s\n", addr, err)
			}
			endp.listenersWg.Done()
		}()
	}
	return nil
}

func (endp *SMTPEndpoint) setDefaultPipeline(localDeliveryName, remoteDeliveryName string, localOpts, remoteOpts map[string]string) error {
	log.Printf("smtp %s: using default pipeline configuration", endp.InstanceName())

	var err error
	var localDelivery module.DeliveryTarget
	if localDeliveryName == "" {
		localDelivery, err = deliveryTarget([]string{"default_local_delivery"})
		if err != nil {
			localDelivery, err = deliveryTarget([]string{"default"})
			if err != nil {
				return err
			}
		}
		localOpts = map[string]string{"local_only": ""}
	} else {
		localDelivery, err = deliveryTarget([]string{localDeliveryName})
		if err != nil {
			return err
		}
	}

	if remoteDeliveryName == "" {
		remoteDeliveryName = "default_remote_delivery"
		remoteOpts = map[string]string{"remote_only": ""}
	}

	remoteDelivery, err := deliveryTarget([]string{remoteDeliveryName})
	if err != nil {
		return err
	}

	endp.pipeline = append(endp.pipeline,
		deliveryStep{t: localDelivery, opts: localOpts},
		deliveryStep{t: remoteDelivery, opts: remoteOpts},
	)

	return nil
}

func (endp *SMTPEndpoint) Login(state *smtp.ConnectionState, username, password string) (smtp.User, error) {
	if !endp.Auth.CheckPlain(username, password) {
		return nil, errors.New("smtp: invalid credentials")
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
}
