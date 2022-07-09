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
	"fmt"
	"os"
	"strings"

	"github.com/foxcpp/maddy/internal/auth/pass_table"
	maddycli "github.com/foxcpp/maddy/internal/cli"
	clitools2 "github.com/foxcpp/maddy/internal/cli/clitools"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	maddycli.AddSubcommand(
		&cli.Command{
			Name:   "hash",
			Usage:  "Generate password hashes for use with pass_table",
			Action: hashCommand,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "password",
					Aliases: []string{"p"},
					Usage:   "Use `PASSWORD instead of reading password from stdin\n\t\tWARNING: Provided only for debugging convenience. Don't leave your passwords in shell history!",
				},
				&cli.StringFlag{
					Name:  "hash",
					Usage: "Use specified hash algorithm",
					Value: "bcrypt",
				},
				&cli.IntFlag{
					Name:  "bcrypt-cost",
					Usage: "Specify bcrypt cost value",
					Value: bcrypt.DefaultCost,
				},
				&cli.IntFlag{
					Name:  "argon2-time",
					Usage: "Time factor for Argon2id",
					Value: 3,
				},
				&cli.IntFlag{
					Name:  "argon2-memory",
					Usage: "Memory in KiB to use for Argon2id",
					Value: 1024,
				},
				&cli.IntFlag{
					Name:  "argon2-threads",
					Usage: "Threads to use for Argon2id",
					Value: 1,
				},
			},
		})
}

func hashCommand(ctx *cli.Context) error {
	hashFunc := ctx.String("hash")
	if hashFunc == "" {
		hashFunc = pass_table.DefaultHash
	}

	hashCompute := pass_table.HashCompute[hashFunc]
	if hashCompute == nil {
		var funcs []string
		for k := range pass_table.HashCompute {
			funcs = append(funcs, k)
		}

		return cli.Exit(fmt.Sprintf("Error: Unknown hash function, available: %s", strings.Join(funcs, ", ")), 2)
	}

	opts := pass_table.HashOpts{
		BcryptCost:    bcrypt.DefaultCost,
		Argon2Memory:  1024,
		Argon2Time:    2,
		Argon2Threads: 1,
	}
	if ctx.IsSet("bcrypt-cost") {
		if ctx.Int("bcrypt-cost") > bcrypt.MaxCost {
			return cli.Exit("Error: too big bcrypt cost", 2)
		}
		if ctx.Int("bcrypt-cost") < bcrypt.MinCost {
			return cli.Exit("Error: too small bcrypt cost", 2)
		}
		opts.BcryptCost = ctx.Int("bcrypt-cost")
	}
	if ctx.IsSet("argon2-memory") {
		opts.Argon2Memory = uint32(ctx.Int("argon2-memory"))
	}
	if ctx.IsSet("argon2-time") {
		opts.Argon2Time = uint32(ctx.Int("argon2-time"))
	}
	if ctx.IsSet("argon2-threads") {
		opts.Argon2Threads = uint8(ctx.Int("argon2-threads"))
	}

	var pass string
	if ctx.IsSet("password") {
		pass = ctx.String("password")
	} else {
		var err error
		pass, err = clitools2.ReadPassword("Password")
		if err != nil {
			return err
		}
	}

	if pass == "" {
		fmt.Fprintln(os.Stderr, "WARNING: This is the hash of an empty string")
	}
	if strings.TrimSpace(pass) != pass {
		fmt.Fprintln(os.Stderr, "WARNING: There is leading/trailing whitespace in the string")
	}

	hash, err := hashCompute(opts, pass)
	if err != nil {
		return err
	}
	fmt.Println(hashFunc + ":" + hash)
	return nil
}
