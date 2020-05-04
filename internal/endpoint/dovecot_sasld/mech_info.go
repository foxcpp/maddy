package dovecotsasld

import (
	"github.com/emersion/go-sasl"
	dovecotsasl "github.com/foxcpp/go-dovecot-sasl"
)

var mechInfo = map[string]dovecotsasl.Mechanism{
	sasl.Plain: {
		Plaintext: true,
	},
	sasl.Login: {
		Plaintext: true,
	},
}
