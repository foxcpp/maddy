package msgpipeline

import (
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
)

type Module struct {
	instName string
	log      log.Logger
	*MsgPipeline
}

func NewModule(modName, instName string, aliases, inlineArgs []string) (module.Module, error) {
	return &Module{
		log:      log.Logger{Name: "msgpipeline"},
		instName: instName,
	}, nil
}

func (m *Module) Init(cfg *config.Map) error {
	var hostname string
	cfg.String("hostname", true, true, "", &hostname)
	cfg.Bool("debug", true, false, &m.log.Debug)
	cfg.AllowUnknown()
	other, err := cfg.Process()
	if err != nil {
		return err
	}

	p, err := New(cfg.Globals, other)
	if err != nil {
		return err
	}
	m.MsgPipeline = p

	return nil
}

func (m *Module) Name() string {
	return "msgpipeline"
}

func (m *Module) InstanceName() string {
	return m.instName
}

func init() {
	module.Register("msgpipeline", NewModule)
}
