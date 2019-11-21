package maddy

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/config/parser"
	"github.com/foxcpp/maddy/log"
)

var (
	profileEndpoint   = flag.String("debug.pprof", "", "enable live profiler HTTP endpoint and listen on the specified endpoint")
	blockProfileRate  = flag.Int("debug.blockprofrate", 0, "set blocking profile rate")
	mutexProfileFract = flag.Int("debug.mutexproffract", 0, "set mutex profile fraction)")
)

// Run is the entry point for all maddy code. It takes care of command line arguments parsing,
// logging initialization, directives setup, configuration reading. After all that, it
// calls moduleMain to initialize and run modules.
func Run() int {
	flag.StringVar(&config.LibexecDirectory, "libexec", DefaultLibexecDirectory, "path to the libexec directory")
	flag.BoolVar(&log.DefaultLogger.Debug, "debug", false, "enable debug logging early")

	var (
		configPath   = flag.String("config", filepath.Join(ConfigDirectory, "maddy.conf"), "path to configuration file")
		logTargets   = flag.String("log", "stderr", "default logging target(s)")
		printVersion = flag.Bool("v", false, "print version and exit")
	)
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
	log.DefaultLogger.Out, err = LogOutputOption(strings.Split(*logTargets, " "))
	if err != nil {
		systemdStatusErr(err)
		log.Println(err)
		return 2
	}

	initDebug()

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
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}

	testFile, err := os.Create(filepath.Join(path, "writeable-test"))
	if err != nil {
		return err
	}
	testFile.Close()
	if err := os.Remove(testFile.Name()); err != nil {
		return err
	}
	return nil
}
