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

package rspamd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/target"
)

const modName = "check.rspamd"

type Check struct {
	instName string
	log      log.Logger

	apiPath    string
	flags      string
	settingsID string
	tag        string
	mtaName    string

	ioErrAction       modconfig.FailAction
	errorRespAction   modconfig.FailAction
	addHdrAction      modconfig.FailAction
	rewriteSubjAction modconfig.FailAction

	client *http.Client
}

func New(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	c := &Check{
		instName: instName,
		client:   http.DefaultClient,
		log:      log.Logger{Name: modName, Debug: log.DefaultLogger.Debug},
	}

	switch len(inlineArgs) {
	case 1:
		c.apiPath = inlineArgs[0]
	case 0:
		c.apiPath = "http://127.0.0.1:11333"
	default:
		return nil, fmt.Errorf("%s: unexpected amount of inline arguments", modName)
	}

	return c, nil
}

func (c *Check) Name() string {
	return modName
}

func (c *Check) InstanceName() string {
	return c.instName
}

func (c *Check) Init(cfg *config.Map) error {
	var (
		tlsConfig tls.Config
		flags     []string
	)

	cfg.Custom("tls_client", true, false, func() (interface{}, error) {
		return tls.Config{}, nil
	}, tls2.TLSClientBlock, &tlsConfig)
	cfg.String("api_path", false, false, c.apiPath, &c.apiPath)
	cfg.String("settings_id", false, false, "", &c.settingsID)
	cfg.String("tag", false, false, "maddy", &c.tag)
	cfg.String("hostname", true, false, "", &c.mtaName)
	cfg.Custom("io_error_action", false, false,
		func() (interface{}, error) {
			return modconfig.FailAction{}, nil
		}, modconfig.FailActionDirective, &c.ioErrAction)
	cfg.Custom("error_resp_action", false, false,
		func() (interface{}, error) {
			return modconfig.FailAction{}, nil
		}, modconfig.FailActionDirective, &c.errorRespAction)
	cfg.Custom("add_header_action", false, false,
		func() (interface{}, error) {
			return modconfig.FailAction{Quarantine: true}, nil
		}, modconfig.FailActionDirective, &c.addHdrAction)
	cfg.Custom("rewrite_subj_action", false, false,
		func() (interface{}, error) {
			return modconfig.FailAction{Quarantine: true}, nil
		}, modconfig.FailActionDirective, &c.rewriteSubjAction)
	cfg.StringList("flags", false, false, []string{"pass_all"}, &flags)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	c.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tlsConfig,
		},
	}
	c.flags = strings.Join(flags, ",")

	return nil
}

type state struct {
	c       *Check
	msgMeta *module.MsgMetadata
	log     log.Logger

	mailFrom string
	rcpt     []string
}

func (c *Check) CheckStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return &state{
		c:       c,
		msgMeta: msgMeta,
		log:     target.DeliveryLogger(c.log, msgMeta),
	}, nil
}

func (s *state) CheckConnection(ctx context.Context) module.CheckResult {
	return module.CheckResult{}
}

func (s *state) CheckSender(ctx context.Context, addr string) module.CheckResult {
	s.mailFrom = addr
	return module.CheckResult{}
}

func (s *state) CheckRcpt(ctx context.Context, addr string) module.CheckResult {
	s.rcpt = append(s.rcpt, addr)
	return module.CheckResult{}
}

func addConnHeaders(r *http.Request, meta *module.MsgMetadata, mailFrom string, rcpts []string) {
	r.Header.Add("From", mailFrom)
	for _, rcpt := range rcpts {
		r.Header.Add("Rcpt", rcpt)
	}

	r.Header.Add("Queue-ID", meta.ID)

	conn := meta.Conn
	if conn != nil {
		if meta.Conn.AuthUser != "" {
			r.Header.Add("User", meta.Conn.AuthUser)
		}

		if tcpAddr, ok := conn.RemoteAddr.(*net.TCPAddr); ok {
			r.Header.Add("IP", tcpAddr.IP.String())
		}
		r.Header.Add("Helo", conn.Hostname)
		name, err := conn.RDNSName.Get()
		if err == nil && name != nil {
			r.Header.Add("Hostname", name.(string))
		}

		if conn.TLS.HandshakeComplete {
			r.Header.Add("TLS-Cipher", tls.CipherSuiteName(conn.TLS.CipherSuite))
			switch conn.TLS.Version {
			case tls.VersionTLS13:
				r.Header.Add("TLS-Version", "1.3")
			case tls.VersionTLS12:
				r.Header.Add("TLS-Version", "1.2")
			case tls.VersionTLS11:
				r.Header.Add("TLS-Version", "1.1")
			case tls.VersionTLS10:
				r.Header.Add("TLS-Version", "1.0")
			}
		}
	}
}

func (s *state) CheckBody(ctx context.Context, hdr textproto.Header, body buffer.Buffer) module.CheckResult {
	bodyR, err := body.Open()
	if err != nil {
		return module.CheckResult{
			Reject: true,
			Reason: exterrors.WithFields(err, map[string]interface{}{"check": modName}),
		}
	}

	var buf bytes.Buffer
	if err := textproto.WriteHeader(&buf, hdr); err != nil {
		return module.CheckResult{
			Reject: true,
			Reason: exterrors.WithFields(err, map[string]interface{}{"check": modName}),
		}
	}

	r, err := http.NewRequest("POST", s.c.apiPath+"/checkv2", io.MultiReader(&buf, bodyR))
	if err != nil {
		return module.CheckResult{
			Reject: true,
			Reason: exterrors.WithFields(err, map[string]interface{}{"check": modName}),
		}
	}

	r.Header.Add("Pass", "all") // TODO: does that need to be configurable?
	// TODO: include version (needs maddy.Version moved somewhere to break circular dependency)
	r.Header.Add("User-Agent", "maddy")
	if s.c.tag != "" {
		r.Header.Add("MTA-Tag", s.c.tag)
	}
	if s.c.settingsID != "" {
		r.Header.Add("Settings-ID", s.c.settingsID)
	}
	if s.c.mtaName != "" {
		r.Header.Add("MTA-Name", s.c.mtaName)
	}

	addConnHeaders(r, s.msgMeta, s.mailFrom, s.rcpt)
	r.Header.Add("Content-Length", strconv.Itoa(body.Len()))

	resp, err := s.c.client.Do(r)
	if err != nil {
		return s.c.ioErrAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         451,
				EnhancedCode: exterrors.EnhancedCode{4, 7, 0},
				Message:      "Internal error during policy check",
				CheckName:    modName,
				Err:          err,
			},
		})
	}
	if resp.StatusCode/100 != 2 {
		return s.c.errorRespAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         451,
				EnhancedCode: exterrors.EnhancedCode{4, 7, 0},
				Message:      "Internal error during policy check",
				CheckName:    modName,
				Err:          fmt.Errorf("HTTP %d", resp.StatusCode),
			},
		})
	}
	defer resp.Body.Close()

	var respData response
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return s.c.ioErrAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         451,
				EnhancedCode: exterrors.EnhancedCode{4, 9, 0},
				Message:      "Internal error during policy check",
				CheckName:    modName,
				Err:          err,
			},
		})
	}

	switch respData.Action {
	case "no action":
		return module.CheckResult{}
	case "greylist":
		// uuh... TODO: Implement greylisting?
		hdrAdd := textproto.Header{}
		hdrAdd.Add("X-Spam-Score", strconv.FormatFloat(respData.Score, 'f', 2, 64))
		return module.CheckResult{
			Header: hdrAdd,
		}
	case "add header":
		hdrAdd := textproto.Header{}
		hdrAdd.Add("X-Spam-Flag", "Yes")
		hdrAdd.Add("X-Spam-Score", strconv.FormatFloat(respData.Score, 'f', 2, 64))
		return s.c.addHdrAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         450,
				EnhancedCode: exterrors.EnhancedCode{4, 7, 0},
				Message:      "Message rejected due to local policy",
				CheckName:    modName,
				Misc:         map[string]interface{}{"action": "add header"},
			},
			Header: hdrAdd,
		})
	case "rewrite subject":
		hdrAdd := textproto.Header{}
		hdrAdd.Add("X-Spam-Flag", "Yes")
		hdrAdd.Add("X-Spam-Score", strconv.FormatFloat(respData.Score, 'f', 2, 64))
		return s.c.rewriteSubjAction.Apply(module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:         450,
				EnhancedCode: exterrors.EnhancedCode{4, 7, 0},
				Message:      "Message rejected due to local policy",
				CheckName:    modName,
				Misc:         map[string]interface{}{"action": "rewrite subject"},
			},
			Header: hdrAdd,
		})
	case "soft reject":
		return module.CheckResult{
			Reject: true,
			Reason: &exterrors.SMTPError{
				Code:         450,
				EnhancedCode: exterrors.EnhancedCode{4, 7, 0},
				Message:      "Message rejected due to local policy",
				CheckName:    modName,
				Misc:         map[string]interface{}{"action": "soft reject"},
			},
		}
	case "reject":
		return module.CheckResult{
			Reject: true,
			Reason: &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Message rejected due to local policy",
				CheckName:    modName,
				Misc:         map[string]interface{}{"action": "reject"},
			},
		}
	}

	s.log.Msg("unhandled action", "action", respData.Action)

	return module.CheckResult{}
}

type response struct {
	Score   float64 `json:"score"`
	Action  string  `json:"action"`
	Subject string  `json:"subject"`
	Symbols map[string]struct {
		Name  string  `json:"name"`
		Score float64 `json:"score"`
	}
}

func (s *state) Close() error {
	return nil
}

func init() {
	module.Register(modName, New)
}
