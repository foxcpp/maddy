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

package smtp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMessageDateTime(t *testing.T) {
	valid := []string{
		// RFC 5322 section 3.3.
		"Tue, 26 Jul 2022 17:52:00 +0000",
		"26 Jul 2022 17:52:00 +0000",
		"Tue, 26 Jul 2022 17:52 +0000",
		"Tue, 26 Jul 22 17:52:00 +0000",
		"Tue, 26 Jul 2022 17:52:00 -0700",
		"Tue, 26 Jul 2022 17:52:00 +0000 (UTC)",
		"tue, 26 jul 2022 17:52:00 +0000",

		// RFC 5322 section 4.3 obsolete zones.
		"Tue, 26 Jul 2022 17:52:00 UT",
		"Tue, 26 Jul 2022 17:52:00 ut",
		"Tue, 26 Jul 2022 17:52:00 GMT",
		"Tue, 26 Jul 2022 17:52:00 EST",
		"Tue, 26 Jul 2022 17:52:00 z",
		"Tue, 26 Jul 2022 17:52:00 A",

		// RFC 5322 section 3.3 allows a leap second.
		"Tue, 26 Jul 2022 23:59:60 +0000",
	}

	for _, date := range valid {
		t.Run(date, func(t *testing.T) {
			_, err := parseMessageDateTime(date)
			assert.NoError(t, err)
		})
	}

	invalid := []string{
		// Missing the sign in the numeric zone. This is not valid RFC 5322
		// syntax (issue #518).
		"Tue, 26 Jul 2022 17:52:00 0000",
		"",
		"not a date",
		"Tue, 26 Jul 2022 17:52:00",
		"Tue, 32 Jul 2022 17:52:00 +0000",
		"Tue, 26 Jul 2022 25:52:00 +0000",
	}

	for _, date := range invalid {
		t.Run(date, func(t *testing.T) {
			_, err := parseMessageDateTime(date)
			assert.Error(t, err)
		})
	}
}

func TestParseMessageDateTimeValue(t *testing.T) {
	parsed, err := parseMessageDateTime("Tue, 26 Jul 2022 17:52:00 +0530")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2022, 7, 26, 12, 22, 0, 0, time.UTC), parsed.UTC())
}
