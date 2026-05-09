package storagectl

import (
	"errors"
	"fmt"
	"os"

	"github.com/foxcpp/maddy/framework/module/storage"
	clitools2 "github.com/foxcpp/maddy/internal/cli/clitools"
	"github.com/urfave/cli/v2"
)

func acctList(be storage.ManagedStorage, ctx *cli.Context) error {
	limit := ctx.Int("limit")
	offset := ctx.Int("offset")
	substring := ctx.String("filter")

	list, err := be.ListAccounts(ctx.Context, substring, limit, offset)
	if err != nil {
		return err
	}

	if len(list) == 0 && !ctx.Bool("verbose") {
		fmt.Fprintln(os.Stderr, "No users.")
	}

	for _, user := range list {
		if ctx.Bool("verbose") {
			fmt.Println(user.ID, user.Name, user.CreatedAt)
		} else {
			fmt.Println(user.Name)
		}
	}
	return nil
}

func acctCreate(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}

	dto, err := be.CreateAccount(ctx.Context, username, storage.AccountOptions{})
	if err != nil {
		return err
	}

	if ctx.Bool("no-specialuse") {
		return nil
	}

	createMbox := func(flagName, role string) {
		if name := ctx.String(flagName + "-name"); name != "" {
			if _, err := be.CreateFolder(ctx.Context, dto.Name, name, storage.FolderOptions{
				Role: role,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create %s folder: %v", flagName, err)
			}
		}
	}

	createMbox("sent", `Sent`)
	createMbox("trash", `Trash`)
	createMbox("junk", `Junk`)
	createMbox("drafts", `Drafts`)
	createMbox("archive", `Archive`)

	return nil
}

func acctRemove(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}

	if !ctx.Bool("yes") {
		if !clitools2.Confirmation("Are you sure you want to delete this user account?", false) {
			return errors.New("Cancelled")
		}
	}

	return be.DeleteAccount(ctx.Context, username)
}
