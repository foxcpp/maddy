package modify

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/internal/address"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/module"
)

// replaceAddr is a simple module that replaces matching sender (or recipient) address
// in messages.
//
// If created with modName = "replace_sender", it will change sender address.
// If created with modName = "replace_rcpt", it will change recipient addresses.
//
// Matching is done by either regexp or plain string. Module arguments are as
// arguments for brevity..
type replaceAddr struct {
	modName  string
	instName string

	inlineFromArg string
	inlineToArg   string
	replaceSender bool
	replaceRcpt   bool

	// Matchers. Only one is used.
	fromString string
	fromRegex  *regexp.Regexp

	// Replacement string
	to string
}

func NewReplaceAddr(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	r := replaceAddr{
		modName:       modName,
		instName:      instName,
		replaceSender: modName == "replace_sender",
		replaceRcpt:   modName == "replace_rcpt",
	}

	switch len(inlineArgs) {
	case 0:
		// Not inline definition.
	case 2:
		r.inlineFromArg = inlineArgs[0]
		r.inlineToArg = inlineArgs[1]
	default:
		return nil, fmt.Errorf("%s: invalid amount of inline arguments", modName)
	}

	return &r, nil
}

func (r *replaceAddr) Init(m *config.Map) error {
	var fromCfg, toCfg string
	m.String("from", false, false, r.inlineFromArg, &fromCfg)
	m.String("to", false, false, r.inlineToArg, &toCfg)
	if _, err := m.Process(); err != nil {
		return err
	}

	if fromCfg == "" {
		return fmt.Errorf("%s: missing 'from' argument or directive", r.modName)
	}
	if toCfg == "" {
		return fmt.Errorf("%s: missing 'to' argument or directive", r.modName)
	}

	if strings.HasPrefix(fromCfg, "/") {
		if !strings.HasSuffix(fromCfg, "/") {
			return fmt.Errorf("%s: missing trailing slash in 'from' value", r.modName)
		}

		regex := fromCfg[1 : len(fromCfg)-1]

		// Regexp should match entire string, so add anchors
		// if they are not present.
		if !strings.HasPrefix(regex, "^") {
			regex = "^" + regex
		}
		if !strings.HasSuffix(regex, "$") {
			regex = regex + "$"
		}

		regex = "(?i)" + regex

		var err error
		r.fromRegex, err = regexp.Compile(regex)
		if err != nil {
			return fmt.Errorf("%s: %v", r.modName, err)
		}
	}
	if strings.HasSuffix(fromCfg, "/") && !strings.HasPrefix(fromCfg, "/") {
		return fmt.Errorf("%s: missing leading slash in 'from' value", r.modName)
	}

	r.fromString = fromCfg

	if strings.HasPrefix(toCfg, "/") || strings.HasSuffix(toCfg, "/") {
		return fmt.Errorf("%s: can't use regexp in 'to' value", r.modName)
	}
	if r.fromRegex == nil && strings.Contains(toCfg, "$") {
		return fmt.Errorf("%s: can't reference capture groups in 'to' if 'from' is not a regexp", r.modName)
	}
	r.to = toCfg

	return nil
}

func (r replaceAddr) Name() string {
	return r.modName
}

func (r replaceAddr) InstanceName() string {
	return r.instName
}

func (r replaceAddr) ModStateForMsg(msgMeta *module.MsgMetadata) (module.ModifierState, error) {
	return r, nil
}

func (r replaceAddr) RewriteSender(mailFrom string) (string, error) {
	if r.replaceSender {
		return r.rewrite(mailFrom)
	}
	return mailFrom, nil
}

func (r replaceAddr) RewriteRcpt(rcptTo string) (string, error) {
	if r.replaceRcpt {
		return r.rewrite(rcptTo)
	}
	return rcptTo, nil
}

func (r replaceAddr) RewriteBody(h *textproto.Header, body buffer.Buffer) error {
	return nil
}

func (r replaceAddr) Close() error {
	return nil
}

func (r replaceAddr) rewrite(val string) (string, error) {
	if r.fromRegex == nil {
		if address.Equal(r.fromString, val) {
			return r.to, nil
		}
		return val, nil
	}

	normVal, err := address.ForLookup(val)
	if err != nil {
		// Ouch. Should not happen at this point.
		return val, err
	}

	indx := r.fromRegex.FindStringSubmatchIndex(normVal)
	if indx == nil {
		return val, nil
	}

	return string(r.fromRegex.ExpandString([]byte{}, r.to, val, indx)), nil
}

func init() {
	module.Register("replace_sender", NewReplaceAddr)
	module.Register("replace_rcpt", NewReplaceAddr)
}
