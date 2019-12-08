package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emersion/go-imap/backend"
	"github.com/foxcpp/maddy"
	"github.com/foxcpp/maddy/internal/updatepipe"
	"github.com/urfave/cli"
	"golang.org/x/crypto/bcrypt"
)

type UserDB interface {
	ListUsers() ([]string, error)
	CreateUser(username, password string) error
	CreateUserNoPass(username string) error
	DeleteUser(username string) error
	SetUserPassword(username, newPassword string) error
	Close() error
}

type Storage interface {
	GetUser(username string) (backend.User, error)
	Close() error
}

func main() {
	app := cli.NewApp()
	app.Name = "maddyctl"
	app.Usage = "maddy mail server administration utility"
	app.Version = maddy.BuildInfo()

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Configuration file to use",
			EnvVar: "MADDY_CONFIG",
			Value:  filepath.Join(maddy.ConfigDirectory, "maddy.conf"),
		},
	}

	app.Commands = []cli.Command{
		{
			Name:  "users",
			Usage: "User accounts management",
			Subcommands: []cli.Command{
				{
					Name:  "list",
					Usage: "List created user accounts",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_authdb",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openUserDB(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return usersList(be, ctx)
					},
				},
				{
					Name:        "create",
					Usage:       "Create user account",
					Description: "Reads password from stdin",
					ArgsUsage:   "USERNAME",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_authdb",
						},
						cli.StringFlag{
							Name:  "password,p",
							Usage: "Use `PASSWORD instead of reading password from stdin.\n\t\tWARNING: Provided only for debugging convenience. Don't leave your passwords in shell history!",
						},
						cli.BoolFlag{
							Name:  "null,n",
							Usage: "Create account with null password",
						},
						cli.StringFlag{
							Name:  "hash",
							Usage: "Use specified hash algorithm. Valid values: sha3-512, bcrypt",
							Value: "bcrypt",
						},
						cli.IntFlag{
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
						defer be.Close()
						return usersCreate(be, ctx)
					},
				},
				{
					Name:      "remove",
					Usage:     "Delete user account",
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_authdb",
						},
						cli.BoolFlag{
							Name:  "yes,y",
							Usage: "Don't ask for confirmation",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openUserDB(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return usersRemove(be, ctx)
					},
				},
				{
					Name:        "password",
					Usage:       "Change account password",
					Description: "Reads password from stdin",
					ArgsUsage:   "USERNAME",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_authdb",
						},
						cli.StringFlag{
							Name:  "password,p",
							Usage: "Use `PASSWORD` instead of reading password from stdin.\n\t\tWARNING: Provided only for debugging convenience. Don't leave your passwords in shell history!",
						},
						cli.BoolFlag{
							Name:  "null,n",
							Usage: "Set password to null",
						},
						cli.StringFlag{
							Name:  "hash",
							Usage: "Use specified hash algorithm for password. Supported values vary depending on storage backend.",
							Value: "",
						},
						cli.IntFlag{
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
						defer be.Close()
						return usersPassword(be, ctx)
					},
				},
				{
					Name:      "imap-appendlimit",
					Usage:     "Query or set user's APPENDLIMIT value",
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.IntFlag{
							Name:  "value,v",
							Usage: "Set APPENDLIMIT to specified value (in bytes)",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return usersAppendlimit(be, ctx)
					},
				},
			},
		},
		{
			Name:  "imap-mboxes",
			Usage: "IMAP mailboxes (folders) management",
			Subcommands: []cli.Command{
				{
					Name:      "list",
					Usage:     "Show mailboxes of user",
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.BoolFlag{
							Name:  "subscribed,s",
							Usage: "List only subscribed mailboxes",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return mboxesList(be, ctx)
					},
				},
				{
					Name:      "create",
					Usage:     "Create mailbox",
					ArgsUsage: "USERNAME NAME",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.StringFlag{
							Name:  "special",
							Usage: "Set SPECIAL-USE attribute on mailbox; valid values: archive, drafts, junk, sent, trash",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return mboxesCreate(be, ctx)
					},
				},
				{
					Name:        "remove",
					Usage:       "Remove mailbox",
					Description: "WARNING: All contents of mailbox will be irrecoverably lost.",
					ArgsUsage:   "USERNAME MAILBOX",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.BoolFlag{
							Name:  "yes,y",
							Usage: "Don't ask for confirmation",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return mboxesRemove(be, ctx)
					},
				},
				{
					Name:        "rename",
					Usage:       "Rename mailbox",
					Description: "Rename may cause unexpected failures on client-side so be careful.",
					ArgsUsage:   "USERNAME OLDNAME NEWNAME",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return mboxesRename(be, ctx)
					},
				},
			},
		},
		{
			Name:  "imap-msgs",
			Usage: "IMAP messages management",
			Subcommands: []cli.Command{
				{
					Name:        "add",
					Usage:       "Add message to mailbox",
					ArgsUsage:   "USERNAME MAILBOX",
					Description: "Reads message body (with headers) from stdin. Prints UID of created message on success.",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.StringSliceFlag{
							Name:  "flag,f",
							Usage: "Add flag to message. Can be specified multiple times",
						},
						cli.Int64Flag{
							Name:  "date,d",
							Usage: "Set internal date value to specified UNIX timestamp",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return msgsAdd(be, ctx)
					},
				},
				{
					Name:        "add-flags",
					Usage:       "Add flags to messages",
					ArgsUsage:   "USERNAME MAILBOX SEQ FLAGS...",
					Description: "Add flags to all messages matched by SEQ.",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return msgsFlags(be, ctx)
					},
				},
				{
					Name:        "rem-flags",
					Usage:       "Remove flags from messages",
					ArgsUsage:   "USERNAME MAILBOX SEQ FLAGS...",
					Description: "Remove flags from all messages matched by SEQ.",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return msgsFlags(be, ctx)
					},
				},
				{
					Name:        "set-flags",
					Usage:       "Set flags on messages",
					ArgsUsage:   "USERNAME MAILBOX SEQ FLAGS...",
					Description: "Set flags on all messages matched by SEQ.",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return msgsFlags(be, ctx)
					},
				},
				{
					Name:      "remove",
					Usage:     "Remove messages from mailbox",
					ArgsUsage: "USERNAME MAILBOX SEQSET",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
						cli.BoolFlag{
							Name:  "yes,y",
							Usage: "Don't ask for confirmation",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return msgsRemove(be, ctx)
					},
				},
				{
					Name:        "copy",
					Usage:       "Copy messages between mailboxes",
					Description: "Note: You can't copy between mailboxes of different users. APPENDLIMIT of target mailbox is not enforced.",
					ArgsUsage:   "USERNAME SRCMAILBOX SEQSET TGTMAILBOX",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return msgsCopy(be, ctx)
					},
				},
				{
					Name:        "move",
					Usage:       "Move messages between mailboxes",
					Description: "Note: You can't move between mailboxes of different users. APPENDLIMIT of target mailbox is not enforced.",
					ArgsUsage:   "USERNAME SRCMAILBOX SEQSET TGTMAILBOX",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
						cli.BoolFlag{
							Name:  "yes,y",
							Usage: "Don't ask for confirmation",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return msgsMove(be, ctx)
					},
				},
				{
					Name:        "list",
					Usage:       "List messages in mailbox",
					Description: "If SEQSET is specified - only show messages that match it.",
					ArgsUsage:   "USERNAME MAILBOX [SEQSET]",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
						cli.BoolFlag{
							Name:  "full,f",
							Usage: "Show entire envelope and all server meta-data",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return msgsList(be, ctx)
					},
				},
				{
					Name:        "dump",
					Usage:       "Dump message body",
					Description: "If passed SEQ matches multiple messages - they will be joined.",
					ArgsUsage:   "USERNAME MAILBOX SEQ",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_mailboxes",
						},
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQ instead of sequence numbers",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer be.Close()
						return msgsDump(be, ctx)
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func openStorage(ctx *cli.Context) (Storage, error) {
	cfgPath := ctx.GlobalString("config")
	if cfgPath == "" {
		return nil, errors.New("Error: config is required")
	}

	cfgBlock := ctx.String("cfg-block")
	if cfgBlock == "" {
		return nil, errors.New("Error: cfg-block is required")
	}

	root, node, err := findBlockInCfg(cfgPath, cfgBlock)
	if err != nil {
		return nil, err
	}

	var store Storage
	switch node.Name {
	case "sql":
		store, err = sqlFromCfgBlock(root, node)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("Error: Storage backend is not supported by maddyctl")
	}

	if updStore, ok := store.(updatepipe.Backend); ok {
		if err := updStore.EnableUpdatePipe(updatepipe.ModePush); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Failed to initialize update pipe, do not remove messages from mailboxes open by clients: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "No update pipe support, do not remove messages from mailboxes open by clients\n")
	}

	return store, nil
}

func openUserDB(ctx *cli.Context) (UserDB, error) {
	cfgPath := ctx.GlobalString("config")
	if cfgPath == "" {
		return nil, errors.New("Error: config is required")
	}

	cfgBlock := ctx.String("cfg-block")
	if cfgBlock == "" {
		return nil, errors.New("Error: cfg-block is required")
	}

	root, node, err := findBlockInCfg(cfgPath, cfgBlock)
	if err != nil {
		return nil, err
	}

	switch node.Name {
	case "sql":
		return sqlFromCfgBlock(root, node)
	default:
		return nil, errors.New("Error: Authentication backend is not supported by maddyctl")
	}
}
