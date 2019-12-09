package remote

import (
	"net"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestRemoteDelivery_EHLO_ALabel(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	mod, err := New("", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := mod.Init(config.NewMap(nil, &config.Node{
		Children: []config.Node{
			{
				Name: "authenticate_mx",
				Args: []string{"off"},
			},
			{
				Name: "hostname",
				Args: []string{"тест.invalid"},
			},
		},
	})); err != nil {
		t.Fatal(err)
	}

	tgt := mod.(*Target)
	tgt.resolver = &mockdns.Resolver{Zones: zones}
	tgt.dialer = resolver.DialContext
	tgt.extResolver = nil
	tgt.Log = testutils.Logger(t, "remote")

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})

	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
	if be.Messages[0].State.Hostname != "xn--e1aybc.invalid" {
		t.Error("target/remote should use use Punycode in EHLO")
	}
}
