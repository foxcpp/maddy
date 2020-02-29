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
