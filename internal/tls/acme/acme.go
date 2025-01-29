package acme

import (
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"

	"github.com/caddyserver/certmagic"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/hooks"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

const modName = "tls.loader.acme"

type Loader struct {
	instName string

	names        []string
	store        certmagic.Storage
	cache        *certmagic.Cache
	cfg          *certmagic.Config
	cancelManage context.CancelFunc

	log log.Logger
}

func New(_, instName string) (module.Module, error) {
	return &Loader{
		instName: instName,
		log:      log.Logger{Name: modName},
	}, nil
}

func (l *Loader) Configure(inlineArgs []string, cfg *config.Map) error {
	if len(inlineArgs) != 0 {
		return fmt.Errorf("%s: no inline args expected", modName)
	}

	var (
		hostname       string
		extraNames     []string
		storePath      string
		caPath         string
		testCAPath     string
		email          string
		agreed         bool
		challenge      string
		overrideDomain string
		provider       certmagic.DNSProvider
	)
	cfg.Bool("debug", true, false, &l.log.Debug)
	cfg.String("hostname", true, true, "", &hostname)
	cfg.StringList("extra_names", false, false, nil, &extraNames)
	cfg.String("store_path", false, false,
		filepath.Join(config.StateDirectory, "acme"), &storePath)
	cfg.String("ca", false, false,
		certmagic.LetsEncryptProductionCA, &caPath)
	cfg.String("test_ca", false, false,
		certmagic.LetsEncryptStagingCA, &testCAPath)
	cfg.String("email", false, false,
		"", &email)
	cfg.String("override_domain", false, false,
		"", &overrideDomain)
	cfg.Bool("agreed", false, false, &agreed)
	cfg.Enum("challenge", false, true,
		[]string{"dns-01"}, "dns-01", &challenge)
	cfg.Custom("dns", false, false, func() (interface{}, error) {
		return nil, nil
	}, func(m *config.Map, node config.Node) (interface{}, error) {
		var p certmagic.DNSProvider
		err := modconfig.ModuleFromNode("libdns", node.Args, node, m.Globals, &p)
		return p, err
	}, &provider)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	cmLog := l.log.Zap()

	l.store = &certmagic.FileStorage{Path: storePath}
	l.cache = certmagic.NewCache(certmagic.CacheOptions{
		Logger: cmLog,
		GetConfigForCert: func(c certmagic.Certificate) (*certmagic.Config, error) {
			return l.cfg, nil
		},
	})

	l.cfg = certmagic.New(l.cache, certmagic.Config{
		Storage:           l.store, // not sure if it is necessary to set these twice
		Logger:            cmLog,
		DefaultServerName: hostname,
	})
	issuer := certmagic.NewACMEIssuer(l.cfg, certmagic.ACMEIssuer{
		Logger: cmLog,
		CA:     caPath,
		TestCA: testCAPath,
		Email:  email,
		Agreed: agreed,
	})

	switch challenge {
	case "dns-01":
		issuer.DisableTLSALPNChallenge = true
		issuer.DisableHTTPChallenge = true
		if provider == nil {
			return fmt.Errorf("tls.loader.acme: dns-01 challenge requires a configured DNS provider")
		}
		issuer.DNS01Solver = &certmagic.DNS01Solver{
			DNSManager: certmagic.DNSManager{
				DNSProvider:    provider,
				OverrideDomain: overrideDomain,
			},
		}
	default:
		return fmt.Errorf("tls.loader.acme: challenge not supported")
	}
	l.cfg.Issuers = []certmagic.Issuer{issuer}

	l.names = append([]string{hostname}, extraNames...)

	return nil
}

func (l *Loader) ConfigureTLS(c *tls.Config) error {
	c.GetCertificate = l.cfg.GetCertificate
	return nil
}

func (l *Loader) Start() error {
	manageCtx, cancelManage := context.WithCancel(context.Background())
	err := l.cfg.ManageAsync(manageCtx, l.names)
	if err != nil {
		cancelManage()
		return err
	}
	l.cancelManage = cancelManage
	return nil
}

func (l *Loader) Stop() error {
	l.cancelManage()
	l.cache.Stop()
	return nil
}

func (l *Loader) Name() string {
	return modName
}

func (l *Loader) InstanceName() string {
	return l.instName
}

func init() {
	hooks.AddHook(hooks.EventShutdown, func() {
		certmagic.CleanUpOwnLocks(context.TODO(), log.DefaultLogger.Zap())
	})
}

func init() {
	var _ module.TLSLoader = &Loader{}
	module.Register(modName, New)
}
