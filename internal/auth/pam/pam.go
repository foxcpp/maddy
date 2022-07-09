//go:build cgo && libpam
// +build cgo,libpam

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

package pam

/*
#cgo LDFLAGS: -lpam
#cgo CFLAGS: -DCGO -Wall -Wextra -Werror -Wno-unused-parameter -Wno-error=unused-parameter -Wpedantic -std=c99

#include <stdlib.h>
#include "pam.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

const canCallDirectly = true

var ErrInvalidCredentials = errors.New("pam: invalid credentials or unknown user")

func runPAMAuth(username, password string) error {
	usernameC := C.CString(username)
	passwordC := C.CString(password)
	defer C.free(unsafe.Pointer(usernameC))
	defer C.free(unsafe.Pointer(passwordC))
	errObj := C.run_pam_auth(usernameC, passwordC)
	if errObj.status == 1 {
		return ErrInvalidCredentials
	}
	if errObj.status == 2 {
		return fmt.Errorf("%s: %s", C.GoString(errObj.func_name), C.GoString(errObj.error_msg))
	}
	return nil
}
