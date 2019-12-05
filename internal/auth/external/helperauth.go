package external

import (
	"io"
	"os/exec"
	"strings"

	"github.com/foxcpp/maddy/internal/log"
)

func AuthUsingHelper(l log.Logger, binaryPath, accountName, password string) bool {
	cmd := exec.Command(binaryPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		l.Println("failed to obtain stdin pipe for helper process:", err)
		return false
	}
	if err := cmd.Start(); err != nil {
		l.Println("failed to start helper process:", err)
		return false
	}
	if _, err := io.WriteString(stdin, accountName+"\n"); err != nil {
		l.Println("failed to write stdin of helper process:", err)
		return false
	}
	if _, err := io.WriteString(stdin, password+"\n"); err != nil {
		l.Println("failed to write stdin of helper process:", err)
		return false
	}
	if err := cmd.Wait(); err != nil {
		l.Debugln(err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 is for authentication failure.
			// Exit code 2 is for other errors.
			if exitErr.ExitCode() == 2 {
				l.Println(strings.TrimSpace(string(exitErr.Stderr)))
			}
		} else {
			l.Println("failed to wait for helper process:", err)
		}
		return false
	}
	return true
}
