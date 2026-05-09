package storagectl

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/module/storage"
	maddycli "github.com/foxcpp/maddy/internal/cli"
	"github.com/foxcpp/maddy/internal/cli/ctl"
	"github.com/foxcpp/maddy/internal/updatepipe"
	"github.com/urfave/cli/v2"
)

func init() {
	maddycli.AddSubcommand(
		&cli.Command{
			Name:  "storage",
			Usage: "Message storage management",
			Subcommands: []*cli.Command{
				{
					Name:  "accounts",
					Usage: "Storage account management",
					Description: `
These subcommands can be used to list/create/delete IMAP storage
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
								&cli.StringFlag{
									Name:  "filter",
									Usage: "List only accounts that have string in name",
								},
								&cli.IntFlag{
									Name:  "limit",
									Usage: "Number of accounts to list",
								},
								&cli.IntFlag{
									Name:  "offset",
									Usage: "Number of accounts to skip",
								},
								&cli.BoolFlag{
									Name:  "verbose",
									Usage: "Verbose output",
								},
							},
							Action: func(ctx *cli.Context) error {
								be, err := openStorage(ctx)
								if err != nil {
									return err
								}
								defer ctl.CloseIfNeeded(be)
								return acctList(be, ctx)
							},
						},
						{
							Name:  "create",
							Usage: "Create storage account",
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
								&cli.BoolFlag{
									Name:  "no-specialuse",
									Usage: "Do not create special-use folders",
									Value: false,
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
								defer ctl.CloseIfNeeded(be)
								return acctCreate(be, ctx)
							},
						},
						{
							Name:  "remove",
							Usage: "Delete storage account",
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
								defer ctl.CloseIfNeeded(be)
								return acctRemove(be, ctx)
							},
						},
					},
				},
				{
					Name:  "folders",
					Usage: "List/create/remove/rename folders",
					Subcommands: []*cli.Command{
						{
							Name:      "list",
							Usage:     "Show folder of user",
							ArgsUsage: "USERNAME",
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
								defer ctl.CloseIfNeeded(be)
								return foldersList(be, ctx)
							},
						},
						{
							Name:      "create",
							Usage:     "Create folder",
							ArgsUsage: "USERNAME NAME",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:    "cfg-block",
									Usage:   "Module configuration block to use",
									EnvVars: []string{"MADDY_CFGBLOCK"},
									Value:   "local_mailboxes",
								},
								&cli.StringFlag{
									Name:  "role",
									Usage: "Set role (SPECIAL-USE attribute) on folder; valid values: archive, drafts, junk, sent, trash",
								},
							},
							Action: func(ctx *cli.Context) error {
								be, err := openStorage(ctx)
								if err != nil {
									return err
								}
								defer ctl.CloseIfNeeded(be)
								return foldersCreate(be, ctx)
							},
						},
						{
							Name:        "remove",
							Usage:       "Remove folder",
							Description: "WARNING: All contents of folder will be irrecoverably lost.",
							ArgsUsage:   "USERNAME FOLDER",
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
								defer ctl.CloseIfNeeded(be)
								return foldersRemove(be, ctx)
							},
						},
						{
							Name:        "rename",
							Usage:       "Rename folder",
							Description: "Rename may cause unexpected failures on client-side so be careful.",
							ArgsUsage:   "USERNAME OLDNAME NEWNAME",
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
								defer ctl.CloseIfNeeded(be)
								return foldersRename(be, ctx)
							},
						},
					},
				},
				{
					Name:  "messages",
					Usage: "Messages management",
					Subcommands: []*cli.Command{
						{
							Name:        "add",
							Usage:       "Add message to a folder",
							ArgsUsage:   "USERNAME FOLDER",
							Description: "Reads message body (with headers) from stdin. Prints UID of created message on success.",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:    "cfg-block",
									Usage:   "Module configuration block to use",
									EnvVars: []string{"MADDY_CFGBLOCK"},
									Value:   "local_mailboxes",
								},
								&cli.StringSliceFlag{
									Name:    "flag",
									Aliases: []string{"f"},
									Usage:   "Add flag to message. Can be specified multiple times",
								},
								&cli.TimestampFlag{
									Layout:  time.RFC3339,
									Name:    "date",
									Aliases: []string{"d"},
									Usage:   "Set internal date value to specified one in ISO 8601 format (2006-01-02T15:04:05Z07:00)",
								},
							},
							Action: func(ctx *cli.Context) error {
								be, err := openStorage(ctx)
								if err != nil {
									return err
								}
								defer ctl.CloseIfNeeded(be)
								return msgsAdd(be, ctx)
							},
						},
						{
							Name:      "flag",
							Usage:     "Add flags to messages",
							ArgsUsage: "USERNAME FOLDER FLAG MSGS...",
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
								defer ctl.CloseIfNeeded(be)
								return msgsFlag(be, ctx)
							},
						},
						{
							Name:        "unflag",
							Usage:       "Remove flags from messages",
							ArgsUsage:   "USERNAME FOLDER FLAG MSGS...",
							Description: "Remove flag from all specified messages",
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
								defer ctl.CloseIfNeeded(be)
								return msgsUnflag(be, ctx)
							},
						},
						{
							Name:      "remove",
							Usage:     "Remove messages from folder",
							ArgsUsage: "USERNAME FOLDER MSGS...",
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
								defer ctl.CloseIfNeeded(be)
								return msgsRemove(be, ctx)
							},
						},
						{
							Name:      "copy",
							Usage:     "Copy messages between folders",
							ArgsUsage: "USERNAME FROM TO MSGS...",
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
								defer ctl.CloseIfNeeded(be)
								return msgsCopy(be, ctx)
							},
						},
						{
							Name:      "move",
							Usage:     "Move messages between folders",
							ArgsUsage: "USERNAME FROM TO MSGS...",
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
								defer ctl.CloseIfNeeded(be)
								return msgsMove(be, ctx)
							},
						},
						{
							Name:      "list",
							Usage:     "List messages in folder",
							ArgsUsage: "USERNAME FOLDERS",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:    "cfg-block",
									Usage:   "Module configuration block to use",
									EnvVars: []string{"MADDY_CFGBLOCK"},
									Value:   "local_mailboxes",
								},
								&cli.IntFlag{
									Name:  "limit",
									Usage: "Number of messages to list",
									Value: 10000,
								},
								&cli.IntFlag{
									Name:  "offset",
									Usage: "Number of messages to skip",
								},
								&cli.BoolFlag{
									Name:    "verbose",
									Aliases: []string{"v"},
									Usage:   "Show full server metadata",
								},
							},
							Action: func(ctx *cli.Context) error {
								be, err := openStorage(ctx)
								if err != nil {
									return err
								}
								defer ctl.CloseIfNeeded(be)
								return msgsList(be, ctx)
							},
						},
						{
							Name:        "dump",
							Usage:       "Dump message body",
							Description: "If multiple messages are specified - they will be joined.",
							ArgsUsage:   "USERNAME FOLDER MSGS...",
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
								defer ctl.CloseIfNeeded(be)
								return msgsDump(be, ctx)
							},
						},
					},
				},
			},
		},
	)
}

type managedStorage struct {
	storage.ManagedStorage
	started bool
}

func (m *managedStorage) Close() error {
	if !m.started {
		return nil
	}
	if lm, ok := m.ManagedStorage.(container.LifetimeModule); ok {
		return lm.Stop()
	}
	return nil
}

func openStorage(ctx *cli.Context) (storage.ManagedStorage, error) {
	_, mod, err := ctl.LoadCfgBlockModule(ctx)
	if err != nil {
		return nil, err
	}

	_, ok := mod.(storage.Storage)
	if !ok {
		return nil, cli.Exit(fmt.Sprintf("Error: configuration block %s is not a storage", ctx.String("cfg-block")), 2)
	}

	store, ok := mod.(storage.ManagedStorage)
	if !ok {
		return nil, cli.Exit(fmt.Sprintf("Error: %s does not support management", ctx.String("cfg-block")), 2)
	}

	started := false
	if lt, ok := store.(container.LifetimeModule); ok {
		if err := lt.Start(); err != nil {
			return nil, err
		}
		started = true
	}

	if updStore, ok := mod.(updatepipe.Backend); ok {
		if err := updStore.EnableUpdatePipe(updatepipe.ModePush); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Failed to initialize update pipe, do not remove messages from folders open by clients: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "No update pipe support, do not remove messages from folders open by clients\n")
	}

	return &managedStorage{ManagedStorage: store, started: started}, nil
}
