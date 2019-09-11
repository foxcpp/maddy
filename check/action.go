package check

import (
	"strconv"

	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
)

type FailAction struct {
	Quarantine  bool
	Reject      bool
	ScoreAdjust int
}

func failActionDirective(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) == 0 {
		return nil, m.MatchErr("expected at least 1 argument")
	}
	if len(node.Children) != 0 {
		return nil, m.MatchErr("can't declare block here")
	}

	switch node.Args[0] {
	case "reject", "quarantine", "ignore":
		if len(node.Args) > 1 {
			return nil, m.MatchErr("too many arguments")
		}
		return FailAction{
			Reject:     node.Args[0] == "reject",
			Quarantine: node.Args[0] == "quarantine",
		}, nil
	case "score":
		if len(node.Args) != 2 {
			return nil, m.MatchErr("expected 2 arguments")
		}
		scoreAdj, err := strconv.Atoi(node.Args[1])
		if err != nil {
			return nil, m.MatchErr("%v", err)
		}
		return FailAction{
			ScoreAdjust: scoreAdj,
		}, nil
	default:
		return nil, m.MatchErr("invalid action")
	}
}

// Apply merges the result of check execution with action configuration specified
// in the check configuration.
func (cfa FailAction) Apply(originalRes module.CheckResult) module.CheckResult {
	if originalRes.RejectErr == nil {
		return originalRes
	}

	originalRes.Quarantine = cfa.Quarantine
	originalRes.ScoreAdjust = int32(cfa.ScoreAdjust)
	if !cfa.Reject {
		originalRes.RejectErr = nil
	}
	return originalRes
}
