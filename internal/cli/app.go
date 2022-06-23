package maddycli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/foxcpp/maddy/framework/log"
	"github.com/urfave/cli/v2"
)

var app *cli.App

func init() {
	app = cli.NewApp()
	app.Usage = "composable all-in-one mail server"
	app.Description = `Maddy is Mail Transfer agent (MTA), Mail Delivery Agent (MDA), Mail Submission
Agent (MSA), IMAP server and a set of other essential protocols/schemes
necessary to run secure email server implemented in one executable.

This executable can be used to start the server ('run') and to manipulate
databases used by it (all other subcommands).
`
	app.Authors = []*cli.Author{
		{
			Name:  "Maddy Mail Server maintainers & contributors",
			Email: "~foxcpp/maddy@lists.sr.ht",
		},
	}
	app.ExitErrHandler = func(c *cli.Context, err error) {
		cli.HandleExitCoder(err)
		if err != nil {
			log.Println(err)
			cli.OsExiter(1)
		}
	}
	app.EnableBashCompletion = true
	app.Commands = []*cli.Command{
		{
			Name:   "generate-man",
			Hidden: true,
			Action: func(c *cli.Context) error {
				man, err := app.ToMan()
				if err != nil {
					return err
				}
				fmt.Println(man)
				return nil
			},
		},
		{
			Name:   "generate-fish-completion",
			Hidden: true,
			Action: func(c *cli.Context) error {
				cp, err := app.ToFishCompletion()
				if err != nil {
					return err
				}
				fmt.Println(cp)
				return nil
			},
		},
	}
}

func AddGlobalFlag(f cli.Flag) {
	app.Flags = append(app.Flags, f)
	if err := f.Apply(flag.CommandLine); err != nil {
		log.Println("GlobalFlag", f, "could not be mapped to stdlib flag:", err)
	}
}

func AddSubcommand(cmd *cli.Command) {
	app.Commands = append(app.Commands, cmd)

	if cmd.Name == "run" {
		// Backward compatibility hack to start the server as just ./maddy
		// Needs to be done here so we will register all known flags with
		// stdlib before Run is called.
		app.Action = func(c *cli.Context) error {
			log.Println("WARNING: Starting server not via 'maddy run' is deprecated and will stop working in the next version")
			return cmd.Action(c)
		}
		app.Flags = append(app.Flags, cmd.Flags...)
		for _, f := range cmd.Flags {
			if err := f.Apply(flag.CommandLine); err != nil {
				log.Println("GlobalFlag", f, "could not be mapped to stdlib flag:", err)
			}
		}
	}
}

func Run() {
	// Actual entry point is registered in maddy.go.

	// Print help when called via maddyctl executable. To be removed
	// once backward compatibility hack for 'maddy run' is removed too.
	if strings.Contains(os.Args[0], "maddyctl") && len(os.Args) == 1 {
		if err := app.Run([]string{os.Args[0], "help"}); err != nil {
			log.DefaultLogger.Error("app.Run failed", err)
		}
		return
	}

	if err := app.Run(os.Args); err != nil {
		log.DefaultLogger.Error("app.Run failed", err)
	}
}
