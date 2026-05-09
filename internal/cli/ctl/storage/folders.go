package storagectl

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/foxcpp/maddy/framework/module/storage"
	clitools2 "github.com/foxcpp/maddy/internal/cli/clitools"
	"github.com/urfave/cli/v2"
)

func foldersList(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}

	folders, err := be.ListFolders(ctx.Context, username)
	if err != nil {
		return err
	}

	if len(folders) == 0 {
		fmt.Fprintln(os.Stderr, "No folders.")
	}

	for _, f := range folders {
		if f.Role != "" {
			fmt.Printf("%s\t%s\n", f.Path, f.Role)
		} else {
			fmt.Println(f.Path)
		}
	}
	return nil
}

func foldersCreate(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return cli.Exit("Error: NAME is required", 2)
	}

	opts := storage.FolderOptions{}
	if ctx.IsSet("role") {
		role := ctx.String("role")
		switch strings.ToLower(role) {
		case "archive":
			opts.Role = `\Archive`
		case "drafts":
			opts.Role = `\Drafts`
		case "junk":
			opts.Role = `\Junk`
		case "sent":
			opts.Role = `\Sent`
		case "trash":
			opts.Role = `\Trash`
		default:
			return cli.Exit(fmt.Sprintf("Error: invalid role %q; valid values: archive, drafts, junk, sent, trash", role), 2)
		}
	}

	dto, err := be.CreateFolder(ctx.Context, username, name, opts)
	if err != nil {
		return err
	}

	fmt.Println(dto.Path)
	return nil
}

func foldersRemove(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	folder := ctx.Args().Get(1)
	if folder == "" {
		return cli.Exit("Error: FOLDER is required", 2)
	}

	if !ctx.Bool("yes") {
		if !clitools2.Confirmation(fmt.Sprintf("Are you sure you want to delete folder %q and all its contents?", folder), false) {
			return errors.New("Cancelled")
		}
	}

	return be.DeleteFolder(ctx.Context, username, folder)
}

func foldersRename(be storage.ManagedStorage, ctx *cli.Context) error {
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

	return be.RenameFolder(ctx.Context, username, oldName, newName)
}
