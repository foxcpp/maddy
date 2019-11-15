package maddy

import (
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
	flag.StringVar(&config.StateDirectory, "state", DefaultStateDirectory, "path to the state directory")
	flag.StringVar(&config.LibexecDirectory, "libexec", DefaultLibexecDirectory, "path to the libexec directory")
	flag.StringVar(&config.RuntimeDirectory, "runtime", DefaultRuntimeDirectory, "path to the runtime directory")
	flag.BoolVar(&log.DefaultLogger.Debug, "debug", false, "enable debug logging")

	var (
		configPath   = flag.String("config", filepath.Join(ConfigDirectory, "maddy.conf"), "path to configuration file")
		logTargets   = flag.String("log", "stderr", "default logging target(s)")
		printVersion = flag.Bool("v", false, "print version and exit")
	)
	flag.Parse()

	if *printVersion {
		fmt.Println("maddy", BuildInfo())
		return 0
	}

	flag.Parse()

	var err error
	log.DefaultLogger.Out, err = LogOutputOption(strings.Split(*logTargets, " "))
	if err != nil {
		log.Println(err)
		return 2
	}

	initDebug()

	// Lost the configuration before initDirs to make sure we will
	// interpret the relative path correctly (initDirs changes the working
	// directory).
	f, err := os.Open(*configPath)
	if err != nil {
		log.Printf("cannot open %q: %v\n", configPath, err)
		return 2
	}
	defer f.Close()

	if err := initDirs(); err != nil {
		log.Println(err)
		return 2
	}

	// But but actually parse it after initDirs since it also sets a couple of
	// environment variables that may be used in the configuration.
	cfg, err := parser.Read(f, *configPath)
	if err != nil {
		log.Printf("cannot parse %q: %v\n", configPath, err)
		return 2
	}

	if err := moduleMain(cfg); err != nil {
		log.Println(err)
		return 2
	}

	return 0
}

var ()

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

func initDirs() error {
	if err := ensureDirectoryWritable(config.StateDirectory); err != nil {
		return err
	}
	if err := ensureDirectoryWritable(config.RuntimeDirectory); err != nil {
		return err
	}

	// Make sure all paths we are going to use are absolute
	// before we change the working directory.
	var err error
	config.StateDirectory, err = filepath.Abs(config.StateDirectory)
	if err != nil {
		return err
	}
	config.LibexecDirectory, err = filepath.Abs(config.LibexecDirectory)
	if err != nil {
		return err
	}
	config.RuntimeDirectory, err = filepath.Abs(config.RuntimeDirectory)
	if err != nil {
		return err
	}

	// Make directory constants accessible in configuration by using
	// environment variables expansion.
	if err := os.Setenv("MADDYSTATE", config.StateDirectory); err != nil {
		log.Println(err)
	}
	if err := os.Setenv("MADDYLIBEXEC", config.LibexecDirectory); err != nil {
		log.Println(err)
	}
	if err := os.Setenv("MADDYRUNTIME", config.RuntimeDirectory); err != nil {
		log.Println(err)
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
