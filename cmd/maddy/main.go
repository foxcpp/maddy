package main

import (
	"flag"
	"log"
	"os"

	"github.com/emersion/maddy"
	"github.com/emersion/maddy/module"
	"github.com/mholt/caddy/caddyfile"
)

func main() {
	var configpath string
	flag.StringVar(&configpath, "config", "Maddyfile", "path to Maddyfile")
	flag.Parse()

	f, err := os.Open(configpath)
	if err != nil {
		log.Fatalf("Cannot open %q: %v", configpath, err)
	}
	defer f.Close()

	maddy.Directives = append(maddy.Directives, module.ModuleDirectives...)
	config, err := caddyfile.Parse(configpath, f, maddy.Directives)
	if err != nil {
		log.Fatalf("Cannot parse %q: %v", configpath, err)
	}

	if err := maddy.Start(config); err != nil {
		log.Fatal(err)
	}
}
