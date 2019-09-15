package maddy

import (
	"errors"
	"fmt"
	"os"

	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
)

/*
Config matchers for module interfaces.
*/

func logOutput(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) == 0 {
		return nil, m.MatchErr("expected at least 1 argument")
	}
	if len(node.Children) != 0 {
		return nil, m.MatchErr("can't declare block here")
	}

	return LogOutputOption(node.Args)
}

func LogOutputOption(args []string) (log.FuncLog, error) {
	outs := make([]log.FuncLog, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "stderr":
			outs = append(outs, log.WriterLog(os.Stderr, false))
		case "stderr_ts":
			outs = append(outs, log.WriterLog(os.Stderr, true))
		case "syslog":
			syslogOut, err := log.Syslog()
			if err != nil {
				return nil, fmt.Errorf("failed to connect to syslog daemon: %v", err)
			}
			outs = append(outs, syslogOut)
		case "off":
			if len(args) != 1 {
				return nil, errors.New("'off' can't be combined with other log targets")
			}
			return nil, nil
		default:
			w, err := os.OpenFile(arg, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
			if err != nil {
				return nil, fmt.Errorf("failed to create log file: %v", err)
			}

			outs = append(outs, log.WriterLog(w, true))
		}
	}

	if len(outs) == 1 {
		return outs[0], nil
	}
	return log.MultiLog(outs...), nil
}

func defaultLogOutput() (interface{}, error) {
	return log.DefaultLogger.Out, nil
}
