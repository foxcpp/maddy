package smtp_downstream

import (
	"testing"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestDownstreamDelivery_EHLO_ALabel(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod, err := NewDownstream("", "", nil, []string{"tcp://127.0.0.1:" + testPort})
	if err != nil {
		t.Fatal(err)
	}
	if err := mod.Init(config.NewMap(nil, config.Node{
		Children: []config.Node{
			{
				Name: "hostname",
				Args: []string{"тест.invalid"},
			},
		},
	})); err != nil {
		t.Fatal(err)
	}

	tgt := mod.(*Downstream)
	tgt.log = testutils.Logger(t, "remote")

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})

	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
	if be.Messages[0].State.Hostname != "xn--e1aybc.invalid" {
		t.Error("target/remote should use use Punycode in EHLO")
	}
}
