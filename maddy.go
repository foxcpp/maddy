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
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/caddyserver/certmagic"
	parser "github.com/foxcpp/maddy/framework/cfgparser"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/hooks"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"

	// Import packages for side-effect of module registration.
	_ "github.com/foxcpp/maddy/internal/auth/dovecot_sasl"
	_ "github.com/foxcpp/maddy/internal/auth/external"
	_ "github.com/foxcpp/maddy/internal/auth/ldap"
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

	enableDebugFlags  = false
	profileEndpoint   *string
	blockProfileRate  *int
	mutexProfileFract *int
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

// Run is the entry point for all maddy code. It takes care of command line arguments parsing,
// logging initialization, directives setup, configuration reading. After all that, it
// calls moduleMain to initialize and run modules.
func Run() int {
	certmagic.UserAgent = "maddy/" + Version

	flag.StringVar(&config.LibexecDirectory, "libexec", DefaultLibexecDirectory, "path to the libexec directory")
	flag.BoolVar(&log.DefaultLogger.Debug, "debug", false, "enable debug logging early")

	var (
		configPath   = flag.String("config", filepath.Join(ConfigDirectory, "maddy.conf"), "path to configuration file")
		logTargets   = flag.String("log", "stderr", "default logging target(s)")
		printVersion = flag.Bool("v", false, "print version and build metadata, then exit")
	)

	if enableDebugFlags {
		profileEndpoint = flag.String("debug.pprof", "", "enable live profiler HTTP endpoint and listen on the specified address")
		blockProfileRate = flag.Int("debug.blockprofrate", 0, "set blocking profile rate")
		mutexProfileFract = flag.Int("debug.mutexproffract", 0, "set mutex profile fraction")
	}

	flag.Parse()

	if len(flag.Args()) != 0 {
		fmt.Println("usage:", os.Args[0], "[options]")
		return 2
	}

	if *printVersion {
		fmt.Println("maddy", BuildInfo())
		return 0
	}

	var err error
	log.DefaultLogger.Out, err = LogOutputOption(strings.Split(*logTargets, ","))
	if err != nil {
		systemdStatusErr(err)
		log.Println(err)
		return 2
	}

	initDebug()

	os.Setenv("PATH", config.LibexecDirectory+string(filepath.ListSeparator)+os.Getenv("PATH"))

	f, err := os.Open(*configPath)
	if err != nil {
		systemdStatusErr(err)
		log.Println(err)
		return 2
	}
	defer f.Close()

	cfg, err := parser.Read(f, *configPath)
	if err != nil {
		systemdStatusErr(err)
		log.Println(err)
		return 2
	}

	if err := moduleMain(cfg); err != nil {
		systemdStatusErr(err)
		log.Println(err)
		return 2
	}

	return 0
}

func initDebug() {
	if !enableDebugFlags {
		return
	}

	if *profileEndpoint != "" {
		go func() {
			log.Println("listening on", "http://"+*profileEndpoint, "for profiler requests")
			log.Println("failed to listen on profiler endpoint:", http.ListenAndServe(*profileEndpoint, nil))
		}()
	}

	// These values can also be affected by environment so set them
	// only if argument is specified.
	if *mutexProfileFract != 0 {
		runtime.SetMutexProfileFraction(*mutexProfileFract)
	}
	if *blockProfileRate != 0 {
		runtime.SetBlockProfileRate(*blockProfileRate)
	}
}

func InitDirs() error {
	if config.StateDirectory == "" {
		config.StateDirectory = DefaultStateDirectory
	}
	if config.RuntimeDirectory == "" {
		config.RuntimeDirectory = DefaultRuntimeDirectory
	}
	if config.LibexecDirectory == "" {
		config.LibexecDirectory = DefaultLibexecDirectory
	}

	if err := ensureDirectoryWritable(config.StateDirectory); err != nil {
		return err
	}
	if err := ensureDirectoryWritable(config.RuntimeDirectory); err != nil {
		return err
	}

	// Make sure all paths we are going to use are absolute
	// before we change the working directory.
	if !filepath.IsAbs(config.StateDirectory) {
		return errors.New("statedir should be absolute")
	}
	if !filepath.IsAbs(config.RuntimeDirectory) {
		return errors.New("runtimedir should be absolute")
	}
	if !filepath.IsAbs(config.LibexecDirectory) {
		return errors.New("-libexec should be absolute")
	}

	// Change the working directory to make all relative paths
	// in configuration relative to state directory.
	if err := os.Chdir(config.StateDirectory); err != nil {
		log.Println(err)
	}

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
	return os.Remove(testFile.Name())
}

func ReadGlobals(cfg []config.Node) (map[string]interface{}, []config.Node, error) {
	globals := config.NewMap(nil, config.Node{Children: cfg})
	globals.String("state_dir", false, false, DefaultStateDirectory, &config.StateDirectory)
	globals.String("runtime_dir", false, false, DefaultRuntimeDirectory, &config.RuntimeDirectory)
	globals.String("hostname", false, false, "", nil)
	globals.String("autogenerated_msg_domain", false, false, "", nil)
	globals.Custom("tls", false, false, nil, tls.TLSDirective, nil)
	globals.Bool("storage_perdomain", false, false, nil)
	globals.Bool("auth_perdomain", false, false, nil)
	globals.StringList("auth_domains", false, false, nil, nil)
	globals.Custom("log", false, false, defaultLogOutput, logOutput, &log.DefaultLogger.Out)
	globals.Bool("debug", false, log.DefaultLogger.Debug, &log.DefaultLogger.Debug)
	globals.AllowUnknown()
	unknown, err := globals.Process()
	return globals.Values, unknown, err
}

func moduleMain(cfg []config.Node) error {
	globals, modBlocks, err := ReadGlobals(cfg)
	if err != nil {
		return err
	}

	if err := InitDirs(); err != nil {
		return err
	}

	defer log.DefaultLogger.Out.Close()

	hooks.AddHook(hooks.EventLogRotate, reinitLogging)

	endpoints, mods, err := RegisterModules(globals, modBlocks)
	if err != nil {
		return err
	}

	err = initModules(globals, endpoints, mods)
	if err != nil {
		return err
	}

	systemdStatus(SDReady, "Listening for incoming connections...")

	handleSignals()

	systemdStatus(SDStopping, "Waiting for running transactions to complete...")

	hooks.RunHooks(hooks.EventShutdown)

	return nil
}

type ModInfo struct {
	Instance module.Module
	Cfg      config.Node
}

func RegisterModules(globals map[string]interface{}, nodes []config.Node) (endpoints, mods []ModInfo, err error) {
	mods = make([]ModInfo, 0, len(nodes))

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
				return nil, nil, err
			}

			endpoints = append(endpoints, ModInfo{Instance: inst, Cfg: block})
			continue
		}

		factory := module.Get(modName)
		if factory == nil {
			return nil, nil, config.NodeErr(block, "unknown module or global directive: %s", modName)
		}

		if module.HasInstance(instName) {
			return nil, nil, config.NodeErr(block, "config block named %s already exists", instName)
		}

		inst, err := factory(modName, instName, modAliases, nil)
		if err != nil {
			return nil, nil, err
		}

		block := block
		module.RegisterInstance(inst, config.NewMap(globals, block))
		for _, alias := range modAliases {
			if module.HasInstance(alias) {
				return nil, nil, config.NodeErr(block, "config block named %s already exists", alias)
			}
			module.RegisterAlias(alias, instName)
		}

		log.Debugf("%v:%v: register config block %v %v", block.File, block.Line, instName, modAliases)
		mods = append(mods, ModInfo{Instance: inst, Cfg: block})
	}

	if len(endpoints) == 0 {
		return nil, nil, fmt.Errorf("at least one endpoint should be configured")
	}

	return endpoints, mods, nil
}

func initModules(globals map[string]interface{}, endpoints, mods []ModInfo) error {
	for _, endp := range endpoints {
		if err := endp.Instance.Init(config.NewMap(globals, endp.Cfg)); err != nil {
			return err
		}

		if closer, ok := endp.Instance.(io.Closer); ok {
			endp := endp
			hooks.AddHook(hooks.EventShutdown, func() {
				log.Debugf("close %s (%s)", endp.Instance.Name(), endp.Instance.InstanceName())
				if err := closer.Close(); err != nil {
					log.Printf("module %s (%s) close failed: %v", endp.Instance.Name(), endp.Instance.InstanceName(), err)
				}
			})
		}
	}

	for _, inst := range mods {
		if module.Initialized[inst.Instance.InstanceName()] {
			continue
		}

		return fmt.Errorf("Unused configuration block at %s:%d - %s (%s)",
			inst.Cfg.File, inst.Cfg.Line, inst.Instance.InstanceName(), inst.Instance.Name())
	}

	return nil
}
