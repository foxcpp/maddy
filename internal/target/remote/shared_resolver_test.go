package remote

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/stretchr/testify/require"
)

func TestSharedResolverUsedByRemoteAndDANE(t *testing.T) {
	c := container.New()
	var err error
	c.ExtResolver, err = dns.NewFromConfig(&dns.ClientConfig{Servers: []string{"127.0.0.1"}, Port: "53", Timeout: 1})
	require.NoError(t, err)

	rm, err := New(c, "remote", "remote")
	require.NoError(t, err)
	r := rm.(*Target)

	cfg := config.NewMap(nil, config.Node{Children: []config.Node{{Name: "hostname", Args: []string{"mx.example.test"}}}})
	require.NoError(t, r.Configure(nil, cfg))
	t.Cleanup(func() { require.NoError(t, r.Stop()) })

	dm, err := NewDANEPolicy(c, "mx_auth.dane", "dane")
	require.NoError(t, err)
	d := dm.(*danePolicy)

	require.NoError(t, d.Configure(nil, config.NewMap(nil, config.Node{})))
	require.Same(t, c.ExtResolver, r.extResolver)
	require.Same(t, c.ExtResolver, d.extResolver)
}

func TestRemoteFallsBackToRegularResolver(t *testing.T) {
	c := container.New()

	rm, err := New(c, "remote", "remote")
	require.NoError(t, err)
	r := rm.(*Target)

	cfg := config.NewMap(nil, config.Node{Children: []config.Node{{Name: "hostname", Args: []string{"mx.example.test"}}}})
	require.NoError(t, r.Configure(nil, cfg))
	t.Cleanup(func() { require.NoError(t, r.Stop()) })
	r.resolver = &mockdns.Resolver{Zones: map[string]mockdns.Zone{
		"example.invalid.": {MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}}},
	}}

	ad, records, err := (&remoteDelivery{rt: r}).lookupMX(context.Background(), "example.invalid")
	require.NoError(t, err)
	require.False(t, ad)
	require.Equal(t, "mx.example.invalid.", records[0].Host)
}

func TestDANEWithoutResolverIsNoOp(t *testing.T) {
	c := container.New()
	m, err := NewDANEPolicy(c, "mx_auth.dane", "dane")
	require.NoError(t, err)

	p := m.(*danePolicy)
	require.NoError(t, p.Configure(nil, config.NewMap(nil, config.Node{})))

	d := p.StartDelivery(nil)
	d.PrepareConn(context.Background(), "mx.example.test")
	level, err := d.CheckConn(
		context.Background(),
		module.MXNone,
		module.TLSNone,
		"example.test",
		"mx.example.test",
		tls.ConnectionState{},
	)
	require.NoError(t, err)
	require.Equal(t, module.TLSNone, level)
}
