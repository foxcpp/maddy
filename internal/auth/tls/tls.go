package tls

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/foxcpp/maddy/internal/authz"
)

const modName = "auth.tls"

type CertIdentityFunc = func(*x509.Certificate) []string

func CertIdentityCN(cert *x509.Certificate) []string {
	return []string{cert.Subject.CommonName}
}

func CertIdentitySANEmail(cert *x509.Certificate) []string {
	return cert.EmailAddresses
}

var certIdentityFuncs = map[string]CertIdentityFunc{
	"cn":        CertIdentityCN,
	"san_email": CertIdentitySANEmail,
}

type Auth struct {
	log      *log.Logger
	instName string

	identityFuncs           []CertIdentityFunc
	ignoreRequestedIdentity bool
	identityNormalize       authz.NormalizeFunc
	requireKeyUsage         bool
	requireExtKeyUsage      bool
}

func New(c *container.C, modName, instName string) (module.Module, error) {
	return &Auth{
		log:      c.DefaultLogger.Sublogger(modName),
		instName: instName,
	}, nil
}

func (a *Auth) Configure(inlineArgs []string, cfg *config.Map) error {
	if len(inlineArgs) > 0 {
		return errors.New("inline args not supported")
	}
	config.EnumListMapped[CertIdentityFunc](
		cfg, "identity_fields", false, false,
		certIdentityFuncs, []CertIdentityFunc{CertIdentitySANEmail, CertIdentityCN},
		&a.identityFuncs,
	)
	cfg.Bool("ignore_requested_identity", false, false, &a.ignoreRequestedIdentity)
	cfg.Bool("require_key_usage", false, true, &a.requireKeyUsage)
	cfg.Bool("require_ext_key_usage", false, true, &a.requireExtKeyUsage)
	config.EnumMapped[authz.NormalizeFunc](
		cfg, "identity_normalize", false, false,
		authz.NormalizeFuncs, authz.NormalizeAuto,
		&a.identityNormalize,
	)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if len(a.identityFuncs) == 0 {
		return errors.New("auth.tls: at least one identity field should be specified")
	}

	return nil
}

func (a *Auth) Name() string {
	return modName
}

func (a *Auth) InstanceName() string {
	return a.instName
}

func (a *Auth) identities(cert *x509.Certificate) []string {
	result := make([]string, 0, len(a.identityFuncs))
	for _, identityFunc := range a.identityFuncs {
		result = append(result, identityFunc(cert)...)
	}
	return result
}

func x509Fingerprint(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:])
}

func (a *Auth) authenticateProxied(ctx *module.AuthContext, requestedIdentity string) (finalIdentity string, err error) {
	certUsername, err := a.identityNormalize(ctx.ProxiedTLS.CertUsername)
	if err != nil {
		return "", fmt.Errorf("auth.tls: failed to normalize certificate name %q: %w",
			ctx.ProxiedTLS.CertUsername, err)
	}

	if requestedIdentity != "" && !a.ignoreRequestedIdentity {
		if requestedIdentity == certUsername {
			return certUsername, nil
		}
		return "", fmt.Errorf("auth.tls: requested identity does not match certificate username")
	}

	return certUsername, nil
}

func (a *Auth) AuthExternal(ctx *module.AuthContext, requestedIdentity string) (finalIdentity string, err error) {
	if ctx.ProxiedTLS != nil {
		return a.authenticateProxied(ctx, requestedIdentity)
	}

	if requestedIdentity != "" {
		requestedIdentity, err = a.identityNormalize(requestedIdentity)
		if err != nil {
			return "", fmt.Errorf("auth.tls: failed to normalize requested identity %q: %w",
				requestedIdentity, err)
		}
	}

	if ctx.TLS == nil || !ctx.TLS.HandshakeComplete {
		return "", errors.New("auth.tls: no TLS session to authenticate")
	}
	if len(ctx.TLS.PeerCertificates) == 0 {
		return "", errors.New("auth.tls: no client certificate to authenticate")
	}
	if len(ctx.TLS.VerifiedChains) == 0 {
		return "", errors.New("auth.tls: client certificate is not verified")
	}
	if len(ctx.TLS.VerifiedChains[0]) == 0 {
		return "", errors.New("auth.tls: verified chain is empty")
	}

	leafCert := ctx.TLS.VerifiedChains[0][0]

	if a.requireKeyUsage && leafCert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return "", errors.New("auth.tls: key usage digitalSignature is required")
	}
	if a.requireExtKeyUsage {
		var ok bool
		for _, eku := range leafCert.ExtKeyUsage {
			if eku == x509.ExtKeyUsageClientAuth {
				ok = true
				break
			}
		}
		if !ok {
			return "", errors.New("auth.tls: no client auth EKU found in certificate")
		}
	}

	identities := a.identities(leafCert)
	if len(identities) == 0 {
		return "", errors.New("auth.tls: no client identity in provided certificate")
	}

	if requestedIdentity == "" || a.ignoreRequestedIdentity {
		if a.log.IsDebug() {
			a.log.DebugMsg("accepted client certificate identity",
				"identity", requestedIdentity, "cert_sha256", x509Fingerprint(leafCert))
		}

		return identities[0], nil
	}

	for _, identity := range identities {
		identity, err = a.identityNormalize(identity)
		if err != nil {
			return "", fmt.Errorf("auth.tls: invalid identity %q: %v", identity, err)
		}

		if identity == requestedIdentity {
			if a.log.IsDebug() {
				a.log.DebugMsg("accepted client certificate identity",
					"identity", requestedIdentity, "cert_sha256", x509Fingerprint(leafCert))
			}

			return identity, nil
		}
	}
	return "", errors.New("auth.tls: requested identity is not allowed by provided certificate")
}

func init() {
	modules.Register("auth.tls", New)
}
