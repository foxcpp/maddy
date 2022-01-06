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

package maddy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
)

/*
Config matchers for module interfaces.
*/

// logOut structure wraps log.Output and preserves
// configuration directive it was constructed from, allowing
// dynamic reinitialization for purposes of log file rotation.
type logOut struct {
	args []string
	log.Output
}

func logOutput(_ *config.Map, node config.Node) (interface{}, error) {
	if len(node.Args) == 0 {
		return nil, config.NodeErr(node, "expected at least 1 argument")
	}
	if len(node.Children) != 0 {
		return nil, config.NodeErr(node, "can't declare block here")
	}

	return LogOutputOption(node.Args)
}

func LogOutputOption(args []string) (log.Output, error) {
	outs := make([]log.Output, 0, len(args))
	for i, arg := range args {
		switch arg {
		case "stderr":
			outs = append(outs, log.WriterOutput(os.Stderr, false))
		case "stderr_ts":
			outs = append(outs, log.WriterOutput(os.Stderr, true))
		case "syslog":
			syslogOut, err := log.SyslogOutput()
			if err != nil {
				return nil, fmt.Errorf("failed to connect to syslog daemon: %v", err)
			}
			outs = append(outs, syslogOut)
		case "off":
			if len(args) != 1 {
				return nil, errors.New("'off' can't be combined with other log targets")
			}
			return log.NopOutput{}, nil
		default:
			// Log file paths are converted to absolute to make sure
			// we will be able to recreate them in right location
			// after changing working directory to the state dir.
			absPath, err := filepath.Abs(arg)
			if err != nil {
				return nil, err
			}
			// We change the actual argument, so logOut object will
			// keep the absolute path for reinitialization.
			args[i] = absPath

			w, err := os.OpenFile(absPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666)
			if err != nil {
				return nil, fmt.Errorf("failed to create log file: %v", err)
			}

			outs = append(outs, log.WriteCloserOutput(w, true))
		}
	}

	if len(outs) == 1 {
		return logOut{args, outs[0]}, nil
	}
	return logOut{args, log.MultiOutput(outs...)}, nil
}

func defaultLogOutput() (interface{}, error) {
	return log.DefaultLogger.Out, nil
}

func reinitLogging() {
	out, ok := log.DefaultLogger.Out.(logOut)
	if !ok {
		log.Println("Can't reinitialize logger because it was replaced before, this is a bug")
		return
	}

	newOut, err := LogOutputOption(out.args)
	if err != nil {
		log.Println("Can't reinitialize logger:", err)
		return
	}

	out.Close()

	log.DefaultLogger.Out = newOut
}
