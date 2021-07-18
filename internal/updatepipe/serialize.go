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

package updatepipe

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	mess "github.com/foxcpp/go-imap-mess"
)

func unescapeName(s string) string {
	return strings.ReplaceAll(s, "\x10", ";")
}

func escapeName(s string) string {
	return strings.ReplaceAll(s, ";", "\x10")
}

func parseUpdate(s string) (id string, upd *mess.Update, err error) {
	parts := strings.SplitN(s, ";", 2)
	if len(parts) != 2 {
		return "", nil, errors.New("updatepipe: mismatched parts count")
	}

	upd = &mess.Update{}
	dec := json.NewDecoder(strings.NewReader(unescapeName(parts[1])))
	dec.UseNumber()
	err = dec.Decode(upd)
	if err != nil {
		return "", nil, fmt.Errorf("parseUpdate: %w", err)
	}

	if val, ok := upd.Key.(json.Number); ok {
		upd.Key, _ = strconv.ParseUint(val.String(), 10, 64)
	}

	return parts[0], upd, nil
}

func formatUpdate(myID string, upd mess.Update) (string, error) {
	updBlob, err := json.Marshal(upd)
	if err != nil {
		return "", fmt.Errorf("formatUpdate: %w", err)
	}
	return strings.Join([]string{
		myID,
		escapeName(string(updBlob)),
	}, ";") + "\n", nil
}
