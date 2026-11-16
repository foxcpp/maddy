//go:build integration

package tests_test

import (
	"crypto/tls"
	"net"
	"testing"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/internal/testutils"
	"github.com/foxcpp/maddy/tests"
	"github.com/stretchr/testify/require"
)

func TestIssue717OutboundPolicy(t *testing.T) {
	for _, tc := range []struct {
		name        string
		localPolicy string

		protocols string
		legacyTLS bool

		wantReject bool
		wantTLS    bool
	}{
		{name: "plaintext allowed through queue", localPolicy: "min_tls_level none", protocols: "tls1.0 tls1.3"},
		{name: "plaintext rejected by default", protocols: "tls1.0 tls1.3", wantReject: true},
		{name: "legacy TLS allowed through queue", localPolicy: "min_tls_level none", protocols: "tls1.0 tls1.3", legacyTLS: true, wantTLS: true},
		{name: "failed TLS falls back to plaintext", localPolicy: "min_tls_level none", protocols: "tls1.2 tls1.3", legacyTLS: true},
	} {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()

			test := tests.NewT(tt)
			test.AllowFDSharing()
			test.DNS(map[string]mockdns.Zone{
				"example.invalid.":    {MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}}},
				"mx.example.invalid.": {A: []string{"127.0.0.1"}},
			})
			test.Port("smtp")

			var server *smtp.Server
			var be *testutils.SMTPBackend
			if tc.legacyTLS {
				_, be, server = testutils.SMTPServerSTARTTLS(tt, "127.0.0.1:0", func(s *smtp.Server) {
					s.TLSConfig.MinVersion = tls.VersionTLS10
					s.TLSConfig.MaxVersion = tls.VersionTLS10
				})
			} else {
				be, server = testutils.SMTPServer(tt, "127.0.0.1:0")
			}
			defer server.Close()

			test.RemoteSMTPPort(testutils.EffectiveSMTPServerPort(tt, server))

			test.Config(`
				hostname mx.maddy.test
				tls off
				target.remote outbound_delivery {
					mx_auth {
						local_policy {
							` + tc.localPolicy + `
							min_mx_level none
						}
					}
					tls_client { protocols ` + tc.protocols + ` }
				}
				smtp {env:TEST_ENDPOINT_smtp} {
					deliver_to &outbound_delivery
				}`)

			test.Run(1)
			defer test.Close()

			c := test.Conn("smtp")
			defer c.Close()
			c.SMTPNegotation("client.maddy.test", nil, nil)
			c.Writeln("MAIL FROM:<from@maddy.test>")
			c.ExpectPattern("250 *")
			c.Writeln("RCPT TO:<to@example.invalid>")
			if tc.wantReject {
				c.ExpectPattern("451 *")
				return
			}
			c.ExpectPattern("250 *")
			c.Writeln("DATA")
			c.ExpectPattern("354 *")
			c.Writeln("From: <from@maddy.test>")
			c.Writeln("")
			c.Writeln("Hello!")
			c.Writeln(".")
			c.ExpectPattern("250 *")

			msg := be.Msg(t, 0)
			require.Equal(t, tc.wantTLS, msg.TLS != nil && msg.TLS.HandshakeComplete)
		})
	}
}
