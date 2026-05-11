//go:build integration

/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2026 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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
	"crypto/tls"
	"crypto/x509"
	"os"
	"strconv"
	"testing"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/tests"
	"github.com/stretchr/testify/require"
)

func TestSMTPSASLExternalTLS(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)

	smtpPort := t.Port("smtp")
	t.DNS(nil)
	t.Config(`
		smtp tls://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls {
				loader file {env:TEST_PWD}/testdata/tls/server.crt {env:TEST_PWD}/testdata/tls/server.key
				client_ca {env:TEST_PWD}/testdata/tls/ca.crt
				client_auth verify_if_given
			}
			auth tls
			auth dummy

			# Use authorize_sender to quickly check effective auth username via MAIL FROM.
			defer_sender_reject
			check {
				authorize_sender
			}

			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	caPool := x509.NewCertPool()
	caPEM, err := os.ReadFile("testdata/tls/ca.crt")
	require.NoError(t, err)
	caPool.AppendCertsFromPEM(caPEM)

	loadCert := func(t *tests.T, crtPath, keyPath string) tls.Certificate {
		crt, err := tls.LoadX509KeyPair(crtPath, keyPath)
		require.NoError(t, err)
		return crt
	}

	t.Subtest("no client cert", func(t *tests.T) {
		// No client certificate provided - SASL EXTERNAL will fail.
		smtpConn, err := smtp.DialTLS("127.0.0.1:"+strconv.Itoa(int(smtpPort)), &tls.Config{
			ServerName: "mx.maddy.test",
			RootCAs:    caPool,
		})
		require.NoError(t, err)
		defer smtpConn.Close()

		require.True(t, smtpConn.SupportsAuth(sasl.External))
		require.Error(t, smtpConn.Auth(sasl.NewExternalClient("")))
	})

	t.Subtest("wrong cert usage", func(t *tests.T) {
		smtpConn, err := smtp.DialTLS("127.0.0.1:"+strconv.Itoa(int(smtpPort)), &tls.Config{
			ServerName: "mx.maddy.test",
			RootCAs:    caPool,
			Certificates: []tls.Certificate{
				loadCert(
					t,
					"testdata/tls/client_san_email_no_usage.crt",
					"testdata/tls/client_san_email_no_usage.key",
				),
			},
		})
		require.NoError(t, err)
		defer smtpConn.Close()

		require.True(t, smtpConn.SupportsAuth(sasl.External))
		require.Error(t, smtpConn.Auth(sasl.NewExternalClient("")))
	})

	t.Subtest("CN cert", func(t *tests.T) {
		smtpConn, err := smtp.DialTLS("127.0.0.1:"+strconv.Itoa(int(smtpPort)), &tls.Config{
			ServerName: "mx.maddy.test",
			RootCAs:    caPool,
			Certificates: []tls.Certificate{
				loadCert(
					t,
					"testdata/tls/client_cn.crt",
					"testdata/tls/client_cn.key",
				),
			},
		})
		require.NoError(t, err)
		defer smtpConn.Close()

		require.True(t, smtpConn.SupportsAuth(sasl.External))
		require.NoError(t, smtpConn.Auth(sasl.NewExternalClient("")))

		require.NoError(t, smtpConn.Mail("cn@maddy.test", nil))
	})

	t.Subtest("SAN email cert", func(t *tests.T) {
		smtpConn, err := smtp.DialTLS("127.0.0.1:"+strconv.Itoa(int(smtpPort)), &tls.Config{
			ServerName: "mx.maddy.test",
			RootCAs:    caPool,
			Certificates: []tls.Certificate{
				loadCert(
					t,
					"testdata/tls/client_san_email.crt",
					"testdata/tls/client_san_email.key",
				),
			},
		})
		require.NoError(t, err)
		defer smtpConn.Close()

		require.True(t, smtpConn.SupportsAuth(sasl.External))
		require.NoError(t, smtpConn.Auth(sasl.NewExternalClient("")))

		require.NoError(t, smtpConn.Mail("san@maddy.test", nil))
	})

	t.Subtest("SAN email cert - multiple default", func(t *tests.T) {
		smtpConn, err := smtp.DialTLS("127.0.0.1:"+strconv.Itoa(int(smtpPort)), &tls.Config{
			ServerName: "mx.maddy.test",
			RootCAs:    caPool,
			Certificates: []tls.Certificate{
				loadCert(
					t,
					"testdata/tls/client_san_email_multi.crt",
					"testdata/tls/client_san_email_multi.key",
				),
			},
		})
		require.NoError(t, err)
		defer smtpConn.Close()

		require.True(t, smtpConn.SupportsAuth(sasl.External))
		require.NoError(t, smtpConn.Auth(sasl.NewExternalClient("")))

		require.NoError(t, smtpConn.Mail("san1@maddy.test", nil))
	})

	t.Subtest("SAN email cert - multiple 1", func(t *tests.T) {
		smtpConn, err := smtp.DialTLS("127.0.0.1:"+strconv.Itoa(int(smtpPort)), &tls.Config{
			ServerName: "mx.maddy.test",
			RootCAs:    caPool,
			Certificates: []tls.Certificate{
				loadCert(
					t,
					"testdata/tls/client_san_email_multi.crt",
					"testdata/tls/client_san_email_multi.key",
				),
			},
		})
		require.NoError(t, err)
		defer smtpConn.Close()

		require.True(t, smtpConn.SupportsAuth(sasl.External))
		require.NoError(t, smtpConn.Auth(sasl.NewExternalClient("san1@maddy.test")))

		require.NoError(t, smtpConn.Mail("san1@maddy.test", nil))
	})

	t.Subtest("SAN email cert - multiple 2", func(t *tests.T) {
		smtpConn, err := smtp.DialTLS("127.0.0.1:"+strconv.Itoa(int(smtpPort)), &tls.Config{
			ServerName: "mx.maddy.test",
			RootCAs:    caPool,
			Certificates: []tls.Certificate{
				loadCert(
					t,
					"testdata/tls/client_san_email_multi.crt",
					"testdata/tls/client_san_email_multi.key",
				),
			},
		})
		require.NoError(t, err)
		defer smtpConn.Close()

		require.True(t, smtpConn.SupportsAuth(sasl.External))
		require.NoError(t, smtpConn.Auth(sasl.NewExternalClient("san2@maddy.test")))

		require.NoError(t, smtpConn.Mail("san2@maddy.test", nil))
	})
}
