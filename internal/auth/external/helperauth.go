package external

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/foxcpp/maddy/internal/module"
)

func AuthUsingHelper(binaryPath, accountName, password string) error {
	cmd := exec.Command(binaryPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("helperauth: stdin init: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("helperauth: process start: %w", err)
	}
	if _, err := io.WriteString(stdin, accountName+"\n"); err != nil {
		return fmt.Errorf("helperauth: stdin write: %w", err)
	}
	if _, err := io.WriteString(stdin, password+"\n"); err != nil {
		return fmt.Errorf("helperauth: stdin write: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 is for authentication failure.
			if exitErr.ExitCode() != 1 {
				return fmt.Errorf("helperauth: %w: %v", err, string(exitErr.Stderr))
			}
			return module.ErrUnknownCredentials
		} else {
			return fmt.Errorf("helperauth: process wait: %w", err)
		}
	}
	return nil
}
