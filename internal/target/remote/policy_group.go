package remote

import (
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

// PolicyGroup is a module container for a group of Policy implementations.
//
// It allows to share a set of policy configurations between remote target
// instances using named configuration blocks (module instances) system.
//
// It is registered globally under the name 'mx_auth'. This is also the name of
// corresponding remote target configuration directive. The object does not
// implement any standard module interfaces besides module.Module and is
// specific to the remote target.
type PolicyGroup struct {
	L        []Policy
	instName string
	pols     map[string]Policy
}

func (pg *PolicyGroup) Init(cfg *config.Map) error {
	var debugLog bool
	cfg.Bool("debug", true, false, &debugLog)
	cfg.AllowUnknown()
	other, err := cfg.Process()
	if err != nil {
		return err
	}

	// Policies have defined application order since some of them depend on
	// results of other policies. We first initialize them in the order they
	// are defined in and then reorder depending on the needed order.

	for _, block := range other {
		if _, ok := pg.pols[block.Name]; ok {
			return config.NodeErr(block, "duplicate policy block: %v", block.Name)
		}

		policy, err := policyFromNode(debugLog, cfg, block)
		if err != nil {
			return err
		}

		pg.pols[block.Name] = policy
	}

	for _, name := range [...]string{
		"mtasts",
		// sts_preload should go after mtasts so it will take not effect if
		// MXLevel is already MX_MTASTS.
		"sts_preload",
		"dane",
		"dnssec",
		// localPolicy should be the last one, since it considers levels defined by
		// other policies.
		"local_policy",
	} {
		policy, ok := pg.pols[name]
		if !ok {
			continue
		}
		pg.L = append(pg.L, policy)
	}

	return nil
}

func (PolicyGroup) Name() string {
	return "mx_auth"
}

func (pg PolicyGroup) InstanceName() string {
	return pg.instName
}

func init() {
	module.Register("mx_auth", func(_, instName string, _, _ []string) (module.Module, error) {
		return &PolicyGroup{
			instName: instName,
			pols:     map[string]Policy{},
		}, nil
	})
}
