package msgpipeline

import (
	"strconv"
	"testing"

	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
)

func BenchmarkMsgPipelineSimple(b *testing.B) {
	target := testutils.Target{InstName: "test_target", DiscardMessages: true}
	d := MsgPipeline{msgpipelineCfg: msgpipelineCfg{
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

func BenchmarkMsgPipelineGlobalChecks(b *testing.B) {
	testWithCount := func(checksCount int) {
		b.Run(strconv.Itoa(checksCount), func(b *testing.B) {
			checks := make([]module.Check, 0, checksCount)
			for i := 0; i < checksCount; i++ {
				checks = append(checks, &testutils.Check{InstName: "check_" + strconv.Itoa(i)})
			}

			target := testutils.Target{InstName: "test_target", DiscardMessages: true}
			d := MsgPipeline{msgpipelineCfg: msgpipelineCfg{
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

func BenchmarkMsgPipelineTargets(b *testing.B) {
	testWithCount := func(targetCount int) {
		b.Run(strconv.Itoa(targetCount), func(b *testing.B) {
			targets := make([]module.DeliveryTarget, 0, targetCount)
			for i := 0; i < targetCount; i++ {
				targets = append(targets, &testutils.Target{InstName: "target_" + strconv.Itoa(i), DiscardMessages: true})
			}

			d := MsgPipeline{msgpipelineCfg: msgpipelineCfg{
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
