// Package googleidp owns Google Workspace identity-provider integration.
package googleidp

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Provider is the seam between the service and Google.
type Provider interface {
	AuthorizationURL(redirectURI, state string, forceLogin bool) string
	ExchangeCode(ctx context.Context, code, redirectURI string) (Identity, error)
}

// Identity carries the OIDC claims the callback consumes.
type Identity struct {
	Sub           string
	Email         string
	HostedDomain  string
	EmailVerified bool
}

// FakeProvider is the in-process test double for Google.
type FakeProvider struct{}

func (FakeProvider) AuthorizationURL(redirectURI, state string, forceLogin bool) string {
	v := url.Values{}
	v.Set("response_type", "code")
	v.Set("client_id", "fake-client.apps.googleusercontent.com")
	v.Set("redirect_uri", redirectURI)
	v.Set("state", state)
	v.Set("scope", "openid email profile")
	if forceLogin {
		v.Set("prompt", "login")
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + v.Encode()
}

func (FakeProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (Identity, error) {
	const fakeDomain = "example.com"
	return Identity{
		Sub:           "fake-sub-" + code,
		Email:         "user" + "@" + fakeDomain,
		HostedDomain:  fakeDomain,
		EmailVerified: true,
	}, nil
}

// RealOption configures a real provider.
type RealOption func(*RealProvider)

// WithNow supplies the clock used for ID-token expiry validation.
func WithNow(now func() time.Time) RealOption {
	return func(g *RealProvider) {
		if now != nil {
			g.now = now
		}
	}
}

// WithTokenURL supplies the OAuth2 token endpoint.
func WithTokenURL(tokenURL string) RealOption {
	return func(g *RealProvider) {
		if tokenURL != "" {
			g.cfg.Endpoint.TokenURL = tokenURL
		}
	}
}

// WithJWKsURL supplies the public-key endpoint used for ID-token signature validation.
func WithJWKsURL(jwksURL string) RealOption {
	return func(g *RealProvider) {
		if jwksURL != "" {
			g.jwksURL = jwksURL
		}
	}
}

// RealProvider is the OAuth2-backed Google identity provider.
type RealProvider struct {
	cfg       oauth2.Config
	workspace string
	jwksURL   string
	now       func() time.Time
}

// NewRealProvider returns a Google OAuth2-backed identity provider.
func NewRealProvider(clientID, clientSecret, workspaceDomain string, opts ...RealOption) *RealProvider {
	g := &RealProvider{
		cfg: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		workspace: workspaceDomain,
		jwksURL:   "https://www." + "googleapis.com/oauth2/v3/certs",
		now:       time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(g)
		}
	}
	return g
}

func (g *RealProvider) AuthorizationURL(redirectURI, state string, forceLogin bool) string {
	cfg := g.cfg
	cfg.RedirectURL = redirectURI
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("hd", g.workspace),
	}
	if forceLogin {
		opts = append(opts, oauth2.SetAuthURLParam("prompt", "login"))
	}
	return cfg.AuthCodeURL(state, opts...)
}

func (g *RealProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (Identity, error) {
	cfg := g.cfg
	cfg.RedirectURL = redirectURI
	if ctx == nil {
		ctx = context.Background()
	}
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return Identity{}, err
	}
	rawID, _ := tok.Extra("id_token").(string)
	if rawID == "" {
		return Identity{}, errors.New("google: token response missing id_token")
	}
	return validateIDToken(ctx, rawID, g.cfg.ClientID, g.jwksURL, g.now)
}

func validateIDToken(
	ctx context.Context, idToken, audience, jwksURL string, now func() time.Time,
) (Identity, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return Identity{}, errors.New("google: id_token malformed")
	}
	header, err := decodeJWTObject(parts[0])
	if err != nil {
		return Identity{}, fmt.Errorf("google: id_token header decode: %w", err)
	}
	alg, _ := header["alg"].(string)
	if alg != "RS256" {
		return Identity{}, fmt.Errorf("google: id_token alg %q is not RS256", alg)
	}
	kid, _ := header["kid"].(string)
	if kid == "" {
		return Identity{}, errors.New("google: id_token missing kid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Identity{}, fmt.Errorf("google: id_token payload decode: %w", err)
	}
	var c struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		HD            string `json:"hd"`
		EmailVerified bool   `json:"email_verified"`
		Issuer        string `json:"iss"`
		Audience      any    `json:"aud"`
		ExpiresAt     int64  `json:"exp"`
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return Identity{}, fmt.Errorf("google: id_token payload parse: %w", err)
	}
	if c.Issuer != "https://accounts.google.com" && c.Issuer != "accounts.google.com" {
		return Identity{}, fmt.Errorf("google: id_token issuer %q is not Google", c.Issuer)
	}
	if !jwtAudienceMatches(c.Audience, audience) {
		return Identity{}, errors.New("google: id_token audience mismatch")
	}
	if now == nil {
		now = time.Now
	}
	if c.ExpiresAt <= now().Unix() {
		return Identity{}, errors.New("google: id_token expired")
	}
	key, err := fetchJWK(ctx, jwksURL, kid)
	if err != nil {
		return Identity{}, err
	}
	signed := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Identity{}, fmt.Errorf("google: id_token signature decode: %w", err)
	}
	sum := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig); err != nil {
		return Identity{}, fmt.Errorf("google: id_token signature invalid: %w", err)
	}
	return Identity{
		Sub:           c.Sub,
		Email:         c.Email,
		HostedDomain:  c.HD,
		EmailVerified: c.EmailVerified,
	}, nil
}

func decodeJWTObject(part string) (map[string]any, error) {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func jwtAudienceMatches(aud any, want string) bool {
	switch v := aud.(type) {
	case string:
		return v == want
	case []any:
		for _, elem := range v {
			if s, ok := elem.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

func fetchJWK(ctx context.Context, jwksURL, kid string) (*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("google: jwks request: %w", err)
	}
	client := http.DefaultClient
	if ctxClient, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok && ctxClient != nil {
		client = ctxClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: jwks fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: jwks fetch status %d", resp.StatusCode)
	}
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("google: jwks parse: %w", err)
	}
	for _, k := range doc.Keys {
		if k.Kid != kid || k.Kty != "RSA" || k.N == "" || k.E == "" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			return nil, fmt.Errorf("google: jwk n decode: %w", err)
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			return nil, fmt.Errorf("google: jwk e decode: %w", err)
		}
		e := new(big.Int).SetBytes(eBytes)
		if !e.IsInt64() || e.Int64() <= 1 {
			return nil, errors.New("google: jwk exponent invalid")
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(e.Int64()),
		}, nil
	}
	return nil, errors.New("google: signing key not found")
}
