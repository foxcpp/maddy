package target

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/foxcpp/maddy/module"
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
	// entire value in most cases.
	builder.Grow(256 + len(msgMeta.Conn.Hostname))

	if !msgMeta.DontTraceSender && (strings.Contains(msgMeta.Conn.Proto, "SMTP") ||
		strings.Contains(msgMeta.Conn.Proto, "LMTP")) {

		builder.WriteString("from ")
		builder.WriteString(msgMeta.Conn.Hostname)
		if tcpAddr, ok := msgMeta.Conn.RemoteAddr.(*net.TCPAddr); ok {
			builder.WriteString(" (")
			if msgMeta.Conn.RDNSName != nil {
				rdnsName, err := msgMeta.Conn.RDNSName.GetContext(ctx)
				if err != nil {
					return "", err
				}
				if rdnsName != nil && rdnsName.(string) != "" {
					builder.WriteString(rdnsName.(string))
					builder.WriteRune(' ')
				}

			}
			builder.WriteRune('[')
			builder.WriteString(tcpAddr.IP.String())
			builder.WriteString("])")
		}
	}

	builder.WriteString(" by ")
	builder.WriteString(SanitizeForHeader(ourHostname))
	builder.WriteString(" (envelope-sender <")
	builder.WriteString(SanitizeForHeader(mailFrom))
	builder.WriteString(">)")
	if msgMeta.Conn.Proto != "" {
		builder.WriteString(" with ")
		builder.WriteString(msgMeta.Conn.Proto)
	}
	builder.WriteString(" id ")
	builder.WriteString(msgMeta.ID)
	builder.WriteString("; ")
	builder.WriteString(time.Now().Format(time.RFC1123Z))

	return builder.String(), nil
}
