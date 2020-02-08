package clitools

import (
	"bufio"
	"errors"
	"fmt"
	"os"
)

var stdinScanner = bufio.NewScanner(os.Stdin)

func Confirmation(prompt string, def bool) bool {
	selection := "y/N"
	if def {
		selection = "Y/n"
	}

	fmt.Fprintf(os.Stderr, "%s [%s]: ", prompt, selection)
	if !stdinScanner.Scan() {
		fmt.Fprintln(os.Stderr, stdinScanner.Err())
		return false
	}

	switch stdinScanner.Text() {
	case "Y", "y":
		return true
	case "N", "n":
		return false
	default:
		return def
	}
}

func readPass(tty *os.File, output []byte) ([]byte, error) {
	cursor := output[0:1]
	readen := 0
	for {
		n, err := tty.Read(cursor)
		if n != 1 {
			return nil, errors.New("ReadPassword: invalid read size when not in canonical mode")
		}
		if err != nil {
			return nil, errors.New("ReadPassword: " + err.Error())
		}
		if cursor[0] == '\n' {
			break
		}
		// Esc or Ctrl+D or Ctrl+C.
		if cursor[0] == '\x1b' || cursor[0] == '\x04' || cursor[0] == '\x03' {
			return nil, errors.New("ReadPassword: prompt rejected")
		}
		if cursor[0] == '\x7F' /* DEL */ {
			if readen != 0 {
				readen--
				cursor = output[readen : readen+1]
			}
			continue
		}

		if readen == cap(output) {
			return nil, errors.New("ReadPassword: too long password")
		}

		readen++
		cursor = output[readen : readen+1]
	}

	return output[0:readen], nil
}

func ReadPassword(prompt string) (string, error) {
	termios, err := TurnOnRawIO(os.Stdin)
	hiddenPass := true
	if err != nil {
		hiddenPass = false
		fmt.Fprintln(os.Stderr, "Failed to disable terminal output:", err)
	}

	// There is no meaningful way to handle error here.
	//nolint:errcheck
	defer TcSetAttr(os.Stdin.Fd(), &termios)

	fmt.Fprintf(os.Stderr, "%s: ", prompt)

	if hiddenPass {
		buf := make([]byte, 512)
		buf, err = readPass(os.Stdin, buf)
		if err != nil {
			return "", err
		}
		fmt.Println()

		return string(buf), nil
	} else {
		if !stdinScanner.Scan() {
			return "", stdinScanner.Err()
		}

		return stdinScanner.Text(), nil
	}
}
