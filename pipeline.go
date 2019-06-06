package maddy

import (
	"io"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

type Pipeline struct {
	instName string
	steps    []PipelineStep

	Log log.Logger
}

func NewPipeline(modName, instName string) (module.Module, error) {
	return &Pipeline{
		instName: instName,
		Log:      log.Logger{Name: "pipeline"},
	}, nil
}

func (p *Pipeline) Init(cfg *config.Map) error {
	cfg.Bool("debug", true, &p.Log.Debug)
	cfg.AllowUnknown()
	remainingDirs, err := cfg.Process()
	if err != nil {
		return err
	}

	for _, entry := range remainingDirs {
		step, err := StepFromCfg(cfg.Globals, entry)
		if err != nil {
			return err
		}

		p.steps = append(p.steps, step)
	}

	return nil
}

func (p *Pipeline) InstanceName() string {
	return p.instName
}

func (p *Pipeline) Name() string {
	return "pipeline"
}

// Deliver implements module.DeliveryTarget.
func (p *Pipeline) Deliver(ctx module.DeliveryContext, body io.Reader) error {
	_, _, err := passThroughPipeline(p.steps, &ctx, body)
	return err
}

func init() {
	module.Register("pipeline", NewPipeline)
}
