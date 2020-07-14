package requiretls

import (
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/check"
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
	check.RegisterStatelessCheck("require_tls", modconfig.FailAction{Reject: true}, requireTLS, nil, nil, nil)
}
