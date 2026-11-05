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
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

// dateHeaderLayout is the format used when maddy generates a Date header
// field. It matches RFC 5322, section 3.3.
const dateHeaderLayout = "Mon, 2 Jan 2006 15:04:05 -0700"

// RFC 5322, section 4.3 permits a single letter military time zone in
// // obsolete messages.
var militaryZoneRE = regexp.MustCompile(`(?i)\s([A-IK-Z])(\s*\(.*\))?$`)

// RFC 5322, section 3.3 permits a leap second (60).
var leapSecondRE = regexp.MustCompile(`(\d{2}:\d{2}):60(\s|$)`)

// parseMessageDateTime tries parsing the date based on the formats defined
// in RFC 5322, section 3.3 and the obsolete syntax from section 4.3.
// Inspired by https://github.com/golang/go/blob/master/src/net/mail/message.go
func parseMessageDateTime(maybeDate string) (time.Time, error) {
	if parsed, err := mail.ParseDate(maybeDate); err == nil {
		return parsed, nil
	}

	// net/mail.ParseDate does not implement every obsolete form that RFC 5322
	// requires parsers to accept. Normalize the missing pieces and retry.
	if parsed, err := mail.ParseDate(normalizeObsoleteDate(maybeDate)); err == nil {
		return parsed, nil
	}

	return time.Time{}, fmt.Errorf("date %s could not be parsed", maybeDate)
}

func normalizeObsoleteDate(date string) string {
	// RFC 5322 syntax is case-insensitive, but net/mail only recognizes
	// upper-case zone abbreviations.
	date = strings.ToUpper(date)

	// time.Time cannot represent a leap second, so parse it as the last
	// representable second of the minute.
	date = leapSecondRE.ReplaceAllString(date, "${1}:59${2}")

	// Military time zones SHOULD be treated as -0000 unless there is
	// out-of-band information about their meaning (RFC 5322, section 4.3).
	date = militaryZoneRE.ReplaceAllString(date, " -0000${2}")

	return date
}
