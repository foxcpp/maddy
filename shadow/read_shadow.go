package shadow

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var ErrNoSuchUser = errors.New("shadow: user entry is not present in database")
var ErrWrongPassword = errors.New("shadow: wrong password")

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
	var err error

	for i, value := range [...]*int{
		&res.LastChange, &res.MinPassAge, &res.MaxPassAge,
		&res.WarnPeriod, &res.InactivityPeriod, &res.AcctExpiry, &res.Flags,
	} {
		if parts[2+i] == "" {
			*value = -1
		} else {
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
