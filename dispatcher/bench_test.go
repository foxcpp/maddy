package dispatcher

import (
	"strconv"
	"testing"

	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/testutils"
)

func BenchmarkDispatcherSimple(b *testing.B) {
	target := testutils.Target{InstName: "test_target", DiscardMessages: true}
	d := Dispatcher{dispatcherCfg: dispatcherCfg{
		perSource: map[string]sourceBlock{},
		defaultSource: sourceBlock{
			perRcpt: map[string]*rcptBlock{},
			defaultRcpt: &rcptBlock{
				targets: []module.DeliveryTarget{&target},
			},
		},
	}}

	testutils.BenchDelivery(b, &d, "sender@example.org", []string{"rcpt-X@example.org"})
}

func BenchmarkDispatcherGlobalChecks(b *testing.B) {
	testWithCount := func(checksCount int) {
		b.Run(strconv.Itoa(checksCount), func(b *testing.B) {
			checks := make([]module.Check, 0, checksCount)
			for i := 0; i < checksCount; i++ {
				checks = append(checks, &testutils.Check{InstName: "check_" + strconv.Itoa(i)})
			}

			target := testutils.Target{InstName: "test_target", DiscardMessages: true}
			d := Dispatcher{dispatcherCfg: dispatcherCfg{
				globalChecks: checks,
				perSource:    map[string]sourceBlock{},
				defaultSource: sourceBlock{
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: []module.DeliveryTarget{&target},
					},
				},
			}}

			testutils.BenchDelivery(b, &d, "sender@example.org", []string{"rcpt-X@example.org"})
		})
	}

	testWithCount(5)
	testWithCount(10)
	testWithCount(15)
}

func BenchmarkDispatcherTargets(b *testing.B) {
	testWithCount := func(targetCount int) {
		b.Run(strconv.Itoa(targetCount), func(b *testing.B) {
			targets := make([]module.DeliveryTarget, 0, targetCount)
			for i := 0; i < targetCount; i++ {
				targets = append(targets, &testutils.Target{InstName: "target_" + strconv.Itoa(i), DiscardMessages: true})
			}

			d := Dispatcher{dispatcherCfg: dispatcherCfg{
				perSource: map[string]sourceBlock{},
				defaultSource: sourceBlock{
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						targets: targets,
					},
				},
			}}

			testutils.BenchDelivery(b, &d, "sender@example.org", []string{"rcpt-X@example.org"})
		})
	}

	testWithCount(5)
	testWithCount(10)
	testWithCount(15)
}
