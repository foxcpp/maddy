package imap_filter

import (
	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

// Group wraps multiple modifiers and runs them serially.
//
// It is also registered as a module under 'modifiers' name and acts as a
// module group.
type Group struct {
	instName string
	Filters  []module.IMAPFilter
	log      log.Logger
}

func NewGroup(_, instName string, _, _ []string) (module.Module, error) {
	return &Group{
		instName: instName,
		log:      log.Logger{Name: "imap_filters", Debug: log.DefaultLogger.Debug},
	}, nil
}

func (g *Group) IMAPFilter(accountName string, meta *module.MsgMetadata, hdr textproto.Header, body buffer.Buffer) (folder string, flags []string, err error) {
	var (
		finalFolder string
		finalFlags  []string
	)
	for _, f := range g.Filters {
		folder, flags, err := f.IMAPFilter(accountName, meta, hdr, body)
		if err != nil {
			g.log.Error("IMAP filter failed", err)
			continue
		}
		if folder != "" && finalFolder == "" {
			finalFolder = folder
		}
		finalFlags = append(finalFlags, flags...)
	}
	return finalFolder, finalFlags, nil
}

func (g *Group) Init(cfg *config.Map) error {
	for _, node := range cfg.Block.Children {
		mod, err := modconfig.IMAPFilter(cfg.Globals, append([]string{node.Name}, node.Args...), node)
		if err != nil {
			return err
		}

		g.Filters = append(g.Filters, mod)
	}

	return nil
}

func (g *Group) Name() string {
	return "modifiers"
}

func (g *Group) InstanceName() string {
	return g.instName
}

func init() {
	module.Register("imap_filters", NewGroup)
}
