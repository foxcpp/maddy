//go:build integration

package tests_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/foxcpp/maddy/tests"
	"github.com/jimlambrt/gldap"
	"github.com/stretchr/testify/require"
)

type searchEntry struct {
	dn      string
	options []gldap.Option
}

type MockLDAP struct {
	T             *testing.T
	SearchEntries map[string][]searchEntry
	AllowedBinds  map[string]string
}

func (ml *MockLDAP) HandleBind(w *gldap.ResponseWriter, r *gldap.Request) {
	resp := r.NewBindResponse(
		gldap.WithResponseCode(gldap.ResultInvalidCredentials),
	)

	m, err := r.GetSimpleBindMessage()
	if err != nil {
		require.NoError(ml.T, w.Write(resp))
		return
	}

	pass, ok := ml.AllowedBinds[m.UserName]
	if ok && pass == string(m.Password) {
		resp.SetResultCode(gldap.ResultSuccess)
		require.NoError(ml.T, w.Write(resp))
	}

	require.NoError(ml.T, w.Write(resp))
}

func (ml *MockLDAP) HandleSearch(w *gldap.ResponseWriter, r *gldap.Request) {
	resp := r.NewSearchDoneResponse()
	m, err := r.GetSearchMessage()
	if err != nil {
		ml.T.Logf("not a search message: %s", err)
		require.NoError(ml.T, w.Write(resp))
		return
	}
	ml.T.Logf("search base dn: %s", m.BaseDN)
	ml.T.Logf("search scope: %d", m.Scope)
	ml.T.Logf("search filter: %s", m.Filter)

	entries := ml.SearchEntries[m.Filter]
	for _, entry := range entries {
		ldapEntry := r.NewSearchResponseEntry(entry.dn, entry.options...)
		require.NoError(ml.T, w.Write(ldapEntry))
	}

	resp.SetResultCode(gldap.ResultSuccess)
	require.NoError(ml.T, w.Write(resp))
}

func (ml *MockLDAP) Run(address string) {
	s, err := gldap.NewServer()
	if err != nil {
		ml.T.Fatalf("unable to create server: %s", err.Error())
	}

	// create a router and add a bind handler
	r, err := gldap.NewMux()
	if err != nil {
		ml.T.Fatalf("unable to create router: %s", err.Error())
	}
	require.NoError(ml.T, r.Bind(ml.HandleBind))
	require.NoError(ml.T, r.Search(ml.HandleSearch))
	require.NoError(ml.T, s.Router(r))
	go func() {
		require.NoError(ml.T, s.Run(address))
	}()
	ml.T.Cleanup(func() {
		require.NoError(ml.T, s.Stop())
	})

	for !s.Ready() {
		ml.T.Log("Waiting for server to start")
		time.Sleep(100 * time.Millisecond)
	}
}

func TestLDAPInjectionFilter(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)

	ldapPort := t.Port("ldap")

	ldapSrv := &MockLDAP{
		T: tt,
		AllowedBinds: map[string]string{
			"DC=com,CN=bob":   "bob_pass",
			"DC=com,CN=alice": "alice_pass",
		},
		SearchEntries: map[string][]searchEntry{
			"(&(objectClass=inetOrgPerson)(uid=alice))": {
				{
					dn: "DC=com,CN=alice",
					options: []gldap.Option{
						gldap.WithAttributes(map[string][]string{
							"objectClass": {"inetOrgPerson"},
							"uid":         {"alice"},
							"description": {"prefix_test"},
						}),
					},
				},
			},
			"(&(objectClass=inetOrgPerson)(uid=bob))": {
				{
					dn: "DC=com,CN=bob",
					options: []gldap.Option{
						gldap.WithAttributes(map[string][]string{
							"objectClass": {"inetOrgPerson"},
							"uid":         {"bob"},
							"description": {"prefix_test"},
						}),
					},
				},
			},
			"(&(objectClass=inetOrgPerson)(uid=bob)(description=prefix*))": {
				{
					dn: "DC=com,CN=bob",
					options: []gldap.Option{
						gldap.WithAttributes(map[string][]string{
							"objectClass": {"inetOrgPerson"},
							"uid":         {"bob"},
							"description": {"prefix_test"},
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
			bind plain "DC=com,CN=bob" "bob_pass"
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

	smtpConn := t.Conn("smtp")
	defer smtpConn.MustClose()
	smtpConn.SMTPNegotation("clieht.maddy.test", nil, nil)
	smtpConn.SMTPPlainAuth("alice", "alice_pass", true)

	smtpConn2 := t.Conn("smtp")
	defer smtpConn2.MustClose()
	smtpConn2.SMTPNegotation("clieht.maddy.test", nil, nil)
	smtpConn2.SMTPPlainAuth("bob)(description=prefix*", "bob_pass", false)
}
