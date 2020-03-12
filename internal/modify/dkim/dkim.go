package dkim

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"io"
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

	domains        []string
	selector       string
	signers        map[string]crypto.Signer
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
		signers:  map[string]crypto.Signer{},
		log:      log.Logger{Name: "sign_dkim"},
	}

	if len(inlineArgs) == 0 {
		return m, nil
	}
	if len(inlineArgs) == 1 {
		return nil, errors.New("sign_dkim: at least two arguments required")
	}

	m.domains = inlineArgs[0 : len(inlineArgs)-1]
	m.selector = inlineArgs[len(inlineArgs)-1]

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
	cfg.StringList("domains", false, false, m.domains, &m.domains)
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

	if len(m.domains) == 0 {
		return errors.New("sign_domain: at least one domain is needed")
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

	for _, domain := range m.domains {
		if _, err := idna.ToASCII(domain); err != nil {
			m.log.Printf("warning: unable to convert domain %s to A-labels form, non-EAI messages will not be signed: %v", domain, err)
		}

		keyValues := strings.NewReplacer("{domain}", domain, "{selector}", m.selector)
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
				newKeyAlgo, keyPath, dnsPath, m.selector, domain)
		}

		normDomain, err := dns.ForLookup(domain)
		if err != nil {
			return fmt.Errorf("sign_skim: unable to normalize domain %s: %w", domain, err)
		}
		m.signers[normDomain] = signer
	}

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
	from string
	log  log.Logger
}

func (m *Modifier) ModStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.ModifierState, error) {
	return &state{
		m:    m,
		meta: msgMeta,
		log:  target.DeliveryLogger(m.log, msgMeta),
	}, nil
}

func (s *state) RewriteSender(ctx context.Context, mailFrom string) (string, error) {
	s.from = mailFrom
	return mailFrom, nil
}

func (s state) RewriteRcpt(ctx context.Context, rcptTo string) (string, error) {
	return rcptTo, nil
}

func (s *state) RewriteBody(ctx context.Context, h *textproto.Header, body buffer.Buffer) error {
	defer trace.StartRegion(ctx, "sign_dkim/RewriteBody").End()

	var domain string
	if s.from != "" {
		var err error
		_, domain, err = address.Split(s.from)
		if err != nil {
			return err
		}
	}
	// Use first key for null return path (<>) and postmaster (<postmaster>)
	if domain == "" {
		domain = s.m.domains[0]
	}
	selector := s.m.selector

	normDomain, err := dns.ForLookup(domain)
	if err != nil {
		s.log.Error("unable to normalize domain from envelope sender", err, "domain", domain)
		return nil
	}
	keySigner := s.m.signers[normDomain]
	if keySigner == nil {
		s.log.Msg("no key for domain", "domain", normDomain)
		return nil
	}

	// If the message is non-EAI, we are not allowed to use domains in U-labels,
	// attempt to convert.
	if !s.meta.SMTPOpts.UTF8 {
		var err error
		domain, err = idna.ToASCII(domain)
		if err != nil {
			return nil
		}

		selector, err = idna.ToASCII(selector)
		if err != nil {
			return nil
		}
	}

	opts := dkim.SignOptions{
		Domain:                 domain,
		Selector:               selector,
		Identifier:             "@" + domain,
		Signer:                 keySigner,
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

	s.m.log.DebugMsg("signed", "domain", domain)

	return nil
}

func (s state) Close() error {
	return nil
}

func init() {
	module.Register("sign_dkim", New)
}
