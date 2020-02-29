package pass_table

import (
	"fmt"
	"strings"

	"github.com/foxcpp/maddy/internal/config"
	modconfig "github.com/foxcpp/maddy/internal/config/module"
	"github.com/foxcpp/maddy/internal/module"
	"golang.org/x/text/secure/precis"
)

type Auth struct {
	modName    string
	instName   string
	inlineArgs []string

	table module.Table
}

func New(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) < 1 {
		return nil, fmt.Errorf("%s: specify the table to use", modName)
	}

	return &Auth{
		modName:    modName,
		instName:   instName,
		inlineArgs: inlineArgs,
	}, nil
}

func (a *Auth) Init(cfg *config.Map) error {
	return modconfig.ModuleFromNode(a.inlineArgs, cfg.Block, cfg.Globals, &a.table)
}

func (a *Auth) Name() string {
	return a.modName
}

func (a *Auth) InstanceName() string {
	return a.instName
}

func (a *Auth) AuthPlain(username, password string) error {
	key, err := precis.UsernameCaseMapped.CompareKey(username)
	if err != nil {
		return err
	}

	hash, ok, err := a.table.Lookup(key)
	if !ok {
		return module.ErrUnknownCredentials
	}
	if err != nil {
		return err
	}

	parts := strings.SplitN(hash, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%s: no hash tag", a.modName)
	}
	hashVerify := HashVerify[parts[0]]
	if hashVerify == nil {
		return fmt.Errorf("%s: unknown hash: %s", a.modName, parts[0])
	}
	return hashVerify(password, parts[1])
}

func init() {
	module.Register("pass_table", New)
}
