package dkim

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func (m *Modifier) loadOrGenerateKey(expectedDomain, expectedSelector, keyPath, newKeyAlgo string) (crypto.Signer, error) {
	// Notes:
	// . PKCS #8 (RFC 5208) is a superset of PKCS #1 (RFC 3447).
	// . DKIM records use ASN.1 DER (as in PKCS#1) wrapped in base64 for keys.
	// . For ed25519, no additional encoding is used.

	f, err := os.Open(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return m.generateAndWrite(expectedDomain, expectedSelector, keyPath, newKeyAlgo)
		}
		return nil, err
	}
	defer f.Close()

	pemBlob, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(pemBlob)
	if block == nil {
		return nil, fmt.Errorf("sign_dkim: %s: invalid PEM block", keyPath)
	}

	if block.Type == "ENCRYPTED PRIVATE KEY" {
		return nil, fmt.Errorf("sign_dkim: %s: encrypted, can't use", keyPath)
	}
	if block.Type != "PRIVATE KEY" && block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("sign_dkim: %s: not a private key", keyPath)
	}

	if val := block.Headers["X-DKIM-Selector"]; val != "" && val != expectedSelector {
		return nil, fmt.Errorf("sign_dkim: %s: selector mismatch, want %s, got %s", keyPath, expectedSelector, val)
	}
	if val := block.Headers["X-DKIM-Domain"]; val != "" && val != expectedDomain {
		return nil, fmt.Errorf("sign_dkim: %s: domain mismatch, want %s, got %s", keyPath, expectedDomain, val)
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("sign_dkim: %s: %w", keyPath, err)
	}

	switch key := key.(type) {
	case *rsa.PrivateKey:
		if err := key.Validate(); err != nil {
			return nil, err
		}
		return key, nil
	case ed25519.PrivateKey:
		return key, nil
	case *ecdsa.PublicKey:
		return nil, fmt.Errorf("sign_dkim: %s: ECDSA keys are not supported", keyPath)
	default:
		return nil, fmt.Errorf("sign_dkim: %s: unknown key type: %T", keyPath, key)
	}
}

func (m *Modifier) generateAndWrite(domain, selector, keyPath, newKeyAlgo string) (crypto.Signer, error) {
	wrapErr := func(err error) error {
		return fmt.Errorf("sign_dkim: generate %s: %w", keyPath, err)
	}

	m.log.Printf("generating a new %s keypair...", newKeyAlgo)

	var (
		pkey     crypto.Signer
		dkimName = newKeyAlgo
		err      error
	)
	switch newKeyAlgo {
	case "rsa4096":
		dkimName = "rsa"
		pkey, err = rsa.GenerateKey(rand.Reader, 4096)
	case "rsa2048":
		dkimName = "rsa"
		pkey, err = rsa.GenerateKey(rand.Reader, 2048)
	case "ed25519":
		_, pkey, err = ed25519.GenerateKey(rand.Reader)
	default:
		err = fmt.Errorf("unknown key algorithm: %s", newKeyAlgo)
	}
	if err != nil {
		return nil, wrapErr(err)
	}

	keyBlob, err := x509.MarshalPKCS8PrivateKey(pkey)
	if err != nil {
		return nil, wrapErr(err)
	}

	// 0777 because we have public keys in here too and they don't
	// need protection. Individual private key files have 0600 perms.
	if err := os.MkdirAll(filepath.Dir(keyPath), 0777); err != nil {
		return nil, wrapErr(err)
	}

	dnsPath, err := writeDNSRecord(keyPath, dkimName, pkey)
	if err != nil {
		return nil, wrapErr(err)
	}

	f, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, wrapErr(err)
	}

	if err := pem.Encode(f, &pem.Block{
		Type: "PRIVATE KEY",
		Headers: map[string]string{
			"X-DKIM-Domain":   domain,
			"X-DKIM-Selector": selector,
		},
		Bytes: keyBlob,
	}); err != nil {
		return nil, wrapErr(err)
	}

	m.log.Printf("generated a new %s keypair, private key is in %s, TXT record with public key is in %s,\n"+
		"put its contents into TXT record for %s._domainkey.%s to make signing and verification work",
		newKeyAlgo, keyPath, dnsPath, selector, domain)

	return pkey, nil
}

func writeDNSRecord(keyPath, dkimAlgoName string, pkey crypto.Signer) (string, error) {
	var (
		keyBlob []byte
		pubkey  = pkey.Public()
	)
	switch pubkey := pubkey.(type) {
	case *rsa.PublicKey:
		keyBlob = x509.MarshalPKCS1PublicKey(pubkey)
	case ed25519.PublicKey:
		keyBlob = pubkey
	default:
		panic("sign_dkim.writeDNSRecord: unknown key algorithm")
	}

	dnsPath := keyPath + ".dns"
	if filepath.Ext(keyPath) == ".key" {
		dnsPath = keyPath[:len(keyPath)-4] + ".dns"
	}
	dnsF, err := os.Create(dnsPath)
	if err != nil {
		return "", err
	}
	keyRecord := fmt.Sprintf("v=DKIM1; k=%s; p=%s", dkimAlgoName, base64.StdEncoding.EncodeToString(keyBlob))
	if _, err := io.WriteString(dnsF, keyRecord); err != nil {
		return "", err
	}
	return dnsPath, nil
}
