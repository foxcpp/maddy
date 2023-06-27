package ldap

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/foxcpp/maddy/framework/config"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/go-ldap/ldap/v3"
)

const modName = "auth.ldap"

type Auth struct {
	instName string

	urls           []string
	readBind       func(*ldap.Conn) error
	startls        bool
	tlsCfg         tls.Config
	dialer         *net.Dialer
	requestTimeout time.Duration

	dnTemplate string
	// or
	baseDN         string
	filterTemplate string

	conn     *ldap.Conn
	connLock sync.Mutex

	log log.Logger
}

func New(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	return &Auth{
		instName: instName,
		log:      log.Logger{Name: modName},
		urls:     inlineArgs,
	}, nil
}

func (a *Auth) Init(cfg *config.Map) error {
	a.dialer = &net.Dialer{}

	cfg.Bool("debug", true, false, &a.log.Debug)
	cfg.Custom("tls_client", true, false, func() (interface{}, error) {
		return tls.Config{}, nil
	}, tls2.TLSClientBlock, &a.tlsCfg)
	cfg.Callback("urls", func(m *config.Map, node config.Node) error {
		a.urls = append(a.urls, node.Args...)
		return nil
	})
	cfg.Custom("bind", false, false, func() (interface{}, error) {
		return func(*ldap.Conn) error {
			return nil
		}, nil
	}, readBindDirective, &a.readBind)
	cfg.Bool("starttls", false, false, &a.startls)
	cfg.Duration("connect_timeout", false, false, time.Minute, &a.dialer.Timeout)
	cfg.Duration("request_timeout", false, false, time.Minute, &a.requestTimeout)
	cfg.String("dn_template", false, false, "", &a.dnTemplate)
	cfg.String("base_dn", false, false, "", &a.baseDN)
	cfg.String("filter", false, false, "", &a.filterTemplate)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if a.dnTemplate == "" {
		if a.baseDN == "" {
			return fmt.Errorf("auth.ldap: base_dn not set")
		}
		if a.filterTemplate == "" {
			return fmt.Errorf("auth.ldap: filter not set")
		}
	} else {
		if a.baseDN != "" || a.filterTemplate != "" {
			return fmt.Errorf("auth.ldap: search directives set when dn_template is used")
		}
	}

	if module.NoRun {
		return nil
	}

	var err error
	a.conn, err = a.newConn()
	if err != nil {
		return fmt.Errorf("auth.ldap: %w", err)
	}
	return nil
}

func readBindDirective(c *config.Map, n config.Node) (interface{}, error) {
	if len(n.Args) == 0 {
		return nil, fmt.Errorf("auth.ldap: auth expects at least one argument")
	}
	switch n.Args[0] {
	case "off":
		return func(*ldap.Conn) error { return nil }, nil
	case "unauth":
		if len(n.Args) == 2 {
			return func(c *ldap.Conn) error {
				return c.UnauthenticatedBind(n.Args[1])
			}, nil
		}
		return func(c *ldap.Conn) error {
			return c.UnauthenticatedBind("")
		}, nil
	case "plain":
		if len(n.Args) != 3 {
			return nil, fmt.Errorf("auth.ldap: username and password expected for plaintext bind")
		}
		return func(c *ldap.Conn) error {
			return c.Bind(n.Args[1], n.Args[2])
		}, nil
	case "external":
		return (*ldap.Conn).ExternalBind, nil
	}
	return nil, fmt.Errorf("auth.ldap: unknown bind authentication: %v", n.Args[0])
}

func (a *Auth) Name() string {
	return modName
}

func (a *Auth) InstanceName() string {
	return a.instName
}

func (a *Auth) newConn() (*ldap.Conn, error) {
	var (
		conn   *ldap.Conn
		tlsCfg *tls.Config
	)
	for _, u := range a.urls {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("auth.ldap: invalid server URL: %w", err)
		}
		hostname := parsedURL.Host
		tlsCfg = a.tlsCfg.Clone()
		a.tlsCfg.ServerName = hostname

		conn, err = ldap.DialURL(u, ldap.DialWithDialer(a.dialer), ldap.DialWithTLSConfig(tlsCfg))
		if err != nil {
			a.log.Error("cannot contact directory server", err, "url", u)
			continue
		}
		break
	}
	if conn == nil {
		return nil, fmt.Errorf("auth.ldap: all directory servers are unreachable")
	}

	if a.requestTimeout != 0 {
		conn.SetTimeout(a.requestTimeout)
	}

	if a.startls {
		if err := conn.StartTLS(tlsCfg); err != nil {
			return nil, fmt.Errorf("auth.ldap: %w", err)
		}
	}

	if err := a.readBind(conn); err != nil {
		return nil, fmt.Errorf("auth.ldap: %w", err)
	}

	return conn, nil
}

func (a *Auth) getConn() (*ldap.Conn, error) {
	a.connLock.Lock()
	if a.conn == nil {
		conn, err := a.newConn()
		if err != nil {
			a.connLock.Unlock()
			return nil, err
		}
		a.conn = conn
	}
	if a.conn.IsClosing() {
		a.conn.Close()
		conn, err := a.newConn()
		if err != nil {
			a.connLock.Unlock()
			return nil, err
		}
		a.conn = conn
	}
	return a.conn, nil
}

func (a *Auth) returnConn(conn *ldap.Conn) {
	defer a.connLock.Unlock()
	if err := a.readBind(conn); err != nil {
		a.log.Error("failed to rebind for reading", err)
		conn.Close()
		a.conn = nil
	}
	if a.conn != conn {
		a.conn.Close()
	}
	a.conn = conn
}

func (a *Auth) Lookup(_ context.Context, username string) (string, bool, error) {
	conn, err := a.getConn()
	if err != nil {
		return "", false, err
	}
	defer a.returnConn(conn)

	var userDN string
	if a.dnTemplate != "" {
		return "", false, fmt.Errorf("auth.ldap: lookups require search config but dn_template is used")
	} else {
		req := ldap.NewSearchRequest(
			a.baseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
			2, 0, false,
			strings.ReplaceAll(a.filterTemplate, "{username}", username),
			[]string{"dn"}, nil)
		res, err := conn.Search(req)
		if err != nil {
			return "", false, fmt.Errorf("auth.ldap: search: %w", err)
		}
		if len(res.Entries) > 1 {
			return "", false, fmt.Errorf("auth.ldap: too manu entries returned (%d)", len(res.Entries))
		}
		if len(res.Entries) == 0 {
			return "", false, nil
		}
		userDN = res.Entries[0].DN
	}

	return userDN, true, nil
}

func (a *Auth) AuthPlain(username, password string) error {
	conn, err := a.getConn()
	if err != nil {
		return err
	}
	defer a.returnConn(conn)

	var userDN string
	if a.dnTemplate != "" {
		userDN = strings.ReplaceAll(a.dnTemplate, "{username}", username)
	} else {
		req := ldap.NewSearchRequest(
			a.baseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
			2, 0, false,
			strings.ReplaceAll(a.filterTemplate, "{username}", username),
			[]string{"dn"}, nil)
		res, err := conn.Search(req)
		if err != nil {
			return fmt.Errorf("auth.ldap: search: %w", err)
		}
		if len(res.Entries) > 1 {
			return fmt.Errorf("auth.ldap: too manu entries returned (%d)", len(res.Entries))
		}
		if len(res.Entries) == 0 {
			return module.ErrUnknownCredentials
		}
		userDN = res.Entries[0].DN
	}

	if err := conn.Bind(userDN, password); err != nil {
		return module.ErrUnknownCredentials
	}

	return nil
}

func init() {
	var _ module.PlainAuth = &Auth{}
	var _ module.Table = &Auth{}
	module.Register(modName, New)
	module.Register("table.ldap", New)
}
