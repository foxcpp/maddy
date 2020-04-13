package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/foxcpp/maddy/cmd/maddyctl/clitools"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/urfave/cli"
)

func imapAcctList(be module.Storage, ctx *cli.Context) error {
	mbe, ok := be.(module.ManageableStorage)
	if !ok {
		return errors.New("Error: storage backend does not support accounts management using maddyctl")
	}

	list, err := mbe.ListAccts()
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

func imapAcctCreate(be module.Storage, ctx *cli.Context) error {
	mbe, ok := be.(module.ManageableStorage)
	if !ok {
		return errors.New("Error: storage backend does not support accounts management using maddyctl")
	}

	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}

	return mbe.CreateAcct(username)
}

func imapAcctRemove(be module.Storage, ctx *cli.Context) error {
	mbe, ok := be.(module.ManageableStorage)
	if !ok {
		return errors.New("Error: storage backend does not support accounts management using maddyctl")
	}

	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}

	if !ctx.Bool("yes") {
		if !clitools.Confirmation("Are you sure you want to delete this user account?", false) {
			return errors.New("Cancelled")
		}
	}

	return mbe.DeleteAcct(username)
}
