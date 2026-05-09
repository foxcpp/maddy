//go:build integration

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

package tests_test

import (
	"bufio"
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Smoke test to ensure message delivery is handled correctly.

func TestImapsql2Delivery(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)

	t.DNS(nil)
	t.Port("imap")
	t.Port("smtp")
	t.Config(`
		storage.imapsql2 test_store {
			driver sqlite3
			dsn imapsql.db
		}

		imap2 tcp://127.0.0.1:{env:TEST_PORT_imap} {
			tls off

			auth dummy
			storage &test_store
		}

		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname maddy.test
			tls off

			deliver_to &test_store
		}
	`)
	t.Run(2)
	defer t.Close()

	imapConn, collector := t.IMAP("imap")
	defer imapConn.Close()

	require.NoError(t, imapConn.Login("testusr@maddy.test", "1234").Wait())
	_, err := imapConn.Select("INBOX", nil).Wait()
	require.NoError(t, err)

	smtpConn := t.Conn("smtp")
	defer smtpConn.Close()
	smtpConn.SMTPNegotation("localhost", nil, nil)
	smtpConn.SMTPSimpleMessage("sender@maddy.test", "testusr@maddy.test")

	time.Sleep(500 * time.Millisecond)

	require.NoError(t, imapConn.Noop().Wait())
	require.Len(t, collector.Mailboxes, 1)
	assert.EqualValues(t, *collector.Mailboxes[0].NumMessages, 1)

	body, err := imapConn.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{{}},
	}).Collect()
	require.NoError(t, err)
	require.Len(t, body, 1)

	r := bufio.NewReader(bytes.NewBuffer(body[0].BodySection[0].Bytes))
	hdr, err := textproto.ReadHeader(r)
	require.NoError(t, err)
	assert.Equal(t, hdr.Get("Delivered-To"), "<testusr@maddy.test>")
	assert.Equal(t, hdr.Get("Return-Path"), "<sender@maddy.test>")
	assert.Equal(t, hdr.Get("From"), "<sender.body@maddy.test>")
	assert.Equal(t, hdr.Get("To"), "<testusr.body@maddy.test>")
	assert.Equal(t, hdr.Get("Subject"), "Hi!")

	bodyBlob, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, string(bodyBlob), "Hi!\r\n")
}

func TestImapsql2DeliveryMap(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)

	t.DNS(nil)
	t.Port("imap")
	t.Port("smtp")
	t.Config(`
		storage.imapsql2 test_store {
			delivery_map email_localpart

			driver sqlite3
			dsn imapsql.db
		}

		imap2 tcp://127.0.0.1:{env:TEST_PORT_imap} {
			tls off

			storage_map_normalize precis
			auth dummy
			storage &test_store
		}

		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname maddy.test
			tls off

			deliver_to &test_store
		}
	`)
	t.Run(2)
	defer t.Close()

	imapConn, collector := t.IMAP("imap")
	defer imapConn.Close()

	require.NoError(t, imapConn.Login("testusr", "1234").Wait())
	_, err := imapConn.Select("INBOX", nil).Wait()
	require.NoError(t, err)

	smtpConn := t.Conn("smtp")
	defer smtpConn.Close()
	smtpConn.SMTPNegotation("localhost", nil, nil)
	smtpConn.SMTPSimpleMessage("sender@maddy.test", "testusr@maddy.test")

	time.Sleep(500 * time.Millisecond)

	require.NoError(t, imapConn.Noop().Wait())
	require.Len(t, collector.Mailboxes, 1)
	assert.EqualValues(t, *collector.Mailboxes[0].NumMessages, 1)

	body, err := imapConn.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{{}},
	}).Collect()
	require.NoError(t, err)
	require.Len(t, body, 1)

	r := bufio.NewReader(bytes.NewBuffer(body[0].BodySection[0].Bytes))
	hdr, err := textproto.ReadHeader(r)
	require.NoError(t, err)
	assert.Equal(t, hdr.Get("Delivered-To"), "<testusr@maddy.test>")
	assert.Equal(t, hdr.Get("Return-Path"), "<sender@maddy.test>")
	assert.Equal(t, hdr.Get("From"), "<sender.body@maddy.test>")
	assert.Equal(t, hdr.Get("To"), "<testusr.body@maddy.test>")
	assert.Equal(t, hdr.Get("Subject"), "Hi!")

	bodyBlob, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, string(bodyBlob), "Hi!\r\n")
}
