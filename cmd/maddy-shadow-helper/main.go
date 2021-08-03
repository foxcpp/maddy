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

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"github.com/foxcpp/maddy/internal/auth/shadow"
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
		if errors.Is(err, shadow.ErrNoSuchUser) {
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
		if errors.Is(err, shadow.ErrWrongPassword) {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
