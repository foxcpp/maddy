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

package external

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/foxcpp/maddy/framework/module"
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
		}
		return fmt.Errorf("helperauth: process wait: %w", err)
	}
	return nil
}
