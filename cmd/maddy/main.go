package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/emersion/maddy"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
)

func main() {
	var configpath string
	flag.StringVar(&configpath, "config", filepath.Join(maddy.ConfigDirectory(), "maddy.conf"), "path to configuration file")
	debugFlag := flag.Bool("debug", false, "enable debug logging early")
	flag.Parse()

	if *debugFlag {
		log.DefaultLogger.Debug = true
	}

	absCfg, err := filepath.Abs(configpath)
	if err != nil {
		log.Printf("failed to resolve path to config: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Open(absCfg)
	if err != nil {
		log.Printf("cannot open %q: %v\n", configpath, err)
		os.Exit(1)
	}
	defer f.Close()

	cfg, err := config.Read(f, absCfg)
	if err != nil {
		log.Printf("cannot parse %q: %v\n", configpath, err)
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
