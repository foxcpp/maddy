//go:build !no_imap1

package maddy

import (
	_ "github.com/foxcpp/maddy/internal/endpoint/imap"
	_ "github.com/foxcpp/maddy/internal/storage/v1/imapsql"
)
