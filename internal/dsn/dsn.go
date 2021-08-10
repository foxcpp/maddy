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

// Package dsn contains the utilities used for dsn message (DSN) generation.
//
// It implements RFC 3464 and RFC 3462.
package dsn

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/dns"
)

type ReportingMTAInfo struct {
	ReportingMTA    string
	ReceivedFromMTA string

	// Message sender address, included as 'X-Maddy-Sender: rfc822; ADDR' field.
	XSender string

	// Message identifier, included as 'X-Maddy-MsgId: MSGID' field.
	XMessageID string

	// Time when message was enqueued for delivery by Reporting MTA.
	ArrivalDate time.Time

	// Time when message delivery was attempted last time.
	LastAttemptDate time.Time
}

func (info ReportingMTAInfo) WriteTo(utf8 bool, w io.Writer) error {
	// DSN format uses structure similar to MIME header, so we reuse
	// MIME generator here.
	h := textproto.Header{}

	if info.ReportingMTA == "" {
		return errors.New("dsn: Reporting-MTA field is mandatory")
	}

	reportingMTA, err := dns.SelectIDNA(utf8, info.ReportingMTA)
	if err != nil {
		return fmt.Errorf("dsn: cannot convert Reporting-MTA to a suitable representation: %w", err)
	}

	h.Add("Reporting-MTA", "dns; "+reportingMTA)

	if info.ReceivedFromMTA != "" {
		receivedFromMTA, err := dns.SelectIDNA(utf8, info.ReceivedFromMTA)
		if err != nil {
			return fmt.Errorf("dsn: cannot convert Received-From-MTA to a suitable representation: %w", err)
		}

		h.Add("Received-From-MTA", "dns; "+receivedFromMTA)
	}

	if info.XSender != "" {
		sender, err := address.SelectIDNA(utf8, info.XSender)
		if err != nil {
			return fmt.Errorf("dsn: cannot convert X-Maddy-Sender to a suitable representation: %w", err)
		}

		if utf8 {
			h.Add("X-Maddy-Sender", "utf8; "+sender)
		} else {
			h.Add("X-Maddy-Sender", "rfc822; "+sender)
		}
	}
	if info.XMessageID != "" {
		h.Add("X-Maddy-MsgID", info.XMessageID)
	}

	if !info.ArrivalDate.IsZero() {
		h.Add("Arrival-Date", info.ArrivalDate.Format("Mon, 2 Jan 2006 15:04:05 -0700"))
	}
	if !info.ArrivalDate.IsZero() {
		h.Add("Last-Attempt-Date", info.LastAttemptDate.Format("Mon, 2 Jan 2006 15:04:05 -0700"))
	}

	return textproto.WriteHeader(w, h)
}

type Action string

const (
	ActionFailed    Action = "failed"
	ActionDelayed   Action = "delayed"
	ActionDelivered Action = "delivered"
	ActionRelayed   Action = "relayed"
	ActionExpanded  Action = "expanded"
)

type RecipientInfo struct {
	FinalRecipient string
	RemoteMTA      string

	Action Action
	Status smtp.EnhancedCode

	// DiagnosticCode is the error that will be returned to the sender.
	DiagnosticCode error
}

func (info RecipientInfo) WriteTo(utf8 bool, w io.Writer) error {
	// DSN format uses structure similar to MIME header, so we reuse
	// MIME generator here.
	h := textproto.Header{}

	if info.FinalRecipient == "" {
		return errors.New("dsn: Final-Recipient is required")
	}
	finalRcpt, err := address.SelectIDNA(utf8, info.FinalRecipient)
	if err != nil {
		return fmt.Errorf("dsn: cannot convert Final-Recipient to a suitable representation: %w", err)
	}
	if utf8 {
		h.Add("Final-Recipient", "utf8; "+finalRcpt)
	} else {
		h.Add("Final-Recipient", "rfc822; "+finalRcpt)
	}

	if info.Action == "" {
		return errors.New("dsn: Action is required")
	}
	h.Add("Action", string(info.Action))
	if info.Status[0] == 0 {
		return errors.New("dsn: Status is required")
	}
	h.Add("Status", fmt.Sprintf("%d.%d.%d", info.Status[0], info.Status[1], info.Status[2]))

	if smtpErr, ok := info.DiagnosticCode.(*smtp.SMTPError); ok {
		// Error message may contain newlines if it is received from another SMTP server.
		// But we cannot directly insert CR/LF into Disagnostic-Code so rewrite it.
		h.Add("Diagnostic-Code", fmt.Sprintf("smtp; %d %d.%d.%d %s",
			smtpErr.Code, smtpErr.EnhancedCode[0], smtpErr.EnhancedCode[1], smtpErr.EnhancedCode[2],
			strings.ReplaceAll(strings.ReplaceAll(smtpErr.Message, "\n", " "), "\r", " ")))
	} else if utf8 {
		// It might contain Unicode, so don't include it if we are not allowed to.
		// ... I didn't bother implementing mangling logic to remove Unicode
		// characters.
		errorDesc := info.DiagnosticCode.Error()
		errorDesc = strings.ReplaceAll(strings.ReplaceAll(errorDesc, "\n", " "), "\r", " ")

		h.Add("Diagnostic-Code", "X-Maddy; "+errorDesc)
	}

	if info.RemoteMTA != "" {
		remoteMTA, err := dns.SelectIDNA(utf8, info.RemoteMTA)
		if err != nil {
			return fmt.Errorf("dsn: cannot convert Remote-MTA to a suitable representation: %w", err)
		}

		h.Add("Remote-MTA", "dns; "+remoteMTA)
	}

	return textproto.WriteHeader(w, h)
}

type Envelope struct {
	MsgID string
	From  string
	To    string
}

// GenerateDSN is a top-level function that should be used for generation of the DSNs.
//
// DSN header will be returned, body itself will be written to outWriter.
func GenerateDSN(utf8 bool, envelope Envelope, mtaInfo ReportingMTAInfo, rcptsInfo []RecipientInfo, failedHeader textproto.Header, outWriter io.Writer) (textproto.Header, error) {
	partWriter := textproto.NewMultipartWriter(outWriter)

	reportHeader := textproto.Header{}
	reportHeader.Add("Date", time.Now().Format("Mon, 2 Jan 2006 15:04:05 -0700"))
	reportHeader.Add("Message-Id", envelope.MsgID)
	reportHeader.Add("Content-Transfer-Encoding", "8bit")
	reportHeader.Add("Content-Type", "multipart/report; report-type=delivery-status; boundary="+partWriter.Boundary())
	reportHeader.Add("MIME-Version", "1.0")
	reportHeader.Add("Auto-Submitted", "auto-replied")
	reportHeader.Add("To", envelope.To)
	reportHeader.Add("From", envelope.From)
	reportHeader.Add("Subject", "Undelivered Mail Returned to Sender")

	defer partWriter.Close()

	if err := writeHumanReadablePart(partWriter, mtaInfo, rcptsInfo); err != nil {
		return textproto.Header{}, err
	}
	if err := writeMachineReadablePart(utf8, partWriter, mtaInfo, rcptsInfo); err != nil {
		return textproto.Header{}, err
	}
	return reportHeader, writeHeader(utf8, partWriter, failedHeader)
}

func writeHeader(utf8 bool, w *textproto.MultipartWriter, header textproto.Header) error {
	partHeader := textproto.Header{}
	partHeader.Add("Content-Description", "Undelivered message header")
	if utf8 {
		partHeader.Add("Content-Type", "message/global-headers")
	} else {
		partHeader.Add("Content-Type", "message/rfc822-headers")
	}
	partHeader.Add("Content-Transfer-Encoding", "8bit")
	headerWriter, err := w.CreatePart(partHeader)
	if err != nil {
		return err
	}
	return textproto.WriteHeader(headerWriter, header)
}

func writeMachineReadablePart(utf8 bool, w *textproto.MultipartWriter, mtaInfo ReportingMTAInfo, rcptsInfo []RecipientInfo) error {
	machineHeader := textproto.Header{}
	if utf8 {
		machineHeader.Add("Content-Type", "message/global-delivery-status")
	} else {
		machineHeader.Add("Content-Type", "message/delivery-status")
	}
	machineHeader.Add("Content-Description", "Delivery report")
	machineWriter, err := w.CreatePart(machineHeader)
	if err != nil {
		return err
	}

	// WriteTo will add an empty line after output.
	if err := mtaInfo.WriteTo(utf8, machineWriter); err != nil {
		return err
	}

	for _, rcpt := range rcptsInfo {
		if err := rcpt.WriteTo(utf8, machineWriter); err != nil {
			return err
		}
	}
	return nil
}

// failedText is the text of the human-readable part of DSN.
var failedText = template.Must(template.New("dsn-text").Parse(`
This is the mail delivery system at {{.ReportingMTA}}.

Unfortunately, your message could not be delivered to one or more
recipients. The usual cause of this problem is invalid
recipient address or maintenance at the recipient side.

Contact the postmaster for further assistance, provide the Message ID (below):

Message ID: {{.XMessageID}}
Arrival: {{.ArrivalDate}}
Last delivery attempt: {{.LastAttemptDate}}

`))

func writeHumanReadablePart(w *textproto.MultipartWriter, mtaInfo ReportingMTAInfo, rcptsInfo []RecipientInfo) error {
	humanHeader := textproto.Header{}
	humanHeader.Add("Content-Transfer-Encoding", "8bit")
	humanHeader.Add("Content-Type", `text/plain; charset="utf-8"`)
	humanHeader.Add("Content-Description", "Notification")
	humanWriter, err := w.CreatePart(humanHeader)
	if err != nil {
		return err
	}

	mtaInfo.ArrivalDate = mtaInfo.ArrivalDate.Truncate(time.Second)
	mtaInfo.LastAttemptDate = mtaInfo.LastAttemptDate.Truncate(time.Second)

	if err := failedText.Execute(humanWriter, mtaInfo); err != nil {
		return err
	}

	for _, rcpt := range rcptsInfo {
		if _, err := fmt.Fprintf(humanWriter, "Delivery to %s failed with error: %v\n", rcpt.FinalRecipient, rcpt.DiagnosticCode); err != nil {
			return err
		}
	}

	return nil
}
