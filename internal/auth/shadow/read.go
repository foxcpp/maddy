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
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	ErrNoSuchUser    = errors.New("shadow: user entry is not present in database")
	ErrWrongPassword = errors.New("shadow: wrong password")
)

// Read reads system shadow passwords database and returns all entires in it.
func Read() ([]Entry, error) {
	f, err := os.Open("/etc/shadow")
	if err != nil {
		return nil, err
	}
	scnr := bufio.NewScanner(f)

	var res []Entry
	for scnr.Scan() {
		ent, err := parseEntry(scnr.Text())
		if err != nil {
			return res, err
		}

		res = append(res, *ent)
	}
	if err := scnr.Err(); err != nil {
		return res, err
	}
	return res, nil
}

func parseEntry(line string) (*Entry, error) {
	parts := strings.Split(line, ":")
	if len(parts) != 9 {
		return nil, errors.New("read: malformed entry")
	}

	res := &Entry{
		Name: parts[0],
		Pass: parts[1],
	}

	for i, value := range [...]*int{
		&res.LastChange, &res.MinPassAge, &res.MaxPassAge,
		&res.WarnPeriod, &res.InactivityPeriod, &res.AcctExpiry, &res.Flags,
	} {
		if parts[2+i] == "" {
			*value = -1
		} else {
			var err error
			*value, err = strconv.Atoi(parts[2+i])
			if err != nil {
				return nil, fmt.Errorf("read: invalid value for field %d", 2+i)
			}
		}
	}

	return res, nil
}

func Lookup(name string) (*Entry, error) {
	entries, err := Read()
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.Name == name {
			return &entry, nil
		}
	}

	return nil, ErrNoSuchUser
}
