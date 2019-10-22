package target

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/foxcpp/maddy/module"
)

func SanitizeForHeader(raw string) string {
	return strings.Replace(raw, "\n", "", -1)
}

func GenerateReceived(ctx context.Context, msgMeta *module.MsgMetadata, ourHostname, mailFrom string) (string, error) {
	builder := strings.Builder{}

	// Empirically guessed value that should be enough to fit
	// entire value in most cases.
	builder.Grow(256 + len(msgMeta.SrcHostname))

	if !msgMeta.DontTraceSender && msgMeta.SrcHostname != "" {
		builder.WriteString("from ")
		builder.WriteString(msgMeta.SrcHostname)
		if tcpAddr, ok := msgMeta.SrcAddr.(*net.TCPAddr); ok {
			builder.WriteString(" (")
			if msgMeta.SrcRDNSName != nil {
				rdnsName, err := msgMeta.SrcRDNSName.GetContext(ctx)
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
	if msgMeta.SrcProto != "" {
		builder.WriteString(" with ")
		builder.WriteString(msgMeta.SrcProto)
	}
	builder.WriteString(" id ")
	builder.WriteString(msgMeta.ID)
	builder.WriteString("; ")
	builder.WriteString(time.Now().Format(time.RFC1123Z))

	return builder.String(), nil
}
