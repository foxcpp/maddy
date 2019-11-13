package requiretls

import (
	"github.com/foxcpp/maddy/internal/check"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/module"
)

func requireTLS(ctx check.StatelessCheckContext) module.CheckResult {
	if ctx.MsgMeta.Conn != nil && ctx.MsgMeta.Conn.TLS.HandshakeComplete {
		return module.CheckResult{}
	}

	return module.CheckResult{
		Reason: &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 7, 1},
			Message:      "TLS conversation required",
			CheckName:    "require_tls",
		},
	}
}

func init() {
	check.RegisterStatelessCheck("require_tls", check.FailAction{Reject: true}, requireTLS, nil, nil, nil)
}
