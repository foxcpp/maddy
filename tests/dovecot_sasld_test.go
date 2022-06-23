//go:build integration
// +build integration

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
	"errors"
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/foxcpp/maddy/tests"
)

var ChasquidExecutable string

func init() {
	flag.StringVar(&ChasquidExecutable, "integration.chasquid", "chasquid", "path to chasquid executable for interop tests")
}

const chasquidConf = `smtp_address: "127.0.0.2:44444"
submission_address: "127.0.0.1:44443"

data_dir: "$ROOT"
mail_log_path: "/dev/null"

dovecot_auth: true
dovecot_userdb_path: "$AUTH_CLIENT" # needs any Unix socket, not actually used
dovecot_client_path: "$AUTH_CLIENT"
`

// RSA 1024, valid for *.example.invalid, 127.0.0.1, 127.0.0.2,, 127.0.0.3
// until Nov 18 17:13:45 2029 GMT.
const testServerCert = `-----BEGIN CERTIFICATE-----
MIICDzCCAXigAwIBAgIRAJ1x+qCW7L+Hs6sRU8BHmWkwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xOTExMTgxNzEzNDVaFw0yOTExMTUxNzEz
NDVaMBIxEDAOBgNVBAoTB0FjbWUgQ28wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJ
AoGBAPINKMyuu3AvzndLDS2/BroA+DRUcAhWPBxMxG1b1BkkHisAZWteKajKmwdO
O13N8HHBRPPOD56AAPLZGNxYLHn6nel7AiH8k40/xC5tDOthqA82+00fwJHDFCnW
oDLOLcO17HulPvfCSWfefc+uee4kajPa+47hutqZH2bGMTXhAgMBAAGjZTBjMA4G
A1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAA
MC4GA1UdEQQnMCWCESouZXhhbXBsZS5pbnZhbGlkhwR/AAABhwR/AAAChwR/AAAD
MA0GCSqGSIb3DQEBCwUAA4GBAGRn3C2NbwR4cyQmTRm5jcaqi1kAYyEu6U8Q9PJW
Q15BXMKUTx2lw//QScK9MH2JpKxDuzWDSvaxZMnTxgri2uiplqpe8ydsWj6Wl0q9
2XMGJ9LIxTZk5+cyZP2uOolvmSP/q8VFTyk9Udl6KUZPQyoiiDq4rBFUIxUyb+bX
pHkR
-----END CERTIFICATE-----`

const testServerKey = `-----BEGIN PRIVATE KEY-----
MIICeAIBADANBgkqhkiG9w0BAQEFAASCAmIwggJeAgEAAoGBAPINKMyuu3AvzndL
DS2/BroA+DRUcAhWPBxMxG1b1BkkHisAZWteKajKmwdOO13N8HHBRPPOD56AAPLZ
GNxYLHn6nel7AiH8k40/xC5tDOthqA82+00fwJHDFCnWoDLOLcO17HulPvfCSWfe
fc+uee4kajPa+47hutqZH2bGMTXhAgMBAAECgYEAgPjSDH3uEdDnSlkLJJzskJ+D
oR58s3R/gvTElSCg2uSLzo3ffF4oBHAwOqxMpabdvz8j5mSdne7Gkp9qx72TtEG2
wt6uX1tZhm2UTAkInH8IQDthj98P8vAWQsS6HHEIMErsrW2CyUrAt/+o1BRg/hWW
zixA3CLTthhZTJkaUCECQQD5EM16UcTAKfhr3IZppgq+ZsAOMkeCl3XVV9gHo32i
DL6UFAb27BAYyjfcZB1fPou4RszX0Ryu9yU0P5qm6N47AkEA+MpdAPkaPziY0ok4
e9Tcee6P0mIR+/AHk9GliVX2P74DDoOHyMXOSRBwdb+z2tYjrdjkNEL1Txe+sHny
k/EukwJBAOBqlmqPwNNRPeiaRHZvSSD0XjqsbSirJl48D4gadPoNt66fOQNGAt8D
Xj/z6U9HgQdiq/IOFmVEhT5FzSh1jL8CQQD3Myth8iGQO84tM0c6U3CWfuHMqsEv
0XnV+HNAmHdLMqOa4joi1dh4ZKs5dDdi828UJ/PnsbhI1FEWzLSpJvWdAkAkVWqf
AC/TvWvEZLA6Z5CllyNzZJ7XvtIaNOosxHDolyZ1HMWMlfEb2K2ZXWLy5foKPeoY
Xi3olS9rB0J+Rvjz
-----END PRIVATE KEY-----`

func runChasquid(t *testing.T, authClientPath string) (string, *exec.Cmd) {
	tempDir, err := ioutil.TempDir("", "maddy-chasquid-interop-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Using", tempDir)

	chasquidConf := strings.NewReplacer(
		"$ROOT", tempDir,
		"$AUTH_CLIENT", authClientPath).Replace(chasquidConf)
	err = ioutil.WriteFile(filepath.Join(tempDir, "chasquid.conf"), []byte(chasquidConf), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "certs", "example.org"), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(tempDir, "certs", "example.org", "fullchain.pem"), []byte(testServerCert), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(tempDir, "certs", "example.org", "privkey.pem"), []byte(testServerKey), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "domains", "example.org"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	err = ioutil.WriteFile(filepath.Join(tempDir, "chasquid.conf"), []byte(chasquidConf), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(ChasquidExecutable, "-v=2", "-config_dir", tempDir)
	t.Log("Launching", cmd.String())
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	ready := make(chan struct{}, 1)

	go func() {
		scnr := bufio.NewScanner(stderr)
		for scnr.Scan() {
			line := scnr.Text()

			// One of messages printed near completing initialization.
			if strings.Contains(line, "Loading certificates") {
				time.Sleep(1 * time.Second)
				ready <- struct{}{}
			}

			t.Log("chasquid:", line)
		}
		if err := scnr.Err(); err != nil {
			t.Log("stderr I/O error:", err)
		}
	}()

	<-ready

	return tempDir, cmd
}

func cleanChasquid(t *testing.T, tempDir string, cmd *exec.Cmd) {
	cmd.Process.Signal(syscall.SIGTERM)
	os.RemoveAll(tempDir)
}

func TestSASLServerWithChasquid(tt *testing.T) {
	tt.Parallel()

	_, err := exec.LookPath(ChasquidExecutable)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			tt.Skip("No chasquid executable found, skipping interop. tests")
		}
		tt.Fatal(err)
	}

	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		dovecot_sasld unix://{env:TEST_STATE_DIR}/auth.sock {
			auth pass_table static {
				# tester@example.org:123456
				entry tester@example.org "bcrypt:$2a$04$0SaXE/WOMBOfk5jyaKjo.OHkioRljdhMznLnYCg1nrksu9iLd51Ri"
			}
		}`)
	t.Run(1)
	defer t.Close()

	chasquidDir, cmd := runChasquid(tt, filepath.Join(t.StateDir(), "auth.sock"))
	defer cleanChasquid(tt, chasquidDir, cmd)

	c := t.ConnUnnamed(44443)
	defer c.Close()
	c.SMTPNegotation("localhost", nil, nil)
	c.Writeln("STARTTLS")
	c.ExpectPattern("220 *")
	c.TLS()
	c.Writeln("AUTH PLAIN AHRlc3RAZXhhbXBsZS5vcmcAMTIzNDU2") // 0x00 test@example.org 0x00 123456 (invalid user)
	c.ExpectPattern("535 *")
	c.Writeln("AUTH PLAIN AHRlc3RlckBleGFtcGxlLm9yZwAxMjM0NQ==") // 0x00 tester 0x00 12345 (invalid password)
	c.ExpectPattern("535 *")
	c.Writeln("AUTH PLAIN AHRlc3RlckBleGFtcGxlLm9yZwAxMjM0NTY=") // 0x00 tester 0x00 123456
	c.ExpectPattern("235 *")
}
