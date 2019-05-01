package maddy

import (
	"io"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

type FuncFilter func(ctx *module.DeliveryContext, msg io.Reader) (io.Reader, error)

// FunctionFilter is a wrapper to help create simple filter functions that
// don't need any state or configuration.
//
// See RegisterFunctionFilter.
type FunctionFilter struct {
	modName, instName string
	ctxVarName        string

	ignoreFail bool

	f FuncFilter
}

func (fltr *FunctionFilter) Init(m *config.Map) error {
	m.Bool("ignore_fail", false, &fltr.ignoreFail)
	if _, err := m.Process(); err != nil {
		return err
	}
	return nil
}

func (fltr *FunctionFilter) Apply(ctx *module.DeliveryContext, msg io.Reader) (io.Reader, error) {
	r, err := fltr.f(ctx, msg)
	if fltr.ctxVarName != "" {
		ctx.Ctx[fltr.ctxVarName] = err != nil
	}
	if err != nil && fltr.ignoreFail {
		return nil, nil
	}
	return r, nil
}

func (fltr *FunctionFilter) Name() string {
	return fltr.modName
}

func (fltr *FunctionFilter) InstanceName() string {
	return fltr.instName
}

func NewFuncFilter(name, ctxVarName string, f FuncFilter) module.FuncNewModule {
	return func(modName, instName string) (module.Module, error) {
		return &FunctionFilter{
			modName:    modName,
			instName:   instName,
			ctxVarName: ctxVarName,
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
