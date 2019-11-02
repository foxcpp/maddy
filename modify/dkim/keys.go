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

func (m *Modifier) loadOrGenerateKey(keyPath, newKeyAlgo string) (pkey crypto.Signer, newKey bool, err error) {
	f, err := os.Open(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			pkey, err = m.generateAndWrite(keyPath, newKeyAlgo)
			return pkey, true, err
		}
		return nil, false, err
	}
	defer f.Close()

	pemBlob, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, false, err
	}

	block, _ := pem.Decode(pemBlob)
	if block == nil {
		return nil, false, fmt.Errorf("sign_dkim: %s: invalid PEM block", keyPath)
	}

	var key interface{}
	switch block.Type {
	case "PRIVATE KEY": // RFC 5208 aka PKCS #8
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, false, fmt.Errorf("sign_dkim: %s: %w", keyPath, err)
		}
	case "RSA PRIVATE KEY": // RFC 3447 aka PKCS #1
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, false, fmt.Errorf("sign_dkim: %s: %w", keyPath, err)
		}
	case "EC PRIVATE KEY": // RFC 5915
		key, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, false, fmt.Errorf("sign_dkim: %s: %w", keyPath, err)
		}
	default:
		return nil, false, fmt.Errorf("sign_dkim: %s: not a private key or unsupported format", keyPath)
	}

	switch key := key.(type) {
	case *rsa.PrivateKey:
		if err := key.Validate(); err != nil {
			return nil, false, err
		}
		key.Precompute()
		return key, false, nil
	case ed25519.PrivateKey:
		return key, false, nil
	case *ecdsa.PublicKey:
		return nil, false, fmt.Errorf("sign_dkim: %s: ECDSA keys are not supported", keyPath)
	default:
		return nil, false, fmt.Errorf("sign_dkim: %s: unknown key type: %T", keyPath, key)
	}
}

func (m *Modifier) generateAndWrite(keyPath, newKeyAlgo string) (crypto.Signer, error) {
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

	_, err = writeDNSRecord(keyPath, dkimName, pkey)
	if err != nil {
		return nil, wrapErr(err)
	}

	f, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, wrapErr(err)
	}

	if err := pem.Encode(f, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBlob,
	}); err != nil {
		return nil, wrapErr(err)
	}

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
