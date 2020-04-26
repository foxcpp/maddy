package main

import (
	"errors"
	"fmt"
	"os"

	specialuse "github.com/emersion/go-imap-specialuse"
	"github.com/foxcpp/maddy/cmd/maddyctl/clitools"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/urfave/cli"
)

type SpecialUseUser interface {
	CreateMailboxSpecial(name, specialUseAttr string) error
}

func imapAcctList(be module.Storage, ctx *cli.Context) error {
	mbe, ok := be.(module.ManageableStorage)
	if !ok {
		return errors.New("Error: storage backend does not support accounts management using maddyctl")
	}

	list, err := mbe.ListIMAPAccts()
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

	if err := mbe.CreateIMAPAcct(username); err != nil {
		return err
	}

	act, err := mbe.GetIMAPAcct(username)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	suu, ok := act.(SpecialUseUser)
	if !ok {
		fmt.Fprintf(os.Stderr, "Note: Storage backend does not support SPECIAL-USE IMAP extension")
	}

	createMbox := func(name, specialUseAttr string) error {
		if suu == nil {
			return act.CreateMailbox(name)
		}
		return suu.CreateMailboxSpecial(name, specialUseAttr)
	}

	if name := ctx.String("sent-name"); name != "" {
		if err := createMbox(name, specialuse.Sent); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create sent folder: %v", err)
		}
	}
	if name := ctx.String("trash-name"); name != "" {
		if err := createMbox(name, specialuse.Trash); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create trash folder: %v", err)
		}
	}
	if name := ctx.String("junk-name"); name != "" {
		if err := createMbox(name, specialuse.Junk); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create junk folder: %v", err)
		}
	}
	if name := ctx.String("drafts-name"); name != "" {
		if err := createMbox(name, specialuse.Drafts); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create drafts folder: %v", err)
		}
	}
	if name := ctx.String("archive-name"); name != "" {
		if err := createMbox(name, specialuse.Archive); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create archive folder: %v", err)
		}
	}

	return nil
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

	return mbe.DeleteIMAPAcct(username)
}
