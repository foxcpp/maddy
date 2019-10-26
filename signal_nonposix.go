//+build windows

package maddy

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/foxcpp/maddy/log"
)

func handleSignals() os.Signal {
	sig := make(chan os.Signal, 5)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)

	s := <-sig
	go func() {
		s := waitForSignal()
		log.Printf("forced shutdown due to signal (%v)!", s)
		os.Exit(1)
	}()

	log.Printf("signal received (%v), next signal will force immediate shutdown.", s)
	return s
}
