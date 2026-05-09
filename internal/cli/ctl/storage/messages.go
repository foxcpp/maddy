package storagectl

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/foxcpp/maddy/framework/module/storage"
	clitools2 "github.com/foxcpp/maddy/internal/cli/clitools"
	"github.com/urfave/cli/v2"
)

func msgsAdd(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	folder := ctx.Args().Get(1)
	if folder == "" {
		return cli.Exit("Error: FOLDER is required", 2)
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
		return cli.Exit("Error: empty message, refusing to continue", 2)
	}

	dto, err := be.AddMessage(ctx.Context, username, folder, date, &buf)
	if err != nil {
		return err
	}

	// Add flags if specified.
	flags := ctx.StringSlice("flag")
	for _, flag := range flags {
		if err := be.AddFlag(ctx.Context, username, folder, []string{dto.ID}, flag); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to add flag %q: %v\n", flag, err)
		}
	}

	fmt.Println(dto.ID)
	return nil
}

func msgsFlag(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	folder := ctx.Args().Get(1)
	if folder == "" {
		return cli.Exit("Error: FOLDER is required", 2)
	}
	flag := ctx.Args().Get(2)
	if flag == "" {
		return cli.Exit("Error: FLAG is required", 2)
	}

	msgs := ctx.Args().Slice()
	if len(msgs) < 4 {
		return cli.Exit("Error: at least one MSGS value is required", 2)
	}
	ids := msgs[3:]

	return be.AddFlag(ctx.Context, username, folder, ids, flag)
}

func msgsUnflag(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	folder := ctx.Args().Get(1)
	if folder == "" {
		return cli.Exit("Error: FOLDER is required", 2)
	}
	flag := ctx.Args().Get(2)
	if flag == "" {
		return cli.Exit("Error: FLAG is required", 2)
	}

	msgs := ctx.Args().Slice()
	if len(msgs) < 4 {
		return cli.Exit("Error: at least one MSGS value is required", 2)
	}
	ids := msgs[3:]

	return be.RemoveFlag(ctx.Context, username, folder, ids, flag)
}

func msgsRemove(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	folder := ctx.Args().Get(1)
	if folder == "" {
		return cli.Exit("Error: FOLDER is required", 2)
	}

	msgs := ctx.Args().Slice()
	if len(msgs) < 3 {
		return cli.Exit("Error: at least one MSGS value is required", 2)
	}
	ids := msgs[2:]

	if !ctx.Bool("yes") {
		if !clitools2.Confirmation(fmt.Sprintf("Are you sure you want to delete %d message(s)?", len(ids)), false) {
			return errors.New("Cancelled")
		}
	}

	return be.DeleteMessages(ctx.Context, username, folder, ids)
}

func msgsCopy(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	from := ctx.Args().Get(1)
	if from == "" {
		return cli.Exit("Error: FROM folder is required", 2)
	}
	to := ctx.Args().Get(2)
	if to == "" {
		return cli.Exit("Error: TO folder is required", 2)
	}

	msgs := ctx.Args().Slice()
	if len(msgs) < 4 {
		return cli.Exit("Error: at least one MSGS value is required", 2)
	}
	ids := msgs[3:]

	copied, err := be.CopyMessages(ctx.Context, username, from, to, ids)
	if err != nil {
		return err
	}

	for _, m := range copied {
		fmt.Println(m.ID)
	}
	return nil
}

func msgsMove(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	from := ctx.Args().Get(1)
	if from == "" {
		return cli.Exit("Error: FROM folder is required", 2)
	}
	to := ctx.Args().Get(2)
	if to == "" {
		return cli.Exit("Error: TO folder is required", 2)
	}

	msgs := ctx.Args().Slice()
	if len(msgs) < 4 {
		return cli.Exit("Error: at least one MSGS value is required", 2)
	}
	ids := msgs[3:]

	moved, err := be.MoveMessages(ctx.Context, username, from, to, ids)
	if err != nil {
		return err
	}

	for _, m := range moved {
		fmt.Println(m.ID)
	}
	return nil
}

func msgsList(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	folder := ctx.Args().Get(1)
	if folder == "" {
		return cli.Exit("Error: FOLDER is required", 2)
	}
	limit := ctx.Int("limit")
	offset := ctx.Int("offset")

	messages, err := be.ListMessages(
		ctx.Context, username, folder,
		limit, offset,
	)
	if err != nil {
		return err
	}

	if len(messages) == 0 {
		fmt.Fprintln(os.Stderr, "No messages.")
		return nil
	}

	for _, msg := range messages {
		if ctx.Bool("verbose") {
			fmt.Println("ID:", msg.ID)
			fmt.Println("Subject:", msg.Title)
			fmt.Println("Received:", msg.ReceivedAt.Format(time.RFC3339))
			fmt.Println("Flags:", strings.Join(msg.Flags, ", "))
			fmt.Println()
		} else {
			fmt.Printf("%s\t%s\n", msg.ID, msg.Title)
		}
	}
	return nil
}

func msgsDump(be storage.ManagedStorage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return cli.Exit("Error: USERNAME is required", 2)
	}
	folder := ctx.Args().Get(1)
	if folder == "" {
		return cli.Exit("Error: FOLDER is required", 2)
	}

	msgs := ctx.Args().Slice()
	if len(msgs) < 3 {
		return cli.Exit("Error: at least one MSGS value is required", 2)
	}
	ids := msgs[2:]

	for _, id := range ids {
		rc, err := be.DumpMessage(ctx.Context, username, folder, id)
		if err != nil {
			return fmt.Errorf("failed to dump message %s: %w", id, err)
		}
		if _, err := io.Copy(os.Stdout, rc); err != nil {
			if err := rc.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "failed to close message reader:", err)
			}
			return fmt.Errorf("failed to write message %s: %w", id, err)
		}
		if err := rc.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "failed to close message reader:", err)
		}
	}
	return nil
}
