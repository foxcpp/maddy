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

package target

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/module"
)

func SanitizeForHeader(raw string) string {
	return strings.Replace(raw, "\n", "", -1)
}

func GenerateReceived(ctx context.Context, msgMeta *module.MsgMetadata, ourHostname, mailFrom string) (string, error) {
	if msgMeta.Conn == nil {
		return "", errors.New("can't generate Received for a locally generated message")
	}

	builder := strings.Builder{}

	// Empirically guessed value that should be enough to fit
	// the entire value in most cases.
	builder.Grow(256 + len(msgMeta.Conn.Hostname))

	if !msgMeta.DontTraceSender && (strings.Contains(msgMeta.Conn.Proto, "SMTP") ||
		strings.Contains(msgMeta.Conn.Proto, "LMTP")) {
		// INTERNATIONALIZATION: See RFC 6531 Section 3.7.3.
		hostname, err := dns.SelectIDNA(msgMeta.SMTPOpts.UTF8, msgMeta.Conn.Hostname)
		if err == nil {
			builder.WriteString("from ")
			builder.WriteString(hostname)
		}

		if tcpAddr, ok := msgMeta.Conn.RemoteAddr.(*net.TCPAddr); ok {
			builder.WriteString(" (")
			if msgMeta.Conn.RDNSName != nil {
				rdnsName, err := msgMeta.Conn.RDNSName.GetContext(ctx)
				if err == nil && rdnsName != nil && rdnsName.(string) != "" {
					// INTERNATIONALIZATION: See RFC 6531 Section 3.7.3.
					encoded, err := dns.SelectIDNA(msgMeta.SMTPOpts.UTF8, rdnsName.(string))
					if err == nil {
						builder.WriteString(encoded)
						builder.WriteRune(' ')
					}
				}
			}
			builder.WriteRune('[')
			builder.WriteString(tcpAddr.IP.String())
			builder.WriteString("])")
		}
	}

	if ourHostname != "" {
		ourHostname, err := dns.SelectIDNA(msgMeta.SMTPOpts.UTF8, ourHostname)
		if err == nil {
			builder.WriteString(" by ")
			builder.WriteString(SanitizeForHeader(ourHostname))
		}
	}

	if mailFrom != "" {
		// INTERNATIONALIZATION: See RFC 6531 Section 3.7.3.
		mailFrom, err := address.SelectIDNA(msgMeta.SMTPOpts.UTF8, mailFrom)
		if err == nil {
			builder.WriteString(" (envelope-sender <")
			builder.WriteString(SanitizeForHeader(mailFrom))
			builder.WriteString(">)")
		}
	}

	if msgMeta.Conn.Proto != "" {
		builder.WriteString(" with ")
		if msgMeta.SMTPOpts.UTF8 {
			builder.WriteString("UTF8")
		}
		builder.WriteString(msgMeta.Conn.Proto)
	}
	builder.WriteString(" id ")
	builder.WriteString(msgMeta.ID)
	builder.WriteString("; ")
	builder.WriteString(time.Now().Format(time.RFC1123Z))

	return strings.TrimSpace(builder.String()), nil
}
