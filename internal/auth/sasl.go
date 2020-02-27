package auth

import (
	"errors"
	"fmt"
	"net"

	"github.com/emersion/go-sasl"
	"github.com/foxcpp/maddy/internal/config"
	modconfig "github.com/foxcpp/maddy/internal/config/module"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"golang.org/x/text/secure/precis"
)

var (
	ErrUnsupportedMech = errors.New("Unsupported SASL mechanism")
)

// SASLAuth is a wrapper that initializes sasl.Server using authenticators that
// call maddy module objects.
//
// It supports reporting of multiple authorization identities so multiple
// accounts can be associated with a single set of credentials.
type SASLAuth struct {
	Log         log.Logger
	OnlyFirstID bool

	Plain []module.PlainAuth
}

func (s *SASLAuth) SASLMechanisms() []string {
	var mechs []string

	if len(s.Plain) != 0 {
		mechs = append(mechs, sasl.Plain, sasl.Login)
	}

	return mechs
}

func (s *SASLAuth) AuthPlain(username, password string) ([]string, error) {
	if len(s.Plain) == 0 {
		return nil, ErrUnsupportedMech
	}

	var lastErr error
	accounts := make([]string, 0, 1)
	for _, p := range s.Plain {
		pAccs, err := p.AuthPlain(username, password)
		if err != nil {
			lastErr = err
			continue
		}

		if s.OnlyFirstID {
			return pAccs, nil
		}
		accounts = append(accounts, pAccs...)
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no auth. provider accepted creds, last err: %w", lastErr)
	}
	return accounts, nil
}

func filterIdentity(accounts []string, identity string) ([]string, error) {
	if identity == "" {
		return accounts, nil
	}

	matchFound := false
	for _, acc := range accounts {
		if precis.UsernameCaseMapped.Compare(acc, identity) {
			accounts = []string{identity}
			matchFound = true
			break
		}
	}
	if !matchFound {
		return nil, errors.New("auth: invalid credentials")
	}
	return accounts, nil
}

// CreateSASL creates the sasl.Server instance for the corresponding mechanism.
//
// successCb will be called with the slice of authorization identities
// associated with credentials used.
// If it fails - authentication will fail too.
func (s *SASLAuth) CreateSASL(mech string, remoteAddr net.Addr, successCb func([]string) error) sasl.Server {
	switch mech {
	case sasl.Plain:
		return sasl.NewPlainServer(func(identity, username, password string) error {
			accounts, err := s.AuthPlain(username, password)
			if err != nil {
				s.Log.Error("authentication failed", err, "username", username, "identity", identity, "src_ip", remoteAddr)
				return errors.New("auth: invalid credentials")
			}
			if len(accounts) == 0 {
				accounts = []string{username}
			}
			accounts, err = filterIdentity(accounts, identity)
			if err != nil {
				s.Log.Error("not authorized", err, "username", username, "identity", identity, "src_ip", remoteAddr)
				return errors.New("auth: invalid credentials")
			}

			return successCb(accounts)
		})
	case sasl.Login:
		return sasl.NewLoginServer(func(username, password string) error {
			accounts, err := s.AuthPlain(username, password)
			if err != nil {
				s.Log.Error("authentication failed", err, "username", username, "src_ip", remoteAddr)
				return errors.New("auth: invalid credentials")
			}

			return successCb(accounts)
		})
	}
	return FailingSASLServ{Err: ErrUnsupportedMech}
}

// AddProvider adds the SASL authentication provider to its mapping by parsing
// the 'auth' configuration directive.
func (s *SASLAuth) AddProvider(m *config.Map, node *config.Node) error {
	mod, err := modconfig.SASLAuthDirective(m, node)
	if err != nil {
		return err
	}
	saslAuth := mod.(module.SASLProvider)
	for _, mech := range saslAuth.SASLMechanisms() {
		switch mech {
		case sasl.Login, sasl.Plain:
			plainAuth, ok := saslAuth.(module.PlainAuth)
			if !ok {
				return m.MatchErr("auth: provider does not implement PlainAuth even though it reports PLAIN/LOGIN mechanism")
			}
			s.Plain = append(s.Plain, plainAuth)
		default:
			return m.MatchErr("auth: unknown SASL mechanism")
		}
	}

	return nil
}

type FailingSASLServ struct{ Err error }

func (s FailingSASLServ) Next([]byte) ([]byte, bool, error) {
	return nil, true, s.Err
}
