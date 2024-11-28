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
	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
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

type (
	Modifier struct {
		instName string

		domains         []string
		selector        string
		signers         map[string]crypto.Signer
		oversignHeader  []string
		signHeader      []string
		headerCanon     dkim.Canonicalization
		bodyCanon       dkim.Canonicalization
		sigExpiry       time.Duration
		hash            crypto.Hash
		multipleFromOk  bool
		signSubdomains  bool
		keyPathTemplate string
		hashName        string
		newKeyAlgo      string
		table           module.MutableTable
		storeKeysInDB   bool

		log log.Logger
	}

	DKIM struct {
		Domain     string        `json:"domain"`
		PrivateKey string        `json:"privateKey,omitempty"`
		PublicKey  string        `json:"publicKey,omitempty"`
		DNSName    string        `json:"dnsName"`
		DNSValue   string        `json:"dnsValue"`
		Expires    time.Time     `json:"expires,omitempty"`
		pkey       crypto.Signer `json:"-"`
	}
)

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	m := &Modifier{
		instName: instName,
		signers:  map[string]crypto.Signer{},
		log:      log.Logger{Name: "modify.dkim"},
	}

	if len(inlineArgs) == 0 {
		return m, nil
	}
	if len(inlineArgs) == 1 {
		return nil, errors.New("modify.dkim: at least two arguments required")
	}

	m.domains = inlineArgs[0 : len(inlineArgs)-1]
	m.selector = inlineArgs[len(inlineArgs)-1]

	return m, nil
}

func (m *Modifier) Name() string {
	return "modify.dkim"
}

func (m *Modifier) InstanceName() string {
	return m.instName
}

func (m *Modifier) Init(cfg *config.Map) error {

	cfg.Bool("debug", true, false, &m.log.Debug)
	cfg.Bool("store_keys_in_database", false, false, &m.storeKeysInDB)
	cfg.StringList("domains", false, false, m.domains, &m.domains)
	cfg.String("selector", false, false, m.selector, &m.selector)
	cfg.Custom("domain_table", true, false, nil, modconfig.TableDirective, &m.table)
	cfg.String("key_path", false, false, "dkim_keys/{domain}_{selector}.key", &m.keyPathTemplate)
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
		[]string{"sha256"}, "sha256", &m.hashName)
	cfg.Enum("newkey_algo", false, false,
		[]string{"rsa4096", "rsa2048", "ed25519"}, "rsa2048", &m.newKeyAlgo)
	cfg.Bool("allow_multiple_from", false, false, &m.multipleFromOk)
	cfg.Bool("sign_subdomains", false, false, &m.signSubdomains)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	if len(m.domains) == 0 {
		return errors.New("sign_domain: at least one domain is needed")
	}
	if m.selector == "" {
		return errors.New("sign_domain: selector is not specified")
	}
	if m.signSubdomains && len(m.domains) > 1 {
		return errors.New("sign_domain: only one domain is supported when sign_subdomains is enabled")
	}

	m.hash = hashFuncs[m.hashName]
	if m.hash == 0 {
		panic("modify.dkim.Init: Hash function allowed by config matcher but not present in hashFuncs")
	}

	// If available, include domains from SQL table
	if m.table != nil {
		domains, err := m.table.Keys()
		if err != nil {
			return err
		}

		if len(domains) > 0 {
			m.domains = append(m.domains, domains...)
		}
	}

	storeKeysInDB := m.storeKeysInDB && m.table != nil

	for _, domain := range m.domains {
		if _, err := idna.ToASCII(domain); err != nil {
			m.log.Printf("warning: unable to convert domain %s to A-labels form, non-EAI messages will not be signed: %v", domain, err)
		}

		keyValues := strings.NewReplacer("{domain}", domain, "{selector}", m.selector)
		keyPath := keyValues.Replace(m.keyPathTemplate)

		signer, newKey, err := m.loadOrGenerateKey(domain, keyPath, m.newKeyAlgo, storeKeysInDB)
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
				m.newKeyAlgo, keyPath, dnsPath, m.selector, domain)
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

func (s state) RewriteRcpt(ctx context.Context, rcptTo string) ([]string, error) {
	return []string{rcptTo}, nil
}

func (s *state) RewriteBody(ctx context.Context, h *textproto.Header, body buffer.Buffer) error {
	defer trace.StartRegion(ctx, "modify.dkim/RewriteBody").End()

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

	if s.m.signSubdomains {
		topDomain := s.m.domains[0]
		if strings.HasSuffix(domain, "."+topDomain) {
			domain = topDomain
		}
	}
	normDomain, err := dns.ForLookup(domain)
	if err != nil {
		s.log.Error("unable to normalize domain from envelope sender", err, "domain", domain)
		return nil
	}
	keySigner := s.m.signers[normDomain]
	if keySigner == nil {
		if s.m.table == nil {
			s.log.Msg("no key for domain", "domain", normDomain)
			return nil
		}
		keySigner, err = s.m.generateKeyForDomain(normDomain)
		if err != nil {
			s.log.Msg("no key for domain", "domain", normDomain)
			return err
		}
		s.m.signers[normDomain] = keySigner
		s.m.domains = append(s.m.domains, domain)
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
		return exterrors.WithFields(err, map[string]interface{}{"modifier": "modify.dkim"})
	}
	if err := textproto.WriteHeader(signer, *h); err != nil {
		signer.Close()
		return exterrors.WithFields(err, map[string]interface{}{"modifier": "modify.dkim"})
	}
	r, err := body.Open()
	if err != nil {
		signer.Close()
		return exterrors.WithFields(err, map[string]interface{}{"modifier": "modify.dkim"})
	}
	if _, err := io.Copy(signer, r); err != nil {
		signer.Close()
		return exterrors.WithFields(err, map[string]interface{}{"modifier": "modify.dkim"})
	}

	if err := signer.Close(); err != nil {
		return exterrors.WithFields(err, map[string]interface{}{"modifier": "modify.dkim"})
	}

	h.AddRaw([]byte(signer.Signature()))

	s.m.log.DebugMsg("signed", "domain", domain)

	return nil
}

func (s state) Close() error {
	return nil
}

func init() {
	module.Register("modify.dkim", New)
}
