package address_test

import (
	"testing"

	"github.com/foxcpp/maddy/framework/address"
)

func TestValidMailboxName(t *testing.T) {
	if !address.ValidMailboxName("caddy.bug") {
		t.Error("caddy.bug should be valid mailbox name")
	}
}
