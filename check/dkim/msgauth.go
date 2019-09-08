package dkim

import (
	"io"

	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-msgauth/dkim"
)

func verifyDKIM(r io.Reader) ([]authres.Result, error) {
	// TODO: limit max. number of signatures
	verifications, err := dkim.Verify(r)
	if err != nil {
		return nil, err
	}

	// TODO: add ResultNone

	results := make([]authres.Result, 0, len(verifications))
	for _, verif := range verifications {
		var val authres.ResultValue
		if verif.Err == nil {
			val = authres.ResultPass
		} else {
			val = authres.ResultFail
		}

		results = append(results, &authres.DKIMResult{
			Value:      val,
			Domain:     verif.Domain,
			Identifier: verif.Identifier,
		})
	}

	return results, nil
}
