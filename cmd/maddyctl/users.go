package main

import (
	"errors"
	"fmt"
	"os"

	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/cmd/maddyctl/clitools"
	"github.com/urfave/cli"
	"golang.org/x/crypto/bcrypt"
)

func usersList(be UserDB, ctx *cli.Context) error {
	list, err := be.ListUsers()
	if err != nil {
		return err
	}

	if len(list) == 0 && !ctx.GlobalBool("quiet") {
		fmt.Fprintln(os.Stderr, "No users.")
	}

	for _, user := range list {
		fmt.Println(user)
	}
	return nil
}

func usersCreate(be UserDB, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}

	if ctx.IsSet("null") {
		return be.CreateUserNoPass(username)
	}

	if ctx.IsSet("hash") {
		// XXX: This needs to be updated to work with other backends in future.
		sqlbe, ok := be.(*imapsql.Backend)
		if !ok {
			return errors.New("Error: Storage does not support custom hash functions")
		}
		sqlbe.Opts.DefaultHashAlgo = ctx.String("hash")
	}
	if ctx.IsSet("bcrypt-cost") {
		if ctx.Int("bcrypt-cost") > bcrypt.MaxCost {
			return errors.New("Error: too big bcrypt cost")
		}
		if ctx.Int("bcrypt-cost") < bcrypt.MinCost {
			return errors.New("Error: too small bcrypt cost")
		}

		// XXX: This needs to be updated to work with other backends in future.
		sqlbe, ok := be.(*imapsql.Backend)
		if !ok {
			return errors.New("Error: Storage does not support custom hash cost")
		}

		sqlbe.Opts.BcryptCost = ctx.Int("bcrypt-cost")
	}

	var pass string
	if ctx.IsSet("password") {
		pass = ctx.String("password")
	} else {
		var err error
		pass, err = clitools.ReadPassword("Enter password for new user")
		if err != nil {
			return err
		}
	}

	return be.CreateUser(username, pass)
}

func usersRemove(be UserDB, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}

	if !ctx.Bool("yes") {
		if !clitools.Confirmation("Are you sure you want to delete this user account?", false) {
			return errors.New("Cancelled")
		}
	}

	return be.DeleteUser(username)
}

func usersPassword(be UserDB, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}

	if ctx.IsSet("null") {
		type ResetPassBack interface {
			ResetPassword(string) error
		}
		rpbe, ok := be.(ResetPassBack)
		if !ok {
			return errors.New("Error: Storage does not support null passwords")
		}
		return rpbe.ResetPassword(username)
	}

	if ctx.IsSet("hash") {
		// XXX: This needs to be updated to work with other backends in future.
		sqlbe, ok := be.(*imapsql.Backend)
		if !ok {
			return errors.New("Error: Storage does not support custom hash functions")
		}
		sqlbe.Opts.DefaultHashAlgo = ctx.String("hash")
	}
	if ctx.IsSet("bcrypt-cost") {
		if ctx.Int("bcrypt-cost") > bcrypt.MaxCost {
			return errors.New("Error: too big bcrypt cost")
		}
		if ctx.Int("bcrypt-cost") < bcrypt.MinCost {
			return errors.New("Error: too small bcrypt cost")
		}

		// XXX: This needs to be updated to work with other backends in future.
		sqlbe, ok := be.(*imapsql.Backend)
		if !ok {
			return errors.New("Error: Storage does not support custom hash cost")
		}

		sqlbe.Opts.BcryptCost = ctx.Int("bcrypt-cost")
	}

	var pass string
	if ctx.IsSet("password") {
		pass = ctx.String("password")
	} else {
		var err error
		pass, err = clitools.ReadPassword("Enter new password")
		if err != nil {
			return err
		}
	}

	return be.SetUserPassword(username, pass)
}
