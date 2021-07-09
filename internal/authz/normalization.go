package authz

import (
	"strings"

	"github.com/foxcpp/maddy/framework/address"
	"golang.org/x/text/secure/precis"
)

// NormalizeFuncs defines configurable normalization functions to be used
// in authentication and authorization routines.
var NormalizeFuncs = map[string]func(string) (string, error){
	"precis_casefold_email": address.PRECISFold,
	"precis_casefold":       precis.UsernameCaseMapped.CompareKey,
	"precis_email":          address.PRECIS,
	"precis":                precis.UsernameCasePreserved.CompareKey,
	"casefold": func(s string) (string, error) {
		return strings.ToLower(s), nil
	},
	"noop": func(s string) (string, error) {
		return s, nil
	},
}
