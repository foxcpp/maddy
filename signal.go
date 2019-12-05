//+build darwin dragonfly freebsd linux netbsd openbsd solaris

package maddy

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/foxcpp/maddy/internal/log"
)

// handleSignals function creates and listens on OS signals channel.
//
// OS-specific signals that correspond to the program termination
// (SIGTERM, SIGHUP, SIGINT) will cause this function to return.
//
// SIGUSR1 will call reinitLogging without returning.
func handleSignals() os.Signal {
	sig := make(chan os.Signal, 5)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT, syscall.SIGUSR1)

	for {
		switch s := <-sig; s {
		case syscall.SIGUSR1:
			log.Println("SIGUSR1 received, reinitializing logging")
			reinitLogging()
		default:
			go func() {
				s := handleSignals()
				log.Printf("forced shutdown due to signal (%v)!", s)
				os.Exit(1)
			}()

			log.Printf("signal received (%v), next signal will force immediate shutdown.", s)
			return s
		}
	}
}
