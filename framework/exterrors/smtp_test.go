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

package exterrors

import (
	"errors"
	"testing"
)

func TestSMTPEnchCode(t *testing.T) {
	temp := WithTemporary(errors.New("temp"), true)
	perm := WithTemporary(errors.New("perm"), false)

	if got := SMTPEnchCode(temp, EnhancedCode{0, 4, 0}); got != (EnhancedCode{4, 4, 0}) {
		t.Errorf("temporary error: got %v, want 4.4.0", got)
	}
	if got := SMTPEnchCode(perm, EnhancedCode{0, 4, 0}); got != (EnhancedCode{5, 4, 0}) {
		t.Errorf("permanent error: got %v, want 5.4.0", got)
	}
	if got := SMTPEnchCode(errors.New("plain"), EnhancedCode{0, 4, 0}); got != (EnhancedCode{5, 4, 0}) {
		t.Errorf("plain error: got %v, want 5.4.0", got)
	}
}
