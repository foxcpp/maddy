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

package command

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime/trace"
	"strconv"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/target"
)

const modName = "check.command"

type Stage string

const (
	StageConnection = "conn"
	StageSender     = "sender"
	StageRcpt       = "rcpt"
	StageBody       = "body"
)

var placeholderRe = regexp.MustCompile(`{[a-zA-Z0-9_]+?}`)

type Check struct {
	instName string
	log      log.Logger

	stage   Stage
	actions map[int]modconfig.FailAction
	cmd     string
	cmdArgs []string
}

func New(modName, instName string, aliases, inlineArgs []string) (module.Module, error) {
	c := &Check{
		instName: instName,
		actions: map[int]modconfig.FailAction{
			1: {
				Reject: true,
			},
			2: {
				Quarantine: true,
			},
		},
	}

	if len(inlineArgs) == 0 {
		return nil, errors.New("command: at least one argument is required (command name)")
	}

	c.cmd = inlineArgs[0]
	c.cmdArgs = inlineArgs[1:]

	return c, nil
}

func (c *Check) Name() string {
	return modName
}

func (c *Check) InstanceName() string {
	return c.instName
}

func (c *Check) Init(cfg *config.Map) error {
	// Check whether the inline argument command is usable.
	if _, err := exec.LookPath(c.cmd); err != nil {
		return fmt.Errorf("command: %w", err)
	}

	cfg.Enum("run_on", false, false,
		[]string{StageConnection, StageSender, StageRcpt, StageBody}, StageBody,
		(*string)(&c.stage))

	cfg.AllowUnknown()
	unknown, err := cfg.Process()
	if err != nil {
		return err
	}

	for _, node := range unknown {
		switch node.Name {
		case "code":
			if len(node.Args) < 2 {
				return config.NodeErr(node, "at least two arguments are required: <code> <action>")
			}
			exitCode, err := strconv.Atoi(node.Args[0])
			if err != nil {
				return config.NodeErr(node, "%v", err)
			}
			action, err := modconfig.ParseActionDirective(node.Args[1:])
			if err != nil {
				return config.NodeErr(node, "%v", err)
			}

			c.actions[exitCode] = action
		default:
			return config.NodeErr(node, "unexpected directive: %v", node.Name)
		}
	}

	return nil
}

type state struct {
	c       *Check
	msgMeta *module.MsgMetadata
	log     log.Logger

	mailFrom string
	rcpts    []string
}

func (c *Check) CheckStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return &state{
		c:       c,
		msgMeta: msgMeta,
		log:     target.DeliveryLogger(c.log, msgMeta),
	}, nil
}

func (s *state) expandCommand(address string) (string, []string) {
	expArgs := make([]string, len(s.c.cmdArgs))

	for i, arg := range s.c.cmdArgs {
		expArgs[i] = placeholderRe.ReplaceAllStringFunc(arg, func(placeholder string) string {
			switch placeholder {
			case "{auth_user}":
				if s.msgMeta.Conn == nil {
					return ""
				}
				return s.msgMeta.Conn.AuthUser
			case "{source_ip}":
				if s.msgMeta.Conn == nil {
					return ""
				}
				tcpAddr, _ := s.msgMeta.Conn.RemoteAddr.(*net.TCPAddr)
				if tcpAddr == nil {
					return ""
				}
				return tcpAddr.IP.String()
			case "{source_host}":
				if s.msgMeta.Conn == nil {
					return ""
				}
				return s.msgMeta.Conn.Hostname
			case "{source_rdns}":
				if s.msgMeta.Conn == nil {
					return ""
				}
				valI, err := s.msgMeta.Conn.RDNSName.Get()
				if err != nil {
					return ""
				}
				if valI == nil {
					return ""
				}
				return valI.(string)
			case "{msg_id}":
				return s.msgMeta.ID
			case "{sender}":
				return s.mailFrom
			case "{rcpts}":
				return strings.Join(s.rcpts, "\n")
			case "{address}":
				return address
			}
			return placeholder
		})
	}

	return s.c.cmd, expArgs
}

func (s *state) run(cmdName string, args []string, stdin io.Reader) module.CheckResult {
	cmd := exec.Command(cmdName, args...)
	cmd.Stdin = stdin
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:      450,
				Message:   "Internal server error",
				CheckName: "command",
				Err:       err,
				Misc: map[string]interface{}{
					"cmd": cmd.String(),
				},
			},
			Reject: true,
		}
	}

	if err := cmd.Start(); err != nil {
		return module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:      450,
				Message:   "Internal server error",
				CheckName: "command",
				Err:       err,
				Misc: map[string]interface{}{
					"cmd": cmd.String(),
				},
			},
			Reject: true,
		}
	}

	bufOut := bufio.NewReader(stdout)
	hdr, err := textproto.ReadHeader(bufOut)
	if err != nil && !errors.Is(err, io.EOF) {
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			s.log.Error("failed to kill process", err)
		}

		return module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:      450,
				Message:   "Internal server error",
				CheckName: "command",
				Err:       err,
				Misc: map[string]interface{}{
					"cmd": cmd.String(),
				},
			},
			Reject: true,
		}
	}

	res := module.CheckResult{}
	res.Header = hdr

	err = cmd.Wait()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			// If that's not ExitError, the process may still be running. We do
			// not want this.
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				s.log.Error("failed to kill process", err)
			}
		}
		return s.errorRes(err, res, cmd.String())
	}
	return res
}

func (s *state) errorRes(err error, res module.CheckResult, cmdLine string) module.CheckResult {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		res.Reason = &exterrors.SMTPError{
			Code:      450,
			Message:   "Internal server error",
			CheckName: "command",
			Err:       err,
			Misc: map[string]interface{}{
				"cmd": cmdLine,
			},
		}
		res.Reject = true
		return res
	}

	action, ok := s.c.actions[exitErr.ExitCode()]
	if !ok {
		res.Reason = &exterrors.SMTPError{
			Code:      450,
			Message:   "Internal server error",
			CheckName: "command",
			Err:       err,
			Reason:    "unexpected exit code",
			Misc: map[string]interface{}{
				"cmd":       cmdLine,
				"exit_code": exitErr.ExitCode(),
			},
		}
		res.Reject = true
		return res
	}

	res.Reason = &exterrors.SMTPError{
		Code:         550,
		EnhancedCode: exterrors.EnhancedCode{5, 7, 1},
		Message:      "Message rejected for due to a local policy",
		CheckName:    "command",
		Misc: map[string]interface{}{
			"cmd":       cmdLine,
			"exit_code": exitErr.ExitCode(),
		},
	}

	return action.Apply(res)
}

func (s *state) CheckConnection(ctx context.Context) module.CheckResult {
	if s.c.stage != StageConnection {
		return module.CheckResult{}
	}

	defer trace.StartRegion(ctx, "command/CheckConnection-"+s.c.cmd).End()

	cmdName, cmdArgs := s.expandCommand("")
	return s.run(cmdName, cmdArgs, bytes.NewReader(nil))
}

func (s *state) CheckSender(ctx context.Context, addr string) module.CheckResult {
	s.mailFrom = addr

	if s.c.stage != StageSender {
		return module.CheckResult{}
	}

	defer trace.StartRegion(ctx, "command/CheckSender"+s.c.cmd).End()

	cmdName, cmdArgs := s.expandCommand(addr)
	return s.run(cmdName, cmdArgs, bytes.NewReader(nil))
}

func (s *state) CheckRcpt(ctx context.Context, addr string) module.CheckResult {
	s.rcpts = append(s.rcpts, addr)

	if s.c.stage != StageRcpt {
		return module.CheckResult{}
	}
	defer trace.StartRegion(ctx, "command/CheckRcpt"+s.c.cmd).End()

	cmdName, cmdArgs := s.expandCommand(addr)
	return s.run(cmdName, cmdArgs, bytes.NewReader(nil))
}

func (s *state) CheckBody(ctx context.Context, hdr textproto.Header, body buffer.Buffer) module.CheckResult {
	if s.c.stage != StageBody {
		return module.CheckResult{}
	}

	defer trace.StartRegion(ctx, "command/CheckBody"+s.c.cmd).End()

	cmdName, cmdArgs := s.expandCommand("")

	var buf bytes.Buffer
	_ = textproto.WriteHeader(&buf, hdr)
	bR, err := body.Open()
	if err != nil {
		return module.CheckResult{
			Reason: &exterrors.SMTPError{
				Code:      450,
				Message:   "Internal server error",
				CheckName: "command",
				Err:       err,
				Misc: map[string]interface{}{
					"cmd": cmdName + " " + strings.Join(cmdArgs, " "),
				},
			},
			Reject: true,
		}
	}

	return s.run(cmdName, cmdArgs, io.MultiReader(bytes.NewReader(buf.Bytes()), bR))
}

func (s *state) Close() error {
	return nil
}

func init() {
	module.Register(modName, New)
}
