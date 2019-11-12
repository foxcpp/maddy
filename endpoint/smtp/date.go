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
