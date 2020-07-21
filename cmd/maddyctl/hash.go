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
	"strings"

	"github.com/foxcpp/maddy/cmd/maddyctl/clitools"
	"github.com/foxcpp/maddy/internal/auth/pass_table"
	"github.com/urfave/cli"
	"golang.org/x/crypto/bcrypt"
)

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

		return fmt.Errorf("Error: Unknown hash function, available: %s", strings.Join(funcs, ", "))
	}

	opts := pass_table.HashOpts{
		BcryptCost:    bcrypt.DefaultCost,
		Argon2Memory:  1024,
		Argon2Time:    2,
		Argon2Threads: 1,
	}
	if ctx.IsSet("bcrypt-cost") {
		if ctx.Int("bcrypt-cost") > bcrypt.MaxCost {
			return errors.New("Error: too big bcrypt cost")
		}
		if ctx.Int("bcrypt-cost") < bcrypt.MinCost {
			return errors.New("Error: too small bcrypt cost")
		}
		opts.BcryptCost = ctx.Int("bcrypt-cost")
	}
	if ctx.IsSet("argon2-memory") {
		opts.Argon2Memory = uint32(ctx.Int("argon2-memory"))
	}
	if ctx.IsSet("argon2-time") {
		opts.Argon2Memory = uint32(ctx.Int("argon2-time"))
	}
	if ctx.IsSet("argon2-threads") {
		opts.Argon2Threads = uint8(ctx.Int("argon2-threads"))
	}

	var pass string
	if ctx.IsSet("password") {
		pass = ctx.String("password")
	} else {
		var err error
		pass, err = clitools.ReadPassword("Password")
		if err != nil {
			return err
		}
	}

	if pass == "" {
		fmt.Fprintln(os.Stderr, "WARNING: This is the hash of empty string")
	}

	hash, err := hashCompute(opts, pass)
	if err != nil {
		return err
	}
	fmt.Println(hashFunc + ":" + hash)
	return nil
}
