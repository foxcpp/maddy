package oauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
	"github.com/golang-jwt/jwt/v5"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testModule(t *testing.T, client *http.Client, cfg Config) *OAuth {
	cache, err := lru.New2Q[string, any](5)
	require.NoError(t, err)

	return &OAuth{
		log:      testutils.Logger(t, "auth.oauth"),
		instName: "test_auth",
		cfg:      cfg,
		client:   client,
		keyCache: cache,
	}
}

func TestOAuthIntrospectAuth(t *testing.T) {
	var introspectionResult map[string]any
	var introspectionStatusCode int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/introspect", r.URL.String())
		assert.Equal(t, "Bearer testtoken", r.Header.Get("Authorization"))
		assert.Equal(t, "test-indeed", r.Header.Get("X-Test"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(introspectionStatusCode)

		err := json.NewEncoder(w).Encode(introspectionResult)
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	o := testModule(t, srv.Client(), Config{
		TLSClient: nil,
		AdditionalHTTPHeaders: map[string][]string{
			"X-Test": {"test-indeed"},
		},
		Introspection:        (*OAuth).IntrospectAuth,
		IntrospectionURL:     srv.URL + "/introspect",
		IntrospectionTimeout: 10 * time.Second,
		Scopes:               []string{"email", "offline_access"},
		UsernameAttribute:    "email",
		ActiveAttribute:      "active",
		ActiveValue:          "true",
	})

	t.Run("ok", func(t *testing.T) {
		introspectionResult = map[string]any{
			"active": true,
			"email":  "testuser@maddy.test",
			"scope":  "email offline_access",
		}
		introspectionStatusCode = http.StatusOK

		username, err := o.AuthBearerToken(&module.AuthContext{}, "", "testtoken")
		require.NoError(t, err)
		assert.Equal(t, "testuser@maddy.test", username)
	})
	t.Run("inactive", func(t *testing.T) {
		introspectionResult = map[string]any{
			"active": false,
			"email":  "testuser@maddy.test",
			"scope":  "email offline_access",
		}
		introspectionStatusCode = http.StatusOK

		_, err := o.AuthBearerToken(&module.AuthContext{}, "", "testtoken")
		require.Error(t, err)
	})
	t.Run("no activity", func(t *testing.T) {
		introspectionResult = map[string]any{
			"email": "testuser@maddy.test",
			"scope": "email offline_access",
		}
		introspectionStatusCode = http.StatusOK

		_, err := o.AuthBearerToken(&module.AuthContext{}, "", "testtoken")
		require.Error(t, err)
	})
	t.Run("broken activity value", func(t *testing.T) {
		introspectionResult = map[string]any{
			"active": "whatever",
			"email":  "testuser@maddy.test",
			"scope":  "email offline_access",
		}
		introspectionStatusCode = http.StatusOK

		_, err := o.AuthBearerToken(&module.AuthContext{}, "", "testtoken")
		require.Error(t, err)
	})
	t.Run("introspection error", func(t *testing.T) {
		introspectionResult = map[string]any{}
		introspectionStatusCode = http.StatusInternalServerError

		_, err := o.AuthBearerToken(&module.AuthContext{}, "", "testtoken")
		require.Error(t, err)
	})
	t.Run("no username", func(t *testing.T) {
		introspectionResult = map[string]any{
			"active": true,
			"scope":  "email offline_access",
		}
		introspectionStatusCode = http.StatusOK

		_, err := o.AuthBearerToken(&module.AuthContext{}, "", "testtoken")
		require.Error(t, err)
	})
}

func TestOAuthIntrospectGet(t *testing.T) {
	var introspectionResult map[string]any
	var introspectionStatusCode int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/introspect?token=testtoken", r.URL.String())
		assert.Equal(t, "test-indeed", r.Header.Get("X-Test"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(introspectionStatusCode)

		err := json.NewEncoder(w).Encode(introspectionResult)
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	introspectionResult = map[string]any{
		"active": true,
		"scope":  "email offline_access",
		"email":  "testuser@maddy.test",
	}
	introspectionStatusCode = http.StatusOK

	o := testModule(t, srv.Client(), Config{
		TLSClient: nil,
		AdditionalHTTPHeaders: map[string][]string{
			"X-Test": {"test-indeed"},
		},
		Introspection:        (*OAuth).IntrospectGet,
		IntrospectionURL:     srv.URL + "/introspect?token=",
		IntrospectionTimeout: 10 * time.Second,
		Scopes:               []string{"email", "offline_access"},
		UsernameAttribute:    "email",
		ActiveAttribute:      "active",
		ActiveValue:          "true",
	})

	username, err := o.AuthBearerToken(&module.AuthContext{}, "", "testtoken")
	require.NoError(t, err)
	assert.Equal(t, "testuser@maddy.test", username)
}

func TestOAuthIntrospectPost(t *testing.T) {
	var introspectionResult map[string]any
	var introspectionStatusCode int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/introspect", r.URL.String())
		assert.Equal(t, "test-indeed", r.Header.Get("X-Test"))
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "testtoken", r.Form.Get("token"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(introspectionStatusCode)

		err := json.NewEncoder(w).Encode(introspectionResult)
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	introspectionResult = map[string]any{
		"active": true,
		"scope":  "email offline_access",
		"email":  "testuser@maddy.test",
	}
	introspectionStatusCode = http.StatusOK

	o := testModule(t, srv.Client(), Config{
		TLSClient: nil,
		AdditionalHTTPHeaders: map[string][]string{
			"X-Test": {"test-indeed"},
		},
		Introspection:        (*OAuth).IntrospectPost,
		IntrospectionURL:     srv.URL + "/introspect",
		IntrospectionTimeout: 10 * time.Second,
		Scopes:               []string{"email", "offline_access"},
		UsernameAttribute:    "email",
		ActiveAttribute:      "active",
		ActiveValue:          "true",
	})

	username, err := o.AuthBearerToken(&module.AuthContext{}, "", "testtoken")
	require.NoError(t, err)
	assert.Equal(t, "testuser@maddy.test", username)
}

func TestOAuthIntrospectLocal(t *testing.T) {
	pubKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pubKeyBlob, err := x509.MarshalPKIXPublicKey(pubKey)
	require.NoError(t, err)
	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBlob,
	})

	keyTable := testutils.Table{
		M: map[string]string{
			"default/EDDSA/default":     string(pubKeyPEM),
			"azp_value/EDDSA/default":   string(pubKeyPEM),
			"azp_value/EDDSA/kid_value": string(pubKeyPEM),
		},
	}

	o := testModule(t, nil, Config{
		TLSClient:             nil,
		AdditionalHTTPHeaders: nil,
		Introspection:         (*OAuth).IntrospectLocal,
		IntrospectionURL:      "",
		IntrospectionTimeout:  10 * time.Second,
		Scopes:                []string{"email", "offline_access"},
		UsernameAttribute:     "email",
		ActiveAttribute:       "active",
		ActiveValue:           "true",
		JWTKeyIDTemplate:      "{azp}/{alg}/{kid}",
		JWTExpiryLeeway:       time.Second,
		JWTKeyTable:           keyTable,
		JWTValidMethods:       []string{jwt.SigningMethodEdDSA.Alg()},
		JWTIssuers:            []string{"valid_issuer"},
		JWTAudienceTable: testutils.Table{
			M: map[string]string{
				"valid_aud": "",
			},
		},
	})

	createToken := func(hdr map[string]any, attributes jwt.MapClaims) string {
		tok := jwt.New(jwt.SigningMethodEdDSA)
		tok.Claims = attributes
		for k, v := range hdr {
			tok.Header[k] = v
		}
		attributes["exp"] = time.Now().Add(time.Hour).Unix()
		token, err := tok.SignedString(privateKey)
		require.NoError(t, err)
		return token
	}

	t.Run("ok", func(t *testing.T) {
		token := createToken(map[string]any{
			"kid": "kid_value",
		}, jwt.MapClaims{
			"azp":    "azp_value",
			"active": true,
			"iss":    "valid_issuer",
			"aud":    "valid_aud",
			"scope":  "email offline_access",
			"email":  "testuser@maddy.test",
		})

		username, err := o.AuthBearerToken(&module.AuthContext{}, "", token)
		require.NoError(t, err)
		assert.Equal(t, "testuser@maddy.test", username)
	})

	t.Run("default kid", func(t *testing.T) {
		token := createToken(map[string]any{}, jwt.MapClaims{
			"azp":    "azp_value",
			"active": true,
			"iss":    "valid_issuer",
			"aud":    "valid_aud",
			"scope":  "email offline_access",
			"email":  "testuser@maddy.test",
		})

		username, err := o.AuthBearerToken(&module.AuthContext{}, "", token)
		require.NoError(t, err)
		assert.Equal(t, "testuser@maddy.test", username)
	})

	t.Run("default azp", func(t *testing.T) {
		token := createToken(map[string]any{}, jwt.MapClaims{
			"active": true,
			"iss":    "valid_issuer",
			"aud":    "valid_aud",
			"scope":  "email offline_access",
			"email":  "testuser@maddy.test",
		})

		username, err := o.AuthBearerToken(&module.AuthContext{}, "", token)
		require.NoError(t, err)
		assert.Equal(t, "testuser@maddy.test", username)
	})

	t.Run("missing issuer", func(t *testing.T) {
		token := createToken(map[string]any{}, jwt.MapClaims{
			"active": true,
			"aud":    "valid_aud",
			"scope":  "email offline_access",
			"email":  "testuser@maddy.test",
		})

		_, err := o.AuthBearerToken(&module.AuthContext{}, "", token)
		require.Error(t, err)
	})

	t.Run("invalid issuer", func(t *testing.T) {
		token := createToken(map[string]any{}, jwt.MapClaims{
			"active": true,
			"iss":    "invalid_issuer",
			"aud":    "valid_aud",
			"scope":  "email offline_access",
			"email":  "testuser@maddy.test",
		})

		_, err := o.AuthBearerToken(&module.AuthContext{}, "", token)
		require.Error(t, err)
	})

	t.Run("invalid aud", func(t *testing.T) {
		token := createToken(map[string]any{}, jwt.MapClaims{
			"active": true,
			"iss":    "valid_issuer",
			"aud":    "invalid_aud",
			"scope":  "email offline_access",
			"email":  "testuser@maddy.test",
		})

		_, err := o.AuthBearerToken(&module.AuthContext{}, "", token)
		require.Error(t, err)
	})

	t.Run("wrong key", func(t *testing.T) {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		tok := jwt.New(jwt.SigningMethodEdDSA)
		claims := jwt.MapClaims{
			"active": true,
			"iss":    "valid_issuer",
			"aud":    "invalid_aud",
			"scope":  "email offline_access",
			"email":  "testuser@maddy.test",
		}
		claims["exp"] = time.Now().Add(time.Hour).Unix()
		tok.Claims = claims
		token, err := tok.SignedString(privateKey)
		require.NoError(t, err)

		_, err = o.AuthBearerToken(&module.AuthContext{}, "", token)
		require.Error(t, err)
	})

	t.Run("expired token", func(t *testing.T) {
		tok := jwt.New(jwt.SigningMethodEdDSA)
		claims := jwt.MapClaims{
			"active": true,
			"iss":    "valid_issuer",
			"aud":    "invalid_aud",
			"scope":  "email offline_access",
			"email":  "testuser@maddy.test",
		}
		claims["exp"] = time.Now().Add(-2 * time.Second).Unix()
		tok.Claims = claims
		token, err := tok.SignedString(privateKey)
		require.NoError(t, err)

		_, err = o.AuthBearerToken(&module.AuthContext{}, "", token)
		require.Error(t, err)
	})
}
