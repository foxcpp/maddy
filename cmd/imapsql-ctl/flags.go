package main

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/urfave/cli"
)

func msgsFlags(ctx *cli.Context) error {
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
		return errors.New("Error: MAILBOX is required")
	}
	seqStr := ctx.Args().Get(2)
	if seqStr == "" {
		return errors.New("Error: SEQ is required")
	}

	seq, err := imap.ParseSeqSet(seqStr)
	if err != nil {
		return err
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	mbox, err := u.GetMailbox(name)
	if err != nil {
		return err
	}

	flags := ctx.Args()[3:]
	if len(flags) == 0 {
		return errors.New("Error: at least once FLAG is required")
	}

	var op imap.FlagsOp
	switch ctx.Command.Name {
	case "add-flags":
		op = imap.AddFlags
	case "rem-flags":
		op = imap.RemoveFlags
	case "set-flags":
		op = imap.SetFlags
	default:
		panic("unknown command: " + ctx.Command.Name)
	}

	return mbox.UpdateMessagesFlags(ctx.IsSet("uid"), seq, op, flags)
}
