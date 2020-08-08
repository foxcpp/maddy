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
	"regexp"
	"time"
)

// Taken from https://github.com/emersion/go-imap/blob/09c1d69/date.go.

var dateTimeLayouts = [...]string{
	// Defined in RFC 5322 section 3.3, mentioned as env-date in RFC 3501 page 84.
	"Mon, 02 Jan 2006 15:04:05 -0700",
	"_2 Jan 2006 15:04:05 -0700",
	"_2 Jan 2006 15:04:05 MST",
	"_2 Jan 2006 15:04 -0700",
	"_2 Jan 2006 15:04 MST",
	"_2 Jan 06 15:04:05 -0700",
	"_2 Jan 06 15:04:05 MST",
	"_2 Jan 06 15:04 -0700",
	"_2 Jan 06 15:04 MST",
	"Mon, _2 Jan 2006 15:04:05 -0700",
	"Mon, _2 Jan 2006 15:04:05 MST",
	"Mon, _2 Jan 2006 15:04 -0700",
	"Mon, _2 Jan 2006 15:04 MST",
	"Mon, _2 Jan 06 15:04:05 -0700",
	"Mon, _2 Jan 06 15:04:05 MST",
	"Mon, _2 Jan 06 15:04 -0700",
	"Mon, _2 Jan 06 15:04 MST",
}

// TODO: this is a blunt way to strip any trailing CFWS (comment). A sharper
// one would strip multiple CFWS, and only if really valid according to
// RFC5322.
var commentRE = regexp.MustCompile(`[ \t]+\(.*\)$`)

// Try parsing the date based on the layouts defined in RFC 5322, section 3.3.
// Inspired by https://github.com/golang/go/blob/master/src/net/mail/message.go
func parseMessageDateTime(maybeDate string) (time.Time, error) {
	maybeDate = commentRE.ReplaceAllString(maybeDate, "")
	for _, layout := range dateTimeLayouts {
		parsed, err := time.Parse(layout, maybeDate)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("date %s could not be parsed", maybeDate)
}
