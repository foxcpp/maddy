package check

import (
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
)

// FailAction specifies actions that messages pipeline should take based on the
// result of the check.
//
// Its check module responsibility to apply FailAction on the CheckResult it
// returns. It is intended to be used as follows:
//
// Add the configuration directive to allow user to specify the action:
//     cfg.Custom("SOME_action", false, false,
//     	func() (interface{}, error) {
//     		return check.FailAction{Quarantine: true}, nil
//     	}, check.FailActionDirective, &yourModule.SOMEAction)
// return in func literal is the default value, you might want to adjust it.
//
// Call yourModule.SOMEAction.Apply on CheckResult containing only the
// Reason field:
//     func (yourModule YourModule) CheckConnection() module.CheckResult {
//         return yourModule.SOMEAction.Apply(module.CheckResult{
//             Reason: ...,
//         })
//     }
type FailAction struct {
	Quarantine bool
	Reject     bool
}

func FailActionDirective(m *config.Map, node *config.Node) (interface{}, error) {
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
	default:
		return nil, m.MatchErr("invalid action")
	}
}

// Apply merges the result of check execution with action configuration specified
// in the check configuration.
func (cfa FailAction) Apply(originalRes module.CheckResult) module.CheckResult {
	if originalRes.Reason == nil {
		return originalRes
	}

	originalRes.Quarantine = cfa.Quarantine || originalRes.Quarantine
	originalRes.Reject = cfa.Reject || originalRes.Reject
	return originalRes
}
