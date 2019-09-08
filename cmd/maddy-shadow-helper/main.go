package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/foxcpp/maddy/auth/shadow"
)

func main() {
	scnr := bufio.NewScanner(os.Stdin)

	if !scnr.Scan() {
		fmt.Fprintln(os.Stderr, scnr.Err())
		os.Exit(2)
	}
	username := scnr.Text()

	if !scnr.Scan() {
		fmt.Fprintln(os.Stderr, scnr.Err())
		os.Exit(2)
	}
	password := scnr.Text()

	ent, err := shadow.Lookup(username)
	if err != nil {
		if err == shadow.ErrNoSuchUser {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if !ent.IsAccountValid() {
		fmt.Fprintln(os.Stderr, "account is expired")
		os.Exit(1)
	}

	if !ent.IsPasswordValid() {
		fmt.Fprintln(os.Stderr, "password is expired")
		os.Exit(1)
	}

	if err := ent.VerifyPassword(password); err != nil {
		if err == shadow.ErrWrongPassword {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
