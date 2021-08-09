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

package tls

import (
	"crypto/tls"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/hooks"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

type FileLoader struct {
	instName   string
	inlineArgs []string
	certPaths  []string
	keyPaths   []string
	log        log.Logger

	certs     []tls.Certificate
	certsLock sync.RWMutex

	reloadTick *time.Ticker
	stopTick   chan struct{}
}

func NewFileLoader(_, instName string, _, inlineArgs []string) (module.Module, error) {
	return &FileLoader{
		instName:   instName,
		inlineArgs: inlineArgs,
		log:        log.Logger{Name: "tls.loader.file", Debug: log.DefaultLogger.Debug},
		stopTick:   make(chan struct{}),
	}, nil
}

func (f *FileLoader) Init(cfg *config.Map) error {
	cfg.StringList("certs", false, false, nil, &f.certPaths)
	cfg.StringList("keys", false, false, nil, &f.keyPaths)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if len(f.certPaths) != len(f.keyPaths) {
		return errors.New("tls.loader.file: mismatch in certs and keys count")
	}

	if len(f.inlineArgs)%2 != 0 {
		return errors.New("tls.loader.file: odd amount of arguments")
	}
	for i := 0; i < len(f.inlineArgs); i += 2 {
		f.certPaths = append(f.certPaths, f.inlineArgs[i])
		f.keyPaths = append(f.keyPaths, f.inlineArgs[i+1])
	}

	for _, certPath := range f.certPaths {
		if !filepath.IsAbs(certPath) {
			return fmt.Errorf("tls.loader.file: only absolute paths allowed in certificate paths: sorry :(")
		}
	}

	if err := f.loadCerts(); err != nil {
		return err
	}

	hooks.AddHook(hooks.EventReload, func() {
		f.log.Println("reloading certificates")
		if err := f.loadCerts(); err != nil {
			f.log.Error("reload failed", err)
		}
	})

	f.reloadTick = time.NewTicker(time.Minute)
	go f.reloadTicker()
	return nil
}

func (f *FileLoader) Close() error {
	f.reloadTick.Stop()
	f.stopTick <- struct{}{}
	return nil
}

func (f *FileLoader) Name() string {
	return "tls.loader.file"
}

func (f *FileLoader) InstanceName() string {
	return f.instName
}

func (f *FileLoader) reloadTicker() {
	for {
		select {
		case <-f.reloadTick.C:
			f.log.Debugln("reloading certs")
			if err := f.loadCerts(); err != nil {
				f.log.Error("reload failed", err)
			}
		case <-f.stopTick:
			return
		}
	}
}

func (f *FileLoader) loadCerts() error {
	if len(f.certPaths) != len(f.keyPaths) {
		return errors.New("mismatch in certs and keys count")
	}

	if len(f.certPaths) == 0 {
		return errors.New("tls.loader.file: at least one certificate required")
	}

	certs := make([]tls.Certificate, 0, len(f.certPaths))

	for i := range f.certPaths {
		certPath := f.certPaths[i]
		keyPath := f.keyPaths[i]

		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return fmt.Errorf("failed to load %s and %s: %v", certPath, keyPath, err)
		}
		certs = append(certs, cert)
	}

	f.certsLock.Lock()
	defer f.certsLock.Unlock()
	f.certs = certs

	return nil
}

func (f *FileLoader) ConfigureTLS(c *tls.Config) error {
	// Loader function replaces only the whole slice.
	f.certsLock.RLock()
	defer f.certsLock.RUnlock()

	c.Certificates = f.certs
	return nil
}

func init() {
	var _ module.TLSLoader = &FileLoader{}
	module.Register("tls.loader.file", NewFileLoader)
}
