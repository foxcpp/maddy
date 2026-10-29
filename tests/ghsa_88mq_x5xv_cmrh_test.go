//go:build integration

package tests_test

import (
	"strconv"
	"testing"

	"github.com/foxcpp/maddy/tests"
	"github.com/jimlambrt/gldap"
	"github.com/stretchr/testify/require"
)

func TestLDAPAuthenticationDoesNotRevealUserExistence(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)

	ldapPort := t.Port("ldap")
	ldapSrv := &MockLDAP{
		T: tt,
		AllowedBinds: map[string]string{
			"DC=com,CN=service": "service_pass",
		},
		SearchEntries: map[string][]searchEntry{
			"(&(objectClass=inetOrgPerson)(uid=alice))": {
				{
					dn: "DC=com,CN=alice",
					options: []gldap.Option{
						gldap.WithAttributes(map[string][]string{
							"objectClass": {"inetOrgPerson"},
							"uid":         {"alice"},
						}),
					},
				},
			},
		},
	}
	ldapSrv.Run(":" + strconv.Itoa(int(ldapPort)))

	t.Port("smtp")
	t.DNS(nil)
	t.Config(`
		hostname mx.maddy.test
		tls off

		auth.ldap ldap_auth {
			urls ldap://127.0.0.1:{env:TEST_PORT_ldap}
			bind plain "DC=com,CN=service" "service_pass"
			base_dn "DC=com"
			filter "(&(objectClass=inetOrgPerson)(uid={username}))"
		}

		submission tcp://0.0.0.0:{env:TEST_PORT_smtp} {
			auth &ldap_auth
			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	// An existing username with a bad password performs a user bind and a
	// connection rebind before returning the same SMTP failure as a missing
	// username.
	smtpConn := t.Conn("smtp")
	smtpConn.SMTPNegotation("client.maddy.test", nil, nil)
	bindsBeforeExisting := ldapSrv.BindCount()
	smtpConn.SMTPPlainAuth("alice", "wrong", false)
	bindsAfterExisting := ldapSrv.BindCount()
	require.Equal(tt, 2, bindsAfterExisting-bindsBeforeExisting)
	require.NoError(tt, smtpConn.Close())

	smtpConn = t.Conn("smtp")
	smtpConn.SMTPNegotation("client.maddy.test", nil, nil)
	bindsBeforeMissing := ldapSrv.BindCount()
	smtpConn.SMTPPlainAuth("does-not-exist", "wrong", false)
	bindsAfterMissing := ldapSrv.BindCount()
	require.Equal(tt, bindsAfterExisting-bindsBeforeExisting, bindsAfterMissing-bindsBeforeMissing)
	require.NoError(tt, smtpConn.Close())
}
