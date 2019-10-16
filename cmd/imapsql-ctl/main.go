package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/config"
	"github.com/urfave/cli"
	"golang.org/x/crypto/bcrypt"
)

var backend *imapsql.Backend
var stdinScnr *bufio.Scanner

func connectToDB(ctx *cli.Context) error {
	if ctx.GlobalIsSet("unsafe") && !ctx.GlobalIsSet("quiet") {
		fmt.Fprintln(os.Stderr, "WARNING: Using --unsafe with running server may lead to accidential damage to data due to desynchronization with connected clients.")
	}

	driver := ctx.GlobalString("driver")
	if driver != "" {
		// Construct artificial config file tree from command line arguments
		// and pass to initialization logic.
		cfg := config.Node{
			Name: "sql",
			Children: []config.Node{
				{
					Name: "driver",
					Args: []string{ctx.GlobalString("driver")},
				},
				{
					Name: "dsn",
					Args: []string{ctx.GlobalString("dsn")},
				},
				{
					Name: "fsstore",
					Args: []string{ctx.GlobalString("fsstore")},
				},
			},
		}
		var err error
		backend, err = backendFromNode(make(map[string]interface{}), cfg)
		if err != nil {
			return err
		}
	} else {
		cfg := ctx.GlobalString("config")

		cfgAbs, err := filepath.Abs(cfg)
		if err != nil {
			return err
		}

		cfgBlock := ctx.GlobalString("cfg-block")
		if err := os.Chdir(ctx.GlobalString("state")); err != nil {
			return err
		}

		backend, err = backendFromCfg(cfgAbs, cfgBlock)
		if err != nil {
			return err
		}
	}

	backend.EnableSpecialUseExt()

	return nil
}

func closeBackend(ctx *cli.Context) (err error) {
	if backend != nil {
		return backend.Close()
	}
	return nil
}

func main() {
	stdinScnr = bufio.NewScanner(os.Stdin)

	app := cli.NewApp()
	app.Name = "imapsql-ctl"
	app.Copyright = "(c) 2019 Max Mazurov <fox.cpp@disroot.org>\n   Published under the terms of the MIT license (https://opensource.org/licenses/MIT)"
	app.Usage = "SQL database management utility for maddy"
	app.Version = buildInfo()
	app.After = closeBackend

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Configuration file to use",
			EnvVar: "MADDY_CONFIG",
			Value:  "/etc/maddy/maddy.conf",
		},
		cli.StringFlag{
			Name:   "state",
			Usage:  "State directory to use",
			EnvVar: "MADDY_STATE",
			Value:  "/var/lib/maddy",
		},
		cli.StringFlag{
			Name:   "cfg-block",
			Usage:  "SQL module configuration to use",
			EnvVar: "MADDY_CFGBLOCK",
			Value:  "local_mailboxes",
		},
		cli.StringFlag{
			Name:   "driver",
			Usage:  "Directly specify driver value instead of reading it from config",
			EnvVar: "MADDY_SQL_DRIVER",
			Value:  "",
		},
		cli.StringFlag{
			Name:   "dsn",
			Usage:  "Directly specify dsn value instead of reading it from config",
			EnvVar: "MADDY_SQL_DSN",
			Value:  "",
		},
		cli.StringFlag{
			Name:   "fsstore",
			Usage:  "Directly specify fsstore value instead of reading it from config",
			EnvVar: "MADDY_SQL_FSSTORE",
			Value:  "",
		},
		cli.BoolFlag{
			Name:  "quiet,q",
			Usage: "Don't print user-friendly messages to stderr",
		},
		cli.BoolFlag{
			Name:  "unsafe",
			Usage: "Allow to perform actions that can be safely done only without running server",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:  "mboxes",
			Usage: "Mailboxes (folders) management",
			Subcommands: []cli.Command{
				{
					Name:      "list",
					Usage:     "Show mailboxes of user",
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "subscribed,s",
							Usage: "List only subscribed mailboxes",
						},
					},
					Action: mboxesList,
				},
				{
					Name:      "create",
					Usage:     "Create mailbox",
					ArgsUsage: "USERNAME NAME",
					Action:    mboxesCreate,
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "special",
							Usage: "Set SPECIAL-USE attribute on mailbox; valid values: archive, drafts, junk, sent, trash",
						},
					},
				},
				{
					Name:        "remove",
					Usage:       "Remove mailbox (requires --unsafe)",
					Description: "WARNING: All contents of mailbox will be irrecoverably lost.",
					ArgsUsage:   "USERNAME MAILBOX",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "yes,y",
							Usage: "Don't ask for confirmation",
						},
					},
					Action: mboxesRemove,
				},
				{
					Name:        "rename",
					Usage:       "Rename mailbox (requires --unsafe)",
					Description: "Rename may cause unexpected failures on client-side so be careful.",
					ArgsUsage:   "USERNAME OLDNAME NEWNAME",
					Action:      mboxesRename,
				},
				{
					Name:      "appendlimit",
					Usage:     "Query or set user's APPENDLIMIT value",
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "value,v",
							Usage: "Set APPENDLIMIT to specified value (in bytes). Pass -1 to disable limit.",
						},
					},
					Action: mboxesAppendLimit,
				},
			},
		},
		{
			Name:  "msgs",
			Usage: "Messages management",
			Subcommands: []cli.Command{
				{
					Name:        "add",
					Usage:       "Add message to mailbox (requires --unsafe)",
					ArgsUsage:   "USERNAME MAILBOX",
					Description: "Reads message body (with headers) from stdin. Prints UID of created message on success.",
					Flags: []cli.Flag{
						cli.StringSliceFlag{
							Name:  "flag,f",
							Usage: "Add flag to message. Can be specified multiple times",
						},
						cli.Int64Flag{
							Name:  "date,d",
							Usage: "Set internal date value to specified UNIX timestamp",
						},
					},
					Action: msgsAdd,
				},
				{
					Name:        "add-flags",
					Usage:       "Add flags to messages (requires --unsafe)",
					ArgsUsage:   "USERNAME MAILBOX SEQ FLAGS...",
					Description: "Add flags to all messages matched by SEQ.",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
					},
					Action: msgsFlags,
				},
				{
					Name:        "rem-flags",
					Usage:       "Remove flags from messages (requires --unsafe)",
					ArgsUsage:   "USERNAME MAILBOX SEQ FLAGS...",
					Description: "Remove flags from all messages matched by SEQ.",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
					},
					Action: msgsFlags,
				},
				{
					Name:        "set-flags",
					Usage:       "Set flags on messages (requires --unsafe)",
					ArgsUsage:   "USERNAME MAILBOX SEQ FLAGS...",
					Description: "Set flags on all messages matched by SEQ.",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
					},
					Action: msgsFlags,
				},
				{
					Name:      "remove",
					Usage:     "Remove messages from mailbox (requires --unsafe)",
					ArgsUsage: "USERNAME MAILBOX SEQSET",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
						cli.BoolFlag{
							Name:  "yes,y",
							Usage: "Don't ask for confirmation",
						},
					},
					Action: msgsRemove,
				},
				{
					Name:        "copy",
					Usage:       "Copy messages between mailboxes (requires --unsafe)",
					Description: "Note: You can't copy between mailboxes of different users. APPENDLIMIT of target mailbox is not enforced.",
					ArgsUsage:   "USERNAME SRCMAILBOX SEQSET TGTMAILBOX",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
					},
					Action: msgsCopy,
				},
				{
					Name:        "move",
					Usage:       "Move messages between mailboxes (requires --unsafe)",
					Description: "Note: You can't move between mailboxes of different users. APPENDLIMIT of target mailbox is not enforced.",
					ArgsUsage:   "USERNAME SRCMAILBOX SEQSET TGTMAILBOX",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
					},
					Action: msgsMove,
				},
				{
					Name:        "list",
					Usage:       "List messages in mailbox",
					Description: "If SEQSET is specified - only show messages that match it.",
					ArgsUsage:   "USERNAME MAILBOX [SEQSET]",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQSET instead of sequence numbers",
						},
						cli.BoolFlag{
							Name:  "full,f",
							Usage: "Show entire envelope and all server meta-data",
						},
					},
					Action: msgsList,
				},
				{
					Name:        "dump",
					Usage:       "Dump message body",
					Description: "If passed SEQ matches multiple messages - they will be joined.",
					ArgsUsage:   "USERNAME MAILBOX SEQ",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "uid,u",
							Usage: "Use UIDs for SEQ instead of sequence numbers",
						},
					},
					Action: msgsDump,
				},
			},
		},
		{
			Name:  "users",
			Usage: "User accounts management",
			Subcommands: []cli.Command{
				{
					Name:   "list",
					Usage:  "List created user accounts",
					Action: usersList,
				},
				{
					Name:        "create",
					Usage:       "Create user account",
					Description: "Reads password from stdin",
					ArgsUsage:   "USERNAME",
					Flags: []cli.Flag{
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
					Action: usersCreate,
				},
				{
					Name:      "remove",
					Usage:     "Delete user account (requires --unsafe)",
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "yes,y",
							Usage: "Don't ask for confirmation",
						},
					},
					Action: usersRemove,
				},
				{
					Name:        "password",
					Usage:       "Change account password",
					Description: "Reads password from stdin",
					ArgsUsage:   "USERNAME",
					Flags: []cli.Flag{
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
							Usage: "Use specified hash algorithm. Valid values: sha3-512, bcrypt",
							Value: "sha3-512",
						},
						cli.IntFlag{
							Name:  "bcrypt-cost",
							Usage: "Specify bcrypt cost value",
							Value: bcrypt.DefaultCost,
						},
					},
					Action: usersPassword,
				},
				{
					Name:      "appendlimit",
					Usage:     "Query or set user's APPENDLIMIT value",
					ArgsUsage: "USERNAME",
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "value,v",
							Usage: "Set APPENDLIMIT to specified value (in bytes)",
						},
					},
					Action: usersAppendLimit,
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
