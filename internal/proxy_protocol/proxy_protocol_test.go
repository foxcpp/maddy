package proxy_protocol

import (
	"net"
	"strings"
	"testing"

	"github.com/foxcpp/maddy/framework/config"
)

func TestProxyProtocolBareAddressTrustsOnlyThatAddress(t *testing.T) {
	tests := []struct {
		name          string
		address       string
		otherAddress  string
		wantPrefixLen int
	}{
		{
			name:          "IPv4",
			address:       "192.0.2.10",
			otherAddress:  "192.0.2.11",
			wantPrefixLen: 32,
		},
		{
			name:          "IPv6",
			address:       "2001:db8:1234::10",
			otherAddress:  "2001:db8:1234::20",
			wantPrefixLen: 128,
		},
		{
			name:          "IPv4-mapped IPv6",
			address:       "::ffff:192.0.2.10",
			otherAddress:  "::ffff:192.0.2.11",
			wantPrefixLen: 128,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configuredIP := net.ParseIP(tt.address)
			otherIP := net.ParseIP(tt.otherAddress)
			parsed, err := ProxyProtocolDirective(nil, config.Node{Args: []string{tt.address}})
			if err != nil {
				t.Fatalf("ProxyProtocolDirective() error = %v", err)
			}

			proxy := parsed.(*ProxyProtocol)
			if len(proxy.trust) != 1 {
				t.Fatalf("got %d trusted networks, want 1", len(proxy.trust))
			}

			ones, bits := proxy.trust[0].Mask.Size()
			wantBits := 32
			if strings.Contains(tt.address, ":") {
				wantBits = 128
			}
			if ones != tt.wantPrefixLen || bits != wantBits {
				t.Errorf("trusted network mask = /%d of %d bits, want /%d", ones, bits, tt.wantPrefixLen)
			}
			if !proxy.trust[0].Contains(configuredIP) {
				t.Errorf("trusted network does not contain configured address %s", tt.address)
			}
			if proxy.trust[0].Contains(otherIP) {
				t.Errorf("trusted network unexpectedly contains other address %s", tt.otherAddress)
			}
		})
	}
}

func TestProxyProtocolExplicitIPv6CIDRIsPreserved(t *testing.T) {
	parsed, err := ProxyProtocolDirective(nil, config.Node{Args: []string{"2001:db8:1234::/64"}})
	if err != nil {
		t.Fatalf("ProxyProtocolDirective() error = %v", err)
	}

	proxy := parsed.(*ProxyProtocol)
	ones, bits := proxy.trust[0].Mask.Size()
	if ones != 64 || bits != 128 {
		t.Errorf("trusted network mask = /%d of %d bits, want /64 of 128 bits", ones, bits)
	}
}
