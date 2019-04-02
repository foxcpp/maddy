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
		log.Printf("failed to resolve path to config: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Open(absCfg)
	if err != nil {
		log.Printf("cannot open %q: %v\n", configpath, err)
		os.Exit(1)
	}
	defer f.Close()

	config, err := config.Read(f, absCfg)
	if err != nil {
		log.Printf("cannot parse %q: %v\n", configpath, err)
		os.Exit(1)
	}

	if err := maddy.Start(config); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
