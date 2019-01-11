package maddy

import (
	"io"

	"github.com/emersion/go-dkim"
	"github.com/emersion/go-msgauth"
)

func verifyDKIM(r io.Reader) ([]msgauth.Result, error) {
	// TODO: limit max. number of signatures
	verifications, err := dkim.Verify(r)
	if err != nil {
		return nil, err
	}

	// TODO: add ResultNone

	results := make([]msgauth.Result, 0, len(verifications))
	for _, verif := range verifications {
		var val msgauth.ResultValue
		if verif.Err == nil {
			val = msgauth.ResultPass
		} else {
			val = msgauth.ResultFail
		}

		results = append(results, &msgauth.DKIMResult{
			Value:      val,
			Domain:     verif.Domain,
			Identifier: verif.Identifier,
		})
	}

	return results, nil
}
