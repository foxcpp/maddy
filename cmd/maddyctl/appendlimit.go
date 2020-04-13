package main

import (
	"errors"
	"fmt"

	appendlimit "github.com/emersion/go-imap-appendlimit"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/urfave/cli"
)

// Copied from go-imap-backend-tests.

// AppendLimitUser is extension for backend.User interface which allows to
// set append limit value for testing and administration purposes.
type AppendLimitUser interface {
	appendlimit.User

	// SetMessageLimit sets new value for limit.
	// nil pointer means no limit.
	SetMessageLimit(val *uint32) error
}

func imapAcctAppendlimit(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}
	userAL, ok := u.(AppendLimitUser)
	if !ok {
		return errors.New("Error: module.Storage does not support per-user append limit")
	}

	if ctx.IsSet("value") {
		val := ctx.Int("value")

		var err error
		if val == -1 {
			err = userAL.SetMessageLimit(nil)
		} else {
			val32 := uint32(val)
			err = userAL.SetMessageLimit(&val32)
		}
		if err != nil {
			return err
		}
	} else {
		lim := userAL.CreateMessageLimit()
		if lim == nil {
			fmt.Println("No limit")
		} else {
			fmt.Println(*lim)
		}
	}

	return nil
}
