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

package smtp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/trace"
	"strconv"
	"strings"
	"sync"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

func limitReader(r io.Reader, n int64, err error) *limitedReader {
	return &limitedReader{R: r, N: n, E: err, Enabled: true}
}

type limitedReader struct {
	R       io.Reader
	N       int64
	E       error
	Enabled bool
}

// same as io.LimitedReader.Read except returning the custom error and the option
// to be disabled
func (l *limitedReader) Read(p []byte) (n int, err error) {
	if !l.Enabled {
		return l.R.Read(p)
	}
	if l.N <= 0 {
		return 0, l.E
	}
	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)
	return
}

type Session struct {
	endp *Endpoint

	// Specific for this session.
	// sessionCtx is not used for cancellation or timeouts, only for tracing.
	sessionCtx       context.Context
	cancelRDNS       func()
	connState        module.ConnState
	repeatedMailErrs int
	loggedRcptErrors int

	// Specific for the currently handled message.
	// msgCtx is not used for cancellation or timeouts, only for tracing.
	// It is the subcontext of sessionCtx.
	// Mutex is used to prevent Close from accessing inconsistent state when it
	// is called asynchronously to any SMTP command.
	msgLock     sync.Mutex
	msgCtx      context.Context
	msgTask     *trace.Task
	mailFrom    string
	opts        smtp.MailOptions
	msgMeta     *module.MsgMetadata
	delivery    module.Delivery
	deliveryErr error

	log log.Logger
}

func (s *Session) Reset() {
	s.msgLock.Lock()
	defer s.msgLock.Unlock()

	if s.delivery != nil {
		s.abort(s.msgCtx)
	}
	s.endp.Log.DebugMsg("reset")
}

func (s *Session) releaseLimits() {
	domain := ""
	if s.mailFrom != "" {
		var err error
		_, domain, err = address.Split(s.mailFrom)
		if err != nil {
			return
		}
	}

	addr, ok := s.msgMeta.Conn.RemoteAddr.(*net.TCPAddr)
	if !ok {
		addr = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}
	}
	s.endp.limits.ReleaseMsg(addr.IP, domain)
}

func (s *Session) abort(ctx context.Context) {
	if err := s.delivery.Abort(ctx); err != nil {
		s.endp.Log.Error("delivery abort failed", err)
	}
	s.log.Msg("aborted", "msg_id", s.msgMeta.ID)
	abortedSMTPTransactions.WithLabelValues(s.endp.name).Inc()
	s.cleanSession()
}

func (s *Session) cleanSession() {
	s.releaseLimits()

	s.mailFrom = ""
	s.opts = smtp.MailOptions{}
	s.msgMeta = nil
	s.delivery = nil
	s.deliveryErr = nil
	s.msgCtx = nil
	s.msgTask.End()
}

func (s *Session) AuthPlain(username, password string) error {
	if s.endp.serv.AuthDisabled {
		return smtp.ErrAuthUnsupported
	}

	// Executed before authentication and session initialization.
	if err := s.endp.pipeline.RunEarlyChecks(context.TODO(), &s.connState.ConnectionState); err != nil {
		return s.endp.wrapErr("", true, "AUTH", err)
	}

	err := s.endp.saslAuth.AuthPlain(username, password)
	if err != nil {
		s.endp.Log.Error("authentication failed", err, "username", username, "src_ip", s.connState.RemoteAddr)

		failedLogins.WithLabelValues(s.endp.name).Inc()

		if exterrors.IsTemporary(err) {
			return &smtp.SMTPError{
				Code:         454,
				EnhancedCode: smtp.EnhancedCode{4, 7, 0},
				Message:      "Temporary authentication failure",
			}
		}

		return &smtp.SMTPError{
			Code:         535,
			EnhancedCode: smtp.EnhancedCode{5, 7, 8},
			Message:      "Invalid credentials",
		}
	}

	s.connState.AuthUser = username
	s.connState.AuthPassword = password

	return nil
}

func (s *Session) startDelivery(ctx context.Context, from string, opts smtp.MailOptions) (string, error) {
	var err error
	msgMeta := &module.MsgMetadata{
		Conn:     &s.connState,
		SMTPOpts: opts,
	}
	msgMeta.ID, err = module.GenerateMsgID()
	if err != nil {
		return "", err
	}

	if s.connState.AuthUser != "" {
		s.log.Msg("incoming message",
			"src_host", msgMeta.Conn.Hostname,
			"src_ip", msgMeta.Conn.RemoteAddr.String(),
			"sender", from,
			"msg_id", msgMeta.ID,
			"username", s.connState.AuthUser,
		)
	} else {
		s.log.Msg("incoming message",
			"src_host", msgMeta.Conn.Hostname,
			"src_ip", msgMeta.Conn.RemoteAddr.String(),
			"sender", from,
			"msg_id", msgMeta.ID,
		)
	}

	// INTERNATIONALIZATION: Do not permit non-ASCII addresses unless SMTPUTF8 is
	// used.
	if !opts.UTF8 {
		for _, ch := range from {
			if ch > 128 {
				return "", &exterrors.SMTPError{
					Code:         550,
					EnhancedCode: exterrors.EnhancedCode{5, 6, 7},
					Message:      "SMTPUTF8 is required for non-ASCII senders",
				}
			}
		}
	}

	// Decode punycode, normalize to NFC and case-fold address.
	cleanFrom := from
	if from != "" {
		cleanFrom, err = address.CleanDomain(from)
		if err != nil {
			return "", &exterrors.SMTPError{
				Code:         553,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 7},
				Message:      "Unable to normalize the sender address",
			}
		}
	}

	msgMeta.OriginalFrom = from

	domain := ""
	if cleanFrom != "" {
		_, domain, err = address.Split(cleanFrom)
		if err != nil {
			return "", err
		}
	}
	remoteIP, ok := msgMeta.Conn.RemoteAddr.(*net.TCPAddr)
	if !ok {
		remoteIP = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}
	}
	if err := s.endp.limits.TakeMsg(context.Background(), remoteIP.IP, domain); err != nil {
		return "", err
	}

	s.msgCtx, s.msgTask = trace.NewTask(ctx, "Incoming Message")

	mailCtx, mailTask := trace.NewTask(s.msgCtx, "MAIL FROM")
	defer mailTask.End()

	delivery, err := s.endp.pipeline.Start(mailCtx, msgMeta, cleanFrom)
	if err != nil {
		s.msgCtx = nil
		s.msgTask.End()
		s.endp.limits.ReleaseMsg(remoteIP.IP, domain)
		return msgMeta.ID, err
	}

	startedSMTPTransactions.WithLabelValues(s.endp.name).Inc()

	s.msgMeta = msgMeta
	s.mailFrom = cleanFrom
	s.delivery = delivery

	return msgMeta.ID, nil
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	if s.endp.authAlwaysRequired && s.connState.AuthUser == "" {
		return smtp.ErrAuthRequired
	}

	s.msgLock.Lock()
	defer s.msgLock.Unlock()

	if !s.endp.deferServerReject {
		// Will initialize s.msgCtx.
		msgID, err := s.startDelivery(s.sessionCtx, from, *opts)
		if err != nil {
			if !errors.Is(err, context.DeadlineExceeded) {
				s.log.Error("MAIL FROM error", err, "msg_id", msgID)
			}
			return s.endp.wrapErr(msgID, !opts.UTF8, "MAIL", err)
		}
	}

	// Keep the MAIL FROM argument for deferred startDelivery.
	s.mailFrom = from
	s.opts = *opts

	return nil
}

func (s *Session) fetchRDNSName(ctx context.Context) {
	defer trace.StartRegion(ctx, "rDNS fetch").End()

	tcpAddr, ok := s.connState.RemoteAddr.(*net.TCPAddr)
	if !ok {
		s.connState.RDNSName.Set(nil, nil)
		return
	}

	name, err := dns.LookupAddr(ctx, s.endp.resolver, tcpAddr.IP)
	if err != nil {
		dnsErr, ok := err.(*net.DNSError)
		if ok && dnsErr.IsNotFound {
			s.connState.RDNSName.Set(nil, nil)
			return
		}

		reason, misc := exterrors.UnwrapDNSErr(err)
		misc["reason"] = reason
		if !strings.HasSuffix(reason, "canceled") {
			// Often occurs when transaction completes before rDNS lookup and
			// rDNS name was not actually needed. So do not log cancelation
			// error if that's the case.

			s.log.Error("rDNS error", exterrors.WithFields(err, misc), "src_ip", s.connState.RemoteAddr)
		}
		s.connState.RDNSName.Set(nil, err)
		return
	}

	s.connState.RDNSName.Set(name, nil)
}

func (s *Session) Rcpt(to string) error {
	s.msgLock.Lock()
	defer s.msgLock.Unlock()

	// deferServerReject = true and this is the first RCPT TO command.
	if s.delivery == nil {
		// If we already attempted to initialize the delivery -
		// fail again.
		if s.deliveryErr != nil {
			s.repeatedMailErrs++
			// The deliveryErr is already wrapped.
			return s.deliveryErr
		}

		// It will initialize s.msgCtx.
		msgID, err := s.startDelivery(s.sessionCtx, s.mailFrom, s.opts)
		if err != nil {
			if !errors.Is(err, context.DeadlineExceeded) {
				s.log.Error("MAIL FROM error (deferred)", err, "rcpt", to, "msg_id", msgID)
			}
			s.deliveryErr = s.endp.wrapErr(msgID, !s.opts.UTF8, "RCPT", err)
			return s.deliveryErr
		}
	}

	rcptCtx, rcptTask := trace.NewTask(s.msgCtx, "RCPT TO")
	defer rcptTask.End()

	if err := s.rcpt(rcptCtx, to); err != nil {
		if s.loggedRcptErrors < s.endp.maxLoggedRcptErrors {
			s.log.Error("RCPT error", err, "rcpt", to, "msg_id", s.msgMeta.ID)
			s.loggedRcptErrors++
			if s.loggedRcptErrors == s.endp.maxLoggedRcptErrors {
				s.log.Msg("too many RCPT errors, possible dictonary attack", "src_ip", s.connState.RemoteAddr, "msg_id", s.msgMeta.ID)
			}
		}
		return s.endp.wrapErr(s.msgMeta.ID, !s.opts.UTF8, "RCPT", err)
	}
	s.endp.Log.Msg("RCPT ok", "rcpt", to, "msg_id", s.msgMeta.ID)
	return nil
}

func (s *Session) rcpt(ctx context.Context, to string) error {
	// INTERNATIONALIZATION: Do not permit non-ASCII addresses unless SMTPUTF8 is
	// used.
	if !address.IsASCII(to) && !s.opts.UTF8 {
		return &exterrors.SMTPError{
			Code:         553,
			EnhancedCode: exterrors.EnhancedCode{5, 6, 7},
			Message:      "SMTPUTF8 is required for non-ASCII recipients",
		}
	}
	cleanTo, err := address.CleanDomain(to)
	if err != nil {
		return &exterrors.SMTPError{
			Code:         501,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
			Message:      "Unable to normalize the recipient address",
		}
	}

	return s.delivery.AddRcpt(ctx, cleanTo)
}

func (s *Session) Logout() error {
	s.msgLock.Lock()
	defer s.msgLock.Unlock()

	if s.delivery != nil {
		s.abort(s.msgCtx)

		if s.repeatedMailErrs > s.endp.maxLoggedRcptErrors {
			s.log.Msg("MAIL FROM repeated error a lot of times, possible dictonary attack", "count", s.repeatedMailErrs, "src_ip", s.connState.RemoteAddr)
		}
	}
	if s.cancelRDNS != nil {
		s.cancelRDNS()
	}
	return nil
}

func (s *Session) prepareBody(r io.Reader) (textproto.Header, buffer.Buffer, error) {
	limitr := limitReader(r, int64(s.endp.maxHeaderBytes), &exterrors.SMTPError{
		Code:         552,
		EnhancedCode: exterrors.EnhancedCode{5, 3, 4},
		Message:      "Message header size exceeds limit",
	})

	bufr := bufio.NewReader(limitr)
	header, err := textproto.ReadHeader(bufr)
	if err != nil {
		return textproto.Header{}, nil, fmt.Errorf("I/O error while parsing header: %w", err)
	}

	if s.endp.submission {
		// The MsgMetadata is passed by pointer all the way down.
		if err := s.submissionPrepare(s.msgMeta, &header); err != nil {
			return textproto.Header{}, nil, err
		}
	}

	// the header size check is done. The message size will be checked by go-smtp
	limitr.Enabled = false

	buf, err := s.endp.buffer(bufr)
	if err != nil {
		return textproto.Header{}, nil, fmt.Errorf("I/O error while writing buffer: %w", err)
	}

	return header, buf, nil
}

func (s *Session) Data(r io.Reader) error {
	s.msgLock.Lock()
	defer s.msgLock.Unlock()

	bodyCtx, bodyTask := trace.NewTask(s.msgCtx, "DATA")
	defer bodyTask.End()

	wrapErr := func(err error) error {
		s.log.Error("DATA error", err, "msg_id", s.msgMeta.ID)
		return s.endp.wrapErr(s.msgMeta.ID, !s.opts.UTF8, "DATA", err)
	}

	header, buf, err := s.prepareBody(r)
	if err != nil {
		return wrapErr(err)
	}
	defer func() {
		if err := buf.Remove(); err != nil {
			s.log.Error("failed to remove buffered body", err)
		}

		// go-smtp will call Reset, but it will call Abort if delivery is non-nil.
		s.cleanSession()
	}()

	if err := s.checkRoutingLoops(header); err != nil {
		return wrapErr(err)
	}

	if strings.EqualFold(header.Get("TLS-Required"), "No") {
		s.msgMeta.TLSRequireOverride = true
	}

	if err := s.delivery.Body(bodyCtx, header, buf); err != nil {
		return wrapErr(err)
	}

	if err := s.delivery.Commit(bodyCtx); err != nil {
		return wrapErr(err)
	}

	s.log.Msg("accepted", "msg_id", s.msgMeta.ID)

	return nil
}

type statusWrapper struct {
	sc smtp.StatusCollector
	s  *Session
}

func (sw statusWrapper) SetStatus(rcpt string, err error) {
	sw.sc.SetStatus(rcpt, sw.s.endp.wrapErr(sw.s.msgMeta.ID, !sw.s.opts.UTF8, "DATA", err))
}

func (s *Session) LMTPData(r io.Reader, sc smtp.StatusCollector) error {
	s.msgLock.Lock()
	defer s.msgLock.Unlock()

	bodyCtx, bodyTask := trace.NewTask(s.msgCtx, "DATA")
	defer bodyTask.End()

	wrapErr := func(err error) error {
		s.log.Error("DATA error", err, "msg_id", s.msgMeta.ID)
		return s.endp.wrapErr(s.msgMeta.ID, !s.opts.UTF8, "DATA", err)
	}

	header, buf, err := s.prepareBody(r)
	if err != nil {
		return wrapErr(err)
	}
	defer func() {
		if err := buf.Remove(); err != nil {
			s.log.Error("failed to remove buffered body", err)
		}

		// go-smtp will call Reset, but it will call Abort if delivery is non-nil.
		s.cleanSession()
	}()

	if strings.EqualFold(header.Get("TLS-Required"), "No") {
		s.msgMeta.TLSRequireOverride = true
	}

	if err := s.checkRoutingLoops(header); err != nil {
		return wrapErr(err)
	}

	s.delivery.(module.PartialDelivery).BodyNonAtomic(bodyCtx, statusWrapper{sc, s}, header, buf)

	// We can't really tell whether it is failed completely or succeeded
	// so always commit. Should be harmless, anyway.
	if err := s.delivery.Commit(bodyCtx); err != nil {
		return wrapErr(err)
	}

	s.log.Msg("accepted", "msg_id", s.msgMeta.ID)

	return nil
}

func (s *Session) checkRoutingLoops(header textproto.Header) error {
	// RFC 5321 Section 6.3:
	// >Simple counting of the number of "Received:" header fields in a
	// >message has proven to be an effective, although rarely optimal,
	// >method of detecting loops in mail systems.
	receivedCount := 0
	for f := header.FieldsByKey("Received"); f.Next(); {
		receivedCount++
	}
	if receivedCount > s.endp.maxReceived {
		return &exterrors.SMTPError{
			Code:         554,
			EnhancedCode: exterrors.EnhancedCode{5, 4, 6},
			Message:      fmt.Sprintf("Too many Received header fields (%d), possible forwarding loop", receivedCount),
		}
	}

	return nil
}

func (endp *Endpoint) wrapErr(msgId string, mangleUTF8 bool, command string, err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return &smtp.SMTPError{
			Code:         451,
			EnhancedCode: smtp.EnhancedCode{4, 4, 5},
			Message:      "High load, try again later",
		}
	}

	res := &smtp.SMTPError{
		Code:         554,
		EnhancedCode: smtp.EnhancedCodeNotSet,
		// Err on the side of caution if the error lacks SMTP annotations. If
		// we just pass the error text through, we might accidenetally disclose
		// details of server configuration.
		Message: "Internal server error",
	}

	if exterrors.IsTemporary(err) {
		res.Code = 451
	}

	ctxInfo := exterrors.Fields(err)
	ctxCode, ok := ctxInfo["smtp_code"].(int)
	if ok {
		res.Code = ctxCode
	}
	ctxEnchCode, ok := ctxInfo["smtp_enchcode"].(exterrors.EnhancedCode)
	if ok {
		res.EnhancedCode = smtp.EnhancedCode(ctxEnchCode)
	}
	ctxMsg, ok := ctxInfo["smtp_msg"].(string)
	if ok {
		res.Message = ctxMsg
	}

	if smtpErr, ok := err.(*smtp.SMTPError); ok {
		endp.Log.Printf("plain SMTP error returned, this is deprecated")
		res.Code = smtpErr.Code
		res.EnhancedCode = smtpErr.EnhancedCode
		res.Message = smtpErr.Message
	}

	if msgId != "" {
		res.Message += " (msg ID = " + msgId + ")"
	}

	failedCmds.WithLabelValues(endp.name, command, strconv.Itoa(res.Code),
		fmt.Sprintf("%d.%d.%d",
			res.EnhancedCode[0],
			res.EnhancedCode[1],
			res.EnhancedCode[2])).Inc()

	// INTERNATIONALIZATION: See RFC 6531 Section 3.7.4.1.
	if mangleUTF8 {
		b := strings.Builder{}
		b.Grow(len(res.Message))
		for _, ch := range res.Message {
			if ch > 128 {
				b.WriteRune('?')
			} else {
				b.WriteRune(ch)
			}
		}
		res.Message = b.String()
	}

	return res
}
