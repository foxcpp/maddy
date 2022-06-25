/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package ctl

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/framework/module"
	maddycli "github.com/foxcpp/maddy/internal/cli"
	clitools2 "github.com/foxcpp/maddy/internal/cli/clitools"
	"github.com/urfave/cli/v2"
)

func init() {
	maddycli.AddSubcommand(
		&cli.Command{
			Name:  "imap-mboxes",
			Usage: "IMAP mailboxes (folders) management",
			Subcommands: []*cli.Command{
				{
					Name:      "list",
					Usage:     "Show mailboxes of user",
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "cfg-block",
							Usage:   "Module configuration block to use",
							EnvVars: []string{"MADDY_CFGBLOCK"},
							Value:   "local_mailboxes",
						},
						&cli.BoolFlag{
							Name:    "subscribed",
							Aliases: []string{"s"},
							Usage:   "List only subscribed mailboxes",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return mboxesList(be, ctx)
					},
				},
				{
					Name:      "create",
					Usage:     "Create mailbox",
					ArgsUsage: "USERNAME NAME",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "cfg-block",
							Usage:   "Module configuration block to use",
							EnvVars: []string{"MADDY_CFGBLOCK"},
							Value:   "local_mailboxes",
						},
						&cli.StringFlag{
							Name:  "special",
							Usage: "Set SPECIAL-USE attribute on mailbox; valid values: archive, drafts, junk, sent, trash",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return mboxesCreate(be, ctx)
					},
				},
				{
					Name:        "remove",
					Usage:       "Remove mailbox",
					Description: "WARNING: All contents of mailbox will be irrecoverably lost.",
					ArgsUsage:   "USERNAME MAILBOX",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "cfg-block",
							Usage:   "Module configuration block to use",
							EnvVars: []string{"MADDY_CFGBLOCK"},
							Value:   "local_mailboxes",
						},
						&cli.BoolFlag{
							Name:    "yes",
							Aliases: []string{"y"},
							Usage:   "Don't ask for confirmation",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return mboxesRemove(be, ctx)
					},
				},
				{
					Name:        "rename",
					Usage:       "Rename mailbox",
					Description: "Rename may cause unexpected failures on client-side so be careful.",
					ArgsUsage:   "USERNAME OLDNAME NEWNAME",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "cfg-block",
							Usage:   "Module configuration block to use",
							EnvVars: []string{"MADDY_CFGBLOCK"},
							Value:   "local_mailboxes",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return mboxesRename(be, ctx)
					},
				},
			},
		})
	maddycli.AddSubcommand(&cli.Command{
		Name:  "imap-msgs",
		Usage: "IMAP messages management",
		Subcommands: []*cli.Command{
			{
				Name:        "add",
				Usage:       "Add message to mailbox",
				ArgsUsage:   "USERNAME MAILBOX",
				Description: "Reads message body (with headers) from stdin. Prints UID of created message on success.",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "cfg-block",
						Usage:   "Module configuration block to use",
						EnvVars: []string{"MADDY_CFGBLOCK"},
						Value:   "local_mailboxes",
					},
					&cli.StringSliceFlag{
						Name:    "flag",
						Aliases: []string{"f"},
						Usage:   "Add flag to message. Can be specified multiple times",
					},
					&cli.TimestampFlag{
						Layout:  time.RFC3339,
						Name:    "date",
						Aliases: []string{"d"},
						Usage:   "Set internal date value to specified one in ISO 8601 format (2006-01-02T15:04:05Z07:00)",
					},
				},
				Action: func(ctx *cli.Context) error {
					be, err := openStorage(ctx)
					if err != nil {
						return err
					}
					defer closeIfNeeded(be)
					return msgsAdd(be, ctx)
				},
			},
			{
				Name:        "add-flags",
				Usage:       "Add flags to messages",
				ArgsUsage:   "USERNAME MAILBOX SEQ FLAGS...",
				Description: "Add flags to all messages matched by SEQ.",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "cfg-block",
						Usage:   "Module configuration block to use",
						EnvVars: []string{"MADDY_CFGBLOCK"},
						Value:   "local_mailboxes",
					},
					&cli.BoolFlag{
						Name:    "uid",
						Aliases: []string{"u"},
						Usage:   "Use UIDs for SEQSET instead of sequence numbers",
					},
				},
				Action: func(ctx *cli.Context) error {
					be, err := openStorage(ctx)
					if err != nil {
						return err
					}
					defer closeIfNeeded(be)
					return msgsFlags(be, ctx)
				},
			},
			{
				Name:        "rem-flags",
				Usage:       "Remove flags from messages",
				ArgsUsage:   "USERNAME MAILBOX SEQ FLAGS...",
				Description: "Remove flags from all messages matched by SEQ.",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "cfg-block",
						Usage:   "Module configuration block to use",
						EnvVars: []string{"MADDY_CFGBLOCK"},
						Value:   "local_mailboxes",
					},
					&cli.BoolFlag{
						Name:    "uid",
						Aliases: []string{"u"},
						Usage:   "Use UIDs for SEQSET instead of sequence numbers",
					},
				},
				Action: func(ctx *cli.Context) error {
					be, err := openStorage(ctx)
					if err != nil {
						return err
					}
					defer closeIfNeeded(be)
					return msgsFlags(be, ctx)
				},
			},
			{
				Name:        "set-flags",
				Usage:       "Set flags on messages",
				ArgsUsage:   "USERNAME MAILBOX SEQ FLAGS...",
				Description: "Set flags on all messages matched by SEQ.",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "cfg-block",
						Usage:   "Module configuration block to use",
						EnvVars: []string{"MADDY_CFGBLOCK"},
						Value:   "local_mailboxes",
					},
					&cli.BoolFlag{
						Name:    "uid",
						Aliases: []string{"u"},
						Usage:   "Use UIDs for SEQSET instead of sequence numbers",
					},
				},
				Action: func(ctx *cli.Context) error {
					be, err := openStorage(ctx)
					if err != nil {
						return err
					}
					defer closeIfNeeded(be)
					return msgsFlags(be, ctx)
				},
			},
			{
				Name:      "remove",
				Usage:     "Remove messages from mailbox",
				ArgsUsage: "USERNAME MAILBOX SEQSET",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "cfg-block",
						Usage:   "Module configuration block to use",
						EnvVars: []string{"MADDY_CFGBLOCK"},
						Value:   "local_mailboxes",
					},
					&cli.BoolFlag{
						Name:    "uid,u",
						Aliases: []string{"u"},
						Usage:   "Use UIDs for SEQSET instead of sequence numbers",
					},
					&cli.BoolFlag{
						Name:    "yes",
						Aliases: []string{"y"},
						Usage:   "Don't ask for confirmation",
					},
				},
				Action: func(ctx *cli.Context) error {
					be, err := openStorage(ctx)
					if err != nil {
						return err
					}
					defer closeIfNeeded(be)
					return msgsRemove(be, ctx)
				},
			},
			{
				Name:        "copy",
				Usage:       "Copy messages between mailboxes",
				Description: "Note: You can't copy between mailboxes of different users. APPENDLIMIT of target mailbox is not enforced.",
				ArgsUsage:   "USERNAME SRCMAILBOX SEQSET TGTMAILBOX",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "cfg-block",
						Usage:   "Module configuration block to use",
						EnvVars: []string{"MADDY_CFGBLOCK"},
						Value:   "local_mailboxes",
					},
					&cli.BoolFlag{
						Name:    "uid",
						Aliases: []string{"u"},
						Usage:   "Use UIDs for SEQSET instead of sequence numbers",
					},
				},
				Action: func(ctx *cli.Context) error {
					be, err := openStorage(ctx)
					if err != nil {
						return err
					}
					defer closeIfNeeded(be)
					return msgsCopy(be, ctx)
				},
			},
			{
				Name:        "move",
				Usage:       "Move messages between mailboxes",
				Description: "Note: You can't move between mailboxes of different users. APPENDLIMIT of target mailbox is not enforced.",
				ArgsUsage:   "USERNAME SRCMAILBOX SEQSET TGTMAILBOX",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "cfg-block",
						Usage:   "Module configuration block to use",
						EnvVars: []string{"MADDY_CFGBLOCK"},
						Value:   "local_mailboxes",
					},
					&cli.BoolFlag{
						Name:    "uid",
						Aliases: []string{"u"},
						Usage:   "Use UIDs for SEQSET instead of sequence numbers",
					},
				},
				Action: func(ctx *cli.Context) error {
					be, err := openStorage(ctx)
					if err != nil {
						return err
					}
					defer closeIfNeeded(be)
					return msgsMove(be, ctx)
				},
			},
			{
				Name:        "list",
				Usage:       "List messages in mailbox",
				Description: "If SEQSET is specified - only show messages that match it.",
				ArgsUsage:   "USERNAME MAILBOX [SEQSET]",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "cfg-block",
						Usage:   "Module configuration block to use",
						EnvVars: []string{"MADDY_CFGBLOCK"},
						Value:   "local_mailboxes",
					},
					&cli.BoolFlag{
						Name:    "uid",
						Aliases: []string{"u"},
						Usage:   "Use UIDs for SEQSET instead of sequence numbers",
					},
					&cli.BoolFlag{
						Name:    "full,f",
						Aliases: []string{"f"},
						Usage:   "Show entire envelope and all server meta-data",
					},
				},
				Action: func(ctx *cli.Context) error {
					be, err := openStorage(ctx)
					if err != nil {
						return err
					}
					defer closeIfNeeded(be)
					return msgsList(be, ctx)
				},
			},
			{
				Name:        "dump",
				Usage:       "Dump message body",
				Description: "If passed SEQ matches multiple messages - they will be joined.",
				ArgsUsage:   "USERNAME MAILBOX SEQ",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "cfg-block",
						Usage:   "Module configuration block to use",
						EnvVars: []string{"MADDY_CFGBLOCK"},
						Value:   "local_mailboxes",
					},
					&cli.BoolFlag{
						Name:    "uid",
						Aliases: []string{"u"},
						Usage:   "Use UIDs for SEQ instead of sequence numbers",
					},
				},
				Action: func(ctx *cli.Context) error {
					be, err := openStorage(ctx)
					if err != nil {
						return err
					}
					defer closeIfNeeded(be)
					return msgsDump(be, ctx)
				},
			},
		},
	})
}

func FormatAddress(addr *imap.Address) string {
	return fmt.Sprintf("%s <%s@%s>", addr.PersonalName, addr.MailboxName, addr.HostName)
}

func FormatAddressList(addrs []*imap.Address) string {
	res := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		res = append(res, FormatAddress(addr))
	}
	return strings.Join(res, ", ")
}

func mboxesList(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	mboxes, err := u.ListMailboxes(ctx.Bool("subscribed,s"))
	if err != nil {
		return err
	}

	if len(mboxes) == 0 && !ctx.Bool("quiet") {
		fmt.Fprintln(os.Stderr, "No mailboxes.")
	}

	for _, info := range mboxes {
		if len(info.Attributes) != 0 {
			fmt.Print(info.Name, "\t", info.Attributes, "\n")
		} else {
			fmt.Println(info.Name)
		}
	}

	return nil
}

func mboxesCreate(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return cli.Exit("Error: NAME is required", 2)
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	if ctx.IsSet("special") {
		attr := "\\" + strings.Title(ctx.String("special")) //nolint:staticcheck
		// (nolint) strings.Title is perfectly fine there since special mailbox tags will never use Unicode.

		suu, ok := u.(SpecialUseUser)
		if !ok {
			return cli.Exit("Error: storage backend does not support SPECIAL-USE IMAP extension", 2)
		}

		return suu.CreateMailboxSpecial(name, attr)
	}

	return u.CreateMailbox(name)
}

func mboxesRemove(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return cli.Exit("Error: NAME is required", 2)
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	if !ctx.Bool("yes") {
		status, err := u.Status(name, []imap.StatusItem{imap.StatusMessages})
		if err != nil {
			return err
		}

		if status.Messages != 0 {
			fmt.Fprintf(os.Stderr, "Mailbox %s contains %d messages.\n", name, status.Messages)
		}

		if !clitools2.Confirmation("Are you sure you want to delete that mailbox?", false) {
			return errors.New("Cancelled")
		}
	}

	return u.DeleteMailbox(name)
}

func mboxesRename(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	oldName := ctx.Args().Get(1)
	if oldName == "" {
		return cli.Exit("Error: OLDNAME is required", 2)
	}
	newName := ctx.Args().Get(2)
	if newName == "" {
		return cli.Exit("Error: NEWNAME is required", 2)
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	return u.RenameMailbox(oldName, newName)
}

func msgsAdd(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return cli.Exit("Error: MAILBOX is required", 2)
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	flags := ctx.StringSlice("flag")
	if flags == nil {
		flags = []string{}
	}

	date := time.Now()
	if ctx.IsSet("date") {
		date = *ctx.Timestamp("date")
	}

	buf := bytes.Buffer{}
	if _, err := io.Copy(&buf, os.Stdin); err != nil {
		return err
	}

	if buf.Len() == 0 {
		return cli.Exit("Error: Empty message, refusing to continue", 2)
	}

	status, err := u.Status(name, []imap.StatusItem{imap.StatusUidNext})
	if err != nil {
		return err
	}

	if err := u.CreateMessage(name, flags, date, &buf, nil); err != nil {
		return err
	}

	// TODO: Use APPENDUID
	fmt.Println(status.UidNext)

	return nil
}

func msgsRemove(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return cli.Exit("Error: MAILBOX is required", 2)
	}
	seqset := ctx.Args().Get(2)
	if seqset == "" {
		return cli.Exit("Error: SEQSET is required", 2)
	}

	if !ctx.Bool("uid") {
		fmt.Fprintln(os.Stderr, "WARNING: --uid=true will be the default in 0.7")
	}

	seq, err := imap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, mbox, err := u.GetMailbox(name, true, nil)
	if err != nil {
		return err
	}

	if !ctx.Bool("yes") {
		if !clitools2.Confirmation("Are you sure you want to delete these messages?", false) {
			return errors.New("Cancelled")
		}
	}

	mboxB := mbox.(*imapsql.Mailbox)
	return mboxB.DelMessages(ctx.Bool("uid"), seq)
}

func msgsCopy(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	srcName := ctx.Args().Get(1)
	if srcName == "" {
		return cli.Exit("Error: SRCMAILBOX is required", 2)
	}
	seqset := ctx.Args().Get(2)
	if seqset == "" {
		return cli.Exit("Error: SEQSET is required", 2)
	}
	tgtName := ctx.Args().Get(3)
	if tgtName == "" {
		return cli.Exit("Error: TGTMAILBOX is required", 2)
	}

	if !ctx.Bool("uid") {
		fmt.Fprintln(os.Stderr, "WARNING: --uid=true will be the default in 0.7")
	}

	seq, err := imap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, srcMbox, err := u.GetMailbox(srcName, true, nil)
	if err != nil {
		return err
	}

	return srcMbox.CopyMessages(ctx.Bool("uid"), seq, tgtName)
}

func msgsMove(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	srcName := ctx.Args().Get(1)
	if srcName == "" {
		return cli.Exit("Error: SRCMAILBOX is required", 2)
	}
	seqset := ctx.Args().Get(2)
	if seqset == "" {
		return cli.Exit("Error: SEQSET is required", 2)
	}
	tgtName := ctx.Args().Get(3)
	if tgtName == "" {
		return cli.Exit("Error: TGTMAILBOX is required", 2)
	}

	if !ctx.Bool("uid") {
		fmt.Fprintln(os.Stderr, "WARNING: --uid=true will be the default in 0.7")
	}

	seq, err := imap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, srcMbox, err := u.GetMailbox(srcName, true, nil)
	if err != nil {
		return err
	}

	moveMbox := srcMbox.(*imapsql.Mailbox)

	return moveMbox.MoveMessages(ctx.Bool("uid"), seq, tgtName)
}

func msgsList(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	mboxName := ctx.Args().Get(1)
	if mboxName == "" {
		return cli.Exit("Error: MAILBOX is required", 2)
	}
	seqset := ctx.Args().Get(2)
	uid := ctx.Bool("uid")
	if seqset == "" {
		seqset = "1:*"
		uid = true
	} else if !uid {
		fmt.Fprintln(os.Stderr, "WARNING: --uid=true will be the default in 0.7")
	}

	seq, err := imap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, mbox, err := u.GetMailbox(mboxName, true, nil)
	if err != nil {
		return err
	}

	ch := make(chan *imap.Message, 10)
	go func() {
		err = mbox.ListMessages(uid, seq, []imap.FetchItem{imap.FetchEnvelope, imap.FetchInternalDate, imap.FetchRFC822Size, imap.FetchFlags, imap.FetchUid}, ch)
	}()

	for msg := range ch {
		if !ctx.Bool("full") {
			fmt.Printf("UID %d: %s - %s\n  %v, %v\n\n", msg.Uid, FormatAddressList(msg.Envelope.From), msg.Envelope.Subject, msg.Flags, msg.Envelope.Date)
			continue
		}

		fmt.Println("- Server meta-data:")
		fmt.Println("UID:", msg.Uid)
		fmt.Println("Sequence number:", msg.SeqNum)
		fmt.Println("Flags:", msg.Flags)
		fmt.Println("Body size:", msg.Size)
		fmt.Println("Internal date:", msg.InternalDate.Unix(), msg.InternalDate)
		fmt.Println("- Envelope:")
		if len(msg.Envelope.From) != 0 {
			fmt.Println("From:", FormatAddressList(msg.Envelope.From))
		}
		if len(msg.Envelope.To) != 0 {
			fmt.Println("To:", FormatAddressList(msg.Envelope.To))
		}
		if len(msg.Envelope.Cc) != 0 {
			fmt.Println("CC:", FormatAddressList(msg.Envelope.Cc))
		}
		if len(msg.Envelope.Bcc) != 0 {
			fmt.Println("BCC:", FormatAddressList(msg.Envelope.Bcc))
		}
		if msg.Envelope.InReplyTo != "" {
			fmt.Println("In-Reply-To:", msg.Envelope.InReplyTo)
		}
		if msg.Envelope.MessageId != "" {
			fmt.Println("Message-Id:", msg.Envelope.MessageId)
		}
		if !msg.Envelope.Date.IsZero() {
			fmt.Println("Date:", msg.Envelope.Date.Unix(), msg.Envelope.Date)
		}
		if msg.Envelope.Subject != "" {
			fmt.Println("Subject:", msg.Envelope.Subject)
		}
		fmt.Println()
	}
	return err
}

func msgsDump(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	mboxName := ctx.Args().Get(1)
	if mboxName == "" {
		return cli.Exit("Error: MAILBOX is required", 2)
	}
	seqset := ctx.Args().Get(2)
	uid := ctx.Bool("uid")
	if seqset == "" {
		seqset = "1:*"
		uid = true
	} else if !uid {
		fmt.Fprintln(os.Stderr, "WARNING: --uid=true will be the default in 0.7")
	}

	seq, err := imap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, mbox, err := u.GetMailbox(mboxName, true, nil)
	if err != nil {
		return err
	}

	ch := make(chan *imap.Message, 10)
	go func() {
		err = mbox.ListMessages(uid, seq, []imap.FetchItem{imap.FetchRFC822}, ch)
	}()

	for msg := range ch {
		for _, v := range msg.Body {
			if _, err := io.Copy(os.Stdout, v); err != nil {
				return err
			}
		}
	}
	return err
}

func msgsFlags(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return cli.Exit("Error: MAILBOX is required", 2)
	}
	seqStr := ctx.Args().Get(2)
	if seqStr == "" {
		return cli.Exit("Error: SEQ is required", 2)
	}

	if !ctx.Bool("uid") {
		fmt.Fprintln(os.Stderr, "WARNING: --uid=true will be the default in 0.7")
	}

	seq, err := imap.ParseSeqSet(seqStr)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, mbox, err := u.GetMailbox(name, false, nil)
	if err != nil {
		return err
	}

	flags := ctx.Args().Slice()[3:]
	if len(flags) == 0 {
		return cli.Exit("Error: at least once FLAG is required", 2)
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

	return mbox.UpdateMessagesFlags(ctx.Bool("uid"), seq, op, true, flags)
}
