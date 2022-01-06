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
	"github.com/urfave/cli/v2"
)

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
