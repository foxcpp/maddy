package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/emersion/maddy"
	"github.com/emersion/maddy/config"
)

func main() {
	var configpath string
	flag.StringVar(&configpath, "config", "Maddyfile", "path to Maddyfile")
	flag.Parse()

	absCfg, err := filepath.Abs(configpath)
	if err != nil {
		log.Fatalf("Failed to resolve path to config: %v", err)
	}

	f, err := os.Open(absCfg)
	if err != nil {
		log.Fatalf("Cannot open %q: %v", configpath, err)
	}
	defer f.Close()

	config, err := config.Read(f, absCfg)
	if err != nil {
		log.Fatalf("Cannot parse %q: %v", configpath, err)
	}

	if err := maddy.Start(config); err != nil {
		log.Fatal(err)
	}
}
