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

	store        certmagic.Storage
	cache        *certmagic.Cache
	cfg          *certmagic.Config
	cancelManage context.CancelFunc

	log log.Logger
}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) != 0 {
		return nil, fmt.Errorf("%s: no inline args expected", modName)
	}
	return &Loader{
		instName: instName,
		log:      log.Logger{Name: modName},
	}, nil
}

func (l *Loader) Init(cfg *config.Map) error {
	var (
		hostname   string
		extraNames []string
		storePath  string
		caPath     string
		testCAPath string
		email      string
		agreed     bool
		challenge  string
		provider   certmagic.ACMEDNSProvider
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
	cfg.Bool("agreed", false, false, &agreed)
	cfg.Enum("challenge", false, true,
		[]string{"dns-01"}, "dns-01", &challenge)
	cfg.Custom("dns", false, false, func() (interface{}, error) {
		return nil, nil
	}, func(m *config.Map, node config.Node) (interface{}, error) {
		var p certmagic.ACMEDNSProvider
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
			return &certmagic.Config{
				Storage: l.store,
				Logger:  cmLog,
			}, nil
		},
	})

	l.cfg = certmagic.New(l.cache, certmagic.Config{
		Storage:           l.store, // not sure if it is necessary to set these twice
		Logger:            cmLog,
		DefaultServerName: hostname,
	})
	mngr := certmagic.NewACMEIssuer(l.cfg, certmagic.ACMEIssuer{
		Logger: cmLog,
		CA:     caPath,
		Email:  email,
		Agreed: agreed,
	})

	switch challenge {
	case "dns-01":
		mngr.DisableTLSALPNChallenge = true
		mngr.DisableHTTPChallenge = true
		if provider == nil {
			return fmt.Errorf("tls.loader.acme: dns-01 challenge requires a configured DNS provider")
		}
		mngr.DNS01Solver = &certmagic.DNS01Solver{
			DNSProvider: provider,
		}
	default:
		return fmt.Errorf("tls.loader.acme: challenge not supported")
	}
	l.cfg.Issuers = []certmagic.Issuer{mngr}

	if module.NoRun {
		return nil
	}

	manageCtx, cancelManage := context.WithCancel(context.Background())
	err := l.cfg.ManageAsync(manageCtx, append([]string{hostname}, extraNames...))
	if err != nil {
		cancelManage()
		return err
	}
	l.cancelManage = cancelManage

	return nil
}

func (l *Loader) ConfigureTLS(c *tls.Config) error {
	c.GetCertificate = l.cfg.GetCertificate
	return nil
}

func (l *Loader) Close() error {
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
