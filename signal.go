//+build darwin dragonfly freebsd linux netbsd openbsd solaris

package maddy

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/foxcpp/maddy/log"
)

func waitForSignal() os.Signal {
	sig := make(chan os.Signal, 5)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT, syscall.SIGUSR1)

	for {
		switch s := <-sig; s {
		case syscall.SIGUSR1:
			log.Println("SIGUSR1 received, reinitializing logging")
			reinitLogging()
		default:
			go func() {
				s := waitForSignal()
				log.Printf("forced shutdown due to signal (%v)!", s)
				os.Exit(1)
			}()

			log.Printf("signal received (%v), next signal will force immediate shutdown.", s)
			return s
		}
	}
}
