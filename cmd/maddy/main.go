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
	configPath := flag.String("config", filepath.Join(maddy.ConfigDirectory(), "maddy.conf"), "path to configuration file")
	debugFlag := flag.Bool("debug", false, "enable debug logging early")
	profileEndpoint := flag.String("debug.pprof", "", "enable live profiler HTTP endpoint and listen on the specified endpoint")
	blockProfileRate := flag.Int("debug.blockprofrate", 0, "set blocking profile rate")
	mutexProfileFract := flag.Int("debug.mutexproffract", 0, "set mutex profile fraction")
	flag.Parse()

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

	if *debugFlag {
		log.DefaultLogger.Debug = true
	}

	absCfg, err := filepath.Abs(*configPath)
	if err != nil {
		log.Printf("failed to resolve path to config: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Open(absCfg)
	if err != nil {
		log.Printf("cannot open %q: %v\n", *configPath, err)
		os.Exit(1)
	}
	defer f.Close()

	cfg, err := config.Read(f, absCfg)
	if err != nil {
		log.Printf("cannot parse %q: %v\n", *configPath, err)
		os.Exit(1)
	}

	if *debugFlag {
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

	if err := maddy.Start(cfg); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
