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
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
)

var (
	ErrPartsMismatch = errors.New("updatepipe: mismatched parts count")
)

func unescapeName(s string) string {
	return strings.ReplaceAll(s, "\x10", ";")
}

func escapeName(s string) string {
	return strings.ReplaceAll(s, ";", "\x10")
}

type message struct {
	SeqNum uint32
	Flags  []string
}

func parseUpdate(s string) (id string, upd backend.Update, err error) {
	parts := strings.SplitN(s, ";", 5)
	if len(parts) != 5 {
		return "", nil, ErrPartsMismatch
	}

	updBase := backend.NewUpdate(unescapeName(parts[2]), unescapeName(parts[3]))
	switch parts[1] {
	case "ExpungeUpdate":
		exUpd := &backend.ExpungeUpdate{Update: updBase}
		if err := json.Unmarshal([]byte(parts[4]), &exUpd.SeqNum); err != nil {
			return "", nil, err
		}
		upd = exUpd
	case "MailboxUpdate":
		mboxUpd := &backend.MailboxUpdate{Update: updBase}
		if err := json.Unmarshal([]byte(parts[4]), &mboxUpd.MailboxStatus); err != nil {
			return "", nil, err
		}
		upd = mboxUpd
	case "MessageUpdate":
		// imap.Message is not JSON-serializable because it contains maps with
		// complex keys.
		// In practice, however, MessageUpdate is used only for FLAGS, so we
		// serialize them only with a SeqNum.

		msg := message{}
		if err := json.Unmarshal([]byte(parts[4]), &msg); err != nil {
			return "", nil, err
		}

		msgUpd := &backend.MessageUpdate{
			Update:  updBase,
			Message: imap.NewMessage(msg.SeqNum, []imap.FetchItem{imap.FetchFlags}),
		}
		msgUpd.Message.Flags = msg.Flags
		upd = msgUpd
	}

	return parts[0], upd, nil
}

func formatUpdate(myID string, upd backend.Update) (string, error) {
	var (
		objType string
		objStr  []byte
		err     error
	)
	switch v := upd.(type) {
	case *backend.ExpungeUpdate:
		objType = "ExpungeUpdate"
		objStr, err = json.Marshal(v.SeqNum)
		if err != nil {
			return "", err
		}
	case *backend.MessageUpdate:
		// imap.Message is not JSON-serializable because it contains maps with
		// complex keys.
		// In practice, however, MessageUpdate is used only for FLAGS, so we
		// serialize them only with a seqnum.

		objType = "MessageUpdate"
		objStr, err = json.Marshal(message{
			SeqNum: v.Message.SeqNum,
			Flags:  v.Message.Flags,
		})
		if err != nil {
			return "", err
		}
	case *backend.MailboxUpdate:
		objType = "MailboxUpdate"
		objStr, err = json.Marshal(v.MailboxStatus)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("updatepipe: unknown update type: %T", upd)
	}

	return strings.Join([]string{
		myID,
		objType,
		escapeName(upd.Username()),
		escapeName(upd.Mailbox()),
		string(objStr),
	}, ";") + "\n", nil
}
