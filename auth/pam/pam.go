// +build cgo,libpam

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
