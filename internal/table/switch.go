package table

import (
	"context"
	"errors"
	"fmt"

	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
)

type switchCase struct {
	table module.Table
	value string
}

type Switch struct {
	instName string

	multiMatch   bool
	cases        []switchCase
	defaultValue *string
}

func NewSwitch(_ *container.C, _, instName string) (module.Module, error) {
	return &Switch{
		instName: instName,
	}, nil
}

func (s *Switch) Configure(inlineArgs []string, cfg *config.Map) error {
	if len(inlineArgs) != 0 {
		return errors.New("inline arguments not supported")
	}

	cfg.Bool("allow_multiple", false, false, &s.multiMatch)
	cfg.Callback("case", func(m *config.Map, node config.Node) error {
		if len(node.Args) < 2 {
			return fmt.Errorf("expected at least two arguments")
		}
		value := node.Args[0]
		node.Args = node.Args[1:]
		tab, err := modconfig.ModuleDirective[module.Table]("table", m, node)
		if err != nil {
			return err
		}
		s.cases = append(s.cases, switchCase{
			table: tab,
			value: value,
		})
		return nil
	})
	cfg.Callback("default", func(m *config.Map, node config.Node) error {
		if len(node.Args) != 1 {
			return fmt.Errorf("expected one argument")
		}
		s.defaultValue = &node.Args[0]
		return nil
	})
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if len(s.cases) == 0 && s.defaultValue == nil {
		return fmt.Errorf("no default value set and no cases present, invalid switch")
	}

	return nil
}

func (s *Switch) Name() string {
	return "table.switch"
}

func (s *Switch) InstanceName() string {
	return s.instName
}

func (s *Switch) Lookup(ctx context.Context, str string) (string, bool, error) {
	var lastValue *string
	for _, c := range s.cases {
		_, ok, err := c.table.Lookup(ctx, str)
		if err != nil {
			return "", false, err
		}
		if ok {
			if !s.multiMatch {
				return c.value, true, nil
			}
			lastValue = &c.value
		}
	}

	if lastValue != nil {
		return *lastValue, true, nil
	}

	if s.defaultValue != nil {
		return *s.defaultValue, true, nil
	}
	return "", false, nil
}

func (s *Switch) LookupMulti(ctx context.Context, str string) ([]string, error) {
	var results []string
	for _, c := range s.cases {
		res, ok, err := c.table.Lookup(ctx, str)
		if err != nil {
			return nil, err
		}
		if ok {
			results = append(results, c.value)
			if !s.multiMatch {
				break
			}
		}
	}
	return results, nil
}

func init() {
	modules.Register("table.switch", NewSwitch)
}
