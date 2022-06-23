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
	"strings"

	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/auth/pass_table"
	maddycli "github.com/foxcpp/maddy/internal/cli"
	clitools2 "github.com/foxcpp/maddy/internal/cli/clitools"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	maddycli.AddSubcommand(
		&cli.Command{
			Name:  "creds",
			Usage: "Local credentials management",
			Description: `These commands manipulate credential databases used by 
maddy mail server.

Corresponding credential database should be defined in maddy.conf as
a top-level config block. By default the block name should be local_authdb (
can be changed using --cfg-block argument for subcommands).

Note that it is not enough to create user credentials in order to grant 
IMAP access - IMAP account should be also created using 'imap-acct create' subcommand.
`,
			Subcommands: []*cli.Command{
				{
					Name:  "list",
					Usage: "List created credentials",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "cfg-block",
							Usage:   "Module configuration block to use",
							EnvVars: []string{"MADDY_CFGBLOCK"},
							Value:   "local_authdb",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openUserDB(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return usersList(be, ctx)
					},
				},
				{
					Name:  "create",
					Usage: "Create user account",
					Description: `Reads password from stdin.

If configuration block uses auth.pass_table, then hash algorithm can be configured
using command flags. Otherwise, these options cannot be used. 
`,
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "cfg-block",
							Usage:   "Module configuration block to use",
							EnvVars: []string{"MADDY_CFGBLOCK"},
							Value:   "local_authdb",
						},
						&cli.StringFlag{
							Name:    "password",
							Aliases: []string{"p"},
							Usage:   "Use `PASSWORD instead of reading password from stdin.\n\t\tWARNING: Provided only for debugging convenience. Don't leave your passwords in shell history!",
						},
						&cli.StringFlag{
							Name:  "hash",
							Usage: "Use specified hash algorithm. Valid values: " + strings.Join(pass_table.Hashes, ", "),
							Value: "bcrypt",
						},
						&cli.IntFlag{
							Name:  "bcrypt-cost",
							Usage: "Specify bcrypt cost value",
							Value: bcrypt.DefaultCost,
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openUserDB(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return usersCreate(be, ctx)
					},
				},
				{
					Name:      "remove",
					Usage:     "Delete user account",
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "cfg-block",
							Usage:   "Module configuration block to use",
							EnvVars: []string{"MADDY_CFGBLOCK"},
							Value:   "local_authdb",
						},
						&cli.BoolFlag{
							Name:    "yes",
							Aliases: []string{"y"},
							Usage:   "Don't ask for confirmation",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openUserDB(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return usersRemove(be, ctx)
					},
				},
				{
					Name:        "password",
					Usage:       "Change account password",
					Description: "Reads password from stdin",
					ArgsUsage:   "USERNAME",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "cfg-block",
							Usage:   "Module configuration block to use",
							EnvVars: []string{"MADDY_CFGBLOCK"},
							Value:   "local_authdb",
						},
						&cli.StringFlag{
							Name:    "password",
							Aliases: []string{"p"},
							Usage:   "Use `PASSWORD` instead of reading password from stdin.\n\t\tWARNING: Provided only for debugging convenience. Don't leave your passwords in shell history!",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openUserDB(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return usersPassword(be, ctx)
					},
				},
			},
		})
}

func usersList(be module.PlainUserDB, ctx *cli.Context) error {
	list, err := be.ListUsers()
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

func usersCreate(be module.PlainUserDB, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}

	var pass string
	if ctx.IsSet("password") {
		pass = ctx.String("password")
	} else {
		var err error
		pass, err = clitools2.ReadPassword("Enter password for new user")
		if err != nil {
			return err
		}
	}

	if beHash, ok := be.(*pass_table.Auth); ok {
		return beHash.CreateUserHash(username, pass, ctx.String("hash"), pass_table.HashOpts{
			BcryptCost: ctx.Int("bcrypt-cost"),
		})
	} else if ctx.IsSet("hash") || ctx.IsSet("bcrypt-cost") {
		return cli.Exit("Error: --hash cannot be used with non-pass_table credentials DB", 2)
	} else {
		return be.CreateUser(username, pass)
	}
}

func usersRemove(be module.PlainUserDB, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}

	if !ctx.Bool("yes") {
		if !clitools2.Confirmation("Are you sure you want to delete this user account?", false) {
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
		pass, err = clitools2.ReadPassword("Enter new password")
		if err != nil {
			return err
		}
	}

	return be.SetUserPassword(username, pass)
}
