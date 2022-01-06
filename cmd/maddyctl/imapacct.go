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

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/foxcpp/maddy/cmd/maddyctl/clitools"
	"github.com/foxcpp/maddy/framework/module"
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
