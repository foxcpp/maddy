package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/foxcpp/maddy"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/hooks"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/updatepipe"
	parser "github.com/foxcpp/maddy/pkg/cfgparser"
	"github.com/urfave/cli"
	"golang.org/x/crypto/bcrypt"
)

func closeIfNeeded(i interface{}) {
	if c, ok := i.(io.Closer); ok {
		c.Close()
	}
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
			Name:  "creds",
			Usage: "Local credentials management",
			Subcommands: []cli.Command{
				{
					Name:  "list",
					Usage: "List created credentials",
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
		},
		{
			Name:  "imap-acct",
			Usage: "IMAP storage accounts management",
			Subcommands: []cli.Command{
				{
					Name:  "list",
					Usage: "List storage accounts",
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
						defer closeIfNeeded(be)
						return imapAcctList(be, ctx)
					},
				},
				{
					Name:      "create",
					Usage:     "Create IMAP storage account",
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "cfg-block",
							Usage:  "Module configuration block to use",
							EnvVar: "MADDY_CFGBLOCK",
							Value:  "local_authdb",
						},
					},
					Action: func(ctx *cli.Context) error {
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return imapAcctCreate(be, ctx)
					},
				},
				{
					Name:      "remove",
					Usage:     "Delete IMAP storage account",
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
						be, err := openStorage(ctx)
						if err != nil {
							return err
						}
						defer closeIfNeeded(be)
						return imapAcctRemove(be, ctx)
					},
				},
				{
					Name:      "appendlimit",
					Usage:     "Query or set accounts's APPENDLIMIT value",
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
						defer closeIfNeeded(be)
						return imapAcctAppendlimit(be, ctx)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
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
						defer closeIfNeeded(be)
						return msgsDump(be, ctx)
					},
				},
			},
		},
		{
			Name:   "hash",
			Usage:  "Generate password hashes for use with pass_table",
			Action: hashCommand,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "password,p",
					Usage: "Use `PASSWORD instead of reading password from stdin\n\t\tWARNING: Provided only for debugging convenience. Don't leave your passwords in shell history!",
				},
				cli.StringFlag{
					Name:  "hash",
					Usage: "Use specified hash algorithm",
					Value: "bcrypt",
				},
				cli.IntFlag{
					Name:  "bcrypt-cost",
					Usage: "Specify bcrypt cost value",
					Value: bcrypt.DefaultCost,
				},
				cli.IntFlag{
					Name:  "argon2-time",
					Usage: "Time factor for Argon2id",
					Value: 3,
				},
				cli.IntFlag{
					Name:  "argon2-memory",
					Usage: "Memory in KiB to use for Argon2id",
					Value: 1024,
				},
				cli.IntFlag{
					Name:  "argon2-threads",
					Usage: "Threads to use for Argon2id",
					Value: 1,
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func getCfgBlockModule(ctx *cli.Context) (map[string]interface{}, *maddy.ModInfo, error) {
	cfgPath := ctx.GlobalString("config")
	if cfgPath == "" {
		return nil, nil, errors.New("Error: config is required")
	}
	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("Error: failed to open config: %w", err)
	}
	defer cfgFile.Close()
	cfgNodes, err := parser.Read(cfgFile, cfgFile.Name())
	if err != nil {
		return nil, nil, fmt.Errorf("Error: failed to parse config: %w", err)
	}

	globals, cfgNodes, err := maddy.ReadGlobals(cfgNodes)
	if err != nil {
		return nil, nil, err
	}

	if err := maddy.InitDirs(); err != nil {
		return nil, nil, err
	}

	_, mods, err := maddy.RegisterModules(globals, cfgNodes)
	if err != nil {
		return nil, nil, err
	}
	defer hooks.RunHooks(hooks.EventShutdown)

	cfgBlock := ctx.String("cfg-block")
	if cfgBlock == "" {
		return nil, nil, errors.New("Error: cfg-block is required")
	}
	var mod maddy.ModInfo
	for _, m := range mods {
		if m.Instance.InstanceName() == cfgBlock {
			mod = m
			break
		}
	}
	if mod.Instance == nil {
		return nil, nil, fmt.Errorf("Error: unknown configuration block: %s", cfgBlock)
	}

	return globals, &mod, nil
}

func openStorage(ctx *cli.Context) (module.Storage, error) {
	globals, mod, err := getCfgBlockModule(ctx)
	if err != nil {
		return nil, err
	}

	storage, ok := mod.Instance.(module.Storage)
	if !ok {
		return nil, fmt.Errorf("Error: configuration block %s is not an IMAP storage", ctx.String("cfg-block"))
	}

	if err := mod.Instance.Init(config.NewMap(globals, mod.Cfg)); err != nil {
		return nil, fmt.Errorf("Error: module initialization failed: %w", err)
	}

	if updStore, ok := mod.Instance.(updatepipe.Backend); ok {
		if err := updStore.EnableUpdatePipe(updatepipe.ModePush); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Failed to initialize update pipe, do not remove messages from mailboxes open by clients: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "No update pipe support, do not remove messages from mailboxes open by clients\n")
	}

	return storage, nil
}

func openUserDB(ctx *cli.Context) (module.PlainUserDB, error) {
	globals, mod, err := getCfgBlockModule(ctx)
	if err != nil {
		return nil, err
	}

	userDB, ok := mod.Instance.(module.PlainUserDB)
	if !ok {
		return nil, fmt.Errorf("Error: configuration block %s is not a local credentials store", ctx.String("cfg-block"))
	}

	if err := mod.Instance.Init(config.NewMap(globals, mod.Cfg)); err != nil {
		return nil, fmt.Errorf("Error: module initialization failed: %w", err)
	}

	return userDB, nil
}
