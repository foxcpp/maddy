package pass_table

import (
	"fmt"
	"strings"

	"github.com/foxcpp/maddy/internal/config"
	modconfig "github.com/foxcpp/maddy/internal/config/module"
	"github.com/foxcpp/maddy/internal/module"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/secure/precis"
)

type Auth struct {
	modName    string
	instName   string
	inlineArgs []string

	table module.Table
}

func New(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	return &Auth{
		modName:    modName,
		instName:   instName,
		inlineArgs: inlineArgs,
	}, nil
}

func (a *Auth) Init(cfg *config.Map) error {
	if len(a.inlineArgs) != 0 {
		return modconfig.ModuleFromNode(a.inlineArgs, cfg.Block, cfg.Globals, &a.table)
	}

	cfg.Custom("table", false, true, nil, modconfig.TableDirective, &a.table)
	_, err := cfg.Process()
	return err
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
		return fmt.Errorf("%s: auth plain %s: no hash tag", a.modName, key)
	}
	hashVerify := HashVerify[parts[0]]
	if hashVerify == nil {
		return fmt.Errorf("%s: auth plain %s: unknown hash: %s", a.modName, key, parts[0])
	}
	return hashVerify(password, parts[1])
}

func (a *Auth) ListUsers() ([]string, error) {
	tbl, ok := a.table.(module.MutableTable)
	if !ok {
		return nil, fmt.Errorf("%s: table is not mutable, no management functionality available", a.modName)
	}

	l, err := tbl.Keys()
	if err != nil {
		return nil, fmt.Errorf("%s: list users: %w", a.modName, err)
	}
	return l, nil
}

func (a *Auth) CreateUser(username, password string) error {
	tbl, ok := a.table.(module.MutableTable)
	if !ok {
		return fmt.Errorf("%s: table is not mutable, no management functionality available", a.modName)
	}

	key, err := precis.UsernameCaseMapped.CompareKey(username)
	if err != nil {
		return fmt.Errorf("%s: create user %s (raw): %w", a.modName, username, err)
	}

	_, ok, err = tbl.Lookup(key)
	if err != nil {
		return fmt.Errorf("%s: create user %s: %w", a.modName, key, err)
	}
	if ok {
		return fmt.Errorf("%s: credentials for %s already exist", a.modName, key)
	}

	// TODO: Allow to customize hash function.
	hash, err := HashCompute[HashBcrypt](HashOpts{
		BcryptCost: bcrypt.DefaultCost,
	}, password)
	if err != nil {
		return fmt.Errorf("%s: create user %s: hash generation: %w", a.modName, key, err)
	}

	if err := tbl.SetKey(key, "bcrypt:"+hash); err != nil {
		return fmt.Errorf("%s: create user %s: %w", a.modName, key, err)
	}
	return nil
}

func (a *Auth) SetUserPassword(username, password string) error {
	tbl, ok := a.table.(module.MutableTable)
	if !ok {
		return fmt.Errorf("%s: table is not mutable, no management functionality available", a.modName)
	}

	key, err := precis.UsernameCaseMapped.CompareKey(username)
	if err != nil {
		return fmt.Errorf("%s: set password %s (raw): %w", a.modName, username, err)
	}

	// TODO: Allow to customize hash function.
	hash, err := HashCompute[HashBcrypt](HashOpts{
		BcryptCost: bcrypt.DefaultCost,
	}, password)
	if err != nil {
		return fmt.Errorf("%s: set password %s: hash generation: %w", a.modName, key, err)
	}

	if err := tbl.SetKey(key, "bcrypt:"+hash); err != nil {
		return fmt.Errorf("%s: set password %s: %w", a.modName, key, err)
	}
	return nil
}

func (a *Auth) DeleteUser(username string) error {
	tbl, ok := a.table.(module.MutableTable)
	if !ok {
		return fmt.Errorf("%s: table is not mutable, no management functionality available", a.modName)
	}

	key, err := precis.UsernameCaseMapped.CompareKey(username)
	if err != nil {
		return fmt.Errorf("%s: del user %s (raw): %w", a.modName, username, err)
	}

	if err := tbl.RemoveKey(key); err != nil {
		return fmt.Errorf("%s: del user %s: %w", a.modName, key, err)
	}
	return nil
}

func init() {
	module.Register("pass_table", New)
}
