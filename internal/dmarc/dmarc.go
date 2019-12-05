package dmarc

import (
	"context"

	"github.com/emersion/go-msgauth/dmarc"
)

type (
	Resolver interface {
		LookupTXT(context.Context, string) ([]string, error)
	}

	Record         = dmarc.Record
	Policy         = dmarc.Policy
	AlignmentMode  = dmarc.AlignmentMode
	FailureOptions = dmarc.FailureOptions
)

const (
	PolicyNone       = dmarc.PolicyNone
	PolicyReject     = dmarc.PolicyReject
	PolicyQuarantine = dmarc.PolicyQuarantine
)
