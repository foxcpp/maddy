package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/foxcpp/maddy/cmd/maddyctl/clitools"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/urfave/cli"
)

func usersList(be module.PlainUserDB, ctx *cli.Context) error {
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

func usersCreate(be module.PlainUserDB, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
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

func usersRemove(be module.PlainUserDB, ctx *cli.Context) error {
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

func usersPassword(be module.PlainUserDB, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
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
