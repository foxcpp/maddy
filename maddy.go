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

package maddy

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/caddyserver/certmagic"
	parser "github.com/foxcpp/maddy/framework/cfgparser"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/hooks"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/resource/netresource"
	"github.com/foxcpp/maddy/internal/authz"
	maddycli "github.com/foxcpp/maddy/internal/cli"
	"github.com/urfave/cli/v2"

	// Import packages for side-effect of module registration.
	_ "github.com/foxcpp/maddy/internal/auth/dovecot_sasl"
	_ "github.com/foxcpp/maddy/internal/auth/external"
	_ "github.com/foxcpp/maddy/internal/auth/ldap"
	_ "github.com/foxcpp/maddy/internal/auth/netauth"
	_ "github.com/foxcpp/maddy/internal/auth/pam"
	_ "github.com/foxcpp/maddy/internal/auth/pass_table"
	_ "github.com/foxcpp/maddy/internal/auth/plain_separate"
	_ "github.com/foxcpp/maddy/internal/auth/shadow"
	_ "github.com/foxcpp/maddy/internal/check/authorize_sender"
	_ "github.com/foxcpp/maddy/internal/check/command"
	_ "github.com/foxcpp/maddy/internal/check/dkim"
	_ "github.com/foxcpp/maddy/internal/check/dns"
	_ "github.com/foxcpp/maddy/internal/check/dnsbl"
	_ "github.com/foxcpp/maddy/internal/check/milter"
	_ "github.com/foxcpp/maddy/internal/check/requiretls"
	_ "github.com/foxcpp/maddy/internal/check/rspamd"
	_ "github.com/foxcpp/maddy/internal/check/spf"
	_ "github.com/foxcpp/maddy/internal/endpoint/dovecot_sasld"
	_ "github.com/foxcpp/maddy/internal/endpoint/imap"
	_ "github.com/foxcpp/maddy/internal/endpoint/openmetrics"
	_ "github.com/foxcpp/maddy/internal/endpoint/smtp"
	_ "github.com/foxcpp/maddy/internal/imap_filter"
	_ "github.com/foxcpp/maddy/internal/imap_filter/command"
	_ "github.com/foxcpp/maddy/internal/libdns"
	_ "github.com/foxcpp/maddy/internal/modify"
	_ "github.com/foxcpp/maddy/internal/modify/dkim"
	_ "github.com/foxcpp/maddy/internal/storage/blob/fs"
	_ "github.com/foxcpp/maddy/internal/storage/blob/s3"
	_ "github.com/foxcpp/maddy/internal/storage/imapsql"
	_ "github.com/foxcpp/maddy/internal/table"
	_ "github.com/foxcpp/maddy/internal/target/queue"
	_ "github.com/foxcpp/maddy/internal/target/remote"
	_ "github.com/foxcpp/maddy/internal/target/smtp"
	_ "github.com/foxcpp/maddy/internal/tls"
	_ "github.com/foxcpp/maddy/internal/tls/acme"
)

var (
	Version = "go-build"

	enableDebugFlags = false
)

func BuildInfo() string {
	version := Version
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}

	return fmt.Sprintf(`%s %s/%s %s

default config: %s
default state_dir: %s
default runtime_dir: %s`,
		version, runtime.GOOS, runtime.GOARCH, runtime.Version(),
		filepath.Join(ConfigDirectory, "maddy.conf"),
		DefaultStateDirectory,
		DefaultRuntimeDirectory)
}

func init() {
	maddycli.AddGlobalFlag(
		&cli.PathFlag{
			Name:    "config",
			Usage:   "Configuration file to use",
			EnvVars: []string{"MADDY_CONFIG"},
			Value:   filepath.Join(ConfigDirectory, "maddy.conf"),
		},
	)
	maddycli.AddGlobalFlag(&cli.BoolFlag{
		Name:        "debug",
		Usage:       "enable debug logging early",
		Destination: &log.DefaultLogger.Debug,
	})
	maddycli.AddSubcommand(&cli.Command{
		Name:  "verify-config",
		Usage: "Check configuration file for errors",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "enable debug logging early",
				Destination: &log.DefaultLogger.Debug,
			},
		},
		Action: VerifyConfig,
	})
	maddycli.AddSubcommand(&cli.Command{
		Name:  "run",
		Usage: "Start the server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "libexec",
				Value:       DefaultLibexecDirectory,
				Usage:       "path to the libexec directory",
				Destination: &config.LibexecDirectory,
			},
			&cli.StringSliceFlag{
				Name:  "log",
				Usage: "default logging target(s)",
				Value: cli.NewStringSlice("stderr"),
			},
			&cli.BoolFlag{
				Name:   "v",
				Usage:  "print version and build metadata, then exit",
				Hidden: true,
			},
		},
		Action: Run,
	})
	maddycli.AddSubcommand(&cli.Command{
		Name:  "version",
		Usage: "Print version and build metadata, then exit",
		Action: func(c *cli.Context) error {
			fmt.Println(BuildInfo())
			return nil
		},
	})

	if enableDebugFlags {
		maddycli.AddGlobalFlag(&cli.StringFlag{
			Name:  "debug.pprof",
			Usage: "enable live profiler HTTP endpoint and listen on the specified address",
		})
		maddycli.AddGlobalFlag(&cli.IntFlag{
			Name:  "debug.blockprofrate",
			Usage: "set blocking profile rate",
		})
		maddycli.AddGlobalFlag(&cli.IntFlag{
			Name:  "debug.mutexproffract",
			Usage: "set mutex profile fraction",
		})
	}
}

// Run is the entry point for all server-running code. It takes care of command line arguments processing,
// logging initialization, directives setup, configuration reading. After all that, it
// calls moduleMain to initialize and run modules.
func Run(c *cli.Context) error {
	certmagic.UserAgent = "maddy/" + Version

	if c.NArg() != 0 {
		return cli.Exit(fmt.Sprintln("usage:", os.Args[0], "[options]"), 2)
	}

	if c.Bool("v") {
		fmt.Println("maddy", BuildInfo())
		return nil
	}

	var err error
	log.DefaultLogger.Out, err = LogOutputOption(c.StringSlice("log"))
	if err != nil {
		systemdStatusErr(err)
		return cli.Exit(err.Error(), 2)
	}

	initDebug(c)

	os.Setenv("PATH", config.LibexecDirectory+string(filepath.ListSeparator)+os.Getenv("PATH"))

	hooks.AddHook(hooks.EventLogRotate, reinitLogging)
	defer log.DefaultLogger.Out.Close()
	defer hooks.RunHooks(hooks.EventShutdown)

	hooks.AddHook(hooks.EventShutdown, netresource.CloseAllListeners)

	if err := moduleMain(c.Path("config")); err != nil {
		systemdStatusErr(err)
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

func VerifyConfig(c *cli.Context) error {
	os.Setenv("PATH", config.LibexecDirectory+string(filepath.ListSeparator)+os.Getenv("PATH"))

	if _, err := moduleConfigure(c.Path("config")); err != nil {
		return cli.Exit(err.Error(), 2)
	}

	log.DefaultLogger.Msg("No errors detected")

	return nil
}

func initDebug(c *cli.Context) {
	if !enableDebugFlags {
		return
	}

	if c.IsSet("debug.pprof") {
		profileEndpoint := c.String("debug.pprof")
		go func() {
			log.Println("listening on", "http://"+profileEndpoint, "for profiler requests")
			log.Println("failed to listen on profiler endpoint:", http.ListenAndServe(profileEndpoint, nil))
		}()
	}

	// These values can also be affected by environment so set them
	// only if argument is specified.
	if c.IsSet("debug.mutexproffract") {
		runtime.SetMutexProfileFraction(c.Int("debug.mutexproffract"))
	}
	if c.IsSet("debug.blockprofrate") {
		runtime.SetBlockProfileRate(c.Int("debug.blockprofrate"))
	}
}

func InitDirs(c *container.C) error {
	if c.Config.StateDirectory == "" {
		c.Config.StateDirectory = DefaultStateDirectory
	}
	if c.Config.RuntimeDirectory == "" {
		c.Config.RuntimeDirectory = DefaultRuntimeDirectory
	}
	if c.Config.LibexecDirectory == "" {
		c.Config.LibexecDirectory = DefaultLibexecDirectory
	}

	if err := ensureDirectoryWritable(c.Config.StateDirectory); err != nil {
		return err
	}
	if err := ensureDirectoryWritable(c.Config.RuntimeDirectory); err != nil {
		return err
	}

	// Make sure all paths we are going to use are absolute
	// before we change the working directory.
	if !filepath.IsAbs(c.Config.StateDirectory) {
		return errors.New("statedir should be absolute")
	}
	if !filepath.IsAbs(c.Config.RuntimeDirectory) {
		return errors.New("runtimedir should be absolute")
	}
	if !filepath.IsAbs(c.Config.LibexecDirectory) {
		return errors.New("-libexec should be absolute")
	}

	// Change the working directory to make all relative paths
	// in configuration relative to state directory.
	if err := os.Chdir(c.Config.StateDirectory); err != nil {
		log.Println(err)
	}

	config.StateDirectory = c.Config.StateDirectory
	config.RuntimeDirectory = c.Config.RuntimeDirectory
	config.LibexecDirectory = c.Config.LibexecDirectory

	return nil
}

func ensureDirectoryWritable(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}

	testFile, err := os.Create(filepath.Join(path, "writeable-test"))
	if err != nil {
		return err
	}
	testFile.Close()
	return os.RemoveAll(testFile.Name())
}

func ReadGlobals(c *container.C, cfg []config.Node) (map[string]interface{}, []config.Node, error) {
	globals := config.NewMap(nil, config.Node{Children: cfg})
	globals.String("state_dir", false, false, DefaultStateDirectory, &c.Config.StateDirectory)
	globals.String("runtime_dir", false, false, DefaultRuntimeDirectory, &c.Config.RuntimeDirectory)
	globals.String("hostname", false, false, "", nil)
	globals.String("autogenerated_msg_domain", false, false, "", nil)
	globals.Custom("tls", false, false, nil, tls.TLSDirective, nil)
	globals.Custom("tls_client", false, false, nil, tls.TLSClientBlock, nil)
	globals.Bool("storage_perdomain", false, false, nil)
	globals.Bool("auth_perdomain", false, false, nil)
	globals.StringList("auth_domains", false, false, nil, nil)
	globals.Custom("log", false, false, defaultLogOutput, logOutput, &c.DefaultLogger.Out)
	globals.Bool("debug", false, log.DefaultLogger.Debug, &c.DefaultLogger.Debug)
	config.EnumMapped(globals, "auth_map_normalize", true, false, authz.NormalizeFuncs, authz.NormalizeAuto, nil)
	modconfig.Table(globals, "auth_map", true, false, nil, nil)
	globals.AllowUnknown()
	unknown, err := globals.Process()
	return globals.Values, unknown, err
}

func ReadConfig(path string) ([]config.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parser.Read(f, path)
}

func moduleConfigure(configPath string) (*container.C, error) {
	c := container.New()
	container.Global = c

	cfg, err := ReadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", configPath, err)
	}

	globals, modBlocks, err := ReadGlobals(c, cfg)
	if err != nil {
		return nil, err
	}

	if err := InitDirs(c); err != nil {
		return nil, err
	}

	err = RegisterModules(c, globals, modBlocks)
	if err != nil {
		return nil, err
	}

	for _, inst := range c.Modules.NotInitialized() {
		return nil, fmt.Errorf("unused configuration block %s (%s)",
			inst.InstanceName(), inst.Name())
	}

	return c, nil
}

func moduleStart(c *container.C) error {
	return c.Lifetime.StartAll()
}

func moduleStop(c *container.C) {
	c.Lifetime.StopAll()
}

func moduleMain(configPath string) error {
	log.DefaultLogger.Msg("loading configuration...")

	// Make path absolute to make sure we can still read it if current directory changes (in moduleConfigure).
	configPath, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}

	c, err := moduleConfigure(configPath)
	if err != nil {
		return err
	}

	c.DefaultLogger.Msg("configuration loaded")

	if err := moduleStart(c); err != nil {
		return err
	}
	c.DefaultLogger.Msg("server started", "version", Version)

	systemdStatus(SDReady, "Configuration running.")
	asyncStopWg := sync.WaitGroup{} // Some containers might still be waiting on moduleStop
	for handleSignals() {
		hooks.RunHooks(hooks.EventReload)

		c = moduleReload(c, configPath, &asyncStopWg)
	}

	c.DefaultLogger.Msg("server stopping...")

	systemdStatus(SDStopping, "Waiting for old configuration to stop...")
	asyncStopWg.Wait()

	systemdStatus(SDStopping, "Waiting for current configuration to stop...")
	moduleStop(c)
	c.DefaultLogger.Msg("server stopped")

	return nil
}

func moduleReload(oldContainer *container.C, configPath string, asyncStopWg *sync.WaitGroup) *container.C {
	oldContainer.DefaultLogger.Msg("reloading server...")
	systemdStatus(SDReloading, "Reloading server...")

	oldContainer.DefaultLogger.Msg("loading new configuration...")
	newContainer, err := moduleConfigure(configPath)
	if err != nil {
		oldContainer.DefaultLogger.Error("failed to load new configuration", err)
		return oldContainer
	}

	oldContainer.DefaultLogger.Msg("configuration loaded")

	netresource.ResetListenersUsage()
	oldContainer.DefaultLogger.Msg("starting new server")
	if err := moduleStart(newContainer); err != nil {
		oldContainer.DefaultLogger.Error("failed to start new server", err)
		container.Global = oldContainer
		return oldContainer
	}

	newContainer.DefaultLogger.Msg("server started", "version", Version)

	systemdStatus(SDReloading, "New configuration running. Waiting for old connections and transactions to finish...")

	asyncStopWg.Add(1)
	go func() {
		defer asyncStopWg.Done()
		defer netresource.CloseUnusedListeners()

		oldContainer.DefaultLogger.Msg("stopping old server")
		moduleStop(oldContainer)
		oldContainer.DefaultLogger.Msg("old server stopped")

		systemdStatus(SDReloading, "Configuration running.")
	}()

	return newContainer
}

func RegisterModules(c *container.C, globals map[string]interface{}, nodes []config.Node) (err error) {
	var endpoints []struct {
		Endpoint module.LifetimeModule
		Cfg      *config.Map
	}

	for _, block := range nodes {
		var instName string
		var modAliases []string
		if len(block.Args) == 0 {
			instName = block.Name
		} else {
			instName = block.Args[0]
			modAliases = block.Args[1:]
		}

		modName := block.Name

		endpFactory := module.GetEndpoint(modName)
		if endpFactory != nil {
			inst, err := endpFactory(modName, block.Args)
			if err != nil {
				return err
			}

			endpoints = append(endpoints, struct {
				Endpoint module.LifetimeModule
				Cfg      *config.Map
			}{Endpoint: inst, Cfg: config.NewMap(globals, block)})
			continue
		}

		factory := module.Get(modName)
		if factory == nil {
			return config.NodeErr(block, "unknown module or global directive: %s", modName)
		}

		inst, err := factory(modName, instName)
		if err != nil {
			return err
		}

		err = c.Modules.Register(inst, func() error {
			err := inst.Configure(nil, config.NewMap(globals, block))
			if err != nil {
				return err
			}

			if lt, ok := inst.(module.LifetimeModule); ok {
				c.Lifetime.Add(lt)
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, module.ErrInstanceNameDuplicate) {
				return config.NodeErr(block, "config block named %s already exists", inst.InstanceName())
			}
			return err
		}

		for _, alias := range modAliases {
			if err := c.Modules.AddAlias(instName, alias); err != nil {
				if errors.Is(err, module.ErrInstanceNameDuplicate) {
					return config.NodeErr(block, "config block named %s already exists", alias)
				}
				return err
			}
		}

		log.Debugf("%v:%v: register config block %v %v", block.File, block.Line, instName, modAliases)
	}

	if len(endpoints) == 0 {
		return fmt.Errorf("at least one endpoint should be configured")
	}

	// Endpoints are configured directly after registration.
	for _, endp := range endpoints {
		if err := endp.Endpoint.Configure(nil, endp.Cfg); err != nil {
			return err
		}
		c.Lifetime.Add(endp.Endpoint)
	}

	return nil
}
