package authz

import (
	"strings"

	"github.com/foxcpp/maddy/framework/address"
	"golang.org/x/text/secure/precis"
)

type NormalizeFunc func(string) (string, error)

func NormalizeNoop(s string) (string, error) {
	return s, nil
}

// NormalizeAuto applies address.PRECISFold to valid emails and
// plain UsernameCaseMapped profile to other strings.
func NormalizeAuto(s string) (string, error) {
	if address.Valid(s) {
		return address.PRECISFold(s)
	}
	return precis.UsernameCaseMapped.CompareKey(s)
}

// NormalizeFuncs defines configurable normalization functions to be used
// in authentication and authorization routines.
var NormalizeFuncs = map[string]NormalizeFunc{
	"auto":                  NormalizeAuto,
	"precis_casefold_email": address.PRECISFold,
	"precis_casefold":       precis.UsernameCaseMapped.CompareKey,
	"precis_email":          address.PRECIS,
	"precis":                precis.UsernameCasePreserved.CompareKey,
	"casefold": func(s string) (string, error) {
		return strings.ToLower(s), nil
	},
	"noop": NormalizeNoop,
}
