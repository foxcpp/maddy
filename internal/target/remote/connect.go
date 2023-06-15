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

package remote

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"runtime/trace"
	"sort"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/smtpconn"
)

type mxConn struct {
	*smtpconn.C

	// Domain this MX belongs to.
	domain   string
	dnssecOk bool

	// Errors occurred previously on this connection.
	errored bool

	reuseLimit int

	// Amount of times connection was used for an SMTP transaction.
	transactions int

	// MX/TLS security level established for this connection.
	mxLevel  module.MXLevel
	tlsLevel module.TLSLevel
}

func (c *mxConn) Usable() bool {
	if c.C == nil || c.transactions > c.reuseLimit || c.C.Client() == nil {
		return false
	}
	return c.C.Client().Reset() == nil
}

func (c *mxConn) Close() error {
	return c.C.Close()
}

func isVerifyError(err error) bool {
	_, ok := err.(x509.UnknownAuthorityError)
	if ok {
		return true
	}
	_, ok = err.(x509.HostnameError)
	if ok {
		return true
	}
	_, ok = err.(x509.ConstraintViolationError)
	if ok {
		return true
	}
	_, ok = err.(x509.CertificateInvalidError)
	if ok {
		return true
	}
	_, ok = err.(*tls.CertificateVerificationError)
	return ok
}

// connect attempts to connect to the MX, first trying STARTTLS with X.509
// verification but falling back to unauthenticated TLS or plaintext as
// necessary.
//
// Return values:
// - tlsLevel    TLS security level that was estabilished.
// - tlsErr      Error that prevented TLS from working if tlsLevel != TLSAuthenticated
func (rd *remoteDelivery) connect(ctx context.Context, conn mxConn, host string, tlsCfg *tls.Config) (tlsLevel module.TLSLevel, tlsErr, err error) {
	tlsLevel = module.TLSAuthenticated
	if rd.rt.tlsConfig != nil {
		tlsCfg = rd.rt.tlsConfig.Clone()
		tlsCfg.ServerName = host
	}

	rd.Log.DebugMsg("trying", "remote_server", host, "domain", conn.domain)

retry:
	// smtpconn.C default TLS behavior is not useful for us, we want to handle
	// TLS errors separately hence starttls=false.
	_, err = conn.Connect(ctx, config.Endpoint{
		Host: host,
		Port: smtpPort,
	}, false, nil)
	if err != nil {
		return module.TLSNone, nil, err
	}

	starttlsOk, _ := conn.Client().Extension("STARTTLS")
	if starttlsOk && tlsCfg != nil {
		if err := conn.Client().StartTLS(tlsCfg); err != nil {
			tlsErr = err

			// Attempt TLS without authentication. It is still better than
			// plaintext and we might be able to actually authenticate the
			// server using DANE-EE/DANE-TA later.
			//
			// Check tlsLevel is to avoid looping forever if the same verify
			// error happens with InsecureSkipVerify too (e.g. certificate is
			// *too* broken).
			if isVerifyError(err) && tlsLevel == module.TLSAuthenticated {
				rd.Log.Error("TLS verify error, trying without authentication", err, "remote_server", host, "domain", conn.domain)
				tlsCfg.InsecureSkipVerify = true
				tlsLevel = module.TLSEncrypted

				// TODO: Check go-smtp code to make TLS verification errors
				// non-sticky so we can properly send QUIT in this case.
				conn.DirectClose()

				goto retry
			}

			rd.Log.Error("TLS error, trying plaintext", err, "remote_server", host, "domain", conn.domain)
			tlsCfg = nil
			tlsLevel = module.TLSNone
			conn.DirectClose()

			goto retry
		}
	} else {
		tlsLevel = module.TLSNone
	}

	return tlsLevel, tlsErr, nil
}

func (rd *remoteDelivery) attemptMX(ctx context.Context, conn *mxConn, record *net.MX) error {
	mxLevel := module.MXNone

	connCtx, cancel := context.WithCancel(ctx)
	// Cancel async policy lookups if rd.connect fails.
	defer cancel()

	for _, p := range rd.policies {
		policyLevel, err := p.CheckMX(connCtx, mxLevel, conn.domain, record.Host, conn.dnssecOk)
		if err != nil {
			return err
		}
		if policyLevel > mxLevel {
			mxLevel = policyLevel
		}

		p.PrepareConn(ctx, record.Host)
	}

	tlsLevel, tlsErr, err := rd.connect(connCtx, *conn, record.Host, rd.rt.tlsConfig)
	if err != nil {
		return err
	}

	// Make decision based on the policy and connection state.
	//
	// Note: All policy errors are marked as temporary to give the local admin
	// chance to troubleshoot them without losing messages.

	tlsState, _ := conn.Client().TLSConnectionState()
	for _, p := range rd.policies {
		policyLevel, err := p.CheckConn(connCtx, mxLevel, tlsLevel, conn.domain, record.Host, tlsState)
		if err != nil {
			conn.Close()
			return exterrors.WithFields(err, map[string]interface{}{"tls_err": tlsErr})
		}
		if policyLevel > tlsLevel {
			tlsLevel = policyLevel
		}
	}

	conn.mxLevel = mxLevel
	conn.tlsLevel = tlsLevel

	mxLevelCnt.WithLabelValues(rd.rt.Name(), mxLevel.String()).Inc()
	tlsLevelCnt.WithLabelValues(rd.rt.Name(), tlsLevel.String()).Inc()

	return nil
}

func (rd *remoteDelivery) connectionForDomain(ctx context.Context, domain string) (*smtpconn.C, error) {
	if c, ok := rd.connections[domain]; ok {
		return c.C, nil
	}

	pooledConn, err := rd.rt.pool.Get(ctx, domain)
	if err != nil {
		return nil, err
	}

	var conn *mxConn
	// Ignore pool for connections with REQUIRETLS to avoid "pool poisoning"
	// where attacker can make messages indeliverable by forcing reuse of old
	// connection with weaker security.
	if pooledConn != nil && !rd.msgMeta.SMTPOpts.RequireTLS {
		conn = pooledConn.(*mxConn)
		rd.Log.Msg("reusing cached connection", "domain", domain, "transactions_counter", conn.transactions)
	} else {
		rd.Log.DebugMsg("opening new connection", "domain", domain, "cache_ignored", pooledConn != nil)
		conn, err = rd.newConn(ctx, domain)
		if err != nil {
			return nil, err
		}
	}

	if rd.msgMeta.SMTPOpts.RequireTLS {
		if conn.tlsLevel < module.TLSAuthenticated {
			conn.Close()
			return nil, &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 30},
				Message:      "TLS it not available or unauthenticated but required (REQUIRETLS)",
				Misc: map[string]interface{}{
					"tls_level": conn.tlsLevel,
				},
			}
		}
		if conn.mxLevel < module.MX_MTASTS {
			conn.Close()
			return nil, &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 30},
				Message:      "Failed to establish the MX record authenticity (REQUIRETLS)",
				Misc: map[string]interface{}{
					"mx_level": conn.mxLevel,
				},
			}
		}
	}

	region := trace.StartRegion(ctx, "remote/limits.TakeDest")
	if err := rd.rt.limits.TakeDest(ctx, domain); err != nil {
		region.End()
		return nil, err
	}
	region.End()

	// Relaxed REQUIRETLS mode is not conforming to the specification strictly
	// but allows to start deploying client support for REQUIRETLS without the
	// requirement for servers in the whole world to support it. The assumption
	// behind it is that MX for the recipient domain is the final destination
	// and all other forwarders behind it already have secure connection to
	// each other. Therefore it is enough to enforce strict security only on
	// the path to the MX even if it does not support the REQUIRETLS to propagate
	// this requirement further.
	if ok, _ := conn.Client().Extension("REQUIRETLS"); rd.rt.relaxedREQUIRETLS && !ok {
		rd.msgMeta.SMTPOpts.RequireTLS = false
	}

	if err := conn.Mail(ctx, rd.mailFrom, rd.msgMeta.SMTPOpts); err != nil {
		conn.Close()
		return nil, err
	}

	rd.connections[domain] = conn
	return conn.C, nil
}

func (rd *remoteDelivery) newConn(ctx context.Context, domain string) (*mxConn, error) {
	conn := mxConn{
		reuseLimit: rd.rt.connReuseLimit,
		C:          smtpconn.New(),
		domain:     domain,
	}

	conn.Dialer = rd.rt.dialer
	conn.Log = rd.Log
	conn.Hostname = rd.rt.hostname
	conn.AddrInSMTPMsg = true
	if rd.rt.connectTimeout != 0 {
		conn.ConnectTimeout = rd.rt.connectTimeout
	}
	if rd.rt.commandTimeout != 0 {
		conn.CommandTimeout = rd.rt.commandTimeout
	}
	if rd.rt.submissionTimeout != 0 {
		conn.SubmissionTimeout = rd.rt.submissionTimeout
	}

	for _, p := range rd.policies {
		p.PrepareDomain(ctx, domain)
	}

	region := trace.StartRegion(ctx, "remote/LookupMX")
	dnssecOk, records, err := rd.lookupMX(ctx, domain)
	region.End()
	if err != nil {
		return nil, err
	}
	conn.dnssecOk = dnssecOk

	var lastErr error
	region = trace.StartRegion(ctx, "remote/Connect+TLS")
	for _, record := range records {
		if record.Host == "." {
			return nil, &exterrors.SMTPError{
				Code:         556,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 10},
				Message:      "Domain does not accept email (null MX)",
			}
		}

		if err := rd.attemptMX(ctx, &conn, record); err != nil {
			if len(records) != 0 {
				rd.Log.Error("cannot use MX", err, "remote_server", record.Host, "domain", domain)
			}
			lastErr = err
			continue
		}
		break
	}
	region.End()

	// Stil not connected? Bail out.
	if conn.Client() == nil {
		return nil, &exterrors.SMTPError{
			Code:         exterrors.SMTPCode(lastErr, 451, 550),
			EnhancedCode: exterrors.SMTPEnchCode(lastErr, exterrors.EnhancedCode{0, 4, 0}),
			Message:      "No usable MXs, last err: " + lastErr.Error(),
			TargetName:   "remote",
			Err:          lastErr,
			Misc: map[string]interface{}{
				"domain": domain,
			},
		}
	}

	return &conn, nil
}

func (rd *remoteDelivery) lookupMX(ctx context.Context, domain string) (dnssecOk bool, records []*net.MX, err error) {
	if rd.rt.extResolver != nil {
		dnssecOk, records, err = rd.rt.extResolver.AuthLookupMX(context.Background(), domain)
	} else {
		records, err = rd.rt.resolver.LookupMX(ctx, dns.FQDN(domain))
	}
	if err != nil {
		reason, misc := exterrors.UnwrapDNSErr(err)
		return false, nil, &exterrors.SMTPError{
			Code:         exterrors.SMTPCode(err, 451, 554),
			EnhancedCode: exterrors.SMTPEnchCode(err, exterrors.EnhancedCode{0, 4, 4}),
			Message:      "MX lookup error",
			TargetName:   "remote",
			Reason:       reason,
			Err:          err,
			Misc:         misc,
		}
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Pref < records[j].Pref
	})

	// Fallback to A/AAA RR when no MX records are present as
	// required by RFC 5321 Section 5.1.
	if len(records) == 0 {
		records = append(records, &net.MX{
			Host: domain,
			Pref: 0,
		})
	}

	return dnssecOk, records, err
}
