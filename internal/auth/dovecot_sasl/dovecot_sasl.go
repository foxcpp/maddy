package dovecotsasl

import (
	"fmt"
	"net"

	"github.com/emersion/go-sasl"
	dovecotsasl "github.com/foxcpp/go-dovecot-sasl"
	"github.com/foxcpp/maddy/internal/auth"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
)

type Auth struct {
	instName       string
	serverEndpoint string
	log            log.Logger

	network string
	addr    string

	mechanisms map[string]dovecotsasl.Mechanism
}

const modName = "dovecot_sasl"

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	a := &Auth{
		instName: instName,
		log:      log.Logger{Name: modName, Debug: log.DefaultLogger.Debug},
	}

	switch len(inlineArgs) {
	case 0:
	case 1:
		a.serverEndpoint = inlineArgs[0]
	default:
		return nil, fmt.Errorf("%s: one or none arguments needed", modName)
	}

	return a, nil
}

func (a *Auth) Name() string {
	return modName
}

func (a *Auth) InstanceName() string {
	return a.instName
}

func (a *Auth) getConn() (*dovecotsasl.Client, error) {
	// TODO: Connection pooling
	conn, err := net.Dial(a.network, a.addr)
	if err != nil {
		return nil, fmt.Errorf("%s: unable to contact server: %v", modName, err)
	}

	cl, err := dovecotsasl.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("%s: unable to contact server: %v", modName, err)
	}

	return cl, nil
}

func (a *Auth) returnConn(cl *dovecotsasl.Client) {
	cl.Close()
}

func (a *Auth) Init(cfg *config.Map) error {
	cfg.String("endpoint", false, false, a.serverEndpoint, &a.serverEndpoint)
	if _, err := cfg.Process(); err != nil {
		return err
	}
	if a.serverEndpoint == "" {
		return fmt.Errorf("%s: missing server endpoint", modName)
	}

	endp, err := config.ParseEndpoint(a.serverEndpoint)
	if err != nil {
		return fmt.Errorf("%s: invalid server endpoint: %v", modName, err)
	}
	if endp.Path == "" {
		return fmt.Errorf("%s: unexpected path in endpoint ", modName)
	}

	// Dial once to check usability and also to get list of mechanisms.
	conn, err := net.Dial(endp.Scheme, endp.Address())
	if err != nil {
		return fmt.Errorf("%s: unable to contact server: %v", modName, err)
	}

	cl, err := dovecotsasl.NewClient(conn)
	if err != nil {
		return fmt.Errorf("%s: unable to contact server: %v", modName, err)
	}

	defer cl.Close()
	a.mechanisms = cl.ConnInfo().Mechs

	a.network = endp.Scheme
	a.addr = endp.Address()

	return nil
}

func (a *Auth) AuthPlain(username, password string) error {
	if _, ok := a.mechanisms[sasl.Plain]; ok {
		cl, err := a.getConn()
		if err != nil {
			return err
		}

		// Pretend it is SMTP even though we really don't know.
		// We also have no connection information to pass to the server...
		return cl.Do("SMTP", sasl.NewPlainClient("", username, password))
	}
	if _, ok := a.mechanisms[sasl.Login]; ok {
		cl, err := a.getConn()
		if err != nil {
			return err
		}

		// Pretend it is SMTP even though we really don't know.
		return cl.Do("SMTP", sasl.NewLoginClient(username, password))
	}

	return auth.ErrUnsupportedMech
}
