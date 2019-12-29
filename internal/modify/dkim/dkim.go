package dkim

import (
	"context"
	"crypto"
	"errors"
	"io"
	"net/mail"
	"path/filepath"
	"runtime/trace"
	"strings"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/dkim"
	"github.com/foxcpp/maddy/internal/address"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/target"
	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"
)

const Day = 86400 * time.Second

var (
	oversignDefault = []string{
		// Directly visible to the user.
		"Subject",
		"Sender",
		"To",
		"Cc",
		"From",
		"Date",

		// Affects body processing.
		"MIME-Version",
		"Content-Type",
		"Content-Transfer-Encoding",

		// Affects user interaction.
		"Reply-To",
		"In-Reply-To",
		"Message-Id",
		"References",

		// Provide additional security benefit for OpenPGP.
		"Autocrypt",
		"Openpgp",
	}
	signDefault = []string{
		// Mailing list information. Not oversigned to prevent signature
		// breakage by aliasing MLMs.
		"List-Id",
		"List-Help",
		"List-Unsubscribe",
		"List-Post",
		"List-Owner",
		"List-Archive",

		// Not oversigned since it can be prepended by intermediate relays.
		"Resent-To",
		"Resent-Sender",
		"Resent-Message-Id",
		"Resent-Date",
		"Resent-From",
		"Resent-Cc",
	}

	hashFuncs = map[string]crypto.Hash{
		"sha256": crypto.SHA256,
	}
)

type Modifier struct {
	instName string

	domain         string
	selector       string
	signer         crypto.Signer
	oversignHeader []string
	signHeader     []string
	headerCanon    dkim.Canonicalization
	bodyCanon      dkim.Canonicalization
	sigExpiry      time.Duration
	hash           crypto.Hash
	senderMatch    map[string]struct{}
	multipleFromOk bool

	log log.Logger
}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	m := &Modifier{
		instName: instName,
		log:      log.Logger{Name: "sign_dkim"},
	}

	switch len(inlineArgs) {
	case 2:
		m.domain = inlineArgs[0]
		m.selector = inlineArgs[1]
	case 0:
		// whatever
	case 1:
		fallthrough
	default:
		return nil, errors.New("sign_dkim: wrong amount of inline arguments")
	}

	return m, nil
}

func (m *Modifier) Name() string {
	return "sign_dkim"
}

func (m *Modifier) InstanceName() string {
	return m.instName
}

func (m *Modifier) Init(cfg *config.Map) error {
	var (
		hashName        string
		keyPathTemplate string
		newKeyAlgo      string
		senderMatch     []string
	)

	cfg.Bool("debug", true, false, &m.log.Debug)
	cfg.String("domain", false, false, m.domain, &m.domain)
	cfg.String("selector", false, false, m.selector, &m.selector)
	cfg.String("key_path", false, false, "dkim_keys/{domain}_{selector}.key", &keyPathTemplate)
	cfg.StringList("oversign_fields", false, false, oversignDefault, &m.oversignHeader)
	cfg.StringList("sign_fields", false, false, signDefault, &m.signHeader)
	cfg.Enum("header_canon", false, false,
		[]string{string(dkim.CanonicalizationRelaxed), string(dkim.CanonicalizationSimple)},
		dkim.CanonicalizationRelaxed, (*string)(&m.headerCanon))
	cfg.Enum("body_canon", false, false,
		[]string{string(dkim.CanonicalizationRelaxed), string(dkim.CanonicalizationSimple)},
		dkim.CanonicalizationRelaxed, (*string)(&m.bodyCanon))
	cfg.Duration("sig_expiry", false, false, 5*Day, &m.sigExpiry)
	cfg.Enum("hash", false, false,
		[]string{"sha256"}, "sha256", &hashName)
	cfg.Enum("newkey_algo", false, false,
		[]string{"rsa4096", "rsa2048", "ed25519"}, "rsa2048", &newKeyAlgo)
	cfg.EnumList("require_sender_match", false, false,
		[]string{"envelope", "auth_domain", "auth_user", "off"}, []string{"envelope", "auth"}, &senderMatch)
	cfg.Bool("allow_multiple_from", false, false, &m.multipleFromOk)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	if m.domain == "" {
		return errors.New("sign_domain: domain is not specified")
	}
	if m.selector == "" {
		return errors.New("sign_domain: selector is not specified")
	}

	m.senderMatch = make(map[string]struct{}, len(senderMatch))
	for _, method := range senderMatch {
		m.senderMatch[method] = struct{}{}
	}
	if _, off := m.senderMatch["off"]; off && len(senderMatch) != 1 {
		return errors.New("sign_domain: require_sender_match: 'off' should not be combined with other methods")
	}

	m.hash = hashFuncs[hashName]
	if m.hash == 0 {
		panic("sign_dkim.Init: Hash function allowed by config matcher but not present in hashFuncs")
	}

	keyValues := strings.NewReplacer("{domain}", m.domain, "{selector}", m.selector)
	keyPath := keyValues.Replace(keyPathTemplate)

	signer, newKey, err := m.loadOrGenerateKey(keyPath, newKeyAlgo)
	if err != nil {
		return err
	}

	if newKey {
		dnsPath := keyPath + ".dns"
		if filepath.Ext(keyPath) == ".key" {
			dnsPath = keyPath[:len(keyPath)-4] + ".dns"
		}
		m.log.Printf("generated a new %s keypair, private key is in %s, TXT record with public key is in %s,\n"+
			"put its contents into TXT record for %s._domainkey.%s to make signing and verification work",
			newKeyAlgo, keyPath, dnsPath, m.selector, m.domain)
	}

	m.signer = signer

	return nil
}

func (m *Modifier) fieldsToSign(h *textproto.Header) []string {
	// Filter out duplicated fields from configs so they
	// will not cause panic() in go-msgauth internals.
	seen := make(map[string]struct{})

	res := make([]string, 0, len(m.oversignHeader)+len(m.signHeader))
	for _, key := range m.oversignHeader {
		if _, ok := seen[strings.ToLower(key)]; ok {
			continue
		}
		seen[strings.ToLower(key)] = struct{}{}

		// Add to signing list once per each key use.
		for field := h.FieldsByKey(key); field.Next(); {
			res = append(res, key)
		}
		// And once more to "oversign" it.
		res = append(res, key)
	}
	for _, key := range m.signHeader {
		if _, ok := seen[strings.ToLower(key)]; ok {
			continue
		}
		seen[strings.ToLower(key)] = struct{}{}

		// Add to signing list once per each key use.
		for field := h.FieldsByKey(key); field.Next(); {
			res = append(res, key)
		}
	}
	return res
}

type state struct {
	m    *Modifier
	meta *module.MsgMetadata
	log  log.Logger
}

func (m *Modifier) ModStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.ModifierState, error) {
	return state{
		m:    m,
		meta: msgMeta,
		log:  target.DeliveryLogger(m.log, msgMeta),
	}, nil
}

func (m *Modifier) shouldSign(eai bool, msgId string, h *textproto.Header, mailFrom string, authName string) (string, bool) {
	if _, off := m.senderMatch["off"]; off {
		if !eai {
			aDomain, err := idna.ToASCII(m.domain)
			if err != nil {
				m.log.Msg("not signing, cannot convert key domain domain into A-labels",
					"from_addr", m.domain, "msg_id", msgId)
				return "", false
			}

			return "@" + aDomain, true
		}
		return "@" + m.domain, true
	}

	fromVal := h.Get("From")
	if fromVal == "" {
		m.log.Msg("not signing, empty From", "msg_id", msgId)
		return "", false
	}
	fromAddrs, err := mail.ParseAddressList(fromVal)
	if err != nil {
		m.log.Msg("not signing, malformed From field", "err", err, "msg_id", msgId)
		return "", false
	}
	if len(fromAddrs) != 1 && !m.multipleFromOk {
		m.log.Msg("not signing, multiple addresses in From", "msg_id", msgId)
		return "", false
	}

	fromAddr := fromAddrs[0].Address
	fromUser, fromDomain, err := address.Split(fromAddr)
	if err != nil {
		m.log.Msg("not signing, malformed address in From",
			"err", err, "from_addr", fromAddr, "msg_id", msgId)
		return "", false
	}

	if !dns.Equal(fromDomain, m.domain) {
		m.log.Msg("not signing, From domain is not key domain",
			"from_domain", fromDomain, "key_domain", m.domain, "msg_id", msgId)
		return "", false
	}

	if _, do := m.senderMatch["envelope"]; do && !address.Equal(fromAddr, mailFrom) {
		m.log.Msg("not signing, From address is not envelope address",
			"from_addr", fromAddr, "envelope", mailFrom, "msg_id", msgId)
		return "", false
	}

	if _, do := m.senderMatch["auth"]; do {
		compareWith := norm.NFC.String(fromUser)
		authName := norm.NFC.String(authName)
		if strings.Contains(authName, "@") {
			compareWith, _ = address.ForLookup(fromAddr)
		}
		if !strings.EqualFold(compareWith, authName) {
			m.log.Msg("not signing, From address is not authenticated identity",
				"from_addr", fromAddr, "auth_id", authName, "msg_id", msgId)
			return "", false
		}
	}

	// Don't include non-ASCII in the identifier if message is
	// non-EAI.
	if !eai {
		aDomain, err := idna.ToASCII(fromDomain)
		if err != nil {
			m.log.Msg("not signing, cannot convert From domain into A-labels",
				"from_addr", fromAddr, "msg_id", msgId)
			return "", false
		}

		if !address.IsASCII(fromUser) {
			return "@" + aDomain, true
		}

		return fromUser + "@" + aDomain, true
	}

	return fromAddr, true
}

func (s state) RewriteSender(ctx context.Context, mailFrom string) (string, error) {
	return mailFrom, nil
}

func (s state) RewriteRcpt(ctx context.Context, rcptTo string) (string, error) {
	return rcptTo, nil
}

func (s state) RewriteBody(ctx context.Context, h *textproto.Header, body buffer.Buffer) error {
	defer trace.StartRegion(ctx, "sign_dkim/RewriteBody").End()

	var authUser string
	if s.meta.Conn != nil {
		authUser = s.meta.Conn.AuthUser
	}

	id, ok := s.m.shouldSign(s.meta.SMTPOpts.UTF8, s.meta.ID, h, s.meta.OriginalFrom, authUser)
	if !ok {
		return nil
	}

	domain := s.m.domain
	selector := s.m.selector

	// If the message is non-EAI, we are not alloed to use domains in U-labels,
	// attempt to convert.
	if !s.meta.SMTPOpts.UTF8 {
		var err error
		domain, err = idna.ToASCII(s.m.domain)
		if err != nil {
			return exterrors.WithFields(err, map[string]interface{}{"modifier": "sign_dkim"})
		}

		selector, err = idna.ToASCII(s.m.selector)
		if err != nil {
			return exterrors.WithFields(err, map[string]interface{}{"modifier": "sign_dkim"})
		}
	}

	opts := dkim.SignOptions{
		Domain:                 domain,
		Selector:               selector,
		Identifier:             id,
		Signer:                 s.m.signer,
		Hash:                   s.m.hash,
		HeaderCanonicalization: s.m.headerCanon,
		BodyCanonicalization:   s.m.bodyCanon,
		HeaderKeys:             s.m.fieldsToSign(h),
	}
	if s.m.sigExpiry != 0 {
		opts.Expiration = time.Now().Add(s.m.sigExpiry)
	}
	signer, err := dkim.NewSigner(&opts)
	if err != nil {
		return exterrors.WithFields(err, map[string]interface{}{"modifier": "sign_dkim"})
	}
	if err := textproto.WriteHeader(signer, *h); err != nil {
		signer.Close()
		return exterrors.WithFields(err, map[string]interface{}{"modifier": "sign_dkim"})
	}
	r, err := body.Open()
	if err != nil {
		signer.Close()
		return exterrors.WithFields(err, map[string]interface{}{"modifier": "sign_dkim"})
	}
	if _, err := io.Copy(signer, r); err != nil {
		signer.Close()
		return exterrors.WithFields(err, map[string]interface{}{"modifier": "sign_dkim"})
	}

	if err := signer.Close(); err != nil {
		return exterrors.WithFields(err, map[string]interface{}{"modifier": "sign_dkim"})
	}

	h.AddRaw([]byte(signer.Signature()))

	s.m.log.DebugMsg("signed", "identifier", id)

	return nil
}

func (s state) Close() error {
	return nil
}

func init() {
	module.Register("sign_dkim", New)
}
