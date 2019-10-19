package main

import (
	"errors"
	"fmt"

	appendlimit "github.com/emersion/go-imap-appendlimit"
	"github.com/urfave/cli"
)

// Copied from go-imap-backend-tests.

// AppendLimitStorage is extension for main backend interface (backend.Storage) which
// allows to set append limit value for testing and administration purposes.
type AppendLimitStorage interface {
	appendlimit.Backend

	// SetMessageLimit sets new value for limit.
	// nil pointer means no limit.
	SetMessageLimit(val *uint32) error
}

// AppendLimitUser is extension for backend.User interface which allows to
// set append limit value for testing and administration purposes.
type AppendLimitUser interface {
	appendlimit.User

	// SetMessageLimit sets new value for limit.
	// nil pointer means no limit.
	SetMessageLimit(val *uint32) error
}

// AppendLimitMbox is extension for backend.Mailbox interface which allows to
// set append limit value for testing and administration purposes.
type AppendLimitMbox interface {
	CreateMessageLimit() *uint32

	// SetMessageLimit sets new value for limit.
	// nil pointer means no limit.
	SetMessageLimit(val *uint32) error
}

func mboxesAppendLimit(be Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return errors.New("Error:MAILBOX is required")
	}

	u, err := be.GetUser(username)
	if err != nil {
		return err
	}

	mbox, err := u.GetMailbox(name)
	if err != nil {
		return err
	}

	mboxAL, ok := mbox.(AppendLimitMbox)
	if !ok {
		return errors.New("Error: Storage does not support per-mailbox append limit")
	}

	if ctx.IsSet("value,v") {
		val := ctx.Int("value,v")

		var err error
		if val == -1 {
			err = mboxAL.SetMessageLimit(nil)
		} else {
			val32 := uint32(val)
			err = mboxAL.SetMessageLimit(&val32)
		}
		if err != nil {
			return err
		}
	} else {
		lim := mboxAL.CreateMessageLimit()
		if lim == nil {
			fmt.Println("No limit")
		} else {
			fmt.Println(*lim)
		}
	}

	return nil
}

func usersAppendlimit(be Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}

	u, err := be.GetUser(username)
	if err != nil {
		return err
	}
	userAL, ok := u.(AppendLimitUser)
	if !ok {
		return errors.New("Error: Storage does not support per-user append limit")
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
