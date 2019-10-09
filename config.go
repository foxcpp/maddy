package maddy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
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

func logOutput(m *config.Map, node *config.Node) (interface{}, error) {
	if len(node.Args) == 0 {
		return nil, m.MatchErr("expected at least 1 argument")
	}
	if len(node.Children) != 0 {
		return nil, m.MatchErr("can't declare block here")
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

			w, err := os.OpenFile(absPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
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

	log.DefaultLogger.Out = newOut
}
