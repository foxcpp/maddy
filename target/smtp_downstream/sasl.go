package smtp_downstream

import (
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/config"
)

type saslClientFactory func(downstreamUser, downstreamPass string) (sasl.Client, error)

// saslAuthDirective returns saslClientFactory function used to create sasl.Client.
// for use in outbound connections.
//
// Authentication information of the current client should be passed in arguments.
func (u *Upstream) saslAuthDirective(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Children) != 0 {
		return nil, m.MatchErr("can't declare a block here")
	}
	if len(node.Args) == 0 {
		return nil, m.MatchErr("at least one argument required")
	}
	switch node.Args[0] {
	case "off":
		return nil, nil
	case "forward":
		if len(node.Args) > 1 {
			return nil, m.MatchErr("no additional arguments required")
		}
		return func(downstreamUser, downstreamPass string) (sasl.Client, error) {
			if downstreamUser == "" || downstreamPass == "" {
				u.log.Printf("client is not authenticated, can't forward credentials")
				return nil, &smtp.SMTPError{
					Code:         530,
					EnhancedCode: smtp.EnhancedCode{5, 7, 0},
					Message:      "Authentication is required",
				}
			}
			// TODO: See if it is useful to support custom identity argument.
			return sasl.NewPlainClient("", downstreamUser, downstreamPass), nil
		}, nil
	case "plain":
		if len(node.Args) != 3 {
			return nil, m.MatchErr("two additional arguments are required (username, password)")
		}
		return func(_, _ string) (sasl.Client, error) {
			return sasl.NewPlainClient("", node.Args[1], node.Args[2]), nil
		}, nil
	case "external":
		if len(node.Args) > 1 {
			return nil, m.MatchErr("no additional arguments required")
		}
		return func(_, _ string) (sasl.Client, error) {
			return sasl.NewExternalClient(""), nil
		}, nil
	default:
		return nil, m.MatchErr("unknown authentication mechanism: %s", node.Args[0])
	}
}
