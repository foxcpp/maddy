package maddy

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/textproto"
	"sort"
	"strings"

	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

var ErrTLSRequired = errors.New("TLS is required for outgoing connections but target server doesn't supports STARTTLS")

type RemoteDelivery struct {
	name       string
	hostname   string
	requireTLS bool

	Log log.Logger
}

func NewRemoteDelivery(_, instName string) (module.Module, error) {
	return &RemoteDelivery{
		name: instName,
		Log:  log.Logger{Name: "remote"},
	}, nil
}

func (rd *RemoteDelivery) Init(cfg *config.Map) error {
	cfg.String("hostname", true, true, "", &rd.hostname)
	cfg.Bool("debug", true, &rd.Log.Debug)
	cfg.Bool("require_tls", false, &rd.requireTLS)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	return nil
}

func (rd *RemoteDelivery) Name() string {
	return "remote"
}

func (rd *RemoteDelivery) InstanceName() string {
	return rd.name
}

func (rd *RemoteDelivery) Deliver(ctx module.DeliveryContext, msg io.Reader) error {
	var body io.ReadSeeker
	if seekable, ok := msg.(io.ReadSeeker); ok {
		body = seekable
	} else {
		bodySlice, err := ioutil.ReadAll(msg)
		if err != nil {
			return errors.New("failed to buffer message")
		}
		body = bytes.NewReader(bodySlice)
	}

	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// TODO: Add field to DeliveryContext.
	msgId := ctx.Ctx["id"]
	partialErr := PartialError{
		Errs: make(map[string]error),
	}

	for _, rcpt := range ctx.To {
		rcptErr := func(temporary bool, err error) {
			if temporary {
				partialErr.TemporaryFailed = append(partialErr.TemporaryFailed, rcpt)
			} else {
				partialErr.Failed = append(partialErr.Failed, rcpt)
			}
			partialErr.Errs[rcpt] = err
		}

		hosts, err := lookupTargetServers(rcpt)
		if err != nil {
			isTmp := isTemporaryErr(err)
			if isTmp {
				rd.Log.Printf("sending %v to %s: temporary error during MX lookup: %v", msgId, rcpt, err)
			} else {
				log.Printf("sending %v to %s: permanent error during MX lookup: %v", msgId, rcpt, err)
			}
			rcptErr(isTmp, err)
			continue
		}
		if len(hosts) == 0 {
			// No mail eXchanger => permanent error for this recipient.
			rd.Log.Printf("sending %v to %s: no MX record found", ctx.Ctx["id"], rcpt)
			rcptErr(false, errors.New("no MX record found"))
			continue
		}

		// TODO: Send to all mailboxes on one server in one session.
		var cl *smtp.Client
		var usedHost string
		var temporaryConnErr bool
		var lastErr error
		for _, host := range hosts {
			cl, err = connectToServer(rd.hostname, host, !rd.requireTLS)
			if err == nil {
				usedHost = host
				break
			} else {
				lastErr = err
				if isTemporaryErr(err) {
					temporaryConnErr = true
				}
				rd.Log.Printf("failed to connect to %s: %v (temporary=%v)", host, err, temporaryConnErr)
			}
		}
		if cl == nil {
			rd.Log.Printf("sending %v to %s: no usable SMTP server found", msgId, rcpt)
			rcptErr(temporaryConnErr, lastErr)
			continue
		}

		if err := cl.Mail(ctx.From); err != nil {
			log.Printf("sending %v to %s: MAIL FROM failed: %v", msgId, rcpt, err)
			rcptErr(isTemporaryErr(err), err)
			continue
		}
		if err := cl.Rcpt(rcpt); err != nil {
			log.Printf("sending %v to %s: RCPT TO failed: %v", msgId, usedHost, err)
			rcptErr(isTemporaryErr(err), err)
			continue
		}
		bodyWriter, err := cl.Data()
		if err != nil {
			log.Printf("sending %v to %s: DATA failed: %v", msgId, usedHost, err)
			rcptErr(isTemporaryErr(err), err)
			continue
		}
		if _, err := io.Copy(bodyWriter, body); err != nil {
			log.Printf("sending %v to %s: body write failed: %v", msgId, usedHost, err)
			// I/O errors are assumed to be temporary.
			rcptErr(isTemporaryErr(err), err)
			continue
		}

		rd.Log.Printf("delivered %v to %s (%s)", msgId, rcpt, usedHost)
		partialErr.Successful = append(partialErr.Successful, rcpt)
		bodyWriter.Close()
	}

	if len(partialErr.Failed) == 0 && len(partialErr.TemporaryFailed) == 0 {
		return nil
	}
	return partialErr
}

func connectToServer(hostname, host string, requireTLS bool) (*smtp.Client, error) {
	cl, err := smtp.Dial(host + ":25")
	if err != nil {
		return nil, err
	}

	if err := cl.Hello(hostname); err != nil {
		return nil, err
	}

	if tlsOk, _ := cl.Extension("STARTTLS"); tlsOk {
		if err := cl.StartTLS(&tls.Config{
			ServerName: host,
		}); err != nil {
			return nil, err
		}
	} else if requireTLS {
		return nil, ErrTLSRequired
	}

	return cl, nil
}

func lookupTargetServers(addr string) ([]string, error) {
	addrParts := strings.Split(addr, "@")
	if len(addrParts) != 2 {
		return nil, errors.New("malformed recipient address")
	}

	records, err := net.LookupMX(addrParts[1])
	if err != nil {
		return nil, err
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Pref < records[j].Pref
	})

	hosts := make([]string, 0, len(records))
	for _, record := range records {
		hosts = append(hosts, record.Host)
	}
	return hosts, nil
}

func isTemporaryErr(err error) bool {
	if protoErr, ok := err.(*textproto.Error); ok {
		return (protoErr.Code / 100) == 4
	}
	if smtpErr, ok := err.(*smtp.SMTPError); ok {
		return (smtpErr.Code / 100) == 4
	}
	if dnsErr, ok := err.(*net.DNSError); ok {
		return dnsErr.Temporary()
	}

	if strings.HasPrefix(err.Error(), "x509") {
		return false
	}

	if err == ErrTLSRequired {
		return false
	}

	// Connection error? Assume it is temporary.
	return true
}

func init() {
	module.Register("remote", NewRemoteDelivery)
}
