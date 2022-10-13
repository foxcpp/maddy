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
	"errors"
	"fmt"
	"os"

	"github.com/emersion/go-imap"
	"github.com/foxcpp/maddy/framework/module"
	maddycli "github.com/foxcpp/maddy/internal/cli"
	clitools2 "github.com/foxcpp/maddy/internal/cli/clitools"
	"github.com/urfave/cli/v2"
)

func init() {
	maddycli.AddSubcommand(
		&cli.Command{
			Name:  "imap-acct",
			Usage: "IMAP storage accounts management",
			Description: `These subcommands can be used to list/create/delete IMAP storage
accounts for any storage backend supported by maddy.

The corresponding storage backend should be configured in maddy.conf and be
defined in a top-level configuration block. By default, the name of that
block should be local_mailboxes but this can be changed using --cfg-block
flag for subcommands.

Note that in default configuration it is not enough to create an IMAP storage
account to grant server access. Additionally, user credentials should
be created using 'creds' subcommand.
`,
			Subcommands: []*cli.Command{
				{
					Name:  "list",
					Usage: "List storage accounts",
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
						return imapAcctList(be, ctx)
					},
				},
				{
					Name:  "create",
					Usage: "Create IMAP storage account",
					Description: `In addition to account creation, this command
creates a set of default folder (mailboxes) with special-use attribute set.`,
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "cfg-block",
							Usage:   "Module configuration block to use",
							EnvVars: []string{"MADDY_CFGBLOCK"},
							Value:   "local_mailboxes",
						},
						&cli.StringFlag{
							Name:  "sent-name",
							Usage: "Name of special mailbox for sent messages, use empty string to not create any",
							Value: "Sent",
						},
						&cli.StringFlag{
							Name:  "trash-name",
							Usage: "Name of special mailbox for trash, use empty string to not create any",
							Value: "Trash",
						},
						&cli.StringFlag{
							Name:  "junk-name",
							Usage: "Name of special mailbox for 'junk' (spam), use empty string to not create any",
							Value: "Junk",
						},
						&cli.StringFlag{
							Name:  "drafts-name",
							Usage: "Name of special mailbox for drafts, use empty string to not create any",
							Value: "Drafts",
						},
						&cli.StringFlag{
							Name:  "archive-name",
							Usage: "Name of special mailbox for archive, use empty string to not create any",
							Value: "Archive",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return imapAcctCreate(be, ctx)
					},
				},
				{
					Name:  "remove",
					Usage: "Delete IMAP storage account",
					Description: `If IMAP connections are open and using the specified account,
messages access will be killed off immediately though connection will remain open. No cache
or other buffering takes effect.`,
					ArgsUsage: "USERNAME",
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
						return imapAcctRemove(be, ctx)
					},
				},
				{
					Name:  "appendlimit",
					Usage: "Query or set accounts's APPENDLIMIT value",
					Description: `APPENDLIMIT value determines the size of a message that
can be saved into a mailbox using IMAP APPEND command. This does not affect the size
of messages that can be delivered to the mailbox from non-IMAP sources (e.g. SMTP).

Global APPENDLIMIT value set via server configuration takes precedence over
per-account values configured using this command.

APPENDLIMIT value (either global or per-account) cannot be larger than
4 GiB due to IMAP protocol limitations.
`,
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "cfg-block",
							Usage:   "Module configuration block to use",
							EnvVars: []string{"MADDY_CFGBLOCK"},
							Value:   "local_mailboxes",
						},
						&cli.IntFlag{
							Name:    "value",
							Aliases: []string{"v"},
							Usage:   "Set APPENDLIMIT to specified value (in bytes)",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return imapAcctAppendlimit(be, ctx)
					},
				},
			},
		})
}

type SpecialUseUser interface {
	CreateMailboxSpecial(name, specialUseAttr string) error
}

func imapAcctList(be module.Storage, ctx *cli.Context) error {
	mbe, ok := be.(module.ManageableStorage)
	if !ok {
		return cli.Exit("Error: storage backend does not support accounts management using maddy command", 2)
	}

	list, err := mbe.ListIMAPAccts()
	if err != nil {
		return err
	}

	if len(list) == 0 && !ctx.Bool("quiet") {
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
		return cli.Exit("Error: storage backend does not support accounts management using maddy command", 2)
	}

	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
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
		if err := createMbox(name, imap.SentAttr); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create sent folder: %v", err)
		}
	}
	if name := ctx.String("trash-name"); name != "" {
		if err := createMbox(name, imap.TrashAttr); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create trash folder: %v", err)
		}
	}
	if name := ctx.String("junk-name"); name != "" {
		if err := createMbox(name, imap.JunkAttr); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create junk folder: %v", err)
		}
	}
	if name := ctx.String("drafts-name"); name != "" {
		if err := createMbox(name, imap.DraftsAttr); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create drafts folder: %v", err)
		}
	}
	if name := ctx.String("archive-name"); name != "" {
		if err := createMbox(name, imap.ArchiveAttr); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create archive folder: %v", err)
		}
	}

	return nil
}

func imapAcctRemove(be module.Storage, ctx *cli.Context) error {
	mbe, ok := be.(module.ManageableStorage)
	if !ok {
		return cli.Exit("Error: storage backend does not support accounts management using maddy command", 2)
	}

	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}

	if !ctx.Bool("yes") {
		if !clitools2.Confirmation("Are you sure you want to delete this user account?", false) {
			return errors.New("Cancelled")
		}
	}

	return mbe.DeleteIMAPAcct(username)
}
