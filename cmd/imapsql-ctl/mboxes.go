package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	eimap "github.com/emersion/go-imap"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/urfave/cli"
)

func mboxesList(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	mboxes, err := u.ListMailboxes(ctx.Bool("subscribed,s"))
	if err != nil {
		return err
	}

	if len(mboxes) == 0 && !ctx.GlobalBool("quiet") {
		fmt.Fprintln(os.Stderr, "No mailboxes.")
	}

	for _, mbox := range mboxes {
		info, err := mbox.Info()
		if err != nil {
			return err
		}

		if len(info.Attributes) != 0 {
			fmt.Print(info.Name, "\t", info.Attributes, "\n")
		} else {
			fmt.Println(info.Name)
		}
	}

	return nil
}

func mboxesCreate(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return errors.New("Error: NAME is required")
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	if ctx.IsSet("special") {
		attr := "\\" + strings.Title(ctx.String("special"))
		return u.(*imapsql.User).CreateMailboxSpecial(name, attr)
	}

	return u.CreateMailbox(name)
}

func mboxesRemove(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

	if !ctx.GlobalBool("unsafe") {
		return errors.New("Error: Refusing to edit mailboxes without --unsafe")
	}

	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return errors.New("Error: NAME is required")
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	mbox, err := u.GetMailbox(name)
	if err != nil {
		return err
	}

	if !ctx.Bool("yes,y") {
		status, err := mbox.Status([]eimap.StatusItem{eimap.StatusMessages})
		if err != nil {
			return err
		}

		if status.Messages != 0 {
			fmt.Fprintf(os.Stderr, "Mailbox %s contains %d messages.\n", name, status.Messages)
		}

		if !Confirmation("Are you sure you want to delete that mailbox?", false) {
			return errors.New("Cancelled")
		}
	}

	if err := u.DeleteMailbox(name); err != nil {
		return err
	}

	return nil
}

func mboxesRename(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

	if !ctx.GlobalBool("unsafe") {
		return errors.New("Error: Refusing to edit mailboxes without --unsafe")
	}

	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	oldName := ctx.Args().Get(1)
	if oldName == "" {
		return errors.New("Error: OLDNAME is required")
	}
	newName := ctx.Args().Get(2)
	if newName == "" {
		return errors.New("Error: NEWNAME is required")
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	return u.RenameMailbox(oldName, newName)
}

func mboxesAppendLimit(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return errors.New("Error: MAILBOX is required")
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	mbox, err := u.GetMailbox(name)
	if err != nil {
		return err
	}

	mboxAL := mbox.(AppendLimitMbox)

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
