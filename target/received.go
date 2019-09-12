package target

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/foxcpp/maddy/dns"
	"github.com/foxcpp/maddy/module"
)

func lookupAddr(r dns.Resolver, ip net.IP) (string, error) {
	names, err := r.LookupAddr(context.Background(), ip.String())
	if err != nil || len(names) == 0 {
		return "", err
	}
	return strings.TrimRight(names[0], "."), nil
}

func SanitizeString(raw string) string {
	return strings.Replace(raw, "\n", "", -1)
}

func GenerateReceived(r dns.Resolver, msgMeta *module.MsgMetadata, mailFrom, rcptTo string) string {
	var received string
	if !msgMeta.DontTraceSender {
		received += "from " + msgMeta.SrcHostname
		if tcpAddr, ok := msgMeta.SrcAddr.(*net.TCPAddr); ok {
			// TODO: Execute rDNS lookup from dispatcher
			// asynchronously to make sure we don't do it multiple times.
			domain, err := lookupAddr(r, tcpAddr.IP)
			if err != nil {
				received += fmt.Sprintf(" ([%v])", tcpAddr.IP)
			} else {
				received += fmt.Sprintf(" (%s [%v])", domain, tcpAddr.IP)
			}
		}
	}
	received += fmt.Sprintf(" by %s (envelope-sender <%s>)", SanitizeString(msgMeta.OurHostname), SanitizeString(mailFrom))
	received += fmt.Sprintf(" with %s id %s", msgMeta.SrcProto, msgMeta.ID)
	if rcptTo != "" {
		received += fmt.Sprintf(" for %s", rcptTo)
	}
	received += fmt.Sprintf("; %s", time.Now().Format(time.RFC1123Z))
	return received
}
