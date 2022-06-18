package authz

import (
	"context"
	"fmt"

	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/module"
)

func AuthorizeEmailUse(ctx context.Context, username string, addrs []string, mapping module.Table) (bool, error) {
	for _, addr := range addrs {
		_, domain, err := address.Split(addr)
		if err != nil {
			return false, fmt.Errorf("authz: %w", err)
		}

		var validEmails []string
		if multi, ok := mapping.(module.MultiTable); ok {
			validEmails, err = multi.LookupMulti(ctx, username)
			if err != nil {
				return false, fmt.Errorf("authz: %w", err)
			}
		} else {
			validEmail, ok, err := mapping.Lookup(ctx, username)
			if err != nil {
				return false, fmt.Errorf("authz: %w", err)
			}
			if ok {
				validEmails = []string{validEmail}
			}
		}

		for _, ent := range validEmails {
			if ent == domain || ent == "*" || ent == addr {
				return true, nil
			}
		}
	}

	return false, nil
}
