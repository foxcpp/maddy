package netauth

import (
	"context"
	"fmt"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/hashicorp/go-hclog"
	"github.com/netauth/netauth/pkg/netauth"
)

const modName = "auth.netauth"

func init() {
	var _ module.PlainAuth = &Auth{}
	var _ module.Table = &Auth{}
	module.Register(modName, New)
	module.Register("table.netauth", New)
}

// Auth binds all methods related to the NetAuth client library.
type Auth struct {
	instName  string
	mustGroup string

	nacl *netauth.Client

	log log.Logger
}

// New creates a new instance of the NetAuth module.
func New(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	return &Auth{
		instName: instName,
		log:      log.Logger{Name: modName},
	}, nil
}

// Init performs deferred initialization actions.
func (a *Auth) Init(cfg *config.Map) error {
	l := hclog.New(&hclog.LoggerOptions{Output: a.log})
	n, err := netauth.NewWithLog(l)
	if err != nil {
		return err
	}
	a.nacl = n
	a.nacl.SetServiceName("maddy")
	cfg.String("require_group", false, false, "", &a.mustGroup)
	cfg.Bool("debug", true, false, &a.log.Debug)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	a.log.Debugln("Debug logging enabled")
	a.log.Debugf("mustGroups status: %s", a.mustGroup)
	return nil
}

// Name returns "auth.netauth" as the fixed module name.
func (a *Auth) Name() string {
	return modName
}

// InstanceName returns the configured name for this instance of the
// plugin.  Given the way that NetAuth works it doesn't really make
// sense to have more than one instance, but this is part of the API.
func (a *Auth) InstanceName() string {
	return a.instName
}

// Lookup requests the entity from the remote NetAuth server,
// potentially returning that the user does not exist at all.
func (a *Auth) Lookup(ctx context.Context, username string) (string, bool, error) {
	e, err := a.nacl.EntityInfo(ctx, username)
	if err != nil {
		return "", false, fmt.Errorf("%s: search: %w", modName, err)
	}

	if a.mustGroup != "" {
		if err := a.checkMustGroup(username); err != nil {
			return "", false, err
		}
	}
	return e.GetID(), true, nil
}

// AuthPlain attempts straightforward authentication of the entity on
// the remote NetAuth server.
func (a *Auth) AuthPlain(username, password string) error {
	a.log.Debugf("attempting to auth user: %s", username)
	if err := a.nacl.AuthEntity(context.Background(), username, password); err != nil {
		return module.ErrUnknownCredentials
	}
	a.log.Debugln("netauth returns successful auth")
	if a.mustGroup != "" {
		if err := a.checkMustGroup(username); err != nil {
			return err
		}
	}
	return nil
}

func (a *Auth) checkMustGroup(username string) error {
	a.log.Debugf("Performing require_group check: must=%s", a.mustGroup)
	groups, err := a.nacl.EntityGroups(context.Background(), username)
	if err != nil {
		return fmt.Errorf("%s: groups: %w", modName, err)
	}
	for _, g := range groups {
		if g.GetName() == a.mustGroup {
			return nil
		}
	}
	return fmt.Errorf("%s: missing required group (%s not in %s)", modName, username, a.mustGroup)
}
