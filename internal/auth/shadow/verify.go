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

package shadow

import (
	"errors"
	"fmt"
	"time"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/sha256_crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
)

const secsInDay = 86400

func (e *Entry) IsAccountValid() bool {
	if e.AcctExpiry == -1 {
		return true
	}

	nowDays := int(time.Now().Unix() / secsInDay)
	return nowDays < e.AcctExpiry
}

func (e *Entry) IsPasswordValid() bool {
	if e.LastChange == -1 || e.MaxPassAge == -1 || e.InactivityPeriod == -1 {
		return true
	}

	nowDays := int(time.Now().Unix() / secsInDay)
	return nowDays < e.LastChange+e.MaxPassAge+e.InactivityPeriod
}

func (e *Entry) VerifyPassword(pass string) (err error) {
	// Do not permit null and locked passwords.
	if e.Pass == "" {
		return errors.New("verify: null password")
	}
	if e.Pass[0] == '!' {
		return errors.New("verify: locked password")
	}

	// crypt.NewFromHash may panic on unknown hash function.
	defer func() {
		if rcvr := recover(); rcvr != nil {
			err = fmt.Errorf("%v", rcvr)
		}
	}()

	if err := crypt.NewFromHash(e.Pass).Verify(e.Pass, []byte(pass)); err != nil {
		if errors.Is(err, crypt.ErrKeyMismatch) {
			return ErrWrongPassword
		}
		return err
	}
	return nil
}
