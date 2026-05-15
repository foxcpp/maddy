package oauth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/golang-jwt/jwt/v5"
	lru "github.com/hashicorp/golang-lru/v2"
)

const modName = "auth.oauth"

type IntrospectionMethod func(mod *OAuth, ctx context.Context, token string) (map[string]interface{}, error)

type Config struct {
	TLSClient             *tls.Config
	AdditionalHTTPHeaders http.Header
	Introspection         IntrospectionMethod
	IntrospectionURL      string
	IntrospectionTimeout  time.Duration
	Scopes                []string
	UsernameAttribute     string
	ActiveAttribute       string
	ActiveValue           string

	JWTKeyIDTemplate string
	JWTKeyTable      module.Table
	JWTValidMethods  []string
	JWTIssuers       []string
	JWTExpiryLeeway  time.Duration
	JWTAudienceTable module.Table
	OIDCDiscoveryURL string
}

type OAuth struct {
	log      *log.Logger
	instName string
	cfg      Config
	client   *http.Client

	keyCache *lru.TwoQueueCache[string, any]
}

func New(c *container.C, modName, instName string) (module.Module, error) {
	keyCache, err := lru.New2Q[string, any](20)
	if err != nil {
		return nil, err
	}
	return &OAuth{
		log:      c.DefaultLogger.Sublogger(modName),
		instName: instName,
		keyCache: keyCache,
	}, nil
}

func (o *OAuth) Name() string {
	return modName
}

func (o *OAuth) InstanceName() string {
	return o.instName
}

func (o *OAuth) Configure(inlineArgs []string, cfg *config.Map) error {
	switch len(inlineArgs) {
	case 0:
	case 1:
		o.cfg.IntrospectionURL = inlineArgs[0]
	default:
		return errors.New("too many arguments")
	}

	introspectionMethods := map[string]IntrospectionMethod{
		"auth":  (*OAuth).IntrospectAuth,
		"get":   (*OAuth).IntrospectGet,
		"post":  (*OAuth).IntrospectPost,
		"local": (*OAuth).IntrospectLocal,
	}

	cfg.Custom("tls_client", true, false, func() (interface{}, error) {
		return &tls.Config{}, nil
	}, tls2.TLSClientBlock, &o.cfg.TLSClient)
	cfg.Callback("http_header", func(m *config.Map, node config.Node) error {
		if len(node.Args) != 2 {
			return errors.New("http_header requires two arguments")
		}
		key := node.Args[0]
		value := node.Args[1]
		if o.cfg.AdditionalHTTPHeaders == nil {
			o.cfg.AdditionalHTTPHeaders = http.Header{}
		}
		o.cfg.AdditionalHTTPHeaders.Add(key, value)
		return nil
	})
	config.EnumMapped(cfg, "introspection", false, false,
		introspectionMethods, (*OAuth).IntrospectAuth, &o.cfg.Introspection)
	cfg.String("introspection_url", false, false,
		"", &o.cfg.IntrospectionURL)
	cfg.Duration("introspection_timeout", false, false,
		5*time.Second, &o.cfg.IntrospectionTimeout)
	cfg.StringList("scopes", false, false, nil, &o.cfg.Scopes)
	cfg.String("username_attribute", false, false, "email", &o.cfg.UsernameAttribute)
	cfg.String("active_attribute", false, false, "", &o.cfg.ActiveAttribute)
	cfg.String("active_value", false, false, "", &o.cfg.ActiveValue)
	cfg.String("jwt_key_id_template", false, false, "{kid_url}", &o.cfg.JWTKeyIDTemplate)
	cfg.Custom("jwt_key_table", false, false, func() (interface{}, error) {
		return nil, nil
	}, modconfig.TableDirective, &o.cfg.JWTKeyTable)
	cfg.StringList("jwt_valid_methods", false, false, []string{
		"HS256", "HS384", "HS512",
		"RS256", "RS384", "RS512",
		"ES256", "ES384", "ES512",
		"PS256", "PS384", "PS512",
		"EdDSA",
	}, &o.cfg.JWTValidMethods)
	cfg.StringList("jwt_issuers", false, false, nil, &o.cfg.JWTIssuers)
	cfg.Duration("jwt_expiry_leeway", false, false, 30*time.Second, &o.cfg.JWTExpiryLeeway)
	cfg.Custom("jwt_audience", false, false, func() (interface{}, error) {
		return nil, nil
	}, modconfig.TableDirective, &o.cfg.JWTAudienceTable)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	o.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: o.cfg.TLSClient,
		},
		Timeout: o.cfg.IntrospectionTimeout,
	}

	return nil
}

func (o *OAuth) AuthBearerToken(authCtx *module.AuthContext, username, token string) (string, error) {
	attributes, err := o.cfg.Introspection(o, context.TODO(), token)
	if err != nil {
		return "", fmt.Errorf("auth.oauth: token introspection failed: %w", err)
	}
	if attributes == nil {
		return "", fmt.Errorf("auth.oauth: token introspection returned no attributes")
	}

	o.log.DebugMsg("token attributes", "attributes", attributes)

	if !o.isActive(attributes) {
		return "", fmt.Errorf("auth.oauth: account is disabled")
	}

	if err := o.validateScope(attributes); err != nil {
		return "", err
	}

	username, ok := o.getUsername(attributes, username)
	if !ok {
		return "", fmt.Errorf("auth.oauth: no username found")
	}

	return username, nil
}

func (o *OAuth) IntrospectAuth(ctx context.Context, token string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", o.cfg.IntrospectionURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create introspection request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	return o.introspectHTTP(ctx, req)
}

func (o *OAuth) IntrospectGet(ctx context.Context, token string) (map[string]interface{}, error) {
	introspectURL := o.cfg.IntrospectionURL
	if strings.Contains(introspectURL, "?") {
		introspectURL += url.QueryEscape(token)
	} else {
		introspectURL += url.PathEscape(token)
	}

	req, err := http.NewRequest("GET", introspectURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create introspection request: %w", err)
	}

	return o.introspectHTTP(ctx, req)
}

func (o *OAuth) IntrospectPost(ctx context.Context, token string) (map[string]interface{}, error) {
	form := url.Values{}
	form.Set("token", token)
	req, err := http.NewRequest("POST", o.cfg.IntrospectionURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create introspection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return o.introspectHTTP(ctx, req)
}

func (o *OAuth) introspectHTTP(ctx context.Context, req *http.Request) (map[string]interface{}, error) {
	for k, v := range o.cfg.AdditionalHTTPHeaders {
		req.Header[k] = append(req.Header[k], v...)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			o.log.Error("failed to close response body", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http: %s", resp.Status)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		return nil, fmt.Errorf("content-type is not application/json")
	}

	var attributes map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&attributes); err != nil {
		return nil, fmt.Errorf("decode response json: %w", err)
	}

	return attributes, nil
}

func (o *OAuth) IntrospectLocal(ctx context.Context, token string) (map[string]interface{}, error) {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods(o.cfg.JWTValidMethods),
		jwt.WithLeeway(o.cfg.JWTExpiryLeeway),
	}

	parsedToken, err := jwt.ParseWithClaims(
		token, jwt.MapClaims{}, o.getJWTKey,
		opts...,
	)
	if err != nil {
		return nil, module.BearerTokenError{
			Err:           fmt.Errorf("jwt: %w", err),
			Status:        "invalid_token",
			OIDCConfigURL: o.cfg.OIDCDiscoveryURL,
		}
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		panic("unexpected claims type")
	}

	if len(o.cfg.JWTIssuers) != 0 {
		iss, err := claims.GetIssuer()
		if err != nil {
			return nil, fmt.Errorf("get issuer: %w", err)
		}
		if iss == "" {
			return nil, module.BearerTokenError{
				Err:           fmt.Errorf("auth.oauth: jwt iss is required"),
				Status:        "invalid_token",
				OIDCConfigURL: o.cfg.OIDCDiscoveryURL,
			}
		}
		var ok bool
		for _, validIss := range o.cfg.JWTIssuers {
			if strings.EqualFold(validIss, iss) {
				ok = true
				break
			}
		}
		if !ok {
			return nil, module.BearerTokenError{
				Err:           fmt.Errorf("auth.oauth: jwt issuer '%s' is invalid", iss),
				Status:        "invalid_token",
				OIDCConfigURL: o.cfg.OIDCDiscoveryURL,
			}
		}
	}

	if o.cfg.JWTAudienceTable != nil {
		audClaim, err := claims.GetAudience()
		if err != nil || len(audClaim) == 0 {
			return nil, module.BearerTokenError{
				Err:           fmt.Errorf("auth.oauth: jwt audience claim is required but missing or invalid"),
				Status:        "invalid_token",
				OIDCConfigURL: o.cfg.OIDCDiscoveryURL,
			}
		}
		for _, aud := range audClaim {
			_, ok, err := o.cfg.JWTAudienceTable.Lookup(ctx, aud)
			if err != nil {
				return nil, fmt.Errorf("auth.oauth: jwt audience table error: %w", err)
			}
			if !ok {
				return nil, module.BearerTokenError{
					Err:           fmt.Errorf("auth.oauth: jwt audience '%s' is invalid", aud),
					Status:        "invalid_token",
					OIDCConfigURL: o.cfg.OIDCDiscoveryURL,
				}
			}
		}
	}

	return claims, nil
}

func (o *OAuth) getJWTKey(token *jwt.Token) (any, error) {
	keyID := o.getKeyID(token)
	if keyID == "" {
		return nil, fmt.Errorf("malformed token: no key ID found")
	}

	cachedKey, ok := o.keyCache.Get(keyID)
	if ok {
		return cachedKey, nil
	}

	keyText, ok, err := o.cfg.JWTKeyTable.Lookup(context.TODO(), keyID)
	if err != nil {
		return nil, fmt.Errorf("lookup JWT key as %s: %w", keyID, err)
	}
	if !ok {
		return nil, fmt.Errorf("no matching JWT key found for %s", keyID)
	}

	var key any

	switch token.Method {
	case jwt.SigningMethodHS256, jwt.SigningMethodHS384, jwt.SigningMethodHS512:
		key = []byte(keyText)
	case jwt.SigningMethodES256, jwt.SigningMethodES384, jwt.SigningMethodES512:
		key, err = jwt.ParseECPublicKeyFromPEM([]byte(keyText))
	case jwt.SigningMethodPS256, jwt.SigningMethodPS384, jwt.SigningMethodPS512:
		fallthrough
	case jwt.SigningMethodRS256, jwt.SigningMethodRS384, jwt.SigningMethodRS512:
		key, err = jwt.ParseRSAPublicKeyFromPEM([]byte(keyText))
	case jwt.SigningMethodEdDSA:
		key, err = jwt.ParseEdPublicKeyFromPEM([]byte(keyText))
	default:
		err = fmt.Errorf("unsupported signing method: %s", token.Method.Alg())
	}
	if err != nil {
		return nil, fmt.Errorf("parse JWT key: %w", err)
	}

	o.keyCache.Add(keyID, key)

	return key, nil
}

func (o *OAuth) getKeyID(token *jwt.Token) string {
	if o.cfg.JWTKeyIDTemplate == "" {
		return token.Header["alg"].(string)
	}

	kid, ok := token.Header["kid"].(string)
	if !ok {
		kid = "default"
	}
	alg, ok := token.Header["alg"].(string)
	if !ok {
		// alg is required, short-circuit into invalid value if missing
		return ""
	}
	alg = strings.ToUpper(alg)

	var azp string
	claims, ok := token.Claims.(jwt.MapClaims)
	if ok {
		azp, ok = claims["azp"].(string)
		if !ok {
			azp = "default"
		}
	}

	return strings.NewReplacer(
		"{alg}", alg,
		"{kid}", kid,
		"{kid_url_path}", url.PathEscape(kid),
		"{azp}", azp,
	).Replace(o.cfg.JWTKeyIDTemplate)
}

func (o *OAuth) validateScope(attributes map[string]interface{}) error {
	if len(o.cfg.Scopes) == 0 {
		return nil
	}

	scopeClaim, ok := attributes["scope"].(string)
	if !ok {
		return module.BearerTokenError{
			Err:           fmt.Errorf("auth.oauth: scope attribute is required"),
			Status:        "insufficient_scope",
			Scope:         strings.Join(o.cfg.Scopes, " "),
			OIDCConfigURL: o.cfg.OIDCDiscoveryURL,
		}
	}
	tokenScopes := strings.Fields(scopeClaim)
	for _, requiredScope := range o.cfg.Scopes {
		var ok bool
		for _, tokenScope := range tokenScopes {
			if tokenScope == requiredScope {
				ok = true
				break
			}
		}
		if !ok {
			return module.BearerTokenError{
				Err:           fmt.Errorf("auth.oauth: scope %v is insufficient", tokenScopes),
				Status:        "insufficient_scope",
				Scope:         strings.Join(o.cfg.Scopes, " "),
				OIDCConfigURL: o.cfg.OIDCDiscoveryURL,
			}
		}
	}

	return nil
}

func (o *OAuth) getUsername(attributes map[string]interface{}, requestedUsername string) (string, bool) {
	usernameList, ok := attributes[o.cfg.UsernameAttribute].([]string)
	if ok {
		if requestedUsername == "" {
			return usernameList[0], true
		}
		for _, username := range usernameList {
			if username == requestedUsername {
				return username, true
			}
		}
		return "", false
	}

	username, ok := attributes[o.cfg.UsernameAttribute].(string)
	if !ok {
		return "", false
	}

	if requestedUsername == "" {
		return username, true
	}

	return username, username == requestedUsername
}

func (o *OAuth) isActive(attributes map[string]interface{}) bool {
	if o.cfg.ActiveAttribute == "" {
		return true
	}
	attrValue, ok := attributes[o.cfg.ActiveAttribute]
	if !ok {
		return false
	}

	if o.cfg.ActiveValue == "" {
		switch attrVal := attrValue.(type) {
		case string:
			return attrVal != ""
		case bool:
			return attrVal
		case float64:
			return attrVal != 0
		default:
			return false
		}
	}

	switch attrVal := attrValue.(type) {
	case string:
		return attrVal == o.cfg.ActiveValue
	case bool:
		if strings.EqualFold(o.cfg.ActiveValue, "true") {
			return attrVal
		} else if strings.EqualFold(o.cfg.ActiveValue, "false") {
			return !attrVal
		}
		return false
	case float64:
		activeFloat, err := strconv.ParseFloat(o.cfg.ActiveValue, 64)
		if err != nil {
			return false
		}
		return attrVal == activeFloat
	default:
		return false
	}
}

func init() {
	var _ module.BearerTokenAuth = &OAuth{}

	modules.Register(modName, New)
}
