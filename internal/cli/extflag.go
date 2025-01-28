package maddycli

import (
	"flag"

	"github.com/urfave/cli/v2"
)

// extFlag implements cli.Flag via standard flag.Flag.
type extFlag struct {
	f *flag.Flag
}

func (e *extFlag) Apply(fs *flag.FlagSet) error {
	fs.Var(e.f.Value, e.f.Name, e.f.Usage)
	return nil
}

func (e *extFlag) Names() []string {
	return []string{e.f.Name}
}

func (e *extFlag) IsSet() bool {
	return false
}

func (e *extFlag) String() string {
	return cli.FlagStringer(e)
}

func (e *extFlag) IsVisible() bool {
	return true
}

func (e *extFlag) TakesValue() bool {
	return false
}

func (e *extFlag) GetUsage() string {
	return e.f.Usage
}

func (e *extFlag) GetValue() string {
	return e.f.Value.String()
}

func (e *extFlag) GetDefaultText() string {
	return e.f.DefValue
}

func (e *extFlag) GetEnvVars() []string {
	return nil
}

func mapStdlibFlags(app *cli.App) {
	// Modified AllowExtFlags from cli lib with -test.* exception removed.
	flag.VisitAll(func(f *flag.Flag) {
		app.Flags = append(app.Flags, &extFlag{f})
	})
}
