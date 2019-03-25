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
	flag.StringVar(&configpath, "config", "Maddyfile", "path to Maddyfile")
	flag.Parse()

	absCfg, err := filepath.Abs(configpath)
	if err != nil {
		log.Println("Failed to resolve path to config: %v", err)
		os.Exit(1)
	}

	f, err := os.Open(absCfg)
	if err != nil {
		log.Println("Cannot open %q: %v", configpath, err)
		os.Exit(1)
	}
	defer f.Close()

	config, err := config.Read(f, absCfg)
	if err != nil {
		log.Println("Cannot parse %q: %v", configpath, err)
		os.Exit(1)
	}

	if err := maddy.Start(config); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
