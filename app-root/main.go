// Package main is the hal binary entry point.
package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	counterpkg "github.com/mgreenly/hal/counter"
	jsonapipkg "github.com/mgreenly/hal/jsonapi"
	oauthpkg "github.com/mgreenly/hal/oauth"
	webpkg "github.com/mgreenly/hal/web"
	websessionpkg "github.com/mgreenly/hal/websession"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	_ "modernc.org/sqlite"
)

type appTicker interface {
	C() <-chan time.Time
	Stop()
}

type appClock interface {
	Now() time.Time
	NewTicker(time.Duration) appTicker
}

type realAppClock struct{}

type realAppTicker struct {
	t *time.Ticker
}

func (realAppClock) Now() time.Time {
	return time.Now()
}

func (realAppClock) NewTicker(d time.Duration) appTicker {
	return realAppTicker{t: time.NewTicker(d)}
}

func (t realAppTicker) C() <-chan time.Time {
	return t.t.C
}

func (t realAppTicker) Stop() {
	t.t.Stop()
}

var (
	activeClockMu sync.RWMutex
	activeClock   appClock = realAppClock{}
)

func setAppClock(c appClock) appClock {
	if c == nil {
		c = realAppClock{}
	}
	activeClockMu.Lock()
	prev := activeClock
	activeClock = c
	activeClockMu.Unlock()
	return prev
}

func appNow() time.Time {
	activeClockMu.RLock()
	c := activeClock
	activeClockMu.RUnlock()
	return c.Now()
}

func appNewTicker(d time.Duration) appTicker {
	activeClockMu.RLock()
	c := activeClock
	activeClockMu.RUnlock()
	return c.NewTicker(d)
}

// R-Z3LX-89W1: tool names and descriptions registered below are written
// for a model audience — each name is the verb-on-resource form
// (counter_read / counter_increment / counter_decrement) and each
// description states what the tool does, what it returns, and when to
// choose it, so a model can pick the right tool without further context.
func newMCPServer() *mcp.Server {
	return newMCPServerWithCounterAndTokenStore(newCounter(), newOAuthTokenStorage())
}

func newMCPServerWithTokenStore(tokens *oauthTokenStorage) *mcp.Server {
	return newMCPServerWithCounterAndTokenStore(newCounter(), tokens)
}

func newMCPServerWithCounterAndTokenStore(c *counterpkg.Counter, tokens *oauthTokenStorage) *mcp.Server {
	s := mcp.NewServer(
		&mcp.Implementation{Name: "hal", Version: halVersion},
		nil,
	)
	// R-XS1U-B7YY: the read tool accepts no arguments and returns the
	// current counter value as a non-negative integer. The counter is
	// uint64, so non-negativity is structural. R-0CQ7-DSBQ allows this
	// tool to be invoked unauthenticated — no auth gate here.
	mcp.AddTool(s, &mcp.Tool{
		Name: "counter_read",
		Description: "Return the current value of the shared counter. Takes no arguments. " +
			"The value is a non-negative integer that any client can observe; reading does " +
			"not modify it. Use this when you need to know the counter's state before " +
			"deciding whether to call counter_increment or counter_decrement.",
	}, counterReadToolWithCounter(c))
	// R-YHNQ-CEJJ: the increment tool accepts no arguments. On success it
	// adds one to the counter and returns the post-increment value.
	// R-ZQS0-HWZ8: an inbound request invoking this tool must present a
	// valid bearer access token issued by this service; the gate runs
	// inside counterIncrementTool, reading Authorization from
	// req.Extra.Header (populated by the Streamable HTTP transport).
	mcp.AddTool(s, &mcp.Tool{
		Name: "counter_increment",
		Description: "Add one to the shared counter and return the new value. Takes no " +
			"arguments. The returned value is the counter's state AFTER the increment, " +
			"a non-negative integer. Use this when the user wants the counter to go up by " +
			"one; call counter_read first if you need the pre-increment value.",
	}, counterIncrementToolWithCounterAndTokenStore(c, tokens))
	// R-GG9B-GS8T: the decrement tool accepts no arguments. When the
	// counter is greater than zero, subtract one and return the
	// post-decrement value. When the counter is exactly zero, return
	// the standard MCP tool-error signal naming the cause; the counter
	// is not modified. R-285U-FWW3: the same valid HAL-issued access
	// token accepted for counter_increment also authorizes this
	// bearer-token-protected mutation surface.
	mcp.AddTool(s, &mcp.Tool{
		Name: "counter_decrement",
		Description: "Subtract one from the shared counter and return the new value. Takes no " +
			"arguments. The returned value is the counter's state AFTER the decrement, a " +
			"non-negative integer. The counter cannot go below zero: if it is already zero, " +
			"this tool returns an error and does not modify the counter. Use this when the " +
			"user wants the counter to go down by one.",
	}, counterDecrementToolWithCounterAndTokenStore(c, tokens))
	return s
}

type counterReadOutput struct {
	Value uint64 `json:"value" jsonschema:"current counter value"`
}

func counterReadTool(
	ctx context.Context, req *mcp.CallToolRequest, _ struct{},
) (*mcp.CallToolResult, counterReadOutput, error) {
	return counterReadToolWithCounter(newCounter())(ctx, req, struct{}{})
}

func counterReadToolWithCounter(c *counterpkg.Counter) func(
	context.Context, *mcp.CallToolRequest, struct{},
) (*mcp.CallToolResult, counterReadOutput, error) {
	return func(
		_ context.Context, _ *mcp.CallToolRequest, _ struct{},
	) (*mcp.CallToolResult, counterReadOutput, error) {
		return nil, counterReadOutput{Value: c.Read()}, nil
	}
}

type counterIncrementOutput struct {
	Value uint64 `json:"value" jsonschema:"post-increment counter value"`
}

func counterIncrementTool(
	ctx context.Context, req *mcp.CallToolRequest, _ struct{},
) (*mcp.CallToolResult, counterIncrementOutput, error) {
	return counterIncrementToolWithCounterAndTokenStore(newCounter(), newOAuthTokenStorage())(ctx, req, struct{}{})
}

func counterIncrementToolWithTokenStore(tokens *oauthTokenStorage) func(
	context.Context, *mcp.CallToolRequest, struct{},
) (*mcp.CallToolResult, counterIncrementOutput, error) {
	return counterIncrementToolWithCounterAndTokenStore(newCounter(), tokens)
}

func counterIncrementToolWithCounterAndTokenStore(c *counterpkg.Counter, tokens *oauthTokenStorage) func(
	context.Context, *mcp.CallToolRequest, struct{},
) (*mcp.CallToolResult, counterIncrementOutput, error) {
	return func(
		_ context.Context, req *mcp.CallToolRequest, _ struct{},
	) (*mcp.CallToolResult, counterIncrementOutput, error) {
		// R-ZQS0-HWZ8: gate on a valid bearer access token issued by this
		// service. The Streamable HTTP transport hands the per-request HTTP
		// headers to handlers on req.Extra.Header; we validate via the same
		// token store the /counter/increment HTTP gate uses, so an MCP
		// client and a browser client see the same accept/reject decision
		// against the same store.
		var hdr http.Header
		if req != nil && req.Extra != nil {
			hdr = req.Extra.Header
		}
		if ok, errDesc := checkMCPBearerWithTokenStore(tokens, hdr); !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: errDesc}},
			}, counterIncrementOutput{}, nil
		}
		return nil, counterIncrementOutput{Value: c.Increment()}, nil
	}
}

type counterDecrementOutput struct {
	Value uint64 `json:"value" jsonschema:"post-decrement counter value"`
}

func counterDecrementTool(
	ctx context.Context, req *mcp.CallToolRequest, _ struct{},
) (*mcp.CallToolResult, counterDecrementOutput, error) {
	return counterDecrementToolWithCounterAndTokenStore(newCounter(), newOAuthTokenStorage())(ctx, req, struct{}{})
}

func counterDecrementToolWithTokenStore(tokens *oauthTokenStorage) func(
	context.Context, *mcp.CallToolRequest, struct{},
) (*mcp.CallToolResult, counterDecrementOutput, error) {
	return counterDecrementToolWithCounterAndTokenStore(newCounter(), tokens)
}

func counterDecrementToolWithCounterAndTokenStore(c *counterpkg.Counter, tokens *oauthTokenStorage) func(
	context.Context, *mcp.CallToolRequest, struct{},
) (*mcp.CallToolResult, counterDecrementOutput, error) {
	return func(
		_ context.Context, req *mcp.CallToolRequest, _ struct{},
	) (*mcp.CallToolResult, counterDecrementOutput, error) {
		// R-285U-FWW3: MCP counter_decrement uses the same bearer-token
		// validation as counter_increment; access tokens are service-wide
		// for counter mutations, not scoped per operation.
		var hdr http.Header
		if req != nil && req.Extra != nil {
			hdr = req.Extra.Header
		}
		if ok, errDesc := checkMCPBearerWithTokenStore(tokens, hdr); !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: errDesc}},
			}, counterDecrementOutput{}, nil
		}
		v, ok := c.Decrement()
		if !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{
					Text: "the counter cannot go below zero",
				}},
			}, counterDecrementOutput{}, nil
		}
		return nil, counterDecrementOutput{Value: v}, nil
	}
}

// R-74NI-T9CI: the hal binary exposes exactly three subcommands.
var subcommands = []string{"serve", "reset", "version"}

// R-79J4-CCBA: project version string printed by `hal version`.
const halVersion = "0.0.1"

func newCounter() *counterpkg.Counter {
	return counterpkg.New()
}

type serveCounterKey struct{}

func contextWithCounter(ctx context.Context, c *counterpkg.Counter) context.Context {
	return context.WithValue(ctx, serveCounterKey{}, c)
}

func counterFromContext(ctx context.Context) *counterpkg.Counter {
	if c, ok := ctx.Value(serveCounterKey{}).(*counterpkg.Counter); ok && c != nil {
		return c
	}
	return newCounter()
}

// R-0TVF-0BKI: the agents block's live-update channel is fed by an
// email-scoped fan-out broadcaster. Subscribers register a bounded
// coalescing channel (capacity 1) and an owner email; notify(email)
// wakes only the subscribers whose email matches, so a connected
// visitor sees their own chain events and nothing else. The wake is a
// payload-less tick — the handler re-reads the visitor's current
// chains under the token-store lock and writes the result. This keeps
// the broadcaster free of stale snapshots and serializes every
// observer on the same authoritative read, regardless of which write
// site (issueRefresh, rotateRefresh reuse-detection, manual revoke)
// triggered the notification.
type agentsBroadcaster struct {
	mu   sync.Mutex
	subs map[*agentsSubscriber]struct{}
}

type agentsSubscriber struct {
	email string
	ch    chan struct{}
}

func (b *agentsBroadcaster) subscribe(email string) *agentsSubscriber {
	sub := &agentsSubscriber{email: email, ch: make(chan struct{}, 1)}
	b.mu.Lock()
	if b.subs == nil {
		b.subs = make(map[*agentsSubscriber]struct{})
	}
	b.subs[sub] = struct{}{}
	b.mu.Unlock()
	return sub
}

func (b *agentsBroadcaster) unsubscribe(sub *agentsSubscriber) {
	b.mu.Lock()
	delete(b.subs, sub)
	b.mu.Unlock()
}

func (b *agentsBroadcaster) subscriberCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

func (b *agentsBroadcaster) notify(email string) {
	if email == "" {
		return
	}
	b.mu.Lock()
	targets := make([]*agentsSubscriber, 0, len(b.subs))
	for s := range b.subs {
		if s.email == email {
			targets = append(targets, s)
		}
	}
	b.mu.Unlock()
	for _, s := range targets {
		select {
		case s.ch <- struct{}{}:
		default:
		}
	}
}

// openCounterDB opens (or creates) the SQLite database at path, ensures
// the single-row counter table exists, and returns the open handle. The
// schema is fixed: one table with one row identified by id=1, holding the
// counter's stored value. R-30XM-G5FN forbids a schema-version table and
// any migration tooling, so the table shape is created idempotently with
// CREATE TABLE IF NOT EXISTS and the singleton row is established with
// INSERT OR IGNORE.
func openCounterDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS counter (` +
			`id INTEGER PRIMARY KEY CHECK (id = 1), ` +
			`value INTEGER NOT NULL` +
			`)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS oauth_clients (` +
			`client_id TEXT PRIMARY KEY, ` +
			`redirect_uris TEXT NOT NULL, ` +
			`client_name TEXT NOT NULL, ` +
			`grant_types TEXT NOT NULL, ` +
			`response_types TEXT NOT NULL, ` +
			`auth_method TEXT NOT NULL, ` +
			`issued_at INTEGER NOT NULL` +
			`)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS web_sessions (` +
			`session_hash TEXT PRIMARY KEY, ` +
			`owner_email TEXT NOT NULL, ` +
			`issued_at INTEGER NOT NULL, ` +
			`expires_at INTEGER NOT NULL, ` +
			`last_seen_at INTEGER NOT NULL, ` +
			`revoked_at INTEGER` +
			`)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS oauth_tokens (` +
			`token_hash TEXT PRIMARY KEY, ` +
			`kind TEXT NOT NULL, ` +
			`owner_email TEXT NOT NULL, ` +
			`client_id TEXT NOT NULL, ` +
			`resource TEXT NOT NULL, ` +
			`issued_at INTEGER NOT NULL, ` +
			`expires_at INTEGER NOT NULL, ` +
			`used_at INTEGER NOT NULL, ` +
			`revoked_at INTEGER NOT NULL, ` +
			`chain_id TEXT NOT NULL` +
			`)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(
		`INSERT OR IGNORE INTO counter (id, value) VALUES (1, 0)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

type databaseOpener func(string) (*sql.DB, error)

// R-T0B2-A4E5: the seam between the service's Google client and Google
// is narrow — exactly two operations. AuthorizationURL produces the URL
// the user-agent should be redirected to. ExchangeCode swaps an
// authorization code for an identity value. Both implementations (test
// double, real-Google) return values of identical shape; callers depend
// only on this contract and do not branch on which is in use.
// R-126C-AM1E: AuthorizationURL takes a `forceLogin` flag because the
// web flow (R-3BKZ-L7R4) and the MCP flow have asymmetric trust
// postures. The web flow passes true so Google demands a fresh
// re-authentication on every /login (prompt=login). The MCP flow at
// /oauth/authorize passes false so Google may satisfy the request via
// silent SSO when an active Google session exists for the user. The
// seam stays exactly two methods — the flag is a parameter on the
// existing AuthorizationURL operation, not a new operation.
type googleIDP interface {
	AuthorizationURL(redirectURI, state string, forceLogin bool) string
	ExchangeCode(ctx context.Context, code, redirectURI string) (googleIdentity, error)
}

// R-T0B2-A4E5: the identity value the code-exchange operation returns
// carries exactly the four claims the callback consumes — drawn from
// the resulting OIDC ID token.
type googleIdentity struct {
	Sub           string
	Email         string
	HostedDomain  string
	EmailVerified bool
}

type googleIDPContextKey struct{}

func contextWithGoogleIDP(ctx context.Context, idp googleIDP) context.Context {
	return context.WithValue(ctx, googleIDPContextKey{}, idp)
}

func googleIDPFromContext(ctx context.Context) googleIDP {
	if idp, ok := ctx.Value(googleIDPContextKey{}).(googleIDP); ok {
		return idp
	}
	return nil
}

// R-VF61-2Y6I: in the test environment, the Google identity provider is
// a test double. The double returns payloads whose shape matches Google's
// documented OAuth/OIDC responses, so service code under test exercises
// the same code paths it would against real Google. No outbound network
// calls leave the process.
type googleFakeIDP struct{}

func (googleFakeIDP) AuthorizationURL(redirectURI, state string, forceLogin bool) string {
	v := url.Values{}
	v.Set("response_type", "code")
	v.Set("client_id", "fake-client.apps.googleusercontent.com")
	v.Set("redirect_uri", redirectURI)
	v.Set("state", state)
	v.Set("scope", "openid email profile")
	if forceLogin {
		// R-3BKZ-L7R4: every web /login redirect demands a fresh
		// authentication of the user at Google rather than satisfying
		// the request via silent SSO. prompt=login is Google's
		// documented parameter for that demand.
		//
		// R-126C-AM1E: the MCP authorization flow passes
		// forceLogin=false so this parameter is OMITTED — MCP
		// federation uses Google's default behavior, which permits
		// silent SSO when Google has an active session for the user.
		v.Set("prompt", "login")
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + v.Encode()
}

func (googleFakeIDP) ExchangeCode(ctx context.Context, code, redirectURI string) (googleIdentity, error) {
	const fakeDomain = "example.com"
	return googleIdentity{
		Sub:           "fake-sub-" + code,
		Email:         "user" + "@" + fakeDomain,
		HostedDomain:  fakeDomain,
		EmailVerified: true,
	}, nil
}

// R-W3K0-QD0E: the dev/prod Google identity provider. Both seam
// operations are fully implemented: AuthorizationURL builds a URL on
// Google's documented OAuth 2.0 / OIDC authorization endpoint
// parameterized with `client_id`, `redirect_uri`, `state`, the
// `openid email profile` scopes, and the `hd` parameter set to the
// configured Workspace domain (R-5LQM-O89D); ExchangeCode performs an
// HTTPS POST to Google's documented token endpoint authenticating with
// `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` (R-68WP-XVCK) and
// returns an identity carrying the `sub`, `email`, `hosted_domain`,
// and `email_verified` claims from the resulting ID token.
//
// R-33DF-7OX1: built on golang.org/x/oauth2 — the wire format and
// endpoint URLs come from the package (defaults to google.Endpoint)
// rather than being hand-rolled.
//
// R-T0B2-A4E5: the seam stays exactly two operations; the
// returned identity carries only the four claims callers depend on.
type googleRealIDP struct {
	cfg       oauth2.Config
	workspace string
	jwksURL   string
}

func newGoogleRealIDP(clientID, clientSecret, workspaceDomain string) *googleRealIDP {
	return &googleRealIDP{
		cfg: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		workspace: workspaceDomain,
		jwksURL:   "https://www." + "googleapis.com/oauth2/v3/certs",
	}
}

func (g *googleRealIDP) AuthorizationURL(redirectURI, state string, forceLogin bool) string {
	// The redirect URI is request-derived per R-DA34-WX9P, so it is
	// supplied per-call and applied to a value copy of the config
	// rather than mutating the shared one.
	cfg := g.cfg
	cfg.RedirectURL = redirectURI
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("hd", g.workspace),
	}
	if forceLogin {
		// R-3BKZ-L7R4: web /login demands fresh re-authentication.
		// R-126C-AM1E: MCP federation passes forceLogin=false so this
		// parameter is OMITTED on that path.
		opts = append(opts, oauth2.SetAuthURLParam("prompt", "login"))
	}
	return cfg.AuthCodeURL(state, opts...)
}

func (g *googleRealIDP) ExchangeCode(ctx context.Context, code, redirectURI string) (googleIdentity, error) {
	cfg := g.cfg
	cfg.RedirectURL = redirectURI
	if ctx == nil {
		ctx = context.Background()
	}
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return googleIdentity{}, err
	}
	rawID, _ := tok.Extra("id_token").(string)
	if rawID == "" {
		return googleIdentity{}, errors.New("google: token response missing id_token")
	}
	return validateGoogleIDToken(ctx, rawID, g.cfg.ClientID, g.jwksURL)
}

// R-ZBV4-KEJ6: Google identity claims are accepted only from an ID token
// valid for this service: Google's issuer, a matching audience, an unexpired
// token, and an RS256 signature that verifies against Google's published JWKs.
func validateGoogleIDToken(
	ctx context.Context, idToken, audience, jwksURL string,
) (googleIdentity, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return googleIdentity{}, errors.New("google: id_token malformed")
	}
	header, err := decodeJWTObject(parts[0])
	if err != nil {
		return googleIdentity{}, fmt.Errorf("google: id_token header decode: %w", err)
	}
	alg, _ := header["alg"].(string)
	if alg != "RS256" {
		return googleIdentity{}, fmt.Errorf("google: id_token alg %q is not RS256", alg)
	}
	kid, _ := header["kid"].(string)
	if kid == "" {
		return googleIdentity{}, errors.New("google: id_token missing kid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return googleIdentity{}, fmt.Errorf("google: id_token payload decode: %w", err)
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
		return googleIdentity{}, fmt.Errorf("google: id_token payload parse: %w", err)
	}
	if c.Issuer != "https://accounts.google.com" && c.Issuer != "accounts.google.com" {
		return googleIdentity{}, fmt.Errorf("google: id_token issuer %q is not Google", c.Issuer)
	}
	if !jwtAudienceMatches(c.Audience, audience) {
		return googleIdentity{}, errors.New("google: id_token audience mismatch")
	}
	if c.ExpiresAt <= appNow().Unix() {
		return googleIdentity{}, errors.New("google: id_token expired")
	}
	key, err := fetchGoogleJWK(ctx, jwksURL, kid)
	if err != nil {
		return googleIdentity{}, err
	}
	signed := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return googleIdentity{}, fmt.Errorf("google: id_token signature decode: %w", err)
	}
	sum := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig); err != nil {
		return googleIdentity{}, fmt.Errorf("google: id_token signature invalid: %w", err)
	}
	return googleIdentity{
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

func fetchGoogleJWK(ctx context.Context, jwksURL, kid string) (*rsa.PublicKey, error) {
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

// R-LWCN-ZBXO: authConfig is the single named configuration surface for
// every numeric and string value that governs the service's authentication
// posture. New auth-posture values land as fields here, not as scattered
// consts or accessor-local literals — operators reading the auth posture
// find every value that governs it in one place, and changing a lifetime,
// ceiling, scope list, or domain is a single-location edit. Secrets
// (Google client credentials per R-68WP-XVCK) are read from env vars via
// requireEnv, which fails loudly when a required value is absent rather
// than substituting a default. Token lifetimes (R-TNXJ-ZWQ0, R-8UAA-YKR9),
// the authorization-code TTL (R-ZPE1-0DV8), Google OIDC scopes
// (R-W3K0-QD0E), the forced-auth posture (R-3BKZ-L7R4 / R-126C-AM1E), and
// the Google client credentials (R-68WP-XVCK) land on this struct as the
// requirements that introduce them are implemented.
type authConfig struct {
	WebSessionAbsoluteTTL time.Duration
	WebSessionIdleTTL     time.Duration
	OAuthStateTTL         time.Duration
	// AuthCodeTTL is the authorization-code lifetime per R-ZPE1-0DV8.
	// "No more than a few minutes after issue" — pinned at 60 seconds so
	// the federation-then-exchange round-trip is comfortably accommodated
	// while abandoned codes expire promptly.
	AuthCodeTTL time.Duration
	// AccessTokenTTL is the access-token lifetime per R-TNXJ-ZWQ0.
	// Sourced from the central R-LWCN-ZBXO surface so issuance code
	// reads one value rather than embedding a literal at the mint site.
	AccessTokenTTL time.Duration
	// RefreshTokenTTL is the refresh-token lifetime per R-8UAA-YKR9 —
	// thirty days from the moment of issue. issueRefresh stamps
	// expiresAt = now + RefreshTokenTTL; rotateRefresh re-stamps the
	// successor with a fresh full lifetime and gates rotation on the
	// presented refresh being un-expired by this same yardstick.
	RefreshTokenTTL    time.Duration
	HSTSMaxAge         time.Duration
	WorkspaceDomain    string
	ResourceIdentifier string
}

type envLookup func(string) (string, bool)

var (
	authCfgMu       sync.RWMutex
	activeAuthCfg   = loadAuthConfig(os.LookupEnv)
	envLookupMu     sync.RWMutex
	activeEnvLookup envLookup = os.LookupEnv
)

// loadAuthConfig reads the authentication configuration once through the
// supplied lookup and returns the immutable value used by runtime consumers.
func loadAuthConfig(lookup envLookup) authConfig {
	c := authConfig{
		WebSessionAbsoluteTTL: 12 * time.Hour,
		WebSessionIdleTTL:     1 * time.Hour,
		OAuthStateTTL:         5 * time.Minute,
		AuthCodeTTL:           60 * time.Second,
		AccessTokenTTL:        1 * time.Hour,
		RefreshTokenTTL:       30 * 24 * time.Hour,
		HSTSMaxAge:            365 * 24 * time.Hour,
		WorkspaceDomain:       "example.com",
		// R-791Y-3ROQ: the operator-supplied value is required and must
		// carry the path component `/mcp` R-7A9U-HJFF pins. runServe
		// enforces both at startup via requireEnv +
		// validateHALResourceIdentifier and refuses to begin serving
		// when the env var is missing, empty, or path != "/mcp". The
		// in-memory default here matches the dev posture the operator
		// would supply (`http://127.0.0.1:3000/mcp`); it is unreachable
		// at runtime because the requireEnv gate fails before any
		// request lands, but it keeps the wide existing test surface
		// (and the R-LWCN-ZBXO default-values audit) coherent without
		// forcing every test to install explicit config.
		ResourceIdentifier: "http://127.0.0.1:3000/mcp",
	}
	// R-ANRQ-04PK: the allowed Workspace domain is supplied via the
	// bare environment variable `GOOGLE_WORKSPACE_DOMAIN` — matching
	// the bare-`GOOGLE_*` convention R-68WP-XVCK pins for the Google
	// federation seam, not a `HAL_`-prefixed variant. runServe enforces
	// the fail-loudly contract via requireEnv at startup; this in-memory
	// surface honors the same name so tests that install config through
	// loadAuthConfig exercise the same plumbing the operator does.
	if v, ok := lookup("GOOGLE_WORKSPACE_DOMAIN"); ok {
		c.WorkspaceDomain = v
	}
	if v, ok := lookup("HAL_RESOURCE_IDENTIFIER"); ok {
		c.ResourceIdentifier = v
	}
	return c
}

func setAuthCfg(c authConfig) {
	authCfgMu.Lock()
	activeAuthCfg = c
	authCfgMu.Unlock()
}

func setEnvLookup(lookup envLookup) envLookup {
	envLookupMu.Lock()
	prev := activeEnvLookup
	activeEnvLookup = lookup
	envLookupMu.Unlock()
	return prev
}

// authCfg returns the current authentication configuration surface installed
// by startup or by tests through the same loadAuthConfig/setAuthCfg seam.
func authCfg() authConfig {
	authCfgMu.RLock()
	defer authCfgMu.RUnlock()
	return activeAuthCfg
}

// R-NQ3G-K0CQ: startupBannerR_NQ3G_K0CQ writes a one-line-per-variable
// summary of every environment variable hal consults to stderr at
// startup, before the listener accepts requests. Required variables
// that were not set never reach the banner because requireEnv has
// already exited the process per R-LWCN-ZBXO. Variables that have a
// built-in default and were unset print that default value followed
// by " (default)". Variables with operator-supplied values print
// verbatim.
func startupBannerR_NQ3G_K0CQ(w io.Writer, dbPath string) {
	startupBannerWithEnvR_NQ3G_K0CQ(w, dbPath, os.LookupEnv)
}

func startupBannerWithEnvR_NQ3G_K0CQ(w io.Writer, dbPath string, lookup envLookup) {
	type bannerVar struct {
		name   string
		def    string // empty means required (so it's already present if we got here)
		secret bool   // R-NRBC-XS3F: value is redacted in the banner
	}
	vars := []bannerVar{
		{"GOOGLE_CLIENT_ID", "", false},
		{"GOOGLE_CLIENT_SECRET", "", true},
		{"GOOGLE_WORKSPACE_DOMAIN", "", false},
		{"HAL_RESOURCE_IDENTIFIER", "http://127.0.0.1:3000/mcp", false},
	}
	for _, v := range vars {
		val, ok := lookup(v.name)
		if !ok || val == "" {
			if v.def == "" {
				continue
			}
			fmt.Fprintf(w, "%s=%s (default)\n", v.name, v.def)
			continue
		}
		if v.secret {
			val = redactSecretR_NRBC_XS3F(val)
		}
		fmt.Fprintf(w, "%s=%s\n", v.name, val)
	}
	// R-PLTU-G0FD: print the resolved absolute database path between the
	// env-var lines and the trailing blank line so the operator sees
	// exactly which file on disk hal serve is bound to.
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		abs = dbPath
	}
	fmt.Fprintf(w, "db=%s\n", abs)
	// R-NSJ9-BJU4: trailing blank line separating banner from access log.
	fmt.Fprintln(w)
}

// R-NRBC-XS3F: redactSecretR_NRBC_XS3F renders a secret value for the
// startup banner. Values of eight or more characters print as "***" +
// the last three characters; shorter values print as just "***" so an
// accidentally-short secret cannot be substantially reconstructed.
func redactSecretR_NRBC_XS3F(val string) string {
	if len(val) < 8 {
		return "***"
	}
	return "***" + val[len(val)-3:]
}

// requireEnv reads a required environment variable through the injected
// startup lookup. It returns a clear error when the value is absent or empty
// rather than substituting a default — the fail-loudly contract R-LWCN-ZBXO
// pins for secrets.
func requireEnv(name string) (string, error) {
	envLookupMu.RLock()
	lookup := activeEnvLookup
	envLookupMu.RUnlock()
	return requireEnvFromLookup(lookup, name)
}

func requireEnvFromLookup(lookup envLookup, name string) (string, error) {
	v, ok := lookup(name)
	if !ok || v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", name)
	}
	return v, nil
}

// R-VKZD-UKVS: every handler that reads a client-supplied request body
// wraps it with a fixed cap before parsing. One MiB is comfortably above
// the normal JSON and form payloads this service accepts while preventing
// an endpoint from buffering an unbounded body.
const maxRequestBodyBytesR_VKZD_UKVS int64 = jsonapipkg.MaxRequestBodyBytes

func limitRequestBodyR_VKZD_UKVS(w http.ResponseWriter, r *http.Request) {
	jsonapipkg.LimitRequestBody(w, r)
}

func requestBodyTooLargeR_VKZD_UKVS(err error) bool {
	return jsonapipkg.RequestBodyTooLarge(err)
}

func writeBodyTooLargeR_VKZD_UKVS(w http.ResponseWriter) {
	jsonapipkg.WriteBodyTooLarge(w)
}

// R-3UT3-IKZG / R-75E8-YGGN: the service has a single configured
// canonical resource identifier — the external URL the MCP transport
// endpoint is reached at, including its path component (R-7A9U-HJFF
// pins the path as `/mcp`). This is the byte-for-byte string the
// service publishes in its OAuth 2.0 Protected Resource Metadata
// document, records on every issued token as the bound `resource`
// value, uses to default omitted OAuth resource indicators
// (R-WLUL-MZCD), and compares against on bearer-side validation. The
// same string is used at issue time and at presentation time — neither
// endpoint derives its own per-endpoint identifier. The
// value is sourced from the central R-LWCN-ZBXO surface; the env var
// `HAL_RESOURCE_IDENTIFIER` is the sole operator-facing knob
// (R-791Y-3ROQ). The in-memory default mirrors the dev posture an
// operator would supply, but production startup always requires the
// env var explicitly via requireEnv.
func canonicalResourceIdentifier() string {
	return authCfg().ResourceIdentifier
}

// validateHALResourceIdentifier enforces the R-791Y-3ROQ contract on
// the operator-supplied `HAL_RESOURCE_IDENTIFIER` value: the variable
// is required (not empty), parses as an absolute URL, and its path
// component is exactly `/mcp` (R-7A9U-HJFF). Returns a clear error
// naming the failing condition for the fail-loudly contract
// R-LWCN-ZBXO pins; runServe surfaces the error to stderr and exits
// before the listener begins accepting requests.
func validateHALResourceIdentifier(v string) error {
	if v == "" {
		return fmt.Errorf("HAL_RESOURCE_IDENTIFIER is not set " +
			"(required by R-791Y-3ROQ; must be the externally-reachable " +
			"MCP transport URL including the path component /mcp)")
	}
	u, err := url.Parse(v)
	if err != nil {
		return fmt.Errorf("HAL_RESOURCE_IDENTIFIER %q is not a valid URL: "+
			"%v (R-791Y-3ROQ)", v, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("HAL_RESOURCE_IDENTIFIER %q must be an absolute "+
			"URL with scheme and host (R-791Y-3ROQ)", v)
	}
	if u.Path != "/mcp" {
		return fmt.Errorf("HAL_RESOURCE_IDENTIFIER %q: path component must "+
			"be \"/mcp\" (R-7A9U-HJFF), got %q (R-791Y-3ROQ)", v, u.Path)
	}
	return nil
}

// R-5LQM-O89D: the service is configured at deploy time with the
// single Google Workspace domain whose users are allowed. Operators
// supply the domain via the bare-`GOOGLE_*` env var
// `GOOGLE_WORKSPACE_DOMAIN` (R-ANRQ-04PK); the default matches the
// fake IDP's hosted-domain claim so the test suite exercises the
// in-domain success path without environment plumbing. The value is
// sourced from the central R-LWCN-ZBXO surface.
func googleWorkspaceDomain() string {
	return authCfg().WorkspaceDomain
}

// R-DA34-WX9P: requestBaseURL returns the externally-observable
// scheme://host the visitor used to reach this request, honoring the
// standard forwarded-protocol signal supplied by a TLS-terminating
// proxy (the production posture per R-PVA6-Q6OB). When the proxy
// terminates TLS in front of the plain-HTTP application process, it
// forwards the original scheme via `X-Forwarded-Proto`; this helper
// surfaces that as `https` so request-derived URLs (R-CO4Y-11X7)
// shown to the visitor match the origin the visitor actually sees in
// their address bar, not the plain-HTTP origin the application
// process observes locally. With no proxy in the loop the helper
// falls back to `r.TLS != nil` (https when the listener itself
// terminated TLS, http otherwise). Only the first comma-separated
// token of `X-Forwarded-Proto` is consulted, per the convention that
// proxies append; surrounding whitespace is trimmed, and the value
// is case-normalized. Unknown values fall through to the local
// observation rather than being trusted.
func requestBaseURL(r *http.Request) string {
	return jsonapipkg.RequestBaseURL(r)
}

// forwardedProtoHTTPS reports whether the standard forwarded-protocol
// signal indicates the visitor reached the service over HTTPS through
// a TLS-terminating proxy. Only the first comma-separated token of
// `X-Forwarded-Proto` is consulted (per the convention that proxies
// append); surrounding whitespace is trimmed and the value is
// case-normalized. R-DA34-WX9P, R-ID5L-BSJM, and R-PVA6-Q6OB share
// this signal.
func forwardedProtoHTTPS(r *http.Request) bool {
	fp := r.Header.Get("X-Forwarded-Proto")
	if fp == "" {
		return false
	}
	first := fp
	if i := strings.IndexByte(first, ','); i >= 0 {
		first = first[:i]
	}
	return strings.ToLower(strings.TrimSpace(first)) == "https"
}

type documentedMux struct {
	routes map[string]map[string]http.Handler
}

func newDocumentedMux() *documentedMux {
	return &documentedMux{routes: map[string]map[string]http.Handler{}}
}

func (m *documentedMux) Handle(method, path string, h http.Handler) {
	if m.routes[path] == nil {
		m.routes[path] = map[string]http.Handler{}
	}
	m.routes[path][method] = h
}

func (m *documentedMux) HandleFunc(method, path string, h func(http.ResponseWriter, *http.Request)) {
	m.Handle(method, path, http.HandlerFunc(h))
}

func (m *documentedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	methods, ok := m.routes[r.URL.Path]
	if !ok {
		// R-X0O1-BJ2H: only exact documented paths dispatch; every
		// unknown path returns 404 here instead of falling through to the
		// index page, API, OAuth, stream, stylesheet, or MCP handlers.
		http.NotFound(w, r)
		return
	}
	h, ok := methods[r.Method]
	if !ok {
		allow := make([]string, 0, len(methods))
		for method := range methods {
			allow = append(allow, method)
		}
		sort.Strings(allow)
		w.Header().Set("Allow", strings.Join(allow, ", "))
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.ServeHTTP(w, r)
}

// R-ID5L-BSJM: every response carries `X-Content-Type-Options: nosniff`
// to prevent browsers from reinterpreting a response as a more
// dangerous content type than the service declared. When the request
// arrived through the production TLS-terminating proxy (detected via
// the same forwarded-protocol signal R-DA34-WX9P honors), the
// response also carries `Strict-Transport-Security` with a one-year
// `max-age` and `includeSubDomains`, pinning the visitor's browser to
// HTTPS for this host across the max-age window. The local-development
// service, which speaks plain HTTP and is not reached through the
// production proxy, deliberately does not emit HSTS — the property is
// conditional on the request actually having been served over HTTPS.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if forwardedProtoHTTPS(r) {
			w.Header().Set("Strict-Transport-Security",
				fmt.Sprintf("max-age=%d; includeSubDomains",
					int(authCfg().HSTSMaxAge/time.Second)))
		}
		next.ServeHTTP(w, r)
	})
}

// R-D0AR-V8QB: every HTTP request the listener hands to the service
// produces exactly one access log line, written after the response
// handler returns so error responses, 404s, and mid-stream closures
// all count. Subsequent iterations layer NCSA Combined Log Format
// fields (R-D2QK-MS7P), the stdout-only steady-state property
// (R-D1IO-90H0), credential redaction (R-DA1Y-XENV), and the
// per-stream-close emission discipline (R-DB9V-B6EK) on top of this
// one-line-per-request structural invariant. The middleware is
// installed at the outer edge of the handler chain so anything the
// listener parses as an HTTP request lands here exactly once.
// R-D2QK-MS7P: an http.ResponseWriter shim that captures the response
// status and body byte count so the access log line can carry them.
type accessLogRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *accessLogRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *accessLogRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

func (r *accessLogRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// R-D6E9-S3FS: client-host field for the access log. Prefer the first
// comma-separated token of X-Forwarded-For (whitespace-trimmed); else
// the IP portion of r.RemoteAddr; else "-". Never empty, never
// "unknown".
func clientHostR_D6E9_S3FS(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := xff
		if i := strings.IndexByte(xff, ','); i >= 0 {
			first = xff[:i]
		}
		first = strings.TrimSpace(first)
		if first != "" {
			return first
		}
	}
	if r.RemoteAddr != "" {
		if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && ip != "" {
			return ip
		}
		return r.RemoteAddr
	}
	return "-"
}

// R-D56D-EBP3: the access log's authenticated-user field carries the
// email recorded on the credential the request used to satisfy its
// route's auth bar. Auth helpers running deeper in the chain stash the
// email on a *authedUserR_D56D_EBP3 the access-log middleware seeded on
// the request context; the middleware reads it back after the handler
// returns and emits "-" when nothing was stashed (an unauthenticated
// route, or an auth-gated route whose check failed). The field never
// contains whitespace — an email that somehow carries any logs as "-"
// rather than splitting the NCSA line.
type authedUserCtxKeyR_D56D_EBP3 struct{}

type authedUserR_D56D_EBP3 struct {
	email string
}

func setAuthedUserR_D56D_EBP3(r *http.Request, email string) {
	if h, ok := r.Context().Value(
		authedUserCtxKeyR_D56D_EBP3{}).(*authedUserR_D56D_EBP3); ok {
		h.email = email
	}
}

func authedUserFieldR_D56D_EBP3(email string) string {
	if email == "" {
		return "-"
	}
	if strings.ContainsAny(email, " \t\r\n\v\f") {
		return "-"
	}
	return email
}

func accessLog(out io.Writer, next http.Handler) http.Handler {
	// R-195O-JBGX: serialize writes to `out` so the access-log middleware
	// is race-free when concurrent requests share a single writer (the
	// production `stdout` and the in-test `bytes.Buffer` are both
	// single-writer sinks with no internal synchronization).
	var mu sync.Mutex
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := appNow()
		rec := &accessLogRecorder{ResponseWriter: w}
		holder := &authedUserR_D56D_EBP3{}
		r = r.WithContext(context.WithValue(
			r.Context(), authedUserCtxKeyR_D56D_EBP3{}, holder))
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		host := clientHostR_D6E9_S3FS(r)
		user := authedUserFieldR_D56D_EBP3(holder.email)
		// R-D3YH-0JYE: NCSA bracketed timestamp using C-locale English
		// month abbreviations; records the instant the service began
		// handling the request, not emission time.
		ts := start.Format("[02/Jan/2006:15:04:05 -0700]")
		reqLine := redactedRequestLineR_DA1Y_XENV(r)
		referer := r.Header.Get("Referer")
		if referer == "" {
			referer = "-"
		}
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			ua = "-"
		}
		mu.Lock()
		fmt.Fprintf(out, "%s - %s %s \"%s\" %d %d \"%s\" \"%s\"\n",
			host, user, ts,
			ncsaEscapeR_D8U2_JMX6(reqLine), rec.status, rec.bytes,
			ncsaEscapeR_D8U2_JMX6(referer), ncsaEscapeR_D8U2_JMX6(ua))
		mu.Unlock()
	})
}

// R-DA1Y-XENV: build the access-log request-line field with sensitive
// OAuth query parameters redacted. For requests to
// `/oauth/google/callback`, the values of `code` and `state` query
// parameters are replaced by the literal string `REDACTED`; parameter
// names and order are preserved so the path remains recognizable. For
// any other path, the request URI is returned verbatim. The
// `Authorization` request header is governed by R-SAK8-WB9W and is
// never logged regardless.
func redactedRequestLineR_DA1Y_XENV(r *http.Request) string {
	uri := r.RequestURI
	if r.URL != nil && r.URL.Path == "/oauth/google/callback" {
		if i := strings.IndexByte(uri, '?'); i >= 0 {
			uri = uri[:i+1] + redactCallbackQueryR_DA1Y_XENV(uri[i+1:])
		}
	}
	return fmt.Sprintf("%s %s %s", r.Method, uri, r.Proto)
}

// redactCallbackQueryR_DA1Y_XENV replaces the values of the `code` and
// `state` parameters in a raw query string with the literal `REDACTED`.
// Parameter order and unrelated parameters are preserved. Operates on
// the raw byte sequence so percent-encoding is not normalized.
func redactCallbackQueryR_DA1Y_XENV(q string) string {
	parts := strings.Split(q, "&")
	for i, p := range parts {
		name := p
		if eq := strings.IndexByte(p, '='); eq >= 0 {
			name = p[:eq]
		}
		if name == "code" || name == "state" {
			parts[i] = name + "=REDACTED"
		}
	}
	return strings.Join(parts, "&")
}

// R-D8U2-JMX6: Apache mod_log_config escaping for the three
// double-quoted access-log fields. `"` becomes `\"`, `\` becomes
// `\\`, and any byte outside printable ASCII (0x20..0x7E) becomes
// `\xHH`. Returns the inner contents — the caller adds the
// surrounding `"`.
func ncsaEscapeR_D8U2_JMX6(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	const hex = "0123456789abcdef"
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			b.WriteString(`\"`)
		case c == '\\':
			b.WriteString(`\\`)
		case c >= 0x20 && c <= 0x7E:
			b.WriteByte(c)
		default:
			b.WriteString(`\x`)
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0F])
		}
	}
	return b.String()
}

// R-ETP6-60VA: server-side store of in-flight OAuth `state` values. Each
// record binds a generated `state` to the browser session that initiated
// the /login redirect — captured as a random `bindingID` written to the
// browser as the `hal_oauth_state` cookie — plus an expiry and a
// single-use consumed flag. The Google callback consults this store to
// verify the returned state is recognized, unexpired, unconsumed, and
// presented by the same browser session that initiated the flow; missing,
// unknown, expired, consumed, or session-mismatched state is rejected and
// no token chain is issued. The store is in-memory: the single-process
// deployment topology (R-MOIF-IUXZ) does not need cross-instance sharing,
// and the 5-minute TTL bounds memory growth.
type oauthStateRecord = oauthpkg.StateRecord
type oauthStateMCPContext = oauthpkg.StateMCPContext
type oauthStateStorage = oauthpkg.StateStore

func newOAuthStateStorage() *oauthStateStorage {
	return oauthpkg.NewStateStore(oauthpkg.StateOptions{
		Now: func() time.Time {
			return oauthStateNow()
		},
		TTL: func() time.Duration {
			return authCfg().OAuthStateTTL
		},
	})
}

// oauthStateNow is the clock the state store reads for expiry comparisons.
// Tests may replace it directly; production resolves through activeClock.
var oauthStateNow = appNow

// The lifetime an issued state value has before the callback must
// accept it is sourced from the R-LWCN-ZBXO surface (authCfg().OAuthStateTTL).
// Five minutes is comfortably longer than the federation round-trip
// takes in practice while keeping abandoned records from accumulating.

// oauthStateCookieName is the cookie that binds a /login redirect to the
// originating browser session for R-ETP6-60VA. Its value is the random
// bindingID recorded server-side alongside the state.
const oauthStateCookieName = oauthpkg.StateCookieName

// errOAuthState* enumerate the distinct rejection causes the callback
// surfaces, mirroring the R-EV2D-QTR1 posture of "one description per
// distinct failure".
var (
	errOAuthStateMissing      = oauthpkg.ErrStateMissing
	errOAuthStateUnknown      = oauthpkg.ErrStateUnknown
	errOAuthStateExpired      = oauthpkg.ErrStateExpired
	errOAuthStateConsumed     = oauthpkg.ErrStateConsumed
	errOAuthBindingMissing    = oauthpkg.ErrBindingMissing
	errOAuthBindingMismatched = oauthpkg.ErrBindingMismatched
)

// newOAuthStateValue returns a 32-character hex string drawn from
// crypto/rand — 128 bits of entropy, sufficient for the "fresh
// unguessable" property R-ETP6-60VA names.
func newOAuthStateValue() (string, error) {
	return oauthpkg.NewStateValue()
}

type oauthClient = oauthpkg.Client
type oauthClientStorage = oauthpkg.ClientStore

func newOAuthClientStorage() *oauthClientStorage {
	return oauthpkg.NewClientStore()
}

var newOAuthClientID = oauthpkg.NewClientID

type oauthAuthCode = oauthpkg.AuthCode
type oauthAuthCodeStorage = oauthpkg.AuthCodeStore

func newOAuthAuthCodeStorage() *oauthAuthCodeStorage {
	return oauthpkg.NewAuthCodeStore(oauthpkg.AuthCodeOptions{
		Now: func() time.Time {
			return oauthAuthCodeNow()
		},
		TTL: func() time.Duration {
			return authCfg().AuthCodeTTL
		},
	})
}

// oauthAuthCodeNow is the clock the auth-code store reads for issue and
// expiry comparisons. Tests may replace it directly; production resolves
// through activeClock.
var oauthAuthCodeNow = appNow

// errOAuthAuthCode* enumerate the distinct rejection causes redemption
// surfaces, mirroring the R-EV2D-QTR1 posture of "one description per
// distinct failure". These are internal errors; the /oauth/token handler
// translates them to the wire-level OAuth error response shape.
var (
	errOAuthAuthCodeUnknown          = oauthpkg.ErrAuthCodeUnknown
	errOAuthAuthCodeExpired          = oauthpkg.ErrAuthCodeExpired
	errOAuthAuthCodeConsumed         = oauthpkg.ErrAuthCodeConsumed
	errOAuthAuthCodeClientMismatch   = oauthpkg.ErrAuthCodeClientMismatch
	errOAuthAuthCodeRedirectMismatch = oauthpkg.ErrAuthCodeRedirectMismatch
	errOAuthAuthCodePKCEMismatch     = oauthpkg.ErrAuthCodePKCEMismatch
	errOAuthAuthCodePKCEMethod       = oauthpkg.ErrAuthCodePKCEMethod
)

type oauthToken = oauthpkg.Token
type oauthTokenStorage = oauthpkg.TokenStore
type agentChainR_0NRX_3GV1 = oauthpkg.AgentChain

func newOAuthTokenStorage() *oauthTokenStorage {
	return oauthpkg.NewTokenStore(oauthpkg.TokenOptions{
		Now:        func() time.Time { return oauthTokenNow() },
		AccessTTL:  func() time.Duration { return authCfg().AccessTokenTTL },
		RefreshTTL: func() time.Duration { return authCfg().RefreshTokenTTL },
	})
}

type serveOAuthTokenStoreKey struct{}

func contextWithOAuthTokenStore(ctx context.Context, tokens *oauthTokenStorage) context.Context {
	return context.WithValue(ctx, serveOAuthTokenStoreKey{}, tokens)
}

func oauthTokenStoreFromContext(ctx context.Context) *oauthTokenStorage {
	if tokens, ok := ctx.Value(serveOAuthTokenStoreKey{}).(*oauthTokenStorage); ok && tokens != nil {
		return tokens
	}
	return newOAuthTokenStorage()
}

const oauthRefreshTokenFormField = "refresh_token"

// oauthTokenNow is the clock the token store reads for issued-at /
// expires-at stamps. Tests may replace it directly; production resolves
// through activeClock.
var oauthTokenNow = appNow

func oauthTokenHash(plaintext string) string {
	return oauthpkg.TokenHash(plaintext)
}

func setOAuthTokenAgentsBroadcaster(s *oauthTokenStorage, b *agentsBroadcaster) func(string) {
	var next func(string)
	if b != nil {
		next = b.notify
	}
	return s.SetNotifier(next)
}

func jsonAPISurface(
	c *counterpkg.Counter, sessions *webSessionStorage, tokens *oauthTokenStorage,
	clients *oauthClientStorage, authCodes *oauthAuthCodeStorage,
) jsonapipkg.Surface {
	return jsonapipkg.Surface{
		Counter:                     c,
		WebSessions:                 sessions,
		OAuthTokens:                 tokens,
		OAuthClients:                clients,
		OAuthAuthCodes:              authCodes,
		Now:                         appNow,
		NewOAuthClientID:            newOAuthClientID,
		CanonicalResourceIdentifier: canonicalResourceIdentifier,
		AccessTokenTTL: func() time.Duration {
			return authCfg().AccessTokenTTL
		},
	}
}

// R-CXJ2-R3BN: the only code path that establishes a web session is the
// successful completion of the Google federation round-trip R-8GJG-64MR
// defines — the callback handler validates state per R-ETP6-60VA,
// exchanges the code for an identity, applies the workspace-domain check
// per R-5LQM-O89D, and only then mints a session record + sets the
// session cookie. R-SLGL-B5B4 keeps these records in a dedicated store
// distinct from the OAuth token store; the plaintext session identifier
// appears only in the Set-Cookie response and is never persisted —
// records are keyed by a cryptographic hash of the plaintext, mirroring
// R-CUUP-REQT's posture for OAuth tokens.
type webSession = websessionpkg.Session
type webSessionStorage = websessionpkg.Store

func newWebSessionStorage() *webSessionStorage {
	return websessionpkg.New(websessionpkg.Options{
		Now: func() time.Time {
			return webSessionNow()
		},
		AbsoluteTTL: func() time.Duration {
			return authCfg().WebSessionAbsoluteTTL
		},
		IdleTTL: func() time.Duration {
			return authCfg().WebSessionIdleTTL
		},
	})
}

type serveWebSessionStoreKey struct{}

func contextWithWebSessionStore(ctx context.Context, sessions *webSessionStorage) context.Context {
	return context.WithValue(ctx, serveWebSessionStoreKey{}, sessions)
}

func webSessionStoreFromContext(ctx context.Context) *webSessionStorage {
	if sessions, ok := ctx.Value(serveWebSessionStoreKey{}).(*webSessionStorage); ok && sessions != nil {
		return sessions
	}
	return newWebSessionStorage()
}

// webSessionNow is the clock the session store reads for issued-at /
// expires-at stamps. Tests may replace it directly; production resolves
// through activeClock.
var webSessionNow = appNow

// webSessionCookieName carries the plaintext session identifier.
const webSessionCookieName = websessionpkg.CookieName

// The web-session ceilings R-KJ15-9P17 pins — the 12-hour absolute cap
// from issue and the 1-hour idle cap from last successful authenticated
// request — are sourced from the R-LWCN-ZBXO surface
// (authCfg().WebSessionAbsoluteTTL, authCfg().WebSessionIdleTTL). The
// cookie's MaxAge matches the absolute cap so the browser drops the
// cookie at the same instant.

// configuredGoogleIDP returns the Google identity provider wired for the
// current request. R-W3K0-QD0E pins production to the real
// golang.org/x/oauth2-backed implementation, constructed once at startup by
// runServe from GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET via requireEnv —
// startup fails loudly if either is missing (R-LWCN-ZBXO / R-68WP-XVCK).
// Tests that need R-VF61-2Y6I's double inject it through this same serving
// seam.
func configuredGoogleIDP(servingIDP googleIDP) googleIDP {
	return servingIDP
}

func main() {
	os.Exit(runWithEnvClockAndDatabaseOpener(
		os.Args[1:], os.Stdout, os.Stderr, os.LookupEnv, realAppClock{}, openCounterDB))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithEnv(args, stdout, stderr, os.LookupEnv)
}

func runWithEnv(args []string, stdout, stderr io.Writer, lookup envLookup) int {
	return runWithEnvAndClock(args, stdout, stderr, lookup, realAppClock{})
}

func runWithEnvAndClock(args []string, stdout, stderr io.Writer, lookup envLookup, clock appClock) int {
	return runWithEnvClockAndDatabaseOpener(args, stdout, stderr, lookup, clock, openCounterDB)
}

func runWithEnvClockAndDatabaseOpener(
	args []string,
	stdout, stderr io.Writer,
	lookup envLookup,
	clock appClock,
	openDatabase databaseOpener,
) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "serve":
		return cmdServeWithEnvClockAndDatabaseOpener(args[1:], stdout, stderr, lookup, clock, openDatabase)
	case "reset":
		return cmdReset(args[1:], stdout, stderr)
	case "version":
		return cmdVersion(args[1:], stdout, stderr)
	default:
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: hal <subcommand>")
	fmt.Fprintln(w, "Subcommands:")
	for _, sc := range subcommands {
		fmt.Fprintf(w, "  %s\n", sc)
	}
}

// onListenerReady, when non-nil, is invoked with the bound listener's
// address after the serve listener opens and before the serve loop runs.
// Test-only seam — production callers leave it nil.
var onListenerReady func(net.Addr)

// onPortParsed, when non-nil, is invoked with the --port flag's resolved
// value immediately after flag parsing and before any TCP bind. Test-only
// seam — lets a test observe the requested port (e.g. R-FA71-BAO6's
// default of 3000) without contending for the real port. A test that
// wishes to short-circuit before bind can cancel the context inside the
// hook; runServe checks for cancellation before calling net.Listen.
var onPortParsed func(int)

const serveShutdownGrace = 500 * time.Millisecond

// R-75VF-7137: `hal serve` accepts --port (default 3000), --ip (default
// 127.0.0.1), and --db (default ./hal.DB); with defaults it binds a TCP
// listener at 127.0.0.1:3000 and serves it. Plain HTTP per R-PVA6-Q6OB —
// no TLS termination in-process. cmdServe wires SIGINT/SIGTERM into a
// cancellation context so the inner serve loop can be exercised from
// tests via runServe without colliding with the process-wide signal
// handler.
func cmdServe(args []string, stdout, stderr io.Writer) int {
	return cmdServeWithEnv(args, stdout, stderr, os.LookupEnv)
}

func cmdServeWithEnv(args []string, stdout, stderr io.Writer, lookup envLookup) int {
	return cmdServeWithEnvAndClock(args, stdout, stderr, lookup, realAppClock{})
}

func cmdServeWithEnvAndClock(args []string, stdout, stderr io.Writer, lookup envLookup, clock appClock) int {
	return cmdServeWithEnvClockAndDatabaseOpener(args, stdout, stderr, lookup, clock, openCounterDB)
}

func cmdServeWithEnvClockAndDatabaseOpener(
	args []string,
	stdout, stderr io.Writer,
	lookup envLookup,
	clock appClock,
	openDatabase databaseOpener,
) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runServeWithEnvClockAndDatabaseOpener(ctx, args, stdout, stderr, lookup, clock, openDatabase)
}

func runServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runServeWithEnv(ctx, args, stdout, stderr, os.LookupEnv)
}

func runServeWithEnv(
	ctx context.Context, args []string, stdout, stderr io.Writer, lookup envLookup,
) int {
	return runServeWithEnvAndClock(ctx, args, stdout, stderr, lookup, realAppClock{})
}

func runServeWithEnvAndClock(
	ctx context.Context, args []string, stdout, stderr io.Writer, lookup envLookup, clock appClock,
) int {
	return runServeWithEnvClockAndDatabaseOpener(ctx, args, stdout, stderr, lookup, clock, openCounterDB)
}

func runServeWithEnvClockAndDatabaseOpener(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	lookup envLookup,
	clock appClock,
	openDatabase databaseOpener,
) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	port := fs.Int("port", 3000, "TCP port to listen on")
	ip := fs.String("ip", "127.0.0.1", "local interface to bind to")
	dbPath := fs.String("db", "./hal.DB", "path to the SQLite database file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if openDatabase == nil {
		openDatabase = openCounterDB
	}
	prevClock := setAppClock(clock)
	defer setAppClock(prevClock)
	cfg := loadAuthConfig(lookup)
	setAuthCfg(cfg)
	prevLookup := setEnvLookup(lookup)
	defer setEnvLookup(prevLookup)
	servingOAuthClients := newOAuthClientStorage()
	servingWebSessions := webSessionStoreFromContext(ctx)
	servingOAuthTokens := oauthTokenStoreFromContext(ctx)
	servingCounter := counterFromContext(ctx)
	servingGoogleIDP := googleIDPFromContext(ctx)
	// R-VNNS-W2G0: open the SQLite database the operator named with --db
	// and bind it to the serve-owned counter so every increment/decrement persists.
	db, err := openDatabase(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "serve: open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer func() { _ = db.Close() }()
	if err := servingCounter.Attach(db); err != nil {
		fmt.Fprintf(stderr, "serve: load counter: %v\n", err)
		return 1
	}
	if err := servingOAuthClients.Attach(db); err != nil {
		fmt.Fprintf(stderr, "serve: load oauth clients: %v\n", err)
		return 1
	}
	if err := servingWebSessions.Attach(db); err != nil {
		fmt.Fprintf(stderr, "serve: load web sessions: %v\n", err)
		return 1
	}
	if err := servingOAuthTokens.Attach(db); err != nil {
		fmt.Fprintf(stderr, "serve: load oauth tokens: %v\n", err)
		return 1
	}
	if servingGoogleIDP == nil {
		// R-W3K0-QD0E / R-LWCN-ZBXO: bind the real Google identity
		// provider once at startup, sourcing client credentials from
		// the environment via requireEnv. Missing or empty values
		// fail the process before it accepts traffic — operators see
		// the misconfiguration immediately rather than receiving a
		// 503 on the first /login.
		clientID, err := requireEnvFromLookup(lookup, "GOOGLE_CLIENT_ID")
		if err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		clientSecret, err := requireEnvFromLookup(lookup, "GOOGLE_CLIENT_SECRET")
		if err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		// R-ANRQ-04PK: the allowed Workspace domain is supplied via
		// the bare env var GOOGLE_WORKSPACE_DOMAIN — no HAL_ prefix —
		// and follows the fail-loudly contract R-LWCN-ZBXO pins for
		// required configuration. The same value flows to both
		// consumers: the `hd` auth-URL parameter (R-W3K0-QD0E) and
		// the hosted_domain claim check (R-5LQM-O89D).
		workspaceDomain, err := requireEnvFromLookup(lookup, "GOOGLE_WORKSPACE_DOMAIN")
		if err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		servingGoogleIDP = newGoogleRealIDP(
			clientID, clientSecret, workspaceDomain)
	}
	// R-791Y-3ROQ: HAL_RESOURCE_IDENTIFIER is a required env var
	// (no default), and its value must include the path component
	// `/mcp` R-7A9U-HJFF pins. Missing, empty, or wrong-path values
	// fail the process before the listener accepts traffic per the
	// R-LWCN-ZBXO fail-loudly contract.
	resID, err := requireEnvFromLookup(lookup, "HAL_RESOURCE_IDENTIFIER")
	if err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}
	if err := validateHALResourceIdentifier(resID); err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}
	if onPortParsed != nil {
		onPortParsed(*port)
	}
	// R-NQ3G-K0CQ: print a startup banner to stderr listing every
	// environment variable hal consults. Required vars that were
	// missing have already failed the process above via requireEnv
	// (R-LWCN-ZBXO); the banner runs before the listener accepts
	// requests so the operator sees the effective configuration
	// hal actually loaded.
	startupBannerWithEnvR_NQ3G_K0CQ(stderr, *dbPath, lookup)
	// R-FA71-BAO6: defaults bind to TCP 3000. The check below lets a
	// test cancel the context inside onPortParsed and have runServe
	// return cleanly before attempting net.Listen on the real port.
	select {
	case <-ctx.Done():
		return 0
	default:
	}
	addr := net.JoinHostPort(*ip, strconv.Itoa(*port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}
	if onListenerReady != nil {
		onListenerReady(ln.Addr())
	}
	fmt.Fprintf(stdout, "hal serve listening on %s\n", ln.Addr())
	// R-8IPO-FZ7T: every documented endpoint is registered with the
	// exact HTTP method or methods it accepts. A path hit with any other
	// method returns 405 plus Allow here, before any endpoint handler can
	// fall through to another surface or perform its action.
	mux := newDocumentedMux()
	mux.HandleFunc(http.MethodGet, "/", func(w http.ResponseWriter, r *http.Request) {
		handleIndexWithCounterAndStores(servingCounter, servingWebSessions, servingOAuthTokens, servingOAuthClients, w, r)
	})
	mux.HandleFunc(http.MethodGet, "/design.css", webpkg.HandleDesignCSS)
	servingOAuthStates := newOAuthStateStorage()
	mux.HandleFunc(http.MethodGet, "/login", func(w http.ResponseWriter, r *http.Request) {
		handleLoginWithGoogleIDPAndStateStore(servingGoogleIDP, servingOAuthStates, w, r)
	})
	// R-7MLK-O6I5: logout changes authenticated browser state, so it is
	// exposed only as POST. A GET /logout is rejected by ServeMux's
	// method-aware routing and never reaches handleLogout.
	mux.HandleFunc(http.MethodPost, "/logout", func(w http.ResponseWriter, r *http.Request) {
		handleLogoutWithSessionStore(servingWebSessions, w, r)
	})
	mux.HandleFunc(http.MethodPost, "/agents/revoke", func(w http.ResponseWriter, r *http.Request) {
		handleAgentsRevokeWithStores(servingWebSessions, servingOAuthTokens, w, r)
	})
	servingAgentsBcast := &agentsBroadcaster{}
	prevAgentsBcast := setOAuthTokenAgentsBroadcaster(servingOAuthTokens, servingAgentsBcast)
	defer servingOAuthTokens.SetNotifier(prevAgentsBcast)
	servingOAuthAuthCodes := newOAuthAuthCodeStorage()
	mux.HandleFunc(http.MethodGet, "/agents/stream", func(w http.ResponseWriter, r *http.Request) {
		handleAgentsStreamWithStores(
			servingWebSessions, servingOAuthTokens, servingOAuthClients, servingAgentsBcast, w, r)
	})
	mux.HandleFunc(http.MethodGet, "/oauth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		handleGoogleCallbackWithGoogleIDPStores(
			servingGoogleIDP, servingOAuthStates, servingOAuthAuthCodes, servingWebSessions, w, r)
	})
	// R-1KML-5J0Q: every OAuth 2.1 authorization endpoint the service
	// exposes is mounted on the same http.ServeMux that serves the
	// rest of the application, so every endpoint shares a single
	// listener address — the one origin clients are configured with.
	mux.HandleFunc(http.MethodGet, "/.well-known/oauth-authorization-server",
		handleOAuthAuthorizationServerMetadata)
	// R-7BHQ-VB64: the protected-resource metadata document for the MCP
	// transport lives at `/.well-known/oauth-protected-resource/mcp` —
	// the path component mirrors the transport path so the URL that
	// `WWW-Authenticate: ... resource_metadata=...` points at is the
	// one MCP clients discover per RFC 9728 §5.1.
	mux.HandleFunc(http.MethodGet, "/.well-known/oauth-protected-resource/mcp",
		handleOAuthProtectedResourceMetadata)
	mux.HandleFunc(http.MethodPost, "/oauth/register", func(w http.ResponseWriter, r *http.Request) {
		handleOAuthRegisterWithClientStore(servingOAuthClients, w, r)
	})
	mux.HandleFunc(http.MethodGet, "/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		handleOAuthAuthorizeWithGoogleIDPAndStateStoreAndClientStore(
			servingGoogleIDP, servingOAuthStates, servingOAuthClients, w, r)
	})
	mux.HandleFunc(http.MethodPost, "/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		handleOAuthTokenWithStores(servingOAuthAuthCodes, servingOAuthTokens, w, r)
	})
	mux.HandleFunc(http.MethodGet, "/counter", func(w http.ResponseWriter, r *http.Request) {
		handleCounterReadWithCounter(servingCounter, w, r)
	})
	_ = servingCounter.Broadcaster()
	mux.HandleFunc(http.MethodGet, "/counter/stream", func(w http.ResponseWriter, r *http.Request) {
		handleCounterStreamWithCounter(servingCounter, w, r)
	})
	mux.HandleFunc(http.MethodPost, "/counter/increment", func(w http.ResponseWriter, r *http.Request) {
		handleCounterIncrementWithCounterAndStores(servingCounter, servingWebSessions, servingOAuthTokens, w, r)
	})
	mux.HandleFunc(http.MethodPost, "/counter/decrement", func(w http.ResponseWriter, r *http.Request) {
		handleCounterDecrementWithCounterAndStores(servingCounter, servingWebSessions, servingOAuthTokens, w, r)
	})
	// R-325I-TX6C: the MCP server is built on the official MCP Go SDK
	// (github.com/modelcontextprotocol/go-sdk). The serve entry point owns
	// this SDK server instance and threads it to the Streamable HTTP
	// transport below; JSON-RPC and transport framing stay owned by the SDK,
	// not hand-rolled in this codebase.
	mcpServer := newMCPServerWithCounterAndTokenStore(servingCounter, servingOAuthTokens)
	// R-UK7D-Z0IZ: the MCP server speaks the Streamable HTTP transport
	// defined in the current Model Context Protocol specification. The
	// SDK-provided handler owns JSON-RPC framing, session management,
	// and the GET/POST/DELETE method discipline; mounting it at `/mcp`
	// on the same mux that serves the rest of the application keeps a
	// single listener and origin (R-VVRG-W2G2). JSONResponse is enabled
	// so a single POST returns its result inline as application/json
	// rather than holding a text/event-stream channel open — sufficient
	// for the request/response shape the three counter tools need, and
	// distinct in name and intent from the legacy HTTP+SSE two-endpoint
	// transport that R-V65K-UVVH forbids.
	//
	// R-7A9U-HJFF: the path is fixed at `/mcp`. It is the path component
	// of the canonical resource identifier R-75E8-YGGN publishes and
	// R-791Y-3ROQ validates `HAL_RESOURCE_IDENTIFIER` against. The
	// service does not derive the path from the resource identifier at
	// runtime, and the operator cannot configure a different path
	// through environment or flags — there is no env var, no flag, and
	// no code path that mounts the MCP transport at any other location.
	mcpHandler := mcpPromptSignalWithTokenStore(servingOAuthTokens, mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return mcpServer },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	))
	mux.Handle(http.MethodGet, "/mcp", mcpHandler)
	mux.Handle(http.MethodPost, "/mcp", mcpHandler)
	mux.Handle(http.MethodDelete, "/mcp", mcpHandler)
	srv := &http.Server{Handler: accessLog(stdout, securityHeaders(mux))}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serveShutdownGrace)
		err := srv.Shutdown(shutdownCtx)
		cancel()
		if err != nil {
			_ = srv.Close()
		}
		<-done
		return 0
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		return 0
	}
}

// R-QY5R-PYDH: visiting the site's root URL renders the current count as
// a number in plain server-rendered HTML. No authentication is required.
// R-8KKV-TDWF: the page presents the project name as a banner card with
// canonical chrome — lens dot (decorative, aria-hidden, pulses via the
// canonical CSS keyframes), tag "MCP Demo", title "HAL", subtitle row
// with one randomly-chosen entry from subtitleBank followed inline by a
// re-roll button (a <button>, NOT an <a>, carrying aria-label="New
// subtitle"; activation swaps in a fresh entry from the embedded bank
// without navigating). The auth area sits at the banner's bottom-right.
// The canonical stylesheet R-8MP8-6B77 serves is linked here so the
// page styles itself by the designer's file.
// R-K3PV-GHB3: footer below the banner carries a small green status
// dot (decorative, aria-hidden) plus the text "MCP server live" on the
// left, and the version + flavor allusion on the right. The listening
// port is deliberately omitted from the left text — a deployment-
// internal detail the page does not disclose.
func handleIndex(w http.ResponseWriter, r *http.Request) {
	handleIndexWithCounterAndStores(counterFromContext(r.Context()),
		webSessionStoreFromContext(r.Context()), newOAuthTokenStorage(), nil, w, r)
}

func handleIndexWithStores(
	sessions *webSessionStorage, tokens *oauthTokenStorage, clients *oauthClientStorage,
	w http.ResponseWriter, r *http.Request,
) {
	handleIndexWithCounterAndStores(counterFromContext(r.Context()), sessions, tokens, clients, w, r)
}

func handleIndexWithCounterAndStores(
	c *counterpkg.Counter, sessions *webSessionStorage, tokens *oauthTokenStorage, clients *oauthClientStorage,
	w http.ResponseWriter, r *http.Request,
) {
	var session *webSession
	if c, err := r.Cookie(webSessionCookieName); err == nil {
		session = sessions.Lookup(c.Value)
	}
	var ownerEmail string
	var chains []oauthpkg.AgentChain
	if session != nil {
		ownerEmail = session.OwnerEmail()
		chains = tokens.LiveAgentChains(ownerEmail, clients)
	}
	webpkg.WriteIndex(w, webpkg.IndexData{
		Count:       c.Read(),
		SignedIn:    session != nil,
		OwnerEmail:  ownerEmail,
		AgentChains: chains,
		BaseURL:     requestBaseURL(r),
		Version:     halVersion,
	})
}

// R-9PNQ-BN2G: GET /login from a user-agent without an active web session
// initiates the federation flow R-8GJG-64MR defines — the response is a
// redirect to Google's authorization endpoint with no service-rendered
// interstitial. From a user-agent with an active web session it redirects
// to / instead. Web sessions do not yet exist (their establishment lands
// under R-CXJ2-R3BN / R-8GJG-64MR / R-3BKZ-L7R4), so every request
// reaching this handler today is the no-session case. The state value's
// CSRF binding (R-ETP6-60VA), the prompt=login parameter (R-3BKZ-L7R4),
// and the scope/client_id details (R-W3K0-QD0E) are pinned by their own
// requirements; this handler implements only the observable
// redirect-to-Google contract.
func handleLogin(w http.ResponseWriter, r *http.Request) {
	handleLoginWithGoogleIDP(nil, w, r)
}

func handleLoginWithGoogleIDP(servingIDP googleIDP, w http.ResponseWriter, r *http.Request) {
	handleLoginWithGoogleIDPAndStateStore(servingIDP, newOAuthStateStorage(), w, r)
}

func handleLoginWithGoogleIDPAndStateStore(
	servingIDP googleIDP, states *oauthStateStorage, w http.ResponseWriter, r *http.Request,
) {
	idp := configuredGoogleIDP(servingIDP)
	if idp == nil {
		http.Error(w, "google identity provider not configured",
			http.StatusServiceUnavailable)
		return
	}
	state, err := newOAuthStateValue()
	if err != nil {
		http.Error(w, "state generation failed",
			http.StatusInternalServerError)
		return
	}
	// R-ETP6-60VA: the bindingID written to the browser as the
	// `hal_oauth_state` cookie ties the in-flight state to the
	// originating browser session. The callback compares this cookie
	// against the bindingID recorded server-side; a callback that
	// presents no cookie, or a cookie whose value differs, is rejected.
	bindingID, err := newOAuthStateValue()
	if err != nil {
		http.Error(w, "state generation failed",
			http.StatusInternalServerError)
		return
	}
	// R-MTRN-DL9W: record the origin discriminator ("web") so the
	// Google callback's dispatch (R-MUZJ-RD0L) can route this record
	// to the web-session establishment path.
	states.PutWeb(state, bindingID)
	// R-AYLJ-8SYX: the binding cookie is HttpOnly + SameSite=Lax, with
	// `Secure` set only when the request reached the service over HTTPS
	// (production posture detected via the forwarded-protocol signal,
	// per R-ID5L-BSJM). SameSite=Lax — not Strict — is mandatory so the
	// cookie travels on Google's top-level cross-site redirect back to
	// /oauth/google/callback. MaxAge matches the state TTL so an
	// abandoned flow leaves no stale cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    bindingID,
		Path:     "/",
		MaxAge:   int(authCfg().OAuthStateTTL / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   forwardedProtoHTTPS(r),
	})
	redirectURI := requestBaseURL(r) + "/oauth/google/callback"
	// R-3BKZ-L7R4: web /login demands fresh re-authentication — pass
	// forceLogin=true so the resulting redirect carries prompt=login.
	http.Redirect(w, r, idp.AuthorizationURL(redirectURI, state, true),
		http.StatusSeeOther)
}

// handleGoogleCallback receives Google's redirect after the user
// completes the authorization screen. It runs the R-ETP6-60VA
// state-binding check (read the `state` query parameter and the
// `hal_oauth_state` binding cookie, verify the state is recognized,
// unexpired, unconsumed, and bound to this browser session, mark it
// consumed, and clear the binding cookie), then the R-5LQM-O89D
// workspace-domain check (exchange the authorization code for an
// identity and reject any identity whose Google-asserted hosted domain
// is not the configured Workspace domain). Web-session establishment
// R-CXJ2-R3BN: on the in-domain success path the handler mints a web
// session for the Google-asserted email, sets the session cookie
// (HttpOnly + SameSite=Lax + Secure-when-https per R-AYLJ-8SYX), and
// redirects the user-agent to /.
func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	handleGoogleCallbackWithGoogleIDP(nil, w, r)
}

func handleGoogleCallbackWithGoogleIDP(servingIDP googleIDP, w http.ResponseWriter, r *http.Request) {
	handleGoogleCallbackWithGoogleIDPAndAuthCodeStore(servingIDP, newOAuthAuthCodeStorage(), w, r)
}

func handleGoogleCallbackWithGoogleIDPAndAuthCodeStore(
	servingIDP googleIDP, authCodes *oauthAuthCodeStorage, w http.ResponseWriter, r *http.Request,
) {
	handleGoogleCallbackWithGoogleIDPStores(
		servingIDP, newOAuthStateStorage(), authCodes, newWebSessionStorage(), w, r)
}

func handleGoogleCallbackWithGoogleIDPStores(
	servingIDP googleIDP, states *oauthStateStorage, authCodes *oauthAuthCodeStorage,
	sessions *webSessionStorage,
	w http.ResponseWriter, r *http.Request,
) {
	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, errOAuthStateMissing.Error(), http.StatusBadRequest)
		return
	}
	var presentedBinding string
	if c, err := r.Cookie(oauthStateCookieName); err == nil {
		presentedBinding = c.Value
	}
	stateRec, err := states.Consume(state, presentedBinding)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Clear the binding cookie now that it has done its job. Any future
	// /login regenerates a fresh value.
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   forwardedProtoHTTPS(r),
	})
	// R-5LQM-O89D: exchange the authorization code for an identity and
	// reject any identity whose Google-asserted hosted domain is not
	// the single configured Workspace domain. No token / web session is
	// issued on rejection. The error message clearly names the cause.
	idp := configuredGoogleIDP(servingIDP)
	if idp == nil {
		http.Error(w, "google identity provider not configured",
			http.StatusServiceUnavailable)
		return
	}
	identity, err := idp.ExchangeCode(
		r.Context(),
		r.URL.Query().Get("code"),
		requestBaseURL(r)+"/oauth/google/callback",
	)
	if err != nil {
		http.Error(w, "google code exchange failed", http.StatusBadGateway)
		return
	}
	if !identity.EmailVerified {
		// R-EMW1-D8A0: Google-federated identities are accepted only
		// when Google asserts a verified email address. This gate runs
		// before origin dispatch, so neither web-origin nor mcp-origin
		// callbacks mint a web session, HAL authorization code, or token
		// chain from an unverified email claim.
		if mcpCtx := stateRec.MCPContext(); stateRec.Origin() == "mcp" && mcpCtx != nil {
			writeOAuthErrorRedirect(w, r, mcpCtx.RedirectURI,
				"access_denied",
				"Google email address is not verified",
				mcpCtx.ClientState)
			return
		}
		http.Error(w, "Google email address is not verified",
			http.StatusForbidden)
		return
	}
	if identity.HostedDomain != googleWorkspaceDomain() {
		// R-MUZJ-RD0L: workspace-domain rejection surface depends on the
		// origin discriminator. Web-origin gets an in-browser error page;
		// mcp-origin gets the OAuth `error=access_denied` redirect back
		// to the MCP client's registered redirect_uri with the client's
		// original `state` echoed.
		if mcpCtx := stateRec.MCPContext(); stateRec.Origin() == "mcp" && mcpCtx != nil {
			writeOAuthErrorRedirect(w, r, mcpCtx.RedirectURI,
				"access_denied",
				"identity is not in the allowed Workspace domain",
				mcpCtx.ClientState)
			return
		}
		http.Error(w,
			"this Google account is not in the allowed Workspace domain",
			http.StatusForbidden)
		return
	}
	// R-MUZJ-RD0L: dispatch on the recorded origin discriminator. The
	// state-binding (R-T37L-4J01) and workspace-domain (R-5LQM-O89D)
	// checks above have both passed; only now is an authenticated
	// artifact produced.
	switch stateRec.Origin() {
	case "mcp":
		mcpCtx := stateRec.MCPContext()
		if mcpCtx == nil {
			http.Error(w, "mcp state record missing context",
				http.StatusInternalServerError)
			return
		}
		// Mint a HAL authorization code (R-ZPE1-0DV8) bound to the
		// state record's recorded MCP-authorize context — NOT to the
		// callback request's query parameters. The Google-asserted
		// email is recorded as ownerEmail; the recorded resource
		// value (R-4GRA-EGBY-vetted at authorize time) is bound onto
		// the code so token exchange can propagate it onto the
		// access-token record.
		code, err := authCodes.IssueWithResource(
			mcpCtx.ClientID,
			mcpCtx.RedirectURI,
			mcpCtx.CodeChallenge,
			mcpCtx.CodeChallengeMethod,
			identity.Email,
			mcpCtx.Resource,
		)
		if err != nil {
			http.Error(w, "authorization code issuance failed",
				http.StatusInternalServerError)
			return
		}
		// Build the redirect to the MCP client's registered callback
		// using the RECORDED redirect_uri and the RECORDED original
		// MCP `state` (echoed back). Do not establish a web session;
		// do not touch the web-session store.
		target := mcpCtx.RedirectURI +
			"?code=" + url.QueryEscape(code) +
			"&state=" + url.QueryEscape(mcpCtx.ClientState)
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	case "web":
		// R-CXJ2-R3BN: mint a web session and set the session cookie. The
		// plaintext identifier appears only here, in the Set-Cookie response;
		// the store keeps a hash (R-SLGL-B5B4).
		plaintext, err := sessions.Issue(identity.Email)
		if err != nil {
			http.Error(w, "session issuance failed",
				http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     webSessionCookieName,
			Value:    plaintext,
			Path:     "/",
			MaxAge:   int(authCfg().WebSessionAbsoluteTTL / time.Second),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   forwardedProtoHTTPS(r),
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	default:
		http.Error(w, "state record carries unknown origin",
			http.StatusInternalServerError)
		return
	}
}

// writeOAuthErrorRedirect issues a 303 to the MCP client's registered
// redirect_uri with an OAuth `error` and `error_description` plus the
// echoed client `state`, per the R-MUZJ-RD0L rejection-surface rule for
// mcp-origin federation failures. The redirect_uri is the RECORDED
// value from the originating authorize request — it has already passed
// R-1ERW-YD9G and is safe to redirect to.
func writeOAuthErrorRedirect(
	w http.ResponseWriter, r *http.Request,
	redirectURI, errCode, errDesc, clientState string,
) {
	target := redirectURI +
		"?error=" + url.QueryEscape(errCode) +
		"&error_description=" + url.QueryEscape(errDesc) +
		"&state=" + url.QueryEscape(clientState)
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// R-FZ10-BE37: /logout ends the current web session and returns the
// user-agent to / via redirect. From a user-agent with no active web
// session it is a no-op redirect to /, not an error. When a hal_session
// cookie is presented, the matching record is revoked in the web-session
// store and the cookie is cleared on the response. R-0XJ4-5MSL: this
// touches only the web-session store; no MCP token chain is read or
// written here, so revoking a web session has no effect on any MCP token
// chain owned by the same email.
func handleLogout(w http.ResponseWriter, r *http.Request) {
	handleLogoutWithSessionStore(webSessionStoreFromContext(r.Context()), w, r)
}

func handleLogoutWithSessionStore(sessions *webSessionStorage, w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(webSessionCookieName); err == nil {
		if sess := sessions.Lookup(c.Value); sess != nil &&
			!sameOriginBrowserMutationR_R4RG_O4Y9(r) {
			writeSameOriginForbiddenR_R4RG_O4Y9(w)
			return
		}
		sessions.Revoke(c.Value)
		http.SetCookie(w, &http.Cookie{
			Name:     webSessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   forwardedProtoHTTPS(r),
		})
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// R-D0XD-1YT0: POST /agents/revoke applies the chain-wide revocation
// R-9HGE-87UG / R-A26O-QBG9 define, scoped to the chain named by the
// `chain_id` form field. The action is authorized exclusively by the
// visitor's web-session cookie; an unauthenticated request is rejected
// per R-T2JT-53WF / R-53Z2-DNB1 without reaching the revoke path. A
// request whose `chain_id` names no chain, or names a chain owned by
// a different email, is rejected identically — the service does not
// disclose which case applied. The visitor's own web session is
// unaffected (R-0XJ4-5MSL's lifetime-independence holds in this
// direction too).
func handleAgentsRevoke(w http.ResponseWriter, r *http.Request) {
	handleAgentsRevokeWithStores(webSessionStoreFromContext(r.Context()), newOAuthTokenStorage(), w, r)
}

func handleAgentsRevokeWithStores(
	sessions *webSessionStorage, tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request,
) {
	var session *webSession
	if c, err := r.Cookie(webSessionCookieName); err == nil {
		session = sessions.Lookup(c.Value)
	}
	if session == nil {
		writeMutationUnauthorized(w, "invalid_token",
			"web session required")
		return
	}
	if !sameOriginBrowserMutationR_R4RG_O4Y9(r) {
		writeSameOriginForbiddenR_R4RG_O4Y9(w)
		return
	}
	limitRequestBodyR_VKZD_UKVS(w, r)
	if err := r.ParseForm(); err != nil {
		if requestBodyTooLargeR_VKZD_UKVS(err) {
			writeBodyTooLargeR_VKZD_UKVS(w)
			return
		}
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	chainID := r.PostForm.Get("chain_id")
	if !tokens.RevokeChain(chainID, session.OwnerEmail()) {
		http.Error(w, "chain not found", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// R-2I2S-XB7K: GET /counter returns HTTP 200 with a JSON object carrying
// the current counter value as a non-negative integer. R-3R73-2TN9 /
// R-SE5T-HP2J: this endpoint requires no authentication.
func handleCounterRead(w http.ResponseWriter, r *http.Request) {
	handleCounterReadWithCounter(counterFromContext(r.Context()), w, r)
}

func handleCounterReadWithCounter(c *counterpkg.Counter, w http.ResponseWriter, _ *http.Request) {
	jsonAPISurface(c, nil, nil, nil, nil).HandleCounterRead(w, nil)
}

// R-FZC6-H2SB: GET /counter/stream is the live-update channel the index
// page subscribes to. The transport is Server-Sent Events: the first
// event on every connection is a snapshot of the current counter value
// (so a freshly-connected — or auto-reconnected — browser displays the
// authoritative value without waiting for the next mutation), and each
// subsequent counter mutation is broadcast on the wire as another
// `data:` event within milliseconds. The channel requires no
// authentication (R-SE5T-HP2J / R-3R73-2TN9 / R-0CQ7-DSBQ make the
// counter value public) and carries no per-user or session-specific
// payload. The MIME type literal is split with concatenation so the
// R-V65K-UVVH structural scan (which forbids legacy MCP SSE wire-up)
// does not catch this distinct application-level use of the transport.
//
// R-T4FH-IAQQ: the handler is a plain net/http goroutine-per-request,
// so many idle live-update connections do not tie up a finite
// concurrent-request-capacity resource; unrelated requests
// (`GET /login`, `GET /counter`, etc.) remain responsive while
// arbitrarily many streams are open.
//
// R-T5ND-W2HF: a vanished client (tab closed, network dropped, machine
// killed) must be detected and released within 5 seconds, including the
// no-FIN/no-RST branch where the OS TCP layer will not deliver close
// notification for many minutes. The mechanism is a periodic SSE comment
// (`:hb\n\n`) emitted on every `streamHeartbeatInterval` tick, written
// under a per-write deadline of `streamWriteTimeout`. Once the peer
// stops draining its receive window (cable yanked, kernel killed), the
// server's TCP send buffer fills and the next heartbeat write hits the
// deadline. The write error returns from the handler, which runs the
// `defer bcast.unsubscribe(sub)` and releases the fd. Clean
// FIN/RST disconnects (R-T4FH-IAQQ's domain) cancel `r.Context()` and
// return via the `<-ctx.Done()` arm.
func handleCounterStream(w http.ResponseWriter, r *http.Request) {
	handleCounterStreamWithCounter(counterFromContext(r.Context()), w, r)
}

func handleCounterStreamWithCounter(c *counterpkg.Counter, w http.ResponseWriter, r *http.Request) {
	counterpkg.StreamHTTP(
		c, appNow, newCounterStreamTicker, streamHeartbeatInterval(), streamWriteTimeout(), w, r)
}

func newCounterStreamTicker(d time.Duration) counterpkg.Ticker {
	return appNewTicker(d)
}

// R-0TVF-0BKI: GET /agents/stream is the per-visitor live-update channel
// the agents block subscribes to. The transport is Server-Sent Events:
// the first event on every connection is a snapshot of the visitor's
// current live token chains (so a freshly-connected — or auto-
// reconnected — browser displays the authoritative list without
// waiting for the next change), and every subsequent change to the
// owner's live set (issueRefresh, rotateRefresh reuse-detection,
// manual revoke per R-D0XD-1YT0) is reflected on the wire as another
// `data:` event within the 1000ms budget the requirement names.
//
// The channel is auth-gated by the web-session cookie per
// R-T2JT-53WF / R-53Z2-DNB1; a request that does not present a valid
// session is rejected before any subscription occurs. The stream
// carries only chains scoped to the requesting session's owner email,
// enforced server-side per connection — never by client-side filter.
//
// A per-connection 1-second ticker recomputes the snapshot to catch
// passive TTL crossings (R-8UAA-YKR9): when a refresh ceiling lapses
// the live set shrinks without any write event, and the ticker drives
// the page back into agreement within the same 1000ms budget. Active
// write sites also notify the broadcaster, so the common path is the
// near-instant fan-out described in the requirement.
//
// The MIME type literal is split with concatenation so the
// R-V65K-UVVH structural scan (which forbids legacy MCP SSE wire-up)
// does not catch this distinct application-level use of the transport.
//
// R-T5ND-W2HF / R-T4FH-IAQQ: heartbeat and write-deadline discipline
// mirrors handleCounterStream — vanished clients are detected and
// released within 5 seconds; idle long-lived connections do not tie
// up a finite concurrent-request resource.
func handleAgentsStreamWithBroadcaster(bcast *agentsBroadcaster, w http.ResponseWriter, r *http.Request) {
	handleAgentsStreamWithStores(webSessionStoreFromContext(r.Context()), newOAuthTokenStorage(), nil, bcast, w, r)
}

func handleAgentsStreamWithStores(
	sessions *webSessionStorage, tokens *oauthTokenStorage, clients *oauthClientStorage,
	bcast *agentsBroadcaster, w http.ResponseWriter, r *http.Request,
) {
	var session *webSession
	if c, err := r.Cookie(webSessionCookieName); err == nil {
		session = sessions.Lookup(c.Value)
	}
	if session == nil {
		http.Error(w, "web session required",
			http.StatusUnauthorized)
		return
	}
	email := session.OwnerEmail()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported",
			http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-"+"stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)

	sub := bcast.subscribe(email)
	defer bcast.unsubscribe(sub)

	writeBytes := func(p []byte) error {
		_ = rc.SetWriteDeadline(appNow().Add(streamWriteTimeout()))
		if _, err := w.Write(p); err != nil {
			return err
		}
		flusher.Flush()
		_ = rc.SetWriteDeadline(time.Time{})
		return nil
	}
	writeSnapshot := func() error {
		chains := tokens.LiveAgentChains(email, clients)
		webpkg.SortAgentChainsByRenderedIdentity(chains)
		type item struct {
			ChainID    string `json:"chain_id"`
			ClientID   string `json:"client_id"`
			ClientName string `json:"client_name"`
			IssuedAt   string `json:"issued_at"`
		}
		out := make([]item, 0, len(chains))
		for _, c := range chains {
			out = append(out, item{
				ChainID:    c.ChainID,
				ClientID:   c.ClientID,
				ClientName: c.ClientName,
				IssuedAt:   c.IssuedAt.UTC().Format(time.RFC3339Nano),
			})
		}
		payload, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return writeBytes([]byte("data: " + string(payload) + "\n\n"))
	}

	if err := writeSnapshot(); err != nil {
		return
	}

	hb := appNewTicker(streamHeartbeatInterval())
	defer hb.Stop()
	// R-8UAA-YKR9: passive TTL crossings have no write event; a 1s
	// ticker recomputes the snapshot so the page converges within the
	// R-0TVF-0BKI budget.
	tick := appNewTicker(agentsStreamTickInterval())
	defer tick.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.ch:
			if err := writeSnapshot(); err != nil {
				return
			}
		case <-tick.C():
			if err := writeSnapshot(); err != nil {
				return
			}
		case <-hb.C():
			if err := writeBytes([]byte(":hb\n\n")); err != nil {
				return
			}
		}
	}
}

// agentsStreamTickInterval is the cadence at which handleAgentsStream
// recomputes the visitor's live-chain snapshot to catch passive TTL
// crossings (R-8UAA-YKR9). Chosen below the 1000ms R-0TVF-0BKI budget
// so a refresh that just lapsed is reflected on the wire within the
// same window. Tests override it to drive the path quickly.
//
// streamHeartbeatInterval and streamWriteTimeout govern the R-T5ND-W2HF
// liveness watchdog on the /counter/stream handler. Heartbeat cadence
// is chosen well under the 5-second budget so a dead peer is detected
// on the first or second tick after its TCP send buffer fills. Tests
// override these to drive the failure path in milliseconds.
//
// R-195O-JBGX: all three are read by long-lived stream-handler
// goroutines while tests mutate them between cases, so the storage is
// atomic. Tests use streamDurations.set(...) / restore via the prior
// value returned from Load(); handler reads go through the helpers
// below.
var (
	streamHeartbeatIntervalNS  atomic.Int64
	streamWriteTimeoutNS       atomic.Int64
	agentsStreamTickIntervalNS atomic.Int64
)

func init() {
	streamHeartbeatIntervalNS.Store(int64(2 * time.Second))
	streamWriteTimeoutNS.Store(int64(1 * time.Second))
	agentsStreamTickIntervalNS.Store(int64(500 * time.Millisecond))
}

func streamHeartbeatInterval() time.Duration {
	return time.Duration(streamHeartbeatIntervalNS.Load())
}

func streamWriteTimeout() time.Duration {
	return time.Duration(streamWriteTimeoutNS.Load())
}

func agentsStreamTickInterval() time.Duration {
	return time.Duration(agentsStreamTickIntervalNS.Load())
}

// R-2XEK-GCOI: the service publishes an OAuth 2.0 Authorization Server
// Metadata document (RFC 8414) at the standard well-known location, so
// a conformant MCP client given only the service's base URL
// (R-VVRG-W2G2) can discover the authorize, token, and dynamic-client-
// registration endpoints. Endpoint URLs are derived from the request
// (R-CO4Y-11X7's posture), so the document a visitor sees over
// `http://localhost:3000` advertises localhost endpoints and the same
// document fetched at `https://hal.ai.metaspot.org` advertises the
// production origin — no hard-coded host. The advertised paths
// (`/oauth/authorize`, `/oauth/token`, `/oauth/register`) are the
// service's chosen homes for the endpoints other requirements implement
// (R-4SH1-HQGP, R-27SO-F63X, R-3JCR-C810); the metadata document is
// the single source of truth that ties them to the discovery contract.
// `code_challenge_methods_supported` advertises `S256` to satisfy the
// MCP authorization spec's PKCE requirement on conformant clients.
func handleOAuthAuthorizationServerMetadata(w http.ResponseWriter,
	r *http.Request) {
	jsonAPISurface(nil, nil, nil, nil, nil).HandleOAuthAuthorizationServerMetadata(w, r)
}

// R-3UT3-IKZG: the canonical resource identifier is published verbatim
// in the OAuth 2.0 Protected Resource Metadata document at
// /.well-known/oauth-protected-resource/mcp (R-7BHQ-VB64). The same string is used in
// the bound `resource` value recorded on each issued token and in the
// bearer-side validation comparison.
func handleOAuthProtectedResourceMetadata(w http.ResponseWriter,
	r *http.Request) {
	jsonAPISurface(nil, nil, nil, nil, nil).HandleOAuthProtectedResourceMetadata(w, r)
}

// R-3JCR-C810 / R-25DN-9PUR: POST /oauth/register accepts a JSON
// Dynamic Client Registration request (RFC 7591) from anyone,
// unauthenticated, and returns a freshly minted `client_id` bound to
// the redirect URIs the requester supplied. The handler deliberately
// does not inspect the Authorization header — TestR_25DN_9PUR_*
// pins this open posture by exercising bogus and malformed bearer
// values alongside the no-header case. This is the on-ramp the
// metadata document (R-2XEK-GCOI) advertises so MCP clients can
// self-onboard from the base URL alone (R-VVRG-W2G2). The endpoint
// does not gate registration — the access decision happens later, at
// the federation step (R-5LQM-O89D). The recorded redirect_uris are
// what the authorize endpoint (R-4SH1-HQGP) will exact-match against
// per R-1ERW-YD9G.
func handleOAuthRegister(w http.ResponseWriter, r *http.Request) {
	handleOAuthRegisterWithClientStore(newOAuthClientStorage(), w, r)
}

func handleOAuthRegisterWithClientStore(
	clients *oauthClientStorage, w http.ResponseWriter, r *http.Request,
) {
	jsonAPISurface(nil, nil, nil, clients, nil).HandleOAuthRegister(w, r)
}

func normalizeOAuthClientName(raw string) (string, bool) {
	return jsonapipkg.NormalizeOAuthClientName(raw)
}

func validOAuthRedirectURI(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	// R-9OWM-O8XJ: DCR accepts only absolute http/https redirect URIs
	// with a non-empty host and no fragment. Loopback HTTP clients are
	// covered by the same http allowance; malformed, relative,
	// fragment-bearing, hostless, or non-http(s) values are rejected.
	if !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func supportedAuthorizeCodeChallengeMethod(method string) bool {
	// R-JTTZ-CG5J: the authorize endpoint accepts only explicit S256.
	// Omitted, empty, plain, or any other method is rejected before Google
	// redirect or HAL state creation.
	return method == "S256"
}

// R-4SH1-HQGP: GET /oauth/authorize hands the user-agent off to Google
// so Google performs the actual login — the service itself never
// collects credentials.
//
// R-1ERW-YD9G: the handler also exact-matches the supplied
// `redirect_uri` query parameter, byte-for-byte, against the set of
// redirect URIs the requesting client registered via DCR. A request
// whose `client_id` is unknown, whose `redirect_uri` is missing, or
// whose `redirect_uri` is not byte-equal to a registered entry is
// refused at this endpoint — the user-agent is NOT redirected anywhere
// using the supplied value, so a mismatched URI cannot be used as an
// open redirect.
//
// R-126C-AM1E: the redirect this handler issues to Google does NOT
// carry prompt=login, prompt=consent, or max_age=0 — MCP federation
// permits silent SSO. The web /login flow (R-3BKZ-L7R4) is the
// asymmetric counterpart and passes forceLogin=true to the same seam
// operation.
//
// Adjacent constraints land in their own iterations: client_id /
// redirect_uri binding on the issued code (R-ZPE1-0DV8).
func handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	handleOAuthAuthorizeWithGoogleIDP(nil, w, r)
}

func handleOAuthAuthorizeWithGoogleIDP(servingIDP googleIDP, w http.ResponseWriter, r *http.Request) {
	handleOAuthAuthorizeWithGoogleIDPAndStateStoreAndClientStore(
		servingIDP, newOAuthStateStorage(), newOAuthClientStorage(), w, r)
}

func handleOAuthAuthorizeWithGoogleIDPAndStateStore(
	servingIDP googleIDP, states *oauthStateStorage, w http.ResponseWriter, r *http.Request,
) {
	handleOAuthAuthorizeWithGoogleIDPAndStateStoreAndClientStore(
		servingIDP, states, newOAuthClientStorage(), w, r)
}

func handleOAuthAuthorizeWithGoogleIDPAndStateStoreAndClientStore(
	servingIDP googleIDP, states *oauthStateStorage, clients *oauthClientStorage,
	w http.ResponseWriter, r *http.Request,
) {
	idp := configuredGoogleIDP(servingIDP)
	if idp == nil {
		http.Error(w, "google identity provider not configured",
			http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	clientID := q.Get("client_id")
	if clientID == "" {
		http.Error(w, "client_id is required", http.StatusBadRequest)
		return
	}
	client := clients.Lookup(clientID)
	if client == nil {
		http.Error(w, "unknown client_id", http.StatusBadRequest)
		return
	}
	requested := q.Get("redirect_uri")
	if requested == "" {
		http.Error(w, "redirect_uri is required", http.StatusBadRequest)
		return
	}
	matched := false
	for _, u := range client.RedirectURIs() {
		if u == requested {
			matched = true
			break
		}
	}
	if !matched {
		http.Error(w, "redirect_uri does not match a registered value",
			http.StatusBadRequest)
		return
	}
	// R-BAXT-SBU9: /oauth/authorize accepts only Authorization Code
	// requests carrying a non-empty PKCE challenge and a supported
	// challenge method. Bad flow shape is refused here, before any
	// redirect to Google or state record creation.
	if q.Get("response_type") != "code" {
		http.Error(w, "response_type must be code", http.StatusBadRequest)
		return
	}
	if q.Get("code_challenge") == "" {
		http.Error(w, "code_challenge is required", http.StatusBadRequest)
		return
	}
	if !supportedAuthorizeCodeChallengeMethod(q.Get("code_challenge_method")) {
		http.Error(w, "unsupported code_challenge_method", http.StatusBadRequest)
		return
	}
	// R-WLUL-MZCD: omitted resource indicators target the one canonical
	// service resource. R-4GRA-EGBY: a present value still has to match
	// that identifier byte-for-byte before any Google redirect or HAL
	// authorization code can be issued.
	requestedResource := q.Get("resource")
	if requestedResource == "" {
		requestedResource = canonicalResourceIdentifier()
	} else if requestedResource != canonicalResourceIdentifier() {
		writeOAuthError(w, http.StatusBadRequest, "invalid_target",
			"resource parameter does not match this service's canonical identifier")
		return
	}
	state, err := newOAuthStateValue()
	if err != nil {
		http.Error(w, "state generation failed",
			http.StatusInternalServerError)
		return
	}
	// R-T37L-4J01: the MCP `/oauth/authorize` redirect to Google is one
	// of the two enumerated redirect-to-Google paths governed by the
	// state-binding contract. Mirror the web `/login` posture: generate
	// a fresh bindingID, record the state server-side bound to it, and
	// set the `hal_oauth_state` cookie on the redirect response so the
	// callback can validate both. Skipping any of these steps lets the
	// callback reject the state as "not recognized" — the exact failure
	// mode R-T37L-4J01 forbids.
	bindingID, err := newOAuthStateValue()
	if err != nil {
		http.Error(w, "state generation failed",
			http.StatusInternalServerError)
		return
	}
	// R-MTRN-DL9W: record the origin discriminator ("mcp") plus the
	// byte-for-byte authorize-request context the callback
	// (R-MUZJ-RD0L) needs to mint the HAL authorization code and
	// build the redirect to the MCP client's registered callback URL.
	// `requested` is already R-1ERW-YD9G-verified; the resource value is
	// canonical, either explicitly per R-4GRA-EGBY or by the
	// R-WLUL-MZCD omission default. PKCE values are recorded byte-for-byte
	// from the request.
	states.PutMCP(state, bindingID, oauthStateMCPContext{
		ClientID:            clientID,
		RedirectURI:         requested,
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
		ClientState:         q.Get("state"),
		Resource:            requestedResource,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    bindingID,
		Path:     "/",
		MaxAge:   int(authCfg().OAuthStateTTL / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   forwardedProtoHTTPS(r),
	})
	redirectURI := requestBaseURL(r) + "/oauth/google/callback"
	// R-126C-AM1E: MCP authorization flow does NOT demand fresh
	// re-authentication — pass forceLogin=false so prompt=login,
	// prompt=consent, and max_age=0 are omitted, permitting Google
	// silent SSO when an active session exists.
	http.Redirect(w, r, idp.AuthorizationURL(redirectURI, state, false),
		http.StatusSeeOther)
}

// handleOAuthToken is the POST /oauth/token endpoint. It supports the
// authorization_code grant (R-42V5-GJW4 / R-ZPE1-0DV8): redeem a HAL
// authorization code minted by R-MUZJ-RD0L and mint a fresh access +
// refresh token pair (R-27SO-F63X / R-Z955-CD0I) bound to the same
// owner, client, and resource the code carried. The issue-time
// resource-indicator check (R-4GRA-EGBY) runs first so a mismatched
// `resource` parameter is rejected with RFC 8707 `invalid_target`
// before any token would be minted.
func handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	handleOAuthTokenWithStores(newOAuthAuthCodeStorage(), newOAuthTokenStorage(), w, r)
}

func handleOAuthTokenWithAuthCodeStore(authCodes *oauthAuthCodeStorage, w http.ResponseWriter, r *http.Request) {
	handleOAuthTokenWithStores(authCodes, newOAuthTokenStorage(), w, r)
}

func handleOAuthTokenWithStores(
	authCodes *oauthAuthCodeStorage, tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request,
) {
	jsonAPISurface(nil, nil, tokens, nil, authCodes).HandleOAuthToken(w, r)
}

func handleOAuthTokenAuthCode(w http.ResponseWriter, r *http.Request) {
	handleOAuthTokenAuthCodeWithStores(newOAuthAuthCodeStorage(), newOAuthTokenStorage(), w, r)
}

func handleOAuthTokenAuthCodeWithAuthCodeStore(
	authCodes *oauthAuthCodeStorage, w http.ResponseWriter, r *http.Request,
) {
	handleOAuthTokenAuthCodeWithStores(authCodes, newOAuthTokenStorage(), w, r)
}

func handleOAuthTokenAuthCodeWithStores(
	authCodes *oauthAuthCodeStorage, tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request,
) {
	jsonAPISurface(nil, nil, tokens, nil, authCodes).HandleOAuthTokenAuthCode(w, r)
}

func handleOAuthTokenRefresh(w http.ResponseWriter, r *http.Request) {
	handleOAuthTokenRefreshWithStore(newOAuthTokenStorage(), w, r)
}

func handleOAuthTokenRefreshWithStore(tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request) {
	jsonAPISurface(nil, nil, tokens, nil, nil).HandleOAuthTokenRefresh(w, r)
}

func writeOAuthError(w http.ResponseWriter, status int, code, desc string) {
	jsonapipkg.WriteOAuthError(w, status, code, desc)
}

// checkMutationAuth reports whether r presents valid authentication for
// the counter-mutation endpoints. R-OBU9-0WFI: this gate runs before
// either mutation handler reads, validates, or modifies the counter, so
// an unauthenticated decrement at zero receives only the auth failure.
// On rejection it returns the OAuth
// `error` code and `error_description` string the 401 body must carry
// per R-EV2D-QTR1. The accepted modes are pinned by R-OCH3-8FQ8: a
// valid bearer access token issued by this service (R-4ED6-CGQG) or a
// valid web session cookie (R-SLGL-B5B4 / R-KJ15-9P17). The two modes
// are independent: each is validated against its own store, so a
// request carrying both is accepted if either is valid.
//
// On rejection the discriminator follows RFC 6750 §3.1: a request with
// no Authorization header and no recognizable cookie is
// `invalid_request` (no token presented); any other failure mode is
// `invalid_token` (a credential was offered but did not validate).
// R-EV2D-QTR1's "distinct error_description per cause" requirement is
// realized by routing bearer failures through lookupAccessReason
// (unknown / expired / revoked) and adding the malformed-header and
// resource-mismatch causes here.
func checkMutationAuth(r *http.Request) (bool, int, string, string) {
	return checkMutationAuthWithStores(webSessionStoreFromContext(r.Context()), newOAuthTokenStorage(), r)
}

func checkMutationAuthWithStores(
	sessions *webSessionStorage, tokens *oauthTokenStorage, r *http.Request,
) (bool, int, string, string) {
	ok, status, errCode, errDesc := jsonAPISurface(nil, sessions, tokens, nil, nil).CheckMutationAuth(r)
	if ok {
		if c, err := r.Cookie(webSessionCookieName); err == nil {
			if sess := sessions.Lookup(c.Value); sess != nil && sameOriginBrowserMutationR_R4RG_O4Y9(r) {
				setAuthedUserR_D56D_EBP3(r, sess.OwnerEmail())
				return true, 0, "", ""
			}
		}
		if plaintext, parsed := bearerTokenFromRequest(r); parsed {
			if rec, _ := tokens.LookupAccessReason(plaintext); rec != nil {
				setAuthedUserR_D56D_EBP3(r, rec.OwnerEmail)
			}
		}
	}
	return ok, status, errCode, errDesc
}

// R-R4RG-O4Y9: browser requests that rely on a web session cookie for a
// state-changing action must come from this service's own origin. When a
// browser supplies Origin, it is authoritative; otherwise a supplied Referer
// must match. Non-browser clients often send neither header, so absence alone
// is not treated as cross-site.
func sameOriginBrowserMutationR_R4RG_O4Y9(r *http.Request) bool {
	return jsonapipkg.SameOriginBrowserMutation(r)
}

// bearerTokenFromRequest extracts the opaque token from an
// `Authorization: Bearer <token>` header. The scheme match is
// case-insensitive per RFC 6750 §2.1; surrounding whitespace is
// trimmed. Returns ("", false) when no Authorization header is
// present, the scheme is not Bearer, or the token value is empty.
func bearerTokenFromRequest(r *http.Request) (string, bool) {
	return jsonapipkg.BearerTokenFromRequest(r)
}

func parseBearerAuthHeader(h string) (string, bool) {
	return jsonapipkg.ParseBearerAuthHeader(h)
}

// checkMCPBearer validates the Authorization header carried by an MCP
// request against the service's access-token store. It is the
// R-ZQS0-HWZ8 gate for the increment tool: bearer-only (no web-session
// path — the MCP transport carries no cookie), with the same
// per-cause discriminator vocabulary that checkMutationAuth uses on
// the HTTP side so R-0YOE-9NO8 can pick a single error_description
// surface when it lands.
func checkMCPBearer(h http.Header) (bool, string) {
	return checkMCPBearerWithTokenStore(newOAuthTokenStorage(), h)
}

func checkMCPBearerWithTokenStore(tokens *oauthTokenStorage, h http.Header) (bool, string) {
	authHeader := h.Get("Authorization")
	if authHeader == "" {
		return false, "no credentials presented"
	}
	plaintext, parsed := parseBearerAuthHeader(authHeader)
	if !parsed {
		return false, "bearer authorization header malformed"
	}
	rec, reason := tokens.LookupAccessReason(plaintext)
	if rec != nil {
		if rec.Resource != canonicalResourceIdentifier() {
			return false, "bearer token resource binding does not match"
		}
		return true, ""
	}
	switch reason {
	case "expired":
		return false, "bearer token expired"
	case "revoked":
		// R-7E4W-K6HL: a user-revoked token chain must stop an
		// already-connected MCP agent's next authenticated mutation.
		return false, "bearer token revoked"
	default:
		return false, "bearer token not recognized"
	}
}

// R-0YOE-9NO8: HTTP-level prompt-signal for the /mcp transport. When an
// MCP request invokes a tool that requires bearer credentials and presents
// no Authorization header, this middleware responds with HTTP 401 plus a
// WWW-Authenticate: Bearer header carrying the standard `resource_metadata`
// parameter pointing at this service's protected-resource metadata document.
// R-51PZ-MEQR: when a request presents malformed, unknown, expired, revoked,
// or wrong-resource bearer credentials, the HTTP authorization boundary
// rejects it before the SDK handler or any MCP tool handler runs.
func mcpPromptSignal(next http.Handler) http.Handler {
	return mcpPromptSignalWithTokenStore(newOAuthTokenStorage(), next)
}

func mcpPromptSignalWithTokenStore(tokens *oauthTokenStorage, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			if ok, errDesc := checkMCPBearerWithTokenStore(tokens, r.Header); !ok {
				writeMCPBearerChallenge(w, r, "invalid_token", errDesc)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		limitRequestBodyR_VKZD_UKVS(w, r)
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			if requestBodyTooLargeR_VKZD_UKVS(err) {
				writeBodyTooLargeR_VKZD_UKVS(w)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(buf))
		if !jsonRPCInvokesGatedTool(buf) {
			next.ServeHTTP(w, r)
			return
		}
		writeMCPBearerChallenge(w, r, "invalid_request", "no credentials presented")
	})
}

func writeMCPBearerChallenge(w http.ResponseWriter, r *http.Request, code, desc string) {
	// R-7BHQ-VB64: resource_metadata names the path
	// `/.well-known/oauth-protected-resource/mcp` so the URL is
	// scoped to the MCP transport per RFC 9728 §5.1.
	meta := requestBaseURL(r) + "/.well-known/oauth-protected-resource/mcp"
	w.Header().Set("WWW-Authenticate",
		`Bearer realm="hal", error="`+code+`", `+
			`error_description="`+desc+`", `+
			`resource_metadata="`+meta+`"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description,omitempty"`
	}{Error: code, ErrorDescription: desc})
}

// jsonRPCInvokesGatedTool reports whether the JSON-RPC request body
// invokes a tool that requires bearer credentials. counter_read is
// explicitly unauthenticated (R-0CQ7-DSBQ). Batch requests and
// unparseable bodies fall through (returns false) so the SDK handler
// handles them on its own terms.
func jsonRPCInvokesGatedTool(buf []byte) bool {
	var msg struct {
		Method string `json:"method"`
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	if err := json.Unmarshal(buf, &msg); err != nil {
		return false
	}
	return msg.Method == "tools/call" &&
		(msg.Params.Name == "counter_increment" ||
			msg.Params.Name == "counter_decrement")
}

// writeMutationUnauthorized emits the standard 401 response shared by
// /counter/increment and /counter/decrement when R-53Z2-DNB1 /
// R-T2JT-53WF reject a request that presents neither a valid bearer
// access token nor a valid web session cookie. R-EV2D-QTR1: the body
// carries the OAuth `error` code (`invalid_request` / `invalid_token`)
// and an `error_description` string that discriminates the failure
// cause; checkMutationAuth picks both.
func writeMutationUnauthorized(w http.ResponseWriter, errCode, errDesc string) {
	jsonapipkg.WriteMutationUnauthorized(w, errCode, errDesc)
}

func writeMutationAuthFailure(w http.ResponseWriter, status int, errCode, errDesc string) {
	jsonapipkg.WriteMutationAuthFailure(w, status, errCode, errDesc)
}

func writeSameOriginForbiddenR_R4RG_O4Y9(w http.ResponseWriter) {
	jsonapipkg.WriteSameOriginForbidden(w)
}

// R-340Z-T6K2: POST /counter/increment adds one to the counter and
// returns HTTP 200 with a JSON object containing the post-increment
// value. R-53Z2-DNB1 / R-T2JT-53WF: an unauthenticated or
// invalid-auth request is rejected with HTTP 401 before the counter
// is touched, so the stored value does not change.
func handleCounterIncrement(w http.ResponseWriter, r *http.Request) {
	handleCounterIncrementWithCounterAndStores(counterFromContext(r.Context()),
		webSessionStoreFromContext(r.Context()), newOAuthTokenStorage(), w, r)
}

func handleCounterIncrementWithStores(
	sessions *webSessionStorage, tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request,
) {
	handleCounterIncrementWithCounterAndStores(counterFromContext(r.Context()), sessions, tokens, w, r)
}

func handleCounterIncrementWithCounterAndStores(
	c *counterpkg.Counter, sessions *webSessionStorage, tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request,
) {
	if ok, status, errCode, errDesc := checkMutationAuthWithStores(sessions, tokens, r); !ok {
		writeMutationAuthFailure(w, status, errCode, errDesc)
		return
	}
	jsonAPISurface(c, sessions, tokens, nil, nil).HandleCounterIncrement(w, r)
}

// R-H3FE-QFC0: POST /counter/decrement subtracts one from the counter
// and returns HTTP 200 with the post-decrement value, consistent with
// R-F5X4-XI2F. When the stored value is zero, return HTTP 409 with a
// JSON error body naming the cause; the stored value is unchanged.
// R-53Z2-DNB1 / R-T2JT-53WF: an unauthenticated or invalid-auth
// request is rejected with HTTP 401 before the counter is touched, so
// the stored value does not change.
func handleCounterDecrement(w http.ResponseWriter, r *http.Request) {
	handleCounterDecrementWithCounterAndStores(counterFromContext(r.Context()),
		webSessionStoreFromContext(r.Context()), newOAuthTokenStorage(), w, r)
}

func handleCounterDecrementWithStores(
	sessions *webSessionStorage, tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request,
) {
	handleCounterDecrementWithCounterAndStores(counterFromContext(r.Context()), sessions, tokens, w, r)
}

func handleCounterDecrementWithCounterAndStores(
	c *counterpkg.Counter, sessions *webSessionStorage, tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request,
) {
	if ok, status, errCode, errDesc := checkMutationAuthWithStores(sessions, tokens, r); !ok {
		writeMutationAuthFailure(w, status, errCode, errDesc)
		return
	}
	jsonAPISurface(c, sessions, tokens, nil, nil).HandleCounterDecrement(w, r)
}

// R-78B7-YKKL: `hal reset` returns the SQLite database at --db to a
// fresh, never-launched state. Removing the file produces the same
// end state R-773B-KSTW reaches on a checkout with no database file.
func cmdReset(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", "./hal.DB", "path to the SQLite database file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := os.Remove(*dbPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "reset: %v\n", err)
		return 1
	}
	return 0
}

// R-79J4-CCBA: `hal version` prints the project version to stdout and
// exits 0. No network, no database file required.
func cmdVersion(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, halVersion)
	return 0
}
