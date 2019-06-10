package maddy

import (
	"io"
	"net"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

type FuncFilter func(r Resolver, ctx *module.DeliveryContext, msg io.Reader) (io.Reader, error)

// functionFilter is a wrapper to help create simple filter functions that
// don't need any state or configuration.
//
// See RegisterFunctionFilter.
type functionFilter struct {
	modName, instName string
	ctxVarName        string

	ignoreFail bool
	resolver   Resolver

	f FuncFilter
}

func (fltr *functionFilter) Init(m *config.Map) error {
	m.Bool("ignore_fail", false, &fltr.ignoreFail)
	if _, err := m.Process(); err != nil {
		return err
	}
	return nil
}

func (fltr *functionFilter) Apply(ctx *module.DeliveryContext, msg io.Reader) (io.Reader, error) {
	r, err := fltr.f(fltr.resolver, ctx, msg)
	if fltr.ctxVarName != "" {
		ctx.Ctx[fltr.ctxVarName] = err != nil
	}
	if err != nil && fltr.ignoreFail {
		return nil, nil
	}
	return r, err
}

func (fltr *functionFilter) Name() string {
	return fltr.modName
}

func (fltr *functionFilter) InstanceName() string {
	return fltr.instName
}

func NewFuncFilter(name, ctxVarName string, f FuncFilter) module.FuncNewModule {
	return func(modName, instName string) (module.Module, error) {
		return &functionFilter{
			modName:    modName,
			instName:   instName,
			ctxVarName: ctxVarName,
			resolver:   net.DefaultResolver,
			f:          f,
		}, nil
	}
}

// RegisterFuncFilter creates and registers module implementing Filter interface
// that runs the passed function.
//
// The module will have one configuration directive: ignore_fail.
// If it is specified by the user - any errors returned by the function will be
// ignored.
//
// This function also registers module instance using the same name as
// module.
//
// If ctxVarName is a non-empty string, the module will set DeliveryContext.Ctx[ctxVarName]
// to a boolean depending on whether the filter function failed.
func RegisterFuncFilter(modName, ctxVarName string, f FuncFilter) {
	ctor := NewFuncFilter(modName, ctxVarName, f)
	module.Register(modName, ctor)
	mod, _ := ctor(modName, modName)
	module.RegisterInstance(mod, config.NewMap(nil, &config.Node{}))
}
