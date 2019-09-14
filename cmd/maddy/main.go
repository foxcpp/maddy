package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"

	"github.com/foxcpp/maddy"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
)

func main() {
	configPath := flag.String("config", filepath.Join(ConfigDirectory, "maddy.conf"), "path to configuration file")

	flag.StringVar(&config.StateDirectory, "state", DefaultStateDirectory, "path to the state directory")
	flag.StringVar(&config.LibexecDirectory, "libexec", DefaultLibexecDirectory, "path to the libexec directory")
	flag.StringVar(&config.RuntimeDirectory, "runtime", DefaultRuntimeDirectory, "path to the runtime directory")

	flag.BoolVar(&log.DefaultLogger.Debug, "debug", false, "enable debug logging")
	profileEndpoint := flag.String("debug.pprof", "", "enable live profiler HTTP endpoint and listen on the specified endpoint")
	blockProfileRate := flag.Int("debug.blockprofrate", 0, "set blocking profile rate")
	mutexProfileFract := flag.Int("debug.mutexproffract", 0, "set mutex profile fraction)")

	flag.Parse()

	if err := ensureDirectoryWritable(config.StateDirectory); err != nil {
		log.Println(err)
		os.Exit(2)
	}
	if err := ensureDirectoryWritable(config.RuntimeDirectory); err != nil {
		log.Println(err)
		os.Exit(2)
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

	f, err := os.Open(*configPath)
	if err != nil {
		log.Printf("cannot open %q: %v\n", *configPath, err)
		os.Exit(1)
	}
	defer f.Close()

	cfg, err := config.Read(f, *configPath)
	if err != nil {
		log.Printf("cannot parse %q: %v\n", *configPath, err)
		os.Exit(1)
	}

	if log.DefaultLogger.Debug {
		// Insert 'debug' directive so other config blocks will inherit it.
		skipInsert := false
		for _, node := range cfg {
			if node.Name == "debug" {
				skipInsert = true
			}
		}
		if !skipInsert {
			cfg = append(cfg, config.Node{Name: "debug"})
		}
	}

	// Make sure all paths we are going to use are absolute
	// before we change the working directory.
	config.StateDirectory, err = filepath.Abs(config.StateDirectory)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	config.LibexecDirectory, err = filepath.Abs(config.LibexecDirectory)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	config.RuntimeDirectory, err = filepath.Abs(config.RuntimeDirectory)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// Change the working directory to make all relative paths
	// in configuration relative to state directory.
	if err := os.Chdir(config.StateDirectory); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if err := maddy.Start(cfg); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
