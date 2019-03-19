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
	ctx := module.DeliveryContext{}
	ctx.From = from
	ctx.To = to
	ctx.Anonymous = u.anonymous
	ctx.AuthUser = u.username
	ctx.SrcHostname = u.state.Hostname
	ctx.SrcAddr = u.state.RemoteAddr
	ctx.OurHostname = u.endp.domain
	ctx.Ctx = make(map[string]interface{})

	currentMsg := r
	var buf bytes.Buffer
	for _, step := range u.endp.pipeline {
		r, cont, err := step.Pass(&ctx, currentMsg)
		if !cont {
			if err == module.ErrSilentDrop {
				return nil
			}
			if err != nil {
				return err
			}
		}

		if r != nil {
			if _, err := io.Copy(&buf, r); err != nil {
				return err
			}
			currentMsg = &buf
		}
	}
	return nil
}

func (u SMTPUser) Logout() error {
	return nil
}

type SMTPEndpoint struct {
	Auth     module.AuthProvider
	serv     *smtp.Server
	name     string
	domain   string
	pipeline []SMTPPipelineStep
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

func NewSMTPEndpoint(instName string, cfg config.CfgTreeNode) (module.Module, error) {
	endp := new(SMTPEndpoint)
	endp.name = instName
	var (
		err     error
		tlsConf *tls.Config
	)
	maxIdle := -1
	maxMsgBytes := -1

	insecureAuth := false
	ioDebug := false
	for _, entry := range cfg.Childrens {
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
		case "maxidle":
			if len(entry.Args) != 1 {
				return nil, errors.New("maxidle: expected 1 argument")
			}
			maxIdle, err = strconv.Atoi(entry.Args[0])
			if err != nil {
				return nil, errors.New("maxidle: invalid integer value")
			}
		case "maxmsgbytes":
			if len(entry.Args) != 1 {
				return nil, errors.New("maxmsgbytes: expected 1 argument")
			}
			maxMsgBytes, err = strconv.Atoi(entry.Args[0])
			if err != nil {
				return nil, errors.New("maxmsgbytes: invalid integer value")
			}
		case "tls":
			if err := setTLS(entry.Args, &tlsConf); err != nil {
				return nil, err
			}
		case "insecureauth":
			log.Printf("smtp %v: insecure auth is on!\n", instName)
			insecureAuth = true
		case "iodebug":
			log.Printf("smtp %v: I/O debugging is on!\n", instName)
			ioDebug = true
		case "filter", "delivery", "match", "stop", "require-auth":
			step, err := StepFromCfg(entry)
			if err != nil {
				return nil, err
			}
			endp.pipeline = append(endp.pipeline, step)
		default:
			return nil, fmt.Errorf("unknown config directive: %v", entry.Name)
		}
	}

	if endp.domain == "" {
		return nil, fmt.Errorf("hostname is not set")
	}

	if len(endp.pipeline) == 0 {
		log.Printf("smtp %s: using default pipeline configuration")

		localDelivery, err := deliveryTarget([]string{"default-local-delivery"})
		if err != nil {
			localDelivery, err = deliveryTarget([]string{"default"})
			if err != nil {
				return nil, errors.New("missing default local delivery target, must set custom")
			}
		}

		remoteDelivery, err := deliveryTarget([]string{"default-remote-delivery"})
		if err != nil {
			return nil, errors.New("missing default remote delivery target, must set custom")
		}

		endp.pipeline = append(endp.pipeline,
			deliveryStep{t: localDelivery, opts: map[string]string{"local-only": ""}},
			deliveryStep{t: remoteDelivery, opts: map[string]string{"remote-only": ""}},
		)
	}

	if endp.Auth == nil {
		endp.Auth, err = authProvider([]string{"default-auth"})
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

	var addresses []Address
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

	for _, addr := range addresses {
		var l net.Listener
		var err error
		l, err = net.Listen(addr.Network(), addr.Address())
		if err != nil {
			return nil, fmt.Errorf("failed to bind on %v: %v", addr, err)
		}
		log.Printf("smtp: listening on %v\n", addr)

		if addr.IsTLS() {
			l = tls.NewListener(l, tlsConf)
		}

		go func() {
			module.WaitGroup.Add(1)
			if err := endp.serv.Serve(l); err != nil {
				log.Printf("smtp: failed to serve %v: %v\n", addr, err)
			}
			module.WaitGroup.Done()
		}()
	}

	return endp, nil
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
	endp.serv.Close()
	return nil
}

func init() {
	module.Register("smtp", NewSMTPEndpoint)
}
