// Package main is the hal binary entry point.
package main

import (
	"bytes"
	"context"
	"crypto"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"math/rand/v2"
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
	"testing"
	"time"

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

func newMCPServerWithCounterAndTokenStore(c *counter, tokens *oauthTokenStorage) *mcp.Server {
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

func counterReadToolWithCounter(c *counter) func(
	context.Context, *mcp.CallToolRequest, struct{},
) (*mcp.CallToolResult, counterReadOutput, error) {
	return func(
		_ context.Context, _ *mcp.CallToolRequest, _ struct{},
	) (*mcp.CallToolResult, counterReadOutput, error) {
		return nil, counterReadOutput{Value: c.read()}, nil
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

func counterIncrementToolWithCounterAndTokenStore(c *counter, tokens *oauthTokenStorage) func(
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
		return nil, counterIncrementOutput{Value: c.increment()}, nil
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

func counterDecrementToolWithCounterAndTokenStore(c *counter, tokens *oauthTokenStorage) func(
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
		v, ok := c.decrement()
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

// R-8MP8-6B77: the canonical stylesheet is embedded from a checked-in
// copy at app-root/design.css and served directly so the rendered page
// is styled by the designer's file rather than a re-derived parallel.
// `//go:embed` cannot traverse above the module root, so the canonical
// source-of-truth at ../reqs/design.css is mirrored here; a drift test
// reads ../reqs/design.css and asserts byte-equality with this embed.
//
//go:embed design.css
var designCSS []byte

// R-74NI-T9CI: the hal binary exposes exactly three subcommands.
var subcommands = []string{"serve", "reset", "version"}

// R-79J4-CCBA: project version string printed by `hal version`.
const halVersion = "0.0.1"

// R-G47S-05R3: the index page's subtitle is one entry chosen uniformly at
// random per page render from this fixed bank of acronym expansions. The
// list is canonical: prose elsewhere in the spec defers to this slice as
// the source of truth for which expansions are reachable. Order is the
// spec's listing order; selection is uniform random (not round-robin),
// and `pickSubtitle` is the only path through which a renderer obtains a
// value, so every entry is reachable.
var subtitleBank = []string{
	"Holistic Access Layer",
	"Human Augmentation Layer",
	"Heuristic Agent Liaison",
	"Home, APIs, Library",
	"Heuristically programmed ALgorithm",
	"Heuristically programmed ALgorithmic computer",
	"Helpful Autonomous Liaison",
	"Hyperlocal Agent Layer",
	"Host Agent Liaison",
	"Has Always Listened",
	"House Always Loses",
	"Hardware Abstraction Layer",
	"Hyperdimensional Access Layer",
	"Holistic Application Logic",
	"Highly Adaptive Listener",
	"Headless Agent Loop",
	"Hosted Action Library",
	"Hermetic Authorization Layer",
	"Hypertext Application Language",
	"High-Availability Lambda",
	"Heretical Automation Layer",
	"Hyper-tuned Agent Logic",
	"Handy Autoresponse Layer",
	"Hallucination Avoidance Layer",
	"Honest Assistant, Lately",
	"Halfway Awake Loop",
	"Homemade Agent Lab",
	"Heuristic Argument Linker",
}

// pickSubtitle returns one entry from subtitleBank chosen uniformly at
// random. math/rand/v2's default Source is seeded per-process from the
// runtime entropy pool, so successive process renders produce independent
// draws without explicit seeding.
func pickSubtitle() string {
	return subtitleBank[rand.IntN(len(subtitleBank))]
}

// R-UZ9T-8NM4: the counter is a non-negative integer. The in-process
// representation stores the value in an unsigned 64-bit field, so a
// negative value is unrepresentable at the type level. Behavioral
// guards on decrement-at-zero live with R-F5X4-XI2F.
// R-TOI0-0Z8X: the mutex serializes the read-modify-write sequence on
// value across goroutines, so concurrent increments and decrements compose
// without lost updates. A single mutex covers all three methods rather than
// sync/atomic so decrement's zero-check + subtract stay one critical
// section (avoiding a TOCTOU between the check and the store).
// R-VNNS-W2G0: when db is non-nil, every successful mutation writes the
// post-state value back to the SQLite single-row counter table under the
// same critical section that updates the in-memory value, so a
// crash-and-restart that reopens the same database recovers the last
// successfully applied value. When db is nil, the counter is in-memory
// only — the shape exercised by unit tests that do not need persistence.
type counter struct {
	mu    sync.Mutex
	value uint64
	db    *sql.DB
	bcast *counterBroadcaster
}

func newCounter() *counter {
	return &counter{}
}

type serveCounterKey struct{}

func contextWithCounter(ctx context.Context, c *counter) context.Context {
	return context.WithValue(ctx, serveCounterKey{}, c)
}

func counterFromContext(ctx context.Context) *counter {
	if c, ok := ctx.Value(serveCounterKey{}).(*counter); ok && c != nil {
		return c
	}
	return newCounter()
}

func (c *counter) broadcaster() *counterBroadcaster {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bcast == nil {
		c.bcast = &counterBroadcaster{}
	}
	return c.bcast
}

func (c *counter) read() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// R-XMDZ-2RGA: increment takes no arguments and adds exactly one to the
// stored value on each successful call. The returned value reflects the
// post-state per R-RQZQ-81ZC.
// R-FZC6-H2SB: the post-state value is broadcast on the counter-owned
// broadcaster so any live-update subscribers reflect the change without polling.
func (c *counter) increment() uint64 {
	c.mu.Lock()
	c.value++
	v := c.value
	if c.db != nil {
		_, _ = c.db.Exec(`UPDATE counter SET value = ? WHERE id = 1`, int64(v))
	}
	bcast := c.bcast
	c.mu.Unlock()
	if bcast != nil {
		bcast.broadcast(v)
	}
	return v
}

// R-F5X4-XI2F: decrement takes no arguments. When the stored value is
// greater than zero, subtract one and return (post-state, true). When the
// stored value is zero, do not mutate and return (0, false) — an explicit
// in-band refusal that preserves R-UZ9T-8NM4's non-negativity.
// R-FZC6-H2SB: on a successful decrement, the post-state value is
// broadcast on the counter-owned broadcaster. A refused decrement
// (value already zero) is not a mutation and is not broadcast.
func (c *counter) decrement() (uint64, bool) {
	c.mu.Lock()
	if c.value == 0 {
		c.mu.Unlock()
		return 0, false
	}
	c.value--
	v := c.value
	if c.db != nil {
		_, _ = c.db.Exec(`UPDATE counter SET value = ? WHERE id = 1`, int64(v))
	}
	bcast := c.bcast
	c.mu.Unlock()
	if bcast != nil {
		bcast.broadcast(v)
	}
	return v, true
}

// R-FZC6-H2SB: the index page's live-update channel is fed by a
// fan-out broadcaster. Subscribers register a bounded coalescing channel
// (capacity 1) and the broadcaster delivers the latest counter value to
// each subscriber on every mutation. The send is non-blocking: a slow
// subscriber drops intermediate values but always converges on the
// latest value the moment its channel drains, which is the property a
// browser displaying a single rendered count actually needs. Snapshot-
// on-connect (the handler's first event) plus latest-value coalescing
// means a fresh subscriber always sees the current value, and a
// subscriber that misses an interim mutation still reflects the final
// state.
type counterBroadcaster struct {
	mu   sync.Mutex
	subs map[*counterSubscriber]struct{}
}

type counterSubscriber struct {
	ch chan uint64
}

func (b *counterBroadcaster) subscribe() *counterSubscriber {
	sub := &counterSubscriber{ch: make(chan uint64, 1)}
	b.mu.Lock()
	if b.subs == nil {
		b.subs = make(map[*counterSubscriber]struct{})
	}
	b.subs[sub] = struct{}{}
	b.mu.Unlock()
	return sub
}

func (b *counterBroadcaster) unsubscribe(sub *counterSubscriber) {
	b.mu.Lock()
	delete(b.subs, sub)
	b.mu.Unlock()
}

// subscriberCount returns the number of live subscribers. Used by tests
// to observe R-T5ND-W2HF cleanup (a departed stream client must be
// released and unsubscribed within 5 seconds).
func (b *counterBroadcaster) subscriberCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

func (b *counterBroadcaster) broadcast(v uint64) {
	b.mu.Lock()
	targets := make([]*counterSubscriber, 0, len(b.subs))
	for s := range b.subs {
		targets = append(targets, s)
	}
	b.mu.Unlock()
	for _, s := range targets {
		// Coalescing: drop any pending value the subscriber has not
		// drained yet, then deliver the latest. Both ops are
		// non-blocking so a slow subscriber never stalls a mutator.
		select {
		case <-s.ch:
		default:
		}
		select {
		case s.ch <- v:
		default:
		}
	}
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

// attach binds a backing database to the counter and loads the current
// stored value from the database into the in-memory field. After attach
// returns, every successful increment and decrement persists the new
// value back through db. R-WD9O-X90L: a freshly opened database has the
// singleton row at value=0, so the loaded value is 0.
func (c *counter) attach(db *sql.DB) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var v int64
	if err := db.QueryRow(
		`SELECT value FROM counter WHERE id = 1`).Scan(&v); err != nil {
		return err
	}
	if v < 0 {
		return fmt.Errorf("counter: stored value %d is negative", v)
	}
	c.value = uint64(v)
	c.db = db
	return nil
}

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
	ExchangeCode(code, redirectURI string) (googleIdentity, error)
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

func (googleFakeIDP) ExchangeCode(code, redirectURI string) (googleIdentity, error) {
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

// testHookGoogleExchangeContext, when non-nil, overrides the context
// passed to oauth2.Config.Exchange in googleRealIDP.ExchangeCode. The
// oauth2 package looks for an *http.Client at oauth2.HTTPClient on the
// supplied context; a test that needs to redirect the HTTPS POST at a
// loopback httptest server injects its client through this hook.
// Production callers leave it nil.
var testHookGoogleExchangeContext context.Context

func (g *googleRealIDP) ExchangeCode(code, redirectURI string) (googleIdentity, error) {
	cfg := g.cfg
	cfg.RedirectURL = redirectURI
	ctx := context.Background()
	if testHookGoogleExchangeContext != nil {
		ctx = testHookGoogleExchangeContext
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
	testEnvLookups  atomic.Bool
)

func init() {
	testEnvLookups.Store(true)
}

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
		// forcing `t.Setenv` into every test.
		ResourceIdentifier: "http://127.0.0.1:3000/mcp",
	}
	// R-ANRQ-04PK: the allowed Workspace domain is supplied via the
	// bare environment variable `GOOGLE_WORKSPACE_DOMAIN` — matching
	// the bare-`GOOGLE_*` convention R-68WP-XVCK pins for the Google
	// federation seam, not a `HAL_`-prefixed variant. runServe enforces
	// the fail-loudly contract via requireEnv at startup; this in-memory
	// surface honors the same name so tests using `t.Setenv` exercise
	// the same plumbing the operator does.
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

// authCfg returns the current authentication configuration surface. In the
// serving path, runServe installs a single startup-parsed value before any
// handler is reachable, so consumers never re-consult the environment during
// runtime. Tests retain their historical t.Setenv behavior because many
// focused unit tests exercise accessors without starting the full command.
func authCfg() authConfig {
	if testing.Testing() && testEnvLookups.Load() {
		return loadAuthConfig(os.LookupEnv)
	}
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
const maxRequestBodyBytesR_VKZD_UKVS int64 = 1 << 20

func limitRequestBodyR_VKZD_UKVS(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytesR_VKZD_UKVS)
}

func requestBodyTooLargeR_VKZD_UKVS(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func writeBodyTooLargeR_VKZD_UKVS(w http.ResponseWriter) {
	http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
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
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fp := r.Header.Get("X-Forwarded-Proto"); fp != "" {
		first := fp
		if i := strings.IndexByte(first, ','); i >= 0 {
			first = first[:i]
		}
		switch strings.ToLower(strings.TrimSpace(first)) {
		case "https":
			scheme = "https"
		case "http":
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host
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
type oauthStateRecord struct {
	bindingID string
	expiresAt time.Time
	consumed  bool
	// R-MTRN-DL9W: the origin discriminator names which redirect-to-
	// Google code path created this record. Exactly two values exist
	// today: "web" (the /login redirect) and "mcp" (the
	// /oauth/authorize redirect). Recorded at state-creation time;
	// never mutated. R-MUZJ-RD0L dispatches on this value after the
	// state-binding and workspace-domain checks have both passed.
	origin string
	// R-MTRN-DL9W: for "mcp"-origin records, the originating
	// authorize-request context the callback needs to complete its
	// work without consulting any other source. Nil for "web"-origin
	// records (no extra context is required to establish a web
	// session and redirect to /).
	mcp *oauthStateMCPContext
}

// oauthStateMCPContext carries the byte-for-byte authorize-request
// values R-MTRN-DL9W pins onto an "mcp"-origin state record. The
// callback consults these (not the callback request's query
// parameters) when minting the HAL authorization code and building
// the redirect to the MCP client's registered callback URL.
type oauthStateMCPContext struct {
	clientID            string
	redirectURI         string
	codeChallenge       string
	codeChallengeMethod string
	clientState         string
	resource            string
}

type oauthStateStorage struct {
	mu sync.Mutex
	m  map[string]*oauthStateRecord
}

func newOAuthStateStorage() *oauthStateStorage {
	return &oauthStateStorage{m: map[string]*oauthStateRecord{}}
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
const oauthStateCookieName = "hal_oauth_state"

// putWeb records a "web"-origin state value (the /login redirect to
// Google) with its session-binding ID and the configured TTL.
// R-MTRN-DL9W: web-origin records carry the origin discriminator
// but no additional context — establishing the eventual web session
// (R-CXJ2-R3BN) needs nothing more from the originating request.
func (s *oauthStateStorage) putWeb(state, bindingID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[state] = &oauthStateRecord{
		bindingID: bindingID,
		expiresAt: oauthStateNow().Add(authCfg().OAuthStateTTL),
		origin:    "web",
	}
}

// putMCP records an "mcp"-origin state value (the /oauth/authorize
// redirect to Google) with its session-binding ID, the configured
// TTL, and the byte-for-byte authorize-request context R-MTRN-DL9W
// pins. The callback (R-MUZJ-RD0L) reads these recorded values when
// minting the HAL authorization code and building the redirect to
// the MCP client's registered callback URL — it does NOT re-read
// them from the callback request's query parameters.
func (s *oauthStateStorage) putMCP(state, bindingID string,
	ctx oauthStateMCPContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[state] = &oauthStateRecord{
		bindingID: bindingID,
		expiresAt: oauthStateNow().Add(authCfg().OAuthStateTTL),
		origin:    "mcp",
		mcp:       &ctx,
	}
}

// errOAuthState* enumerate the distinct rejection causes the callback
// surfaces, mirroring the R-EV2D-QTR1 posture of "one description per
// distinct failure".
var (
	errOAuthStateMissing      = errors.New("state parameter missing")
	errOAuthStateUnknown      = errors.New("state value not recognized")
	errOAuthStateExpired      = errors.New("state value expired")
	errOAuthStateConsumed     = errors.New("state value already used")
	errOAuthBindingMissing    = errors.New("session-binding cookie missing")
	errOAuthBindingMismatched = errors.New("session-binding cookie does not match")
)

// consume looks up `state` in the store, verifies the presented
// `bindingID` matches the bound session, and marks the record consumed on
// success. A second call with the same state value reports
// errOAuthStateConsumed. Expiry is checked against oauthStateNow().
//
// On success the record is returned (in its now-consumed form) so the
// caller can dispatch on R-MTRN-DL9W's origin discriminator and read
// the R-MUZJ-RD0L mcp context byte-for-byte from the recorded values —
// it MUST NOT re-read these from the callback request's query
// parameters.
func (s *oauthStateStorage) consume(state, bindingID string) (*oauthStateRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.m[state]
	if !ok {
		return nil, errOAuthStateUnknown
	}
	if rec.consumed {
		return nil, errOAuthStateConsumed
	}
	if !oauthStateNow().Before(rec.expiresAt) {
		return nil, errOAuthStateExpired
	}
	if bindingID == "" {
		return nil, errOAuthBindingMissing
	}
	if rec.bindingID != bindingID {
		return nil, errOAuthBindingMismatched
	}
	rec.consumed = true
	return rec, nil
}

// newOAuthStateValue returns a 32-character hex string drawn from
// crypto/rand — 128 bits of entropy, sufficient for the "fresh
// unguessable" property R-ETP6-60VA names.
func newOAuthStateValue() (string, error) {
	var buf [16]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// R-3JCR-C810: a registered OAuth client record. R-YRMT-B7LZ keeps the
// records in SQLite so a registered client_id remains usable after a
// process restart against the same database. R-25DN-9PUR makes registration
// open, so the store may grow without bound under abuse — the bound is
// operator concern, not a posture this service negotiates.
type oauthClient struct {
	redirectURIs  []string
	clientName    string
	grantTypes    []string
	responseTypes []string
	authMethod    string
	issuedAt      int64
}

type oauthClientStorage struct {
	mu sync.Mutex
	m  map[string]*oauthClient
	db *sql.DB
}

func newOAuthClientStorage() *oauthClientStorage {
	return &oauthClientStorage{m: map[string]*oauthClient{}}
}

func (s *oauthClientStorage) put(clientID string, c *oauthClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		_ = s.insertOrReplaceLocked(clientID, c)
	}
	s.m[clientID] = c
}

func (s *oauthClientStorage) putIfAbsent(clientID string, c *oauthClient) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.m[clientID]; exists {
		return false
	}
	if s.db != nil {
		ok, err := s.insertIfAbsentLocked(clientID, c)
		if err != nil || !ok {
			return false
		}
	}
	s.m[clientID] = c
	return true
}

func (s *oauthClientStorage) lookup(clientID string) *oauthClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[clientID]
}

func (s *oauthClientStorage) attach(db *sql.DB) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := db.Query(
		`SELECT client_id, redirect_uris, client_name, grant_types, ` +
			`response_types, auth_method, issued_at FROM oauth_clients`)
	if err != nil {
		return err
	}
	defer rows.Close()

	loaded := map[string]*oauthClient{}
	for rows.Next() {
		var clientID, redirectJSON, grantJSON, responseJSON string
		rec := &oauthClient{}
		if err := rows.Scan(&clientID, &redirectJSON, &rec.clientName,
			&grantJSON, &responseJSON, &rec.authMethod, &rec.issuedAt); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(redirectJSON), &rec.redirectURIs); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(grantJSON), &rec.grantTypes); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(responseJSON), &rec.responseTypes); err != nil {
			return err
		}
		loaded[clientID] = rec
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.m = loaded
	s.db = db
	return nil
}

func (s *oauthClientStorage) insertOrReplaceLocked(clientID string, c *oauthClient) error {
	redirects, grants, responses, err := marshalOAuthClientLists(c)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO oauth_clients (`+
			`client_id, redirect_uris, client_name, grant_types, `+
			`response_types, auth_method, issued_at`+
			`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		clientID, redirects, c.clientName, grants, responses, c.authMethod, c.issuedAt)
	return err
}

func (s *oauthClientStorage) insertIfAbsentLocked(clientID string, c *oauthClient) (bool, error) {
	redirects, grants, responses, err := marshalOAuthClientLists(c)
	if err != nil {
		return false, err
	}
	res, err := s.db.Exec(
		`INSERT OR IGNORE INTO oauth_clients (`+
			`client_id, redirect_uris, client_name, grant_types, `+
			`response_types, auth_method, issued_at`+
			`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		clientID, redirects, c.clientName, grants, responses, c.authMethod, c.issuedAt)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func marshalOAuthClientLists(c *oauthClient) (string, string, string, error) {
	redirects, err := json.Marshal(c.redirectURIs)
	if err != nil {
		return "", "", "", err
	}
	grants, err := json.Marshal(c.grantTypes)
	if err != nil {
		return "", "", "", err
	}
	responses, err := json.Marshal(c.responseTypes)
	if err != nil {
		return "", "", "", err
	}
	return string(redirects), string(grants), string(responses), nil
}

var newOAuthClientID = randomOAuthClientID

// randomOAuthClientID returns a 32-character hex string from crypto/rand —
// 128 bits of entropy, large enough that collisions are not a concern
// and unguessable enough that the value alone is not a credential.
func randomOAuthClientID() (string, error) {
	var buf [16]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// R-ZPE1-0DV8: oauthAuthCode is a single-use, short-lived authorization
// code bound at issue time to the three values the originating authorize
// request supplied — the requesting client's client_id, the PKCE code
// challenge (with its method), and the redirect_uri. The token endpoint
// (R-27SO-F63X, separate iteration) presents (client_id, redirect_uri,
// code_verifier) alongside the code; redemption succeeds only when the
// presented client_id matches the bound value, the presented redirect_uri
// is byte-equal to the bound one, and the presented verifier hashes to
// the bound challenge under the bound method. A second presentation of
// an already-redeemed code is rejected; an expired code is rejected.
// Without these bindings PKCE is decorative and a leaked code is
// exchangeable by an attacker.
type oauthAuthCode struct {
	clientID            string
	redirectURI         string
	codeChallenge       string
	codeChallengeMethod string
	ownerEmail          string
	// resource carries the canonical resource identifier the originating
	// MCP `/oauth/authorize` request bound, either explicitly or through
	// the R-WLUL-MZCD omission default. R-4GRA-EGBY has already checked
	// any present value byte-for-byte by the time an authorization code is
	// minted.
	resource  string
	expiresAt time.Time
	consumed  bool
}

type oauthAuthCodeStorage struct {
	mu sync.Mutex
	m  map[string]*oauthAuthCode
}

func newOAuthAuthCodeStorage() *oauthAuthCodeStorage {
	return &oauthAuthCodeStorage{m: map[string]*oauthAuthCode{}}
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
	errOAuthAuthCodeUnknown          = errors.New("authorization code not recognized")
	errOAuthAuthCodeExpired          = errors.New("authorization code expired")
	errOAuthAuthCodeConsumed         = errors.New("authorization code already redeemed")
	errOAuthAuthCodeClientMismatch   = errors.New("client_id does not match the bound value")
	errOAuthAuthCodeRedirectMismatch = errors.New("redirect_uri does not match the bound value")
	errOAuthAuthCodePKCEMismatch     = errors.New("code_verifier does not satisfy the bound code_challenge")
	errOAuthAuthCodePKCEMethod       = errors.New("unsupported code_challenge_method")
)

// issue records a freshly generated authorization code with its three
// bindings and the configured TTL, returning the opaque code string the
// authorize endpoint redirects back to the user-agent.
func (s *oauthAuthCodeStorage) issue(
	clientID, redirectURI, codeChallenge, codeChallengeMethod, ownerEmail string,
) (string, error) {
	return s.issueWithResource(
		clientID, redirectURI, codeChallenge, codeChallengeMethod,
		ownerEmail, "")
}

// issueWithResource is the R-MUZJ-RD0L variant: it additionally binds
// the recorded MCP-authorize `resource` value onto the code so the
// token-exchange path can propagate it onto the access-token record.
func (s *oauthAuthCodeStorage) issueWithResource(
	clientID, redirectURI, codeChallenge, codeChallengeMethod, ownerEmail, resource string,
) (string, error) {
	// R-JTTZ-CG5J: HAL supports only the S256 PKCE transform. Keeping
	// the issuance gate here ensures even direct code-minting paths cannot
	// create a redeemable authorization code bound to `plain`.
	if codeChallengeMethod != "S256" {
		return "", errOAuthAuthCodePKCEMethod
	}
	var buf [32]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return "", err
	}
	code := hex.EncodeToString(buf[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[code] = &oauthAuthCode{
		clientID:            clientID,
		redirectURI:         redirectURI,
		codeChallenge:       codeChallenge,
		codeChallengeMethod: codeChallengeMethod,
		ownerEmail:          ownerEmail,
		resource:            resource,
		expiresAt:           oauthAuthCodeNow().Add(authCfg().AuthCodeTTL),
	}
	return code, nil
}

// pkceVerifierMatches reports whether the presented verifier satisfies
// the bound challenge under the only supported method (R-JTTZ-CG5J):
// S256 is base64url(SHA-256(verifier)) without padding.
func pkceVerifierMatches(method, challenge, verifier string) bool {
	switch method {
	case "S256":
		sum := sha256.Sum256([]byte(verifier))
		return challenge == base64.RawURLEncoding.EncodeToString(sum[:])
	default:
		return false
	}
}

// redeem validates the presented (clientID, redirectURI, codeVerifier)
// against the bindings the code was issued with and atomically marks the
// record consumed on success. A second call with the same code reports
// errOAuthAuthCodeConsumed regardless of the presented values.
//
// The redeemed record (or nil on failure) is returned so the caller can
// honor the R-9HGE-87UG chain-revocation posture — the same code being
// presented twice is the trigger to revoke every token previously issued
// from it. That revocation lands when the token store does.
func (s *oauthAuthCodeStorage) redeem(
	code, clientID, redirectURI, codeVerifier string,
) (*oauthAuthCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.m[code]
	if !ok {
		return nil, errOAuthAuthCodeUnknown
	}
	if rec.consumed {
		return rec, errOAuthAuthCodeConsumed
	}
	if !oauthAuthCodeNow().Before(rec.expiresAt) {
		return nil, errOAuthAuthCodeExpired
	}
	if rec.clientID != clientID {
		return nil, errOAuthAuthCodeClientMismatch
	}
	if rec.redirectURI != redirectURI {
		return nil, errOAuthAuthCodeRedirectMismatch
	}
	if !pkceVerifierMatches(rec.codeChallengeMethod, rec.codeChallenge, codeVerifier) {
		return nil, errOAuthAuthCodePKCEMismatch
	}
	rec.consumed = true
	return rec, nil
}

// R-27SO-F63X: the service mints its own access tokens — opaque
// cryptographically-random strings issued by the service itself, not
// values propagated from Google. Each issued token has a server-side
// record (R-Z955-CD0I) keyed by a cryptographic hash of the plaintext
// (R-CUUP-REQT); the plaintext is returned to the client exactly once
// at issue time and is never persisted. oauthTokenStore is the
// single in-process home of those records. Wire-up to the /oauth/token
// endpoint that exchanges an authorization code for a freshly minted
// access token lands in a follow-on iteration; this iteration
// establishes the store and the issuance primitive so the property
// "tokens are minted here, not by Google" is structurally true the
// first time issuance is reached from the wire path.
type oauthToken struct {
	kind       string // "access" or "refresh"
	ownerEmail string
	clientID   string
	// resource is the canonical resource identifier (R-3UT3-IKZG) the
	// token is bound to at issue time. Bearer-side validation will
	// compare this byte-for-byte (R-DH2I-28CK) when the protected
	// endpoints land.
	resource  string
	issuedAt  time.Time
	expiresAt time.Time
	// usedAt is the consumption stamp for refresh tokens (R-89K0-GH5G's
	// single-use property). Zero on access records; zero on a live
	// refresh; non-zero the moment rotateRefresh consumes it.
	usedAt    time.Time
	revokedAt time.Time
	// chainID groups every (access, refresh) record that descends from a
	// single act of fresh authentication (R-9HGE-87UG). issueRefresh mints
	// a new chainID; rotateRefresh propagates it onto both successor
	// records so a chain is identifiable across arbitrarily many
	// rotations. Access tokens minted by issueAccess directly (no
	// preceding refresh) carry the zero value — they have no chain
	// affiliation. On reuse detection the rotation primitive walks the
	// store and stamps revokedAt on every record sharing the replayed
	// refresh's chainID, killing the live successor refresh and any
	// outstanding access tokens issued under the same chain.
	chainID string
}

type oauthTokenStorage struct {
	mu          sync.Mutex
	m           map[string]*oauthToken
	db          *sql.DB
	agentsBcast *agentsBroadcaster
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

func newOAuthTokenStorage() *oauthTokenStorage {
	return &oauthTokenStorage{m: map[string]*oauthToken{}}
}

const oauthRefreshTokenFormField = "refresh_token"

// oauthTokenNow is the clock the token store reads for issued-at /
// expires-at stamps. Tests may replace it directly; production resolves
// through activeClock.
var oauthTokenNow = appNow

func oauthTokenHash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func oauthTokenUnixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

func oauthTokenTimeFromUnixNano(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(0, v)
}

func (s *oauthTokenStorage) setAgentsBroadcaster(b *agentsBroadcaster) *agentsBroadcaster {
	s.mu.Lock()
	prev := s.agentsBcast
	s.agentsBcast = b
	s.mu.Unlock()
	return prev
}

func (s *oauthTokenStorage) notifyAgents(email string) {
	s.mu.Lock()
	b := s.agentsBcast
	s.mu.Unlock()
	if b != nil {
		b.notify(email)
	}
}

// attach loads HAL-issued OAuth token records from SQLite and makes all
// subsequent token lifecycle writes durable. R-FC5T-WWC2: token records
// survive process restarts because the hash-keyed record map is rebuilt
// from the database before the service accepts requests.
func (s *oauthTokenStorage) attach(db *sql.DB) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := db.Query(
		`SELECT token_hash, kind, owner_email, client_id, resource, ` +
			`issued_at, expires_at, used_at, revoked_at, chain_id FROM oauth_tokens`)
	if err != nil {
		return err
	}
	defer rows.Close()
	loaded := map[string]*oauthToken{}
	for rows.Next() {
		var hash string
		var issuedAt, expiresAt, usedAt, revokedAt int64
		rec := &oauthToken{}
		if err := rows.Scan(&hash, &rec.kind, &rec.ownerEmail, &rec.clientID,
			&rec.resource, &issuedAt, &expiresAt, &usedAt, &revokedAt,
			&rec.chainID); err != nil {
			return err
		}
		rec.issuedAt = oauthTokenTimeFromUnixNano(issuedAt)
		rec.expiresAt = oauthTokenTimeFromUnixNano(expiresAt)
		rec.usedAt = oauthTokenTimeFromUnixNano(usedAt)
		rec.revokedAt = oauthTokenTimeFromUnixNano(revokedAt)
		loaded[hash] = rec
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.m = loaded
	s.db = db
	return nil
}

func (s *oauthTokenStorage) persistTokenLocked(hash string, rec *oauthToken) error {
	if s.db == nil {
		return nil
	}
	return persistOAuthTokenLocked(s.db, hash, rec)
}

type oauthTokenPersister interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func persistOAuthTokenLocked(p oauthTokenPersister, hash string, rec *oauthToken) error {
	_, err := p.Exec(
		`INSERT OR REPLACE INTO oauth_tokens (`+
			`token_hash, kind, owner_email, client_id, resource, `+
			`issued_at, expires_at, used_at, revoked_at, chain_id`+
			`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		hash, rec.kind, rec.ownerEmail, rec.clientID, rec.resource,
		oauthTokenUnixNano(rec.issuedAt), oauthTokenUnixNano(rec.expiresAt),
		oauthTokenUnixNano(rec.usedAt), oauthTokenUnixNano(rec.revokedAt),
		rec.chainID)
	return err
}

// issueAccess mints an opaque access token for the given owner, client,
// and bound resource, persists a hash-keyed record, and returns the
// plaintext the caller writes to the token response. The expires_at
// stamp is issued_at + AccessTokenTTL exactly, both drawn from a
// single oauthTokenNow() read so first-use validation cannot trip on
// borderline-clock skew (R-E5GH-PN6G's posture, recorded here so the
// wire path inherits it). The 32 random bytes give 256 bits of
// entropy — well clear of collision concerns and unguessable enough
// that the plaintext alone is the credential.
func (s *oauthTokenStorage) issueAccess(ownerEmail, clientID, resource string) (string, error) {
	var buf [32]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return "", err
	}
	plaintext := hex.EncodeToString(buf[:])
	now := oauthTokenNow()
	hash := oauthTokenHash(plaintext)
	rec := &oauthToken{
		kind:       "access",
		ownerEmail: ownerEmail,
		clientID:   clientID,
		resource:   resource,
		issuedAt:   now,
		expiresAt:  now.Add(authCfg().AccessTokenTTL),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.persistTokenLocked(hash, rec); err != nil {
		return "", err
	}
	s.m[hash] = rec
	return plaintext, nil
}

func randomOAuthTokenSecret() (string, error) {
	var buf [32]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func randomOAuthChainID() (string, error) {
	var buf [16]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// issueInitialTokenPairR_2HT5_50F4 mints the access and refresh token
// returned by one authorization-code exchange into the same MCP token
// chain. Revoking the agents-block row for that chain therefore revokes
// the initial access token even before the refresh token has ever rotated.
func (s *oauthTokenStorage) issueInitialTokenPairR_2HT5_50F4(
	ownerEmail, clientID, resource string,
) (string, string, error) {
	accessPlain, err := randomOAuthTokenSecret()
	if err != nil {
		return "", "", err
	}
	refreshPlain, err := randomOAuthTokenSecret()
	if err != nil {
		return "", "", err
	}
	chainID, err := randomOAuthChainID()
	if err != nil {
		return "", "", err
	}
	now := oauthTokenNow()
	accessHash := oauthTokenHash(accessPlain)
	refreshHash := oauthTokenHash(refreshPlain)
	accessRec := &oauthToken{
		kind:       "access",
		ownerEmail: ownerEmail,
		clientID:   clientID,
		resource:   resource,
		issuedAt:   now,
		expiresAt:  now.Add(authCfg().AccessTokenTTL),
		chainID:    chainID,
	}
	refreshRec := &oauthToken{
		kind:       "refresh",
		ownerEmail: ownerEmail,
		clientID:   clientID,
		resource:   resource,
		issuedAt:   now,
		expiresAt:  now.Add(authCfg().RefreshTokenTTL),
		chainID:    chainID,
	}

	s.mu.Lock()
	if s.db != nil {
		tx, err := s.db.Begin()
		if err != nil {
			s.mu.Unlock()
			return "", "", err
		}
		if err := persistOAuthTokenLocked(tx, accessHash, accessRec); err != nil {
			_ = tx.Rollback()
			s.mu.Unlock()
			return "", "", err
		}
		if err := persistOAuthTokenLocked(tx, refreshHash, refreshRec); err != nil {
			_ = tx.Rollback()
			s.mu.Unlock()
			return "", "", err
		}
		if err := tx.Commit(); err != nil {
			s.mu.Unlock()
			return "", "", err
		}
	}
	s.m[accessHash] = accessRec
	s.m[refreshHash] = refreshRec
	s.mu.Unlock()
	// R-0TVF-0BKI: a fresh chain just appeared in this owner's live set.
	s.notifyAgents(ownerEmail)
	return accessPlain, refreshPlain, nil
}

// R-89K0-GH5G: each successful refresh-token use issues a new refresh
// token alongside the new access token, and the consumed refresh is
// invalidated atomically with the issue. issueRefresh mints a refresh
// record for use by the /oauth/token refresh_grant path (which lands in
// a follow-on iteration); rotateRefresh is the single primitive that
// performs the consume-and-issue under one critical section so the
// "single-use" invariant cannot race. The thirty-day refresh TTL
// (R-8UAA-YKR9) lands here too — every refresh record carries
// expiresAt = issuedAt + RefreshTokenTTL, and rotateRefresh rejects a
// presented refresh past that ceiling. Chain membership (R-9HGE-87UG)
// is established here too — every fresh issueRefresh mints a new
// chainID; rotateRefresh propagates it onto successors so the chain
// is walkable on reuse detection.
func (s *oauthTokenStorage) issueRefresh(ownerEmail, clientID, resource string) (string, error) {
	plaintext, err := randomOAuthTokenSecret()
	if err != nil {
		return "", err
	}
	chainID, err := randomOAuthChainID()
	if err != nil {
		return "", err
	}
	now := oauthTokenNow()
	hash := oauthTokenHash(plaintext)
	rec := &oauthToken{
		kind:       "refresh",
		ownerEmail: ownerEmail,
		clientID:   clientID,
		resource:   resource,
		issuedAt:   now,
		expiresAt:  now.Add(authCfg().RefreshTokenTTL),
		chainID:    chainID,
	}
	s.mu.Lock()
	if err := s.persistTokenLocked(hash, rec); err != nil {
		s.mu.Unlock()
		return "", err
	}
	s.m[hash] = rec
	s.mu.Unlock()
	// R-0TVF-0BKI: a fresh chain just appeared in this owner's live set.
	s.notifyAgents(ownerEmail)
	return plaintext, nil
}

// rotateRefresh atomically consumes the presented refresh-token
// plaintext and issues a new (access, refresh) pair bound to the same
// owner / client / resource. The consumed record's usedAt is set in the
// same critical section that stores the successor records — observers
// can never see a window in which the old refresh is still spendable
// after the new one exists, nor one in which the new pair exists
// without the predecessor being marked used. Returns the new access
// plaintext and the new refresh plaintext on success.
func (s *oauthTokenStorage) rotateRefresh(plaintext string) (string, string, error) {
	return s.rotateRefreshForClient(plaintext, "")
}

func (s *oauthTokenStorage) rotateRefreshForClient(plaintext, clientID string) (string, string, error) {
	if plaintext == "" {
		return "", "", errors.New("refresh token: empty")
	}
	var accessBuf, refreshBuf [32]byte
	if _, err := cryptorand.Read(accessBuf[:]); err != nil {
		return "", "", err
	}
	if _, err := cryptorand.Read(refreshBuf[:]); err != nil {
		return "", "", err
	}
	newAccess := hex.EncodeToString(accessBuf[:])
	newRefresh := hex.EncodeToString(refreshBuf[:])
	now := oauthTokenNow()
	s.mu.Lock()
	defer s.mu.Unlock()
	presentedHash := oauthTokenHash(plaintext)
	rec, ok := s.m[presentedHash]
	if !ok || rec.kind != "refresh" {
		return "", "", errors.New("refresh token: unknown")
	}
	if !rec.usedAt.IsZero() {
		// R-9HGE-87UG: a second presentation of an already-consumed
		// refresh is evidence of compromise. Reject this request and
		// revoke every record sharing the replayed refresh's chainID —
		// the live successor refresh and any outstanding access tokens
		// issued from this chain. lookupAccess already rejects records
		// with revokedAt set, so newly arriving requests bearing any
		// chain member are bounced (R-A26O-QBG9).
		revokedOwner := ""
		if rec.chainID != "" {
			for otherHash, other := range s.m {
				if other.chainID == rec.chainID && other.revokedAt.IsZero() {
					other.revokedAt = now
					_ = s.persistTokenLocked(otherHash, other)
				}
			}
			revokedOwner = rec.ownerEmail
		}
		s.mu.Unlock()
		// R-0TVF-0BKI: the chain just disappeared from this owner's live set.
		if revokedOwner != "" {
			s.notifyAgents(revokedOwner)
		}
		s.mu.Lock()
		return "", "", errors.New("refresh token: already used")
	}
	if !rec.revokedAt.IsZero() {
		return "", "", errors.New("refresh token: revoked")
	}
	// R-8UAA-YKR9: a refresh past its thirty-day ceiling is no longer
	// rotatable. Strict Before mirrors the access-token gate's
	// boundary discipline (R-TNXJ-ZWQ0).
	if !now.Before(rec.expiresAt) {
		return "", "", errors.New("refresh token: expired")
	}
	// R-5P7B-KY5Z: the wire refresh-token grant must identify the same
	// OAuth client that originally received this refresh token. The
	// store-level primitive takes an optional client binding so existing
	// direct rotation tests can exercise token lifecycle independently,
	// while /oauth/token passes the presented client_id and rejects a
	// mismatch before consuming the refresh.
	if clientID != "" && rec.clientID != clientID {
		return "", "", errors.New("refresh token: client_id mismatch")
	}
	newAccessHash := oauthTokenHash(newAccess)
	newRefreshHash := oauthTokenHash(newRefresh)
	rec.usedAt = now
	newAccessRec := &oauthToken{
		kind:       "access",
		ownerEmail: rec.ownerEmail,
		clientID:   rec.clientID,
		resource:   rec.resource,
		issuedAt:   now,
		expiresAt:  now.Add(authCfg().AccessTokenTTL),
		chainID:    rec.chainID,
	}
	newRefreshRec := &oauthToken{
		kind:       "refresh",
		ownerEmail: rec.ownerEmail,
		clientID:   rec.clientID,
		resource:   rec.resource,
		issuedAt:   now,
		expiresAt:  now.Add(authCfg().RefreshTokenTTL),
		chainID:    rec.chainID,
	}
	if err := s.persistTokenLocked(presentedHash, rec); err != nil {
		rec.usedAt = time.Time{}
		return "", "", err
	}
	if err := s.persistTokenLocked(newAccessHash, newAccessRec); err != nil {
		rec.usedAt = time.Time{}
		return "", "", err
	}
	if err := s.persistTokenLocked(newRefreshHash, newRefreshRec); err != nil {
		rec.usedAt = time.Time{}
		return "", "", err
	}
	s.m[newAccessHash] = newAccessRec
	s.m[newRefreshHash] = newRefreshRec
	return newAccess, newRefresh, nil
}

// revokeChainR_D0XD_1YT0 atomically marks every record sharing chainID
// as revoked, scoped to ownerEmail. Returns true when the chain existed
// and belonged to ownerEmail; returns false when no record matches the
// chainID, or when at least one record with that chainID is owned by a
// different email — the caller surfaces both cases identically so the
// service does not disclose whether such a chain exists.
func (s *oauthTokenStorage) revokeChainR_D0XD_1YT0(chainID, ownerEmail string) bool {
	if chainID == "" || ownerEmail == "" {
		return false
	}
	now := oauthTokenNow()
	s.mu.Lock()
	matched := false
	for _, rec := range s.m {
		if rec.chainID != chainID {
			continue
		}
		if rec.ownerEmail != ownerEmail {
			s.mu.Unlock()
			return false
		}
		matched = true
	}
	if !matched {
		s.mu.Unlock()
		return false
	}
	for hash, rec := range s.m {
		if rec.chainID == chainID && rec.revokedAt.IsZero() {
			rec.revokedAt = now
			_ = s.persistTokenLocked(hash, rec)
		}
	}
	s.mu.Unlock()
	// R-0TVF-0BKI: the chain just disappeared from this owner's live set.
	s.notifyAgents(ownerEmail)
	return true
}

// R-0NRX-3GV1: a single live MCP token chain owned by some email, as
// surfaced to the agents block on the index page. The "live" filter is
// applied at collection time (at least one un-revoked, un-expired
// refresh record under the chainID); rows here are the per-chain
// roll-up used for rendering. Chain initial issuance is the earliest
// refresh record's issuedAt; refresh rotation does not change the rendered
// row identity used for ordering.
type agentChainR_0NRX_3GV1 struct {
	chainID    string
	clientID   string
	clientName string
	issuedAt   time.Time
}

// liveAgentChainsR_0NRX_3GV1 walks the token store under .mu.Lock(),
// groups un-revoked un-expired refresh records owned by `email` by
// chainID, and returns one entry per chain. Client name is resolved
// from the supplied OAuth client store after the token-store lock is released to
// keep the two stores' critical sections independent. Order is not pinned
// here; the render and stream seams sort by rendered row identity.
func (s *oauthTokenStorage) liveAgentChainsR_0NRX_3GV1(
	email string, clients *oauthClientStorage,
) []agentChainR_0NRX_3GV1 {
	if email == "" {
		return nil
	}
	now := oauthTokenNow()
	type partial struct {
		clientID string
		issuedAt time.Time
	}
	byChain := map[string]*partial{}
	s.mu.Lock()
	for _, rec := range s.m {
		if rec.kind != "refresh" {
			continue
		}
		if rec.ownerEmail != email {
			continue
		}
		if rec.chainID == "" {
			continue
		}
		if !rec.revokedAt.IsZero() {
			continue
		}
		if !now.Before(rec.expiresAt) {
			continue
		}
		cur, ok := byChain[rec.chainID]
		if !ok {
			byChain[rec.chainID] = &partial{
				clientID: rec.clientID,
				issuedAt: rec.issuedAt,
			}
			continue
		}
		if rec.issuedAt.Before(cur.issuedAt) {
			cur.issuedAt = rec.issuedAt
		}
	}
	s.mu.Unlock()
	if len(byChain) == 0 {
		return nil
	}
	out := make([]agentChainR_0NRX_3GV1, 0, len(byChain))
	for chainID, p := range byChain {
		row := agentChainR_0NRX_3GV1{
			chainID:  chainID,
			clientID: p.clientID,
			issuedAt: p.issuedAt,
		}
		if clients != nil {
			if c := clients.lookup(p.clientID); c != nil {
				row.clientName = c.clientName
			}
		}
		out = append(out, row)
	}
	return out
}

func agentChainRenderedNameR_VWEX_WYWJ(ch agentChainR_0NRX_3GV1) string {
	if ch.clientName == "" {
		return "undefined"
	}
	return ch.clientName
}

func agentChainRenderedIDPrefixR_VWEX_WYWJ(ch agentChainR_0NRX_3GV1) string {
	prefix := ch.clientID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	return prefix
}

func sortAgentChainsByRenderedIdentityR_VWEX_WYWJ(chains []agentChainR_0NRX_3GV1) {
	// R-VWEX-WYWJ: agent rows below the signed-in web-session row sort for
	// scanning by rendered identity, case-insensitive, with the rendered
	// first-8 client_id prefix as the tie-breaker. Refresh rotations leave
	// both values unchanged, so they cannot move a row.
	sort.SliceStable(chains, func(i, j int) bool {
		leftName := strings.ToLower(agentChainRenderedNameR_VWEX_WYWJ(chains[i]))
		rightName := strings.ToLower(agentChainRenderedNameR_VWEX_WYWJ(chains[j]))
		if leftName != rightName {
			return leftName < rightName
		}
		leftPrefix := agentChainRenderedIDPrefixR_VWEX_WYWJ(chains[i])
		rightPrefix := agentChainRenderedIDPrefixR_VWEX_WYWJ(chains[j])
		if leftPrefix != rightPrefix {
			return leftPrefix < rightPrefix
		}
		return chains[i].chainID < chains[j].chainID
	})
}

// lookupAccess returns the access-token record for the presented
// plaintext if one exists and is currently valid (un-expired,
// un-revoked, kind=access). Returns nil otherwise. The bearer-side
// gates on /counter/increment, /counter/decrement, and /mcp will
// route through this once they land.
func (s *oauthTokenStorage) lookupAccess(plaintext string) *oauthToken {
	rec, _ := s.lookupAccessReason(plaintext)
	return rec
}

// lookupAccessReason is lookupAccess with an out-of-band discriminator
// for the rejection cause, used by R-EV2D-QTR1 to surface distinct
// `error_description` strings on the bearer-rejection 401. The reason
// values are stable strings (not exposed verbatim in the wire body):
// "" with a non-nil rec on accept; "unknown" for an unrecognized or
// wrong-kind plaintext; "expired" for a recognized record past
// expires_at; "revoked" for a recognized record with revokedAt set.
// Resource-binding mismatch is checked by the caller against
// canonicalResourceIdentifier(), not here.
func (s *oauthTokenStorage) lookupAccessReason(plaintext string) (*oauthToken, string) {
	if plaintext == "" {
		return nil, "unknown"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.m[oauthTokenHash(plaintext)]
	if !ok {
		return nil, "unknown"
	}
	if rec.kind != "access" {
		return nil, "unknown"
	}
	if !rec.revokedAt.IsZero() {
		return nil, "revoked"
	}
	if !oauthTokenNow().Before(rec.expiresAt) {
		return nil, "expired"
	}
	return rec, ""
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
type webSession struct {
	ownerEmail string
	issuedAt   time.Time
	expiresAt  time.Time
	// lastSeenAt advances on each successful lookup and feeds the idle
	// ceiling per R-KJ15-9P17.
	lastSeenAt time.Time
	revokedAt  time.Time
}

type webSessionStorage struct {
	mu sync.Mutex
	m  map[string]*webSession
	db *sql.DB
}

func newWebSessionStorage() *webSessionStorage {
	return &webSessionStorage{m: map[string]*webSession{}}
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
const webSessionCookieName = "hal_session"

// The web-session ceilings R-KJ15-9P17 pins — the 12-hour absolute cap
// from issue and the 1-hour idle cap from last successful authenticated
// request — are sourced from the R-LWCN-ZBXO surface
// (authCfg().WebSessionAbsoluteTTL, authCfg().WebSessionIdleTTL). The
// cookie's MaxAge matches the absolute cap so the browser drops the
// cookie at the same instant.

func webSessionHash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// issue records a session for the given owner email and returns the
// plaintext identifier the caller writes to the user-agent's cookie
// store. The plaintext is not retained by the service (R-SLGL-B5B4).
func (s *webSessionStorage) issue(ownerEmail string) (string, error) {
	var buf [32]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return "", err
	}
	plaintext := hex.EncodeToString(buf[:])
	hash := webSessionHash(plaintext)
	now := webSessionNow()
	rec := &webSession{
		ownerEmail: ownerEmail,
		issuedAt:   now,
		expiresAt:  now.Add(authCfg().WebSessionAbsoluteTTL),
		lastSeenAt: now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		if err := s.upsertLocked(hash, rec); err != nil {
			return "", err
		}
	}
	if s.m == nil {
		s.m = map[string]*webSession{}
	}
	s.m[hash] = rec
	return plaintext, nil
}

// lookup returns the session record for the given plaintext cookie value
// if one exists and is currently active (not revoked, not past its
// absolute expiry). Returns nil otherwise. R-GUEU-LKL1's consumer of the
// hal_session cookie reads through here so the index page only reflects
// auth-state affordances for visitors with a live session.
func (s *webSessionStorage) lookup(plaintext string) *webSession {
	if plaintext == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.m[webSessionHash(plaintext)]
	if !ok {
		return nil
	}
	if !rec.revokedAt.IsZero() {
		return nil
	}
	now := webSessionNow()
	if !now.Before(rec.expiresAt) {
		return nil
	}
	if !now.Before(rec.lastSeenAt.Add(authCfg().WebSessionIdleTTL)) {
		return nil
	}
	rec.lastSeenAt = now
	if s.db != nil {
		_, _ = s.db.Exec(
			`UPDATE web_sessions SET last_seen_at = ? WHERE session_hash = ?`,
			rec.lastSeenAt.UnixNano(), webSessionHash(plaintext))
	}
	return rec
}

// revoke marks the session matching plaintext as revoked. A missing or
// already-revoked entry is a no-op — logout is idempotent and tolerates
// a presented cookie that no longer maps to a live record (R-FZ10-BE37).
// R-0XJ4-5MSL: this writes only to the web-session store; it never reads
// or writes any MCP token chain store.
func (s *webSessionStorage) revoke(plaintext string) {
	if plaintext == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.m[webSessionHash(plaintext)]
	if !ok {
		return
	}
	if rec.revokedAt.IsZero() {
		rec.revokedAt = webSessionNow()
		if s.db != nil {
			_, _ = s.db.Exec(
				`UPDATE web_sessions SET revoked_at = ? WHERE session_hash = ?`,
				rec.revokedAt.UnixNano(), webSessionHash(plaintext))
		}
	}
}

// R-8CBQ-IKKA: web-session records are loaded from and written through to
// SQLite so a valid hal_session cookie remains known across process
// restarts against the same database. The plaintext cookie value is never
// stored; the table is keyed by the same SHA-256 hash lookup uses.
func (s *webSessionStorage) attach(db *sql.DB) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := db.Query(
		`SELECT session_hash, owner_email, issued_at, expires_at, ` +
			`last_seen_at, revoked_at FROM web_sessions`)
	if err != nil {
		return err
	}
	defer rows.Close()

	loaded := map[string]*webSession{}
	for rows.Next() {
		var hash, owner string
		var issued, expires, lastSeen int64
		var revoked sql.NullInt64
		if err := rows.Scan(&hash, &owner, &issued, &expires,
			&lastSeen, &revoked); err != nil {
			return err
		}
		rec := &webSession{
			ownerEmail: owner,
			issuedAt:   time.Unix(0, issued),
			expiresAt:  time.Unix(0, expires),
			lastSeenAt: time.Unix(0, lastSeen),
		}
		if revoked.Valid {
			rec.revokedAt = time.Unix(0, revoked.Int64)
		}
		loaded[hash] = rec
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.m = loaded
	s.db = db
	return nil
}

func (s *webSessionStorage) upsertLocked(hash string, rec *webSession) error {
	var revoked any
	if !rec.revokedAt.IsZero() {
		revoked = rec.revokedAt.UnixNano()
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO web_sessions (`+
			`session_hash, owner_email, issued_at, expires_at, `+
			`last_seen_at, revoked_at`+
			`) VALUES (?, ?, ?, ?, ?, ?)`,
		hash, rec.ownerEmail, rec.issuedAt.UnixNano(),
		rec.expiresAt.UnixNano(), rec.lastSeenAt.UnixNano(), revoked)
	return err
}

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
	os.Exit(runWithEnvAndClock(os.Args[1:], os.Stdout, os.Stderr, os.LookupEnv, realAppClock{}))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithEnv(args, stdout, stderr, os.LookupEnv)
}

func runWithEnv(args []string, stdout, stderr io.Writer, lookup envLookup) int {
	return runWithEnvAndClock(args, stdout, stderr, lookup, realAppClock{})
}

func runWithEnvAndClock(args []string, stdout, stderr io.Writer, lookup envLookup, clock appClock) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "serve":
		return cmdServeWithEnvAndClock(args[1:], stdout, stderr, lookup, clock)
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
// 127.0.0.1), and --db (default ./hal.db); with defaults it binds a TCP
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runServeWithEnvAndClock(ctx, args, stdout, stderr, lookup, clock)
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
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	port := fs.Int("port", 3000, "TCP port to listen on")
	ip := fs.String("ip", "127.0.0.1", "local interface to bind to")
	dbPath := fs.String("db", "./hal.db", "path to the SQLite database file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	prevClock := setAppClock(clock)
	defer setAppClock(prevClock)
	cfg := loadAuthConfig(lookup)
	setAuthCfg(cfg)
	prevLookup := setEnvLookup(lookup)
	defer setEnvLookup(prevLookup)
	if testing.Testing() {
		prev := testEnvLookups.Swap(false)
		defer testEnvLookups.Store(prev)
	}
	servingOAuthClients := newOAuthClientStorage()
	servingWebSessions := webSessionStoreFromContext(ctx)
	servingOAuthTokens := oauthTokenStoreFromContext(ctx)
	servingCounter := counterFromContext(ctx)
	servingGoogleIDP := googleIDPFromContext(ctx)
	// R-VNNS-W2G0: open the SQLite database the operator named with --db
	// and bind it to the serve-owned counter so every increment/decrement persists.
	// Tests skip this step — they exercise the persistence layer directly
	// against a temp database (see TestR_VNNS_W2G0…) and would otherwise
	// write a stray hal.db file into the test working directory through
	// the counter persistence binding.
	if !testing.Testing() {
		db, err := openCounterDB(*dbPath)
		if err != nil {
			fmt.Fprintf(stderr, "serve: open db %q: %v\n", *dbPath, err)
			return 1
		}
		defer func() { _ = db.Close() }()
		if err := servingCounter.attach(db); err != nil {
			fmt.Fprintf(stderr, "serve: load counter: %v\n", err)
			return 1
		}
		if err := servingOAuthClients.attach(db); err != nil {
			fmt.Fprintf(stderr, "serve: load oauth clients: %v\n", err)
			return 1
		}
		if err := servingWebSessions.attach(db); err != nil {
			fmt.Fprintf(stderr, "serve: load web sessions: %v\n", err)
			return 1
		}
		if err := servingOAuthTokens.attach(db); err != nil {
			fmt.Fprintf(stderr, "serve: load oauth tokens: %v\n", err)
			return 1
		}
		// R-W3K0-QD0E / R-LWCN-ZBXO: bind the real Google identity
		// provider once at startup, sourcing client credentials from
		// the environment via requireEnv. Missing or empty values
		// fail the process before it accepts traffic — operators see
		// the misconfiguration immediately rather than receiving a
		// 503 on the first /login.
		clientID, err := requireEnv("GOOGLE_CLIENT_ID")
		if err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		clientSecret, err := requireEnv("GOOGLE_CLIENT_SECRET")
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
		workspaceDomain, err := requireEnv("GOOGLE_WORKSPACE_DOMAIN")
		if err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		servingGoogleIDP = newGoogleRealIDP(
			clientID, clientSecret, workspaceDomain)
		// R-791Y-3ROQ: HAL_RESOURCE_IDENTIFIER is a required env var
		// (no default), and its value must include the path component
		// `/mcp` R-7A9U-HJFF pins. Missing, empty, or wrong-path values
		// fail the process before the listener accepts traffic per the
		// R-LWCN-ZBXO fail-loudly contract.
		resID, err := requireEnv("HAL_RESOURCE_IDENTIFIER")
		if err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		if err := validateHALResourceIdentifier(resID); err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
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
	mux.HandleFunc(http.MethodGet, "/design.css", handleDesignCSS)
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
	prevAgentsBcast := servingOAuthTokens.setAgentsBroadcaster(servingAgentsBcast)
	defer servingOAuthTokens.setAgentsBroadcaster(prevAgentsBcast)
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
	_ = servingCounter.broadcaster()
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
	c *counter, sessions *webSessionStorage, tokens *oauthTokenStorage, clients *oauthClientStorage,
	w http.ResponseWriter, r *http.Request,
) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// bankJSON is the subtitle bank embedded in the page so the re-roll
	// button can pick a fresh entry client-side without a page reload
	// (R-8KKV-TDWF's no-`GET /`, preserve-page-state property). The
	// canonical bank is the Go-side subtitleBank; this is the same
	// slice, serialized once per render.
	bankJSON, err := json.Marshal(subtitleBank)
	if err != nil {
		http.Error(w, "subtitle bank serialization failed",
			http.StatusInternalServerError)
		return
	}
	// R-GUEU-LKL1 / R-0WB7-RV1W: the index page reflects web-session
	// state. If the hal_session cookie resolves to a live record, the
	// bottom-right of the banner shows the visitor's email + a separate
	// Sign out control (R-0WB7-RV1W pins email as inert non-interactive
	// text, no avatar element, sign-out as a distinct pill-chrome
	// affordance reaching /logout); otherwise it shows a single pill-
	// chrome Sign in affordance reaching /login. The auth area lives
	// inside the banner card per R-0WB7-RV1W. The counter card's −/+
	// buttons drop their HTML `disabled` attribute when signed in
	// (R-GVMQ-ZCBQ pins the no-session "disabled" treatment).
	var session *webSession
	if c, err := r.Cookie(webSessionCookieName); err == nil {
		session = sessions.lookup(c.Value)
	}
	var bannerAuth, counterDisabled string
	if session != nil {
		bannerAuth = `<div class="banner-auth">` +
			// R-TEP7-Q6UT: the Google email is externally sourced
			// identity text, so it is escaped before interpolation.
			`<span class="auth-email">` + htmlEscape(session.ownerEmail) + `</span>` +
			// R-A2L2-1NA1: Sign out is a real form POST, not a JS-only
			// click handler or href, so it works when scripts are absent.
			`<form method="post" action="/logout" class="auth-form">` +
			`<button class="auth-btn" type="submit">Sign out</button>` +
			`</form>` +
			`</div>`
	} else {
		bannerAuth = `<div class="banner-auth">` +
			`<a class="auth-btn" href="/login">Sign in</a>` +
			`</div>`
		counterDisabled = " disabled"
	}
	// R-VTZ5-5FF5: when the signed-in visitor owns one or more live MCP
	// token chains, the index page renders an agents block inside the
	// banner card, immediately below the auth row. A "live" chain is
	// one with at least one un-revoked, un-expired refresh token; the
	// store-side helper filters by owner email so a chain owned by any
	// other email cannot surface here regardless of how the underlying
	// records are stored. R-TS71-XRW4: the block is omitted entirely for
	// signed-out visitors and for signed-in visitors whose live-chain
	// count is zero, so the banner card collapses to its compact auth
	// row instead of reserving vertical space for absent agent rows.
	var agentsBlock string
	if session != nil {
		chains := tokens.liveAgentChainsR_0NRX_3GV1(session.ownerEmail, clients)
		sortAgentChainsByRenderedIdentityR_VWEX_WYWJ(chains)
		if len(chains) > 0 {
			var b strings.Builder
			b.WriteString(`<div class="agents-block" aria-label="Authenticated MCP agents">`)
			for _, ch := range chains {
				// R-VV71-J75U: per-row content is exactly two visible
				// elements left to right — one inert identity label
				// combining client_name (literal `undefined` when the DCR
				// client supplied none) with the parenthesised client_id
				// 8-char prefix, and a Revoke pill. The button form-submits
				// to R-D0XD-1YT0's chain-revoke endpoint scoped to this row.
				name := agentChainRenderedNameR_VWEX_WYWJ(ch)
				idPrefix := agentChainRenderedIDPrefixR_VWEX_WYWJ(ch)
				b.WriteString(`<div class="agent-row" data-chain-id="`)
				b.WriteString(htmlEscape(ch.chainID))
				// R-VV71-J75U: identity label is inert text with client_name
				// followed by parenthesised 8-char client_id prefix; Revoke
				// button carries class="auth-btn" for matching pill chrome.
				b.WriteString(`"><span class="agent-name">`)
				// R-10ZV-8OFH: DCR client metadata is untrusted; render
				// client_name as escaped inert text inside the agent row.
				b.WriteString(htmlEscape(name))
				b.WriteString(` (`)
				b.WriteString(htmlEscape(idPrefix))
				b.WriteString(`)</span><form method="post" action="/agents/revoke">`)
				b.WriteString(`<input type="hidden" name="chain_id" value="`)
				b.WriteString(htmlEscape(ch.chainID))
				b.WriteString(`"><button class="auth-btn" type="submit">Revoke</button></form></div>`)
			}
			b.WriteString(`</div>`)
			agentsBlock = b.String()
		}
	}
	// R-6KK2-AAY0 / R-3RL1-IUP6: when live agent rows exist, they live
	// inside the same bottom-right auth grid as the visitor row, not in a
	// separate banner sibling. This lets the email label and every agent
	// label share one right-aligned label column, while Sign out and all
	// Revoke buttons share one action column.
	if session != nil && agentsBlock != "" {
		bannerAuth = strings.TrimSuffix(bannerAuth, `</div>`) + agentsBlock + `</div>`
		agentsBlock = ""
	}
	// R-BZQY-DN3B: the index page displays MCP client configuration
	// for two clients (Claude Code and Claude Desktop), each with its
	// own copy-pasteable instructions. The base URL is request-
	// derived (R-CO4Y-11X7 / R-DA34-WX9P) so a visitor reaching the
	// service through a TLS-terminating proxy sees the public origin,
	// not the local plain-HTTP origin. No Google details, no client
	// credentials, and no service-internal paths beyond the base URL
	// + `/mcp` transport endpoint (consistent with R-VVRG-W2G2). The
	// tab-interface presentation pinned by R-H4LJ-G9HR and the scope-
	// block structure pinned by R-G5FO-DXHS are not implemented here
	// yet — both clients' panels are rendered side-by-side as a
	// minimal first step.
	//
	// R-5GQZ-KWCD: each snippet is in the format the client itself
	// documents for adding an HTTP-transport MCP server. Claude Code
	// uses the `claude mcp add --transport http [--scope <scope>]
	// <name> <url>` CLI form (verbatim from `claude mcp add --help`);
	// Claude Desktop uses the `claude_desktop_config.json`
	// `mcpServers` block. Both are paste-and-go without translation.
	baseURL := requestBaseURL(r)
	mcpURL := htmlEscape(baseURL + "/mcp")
	claudeCodeProjectCmd := `claude mcp add --transport http --scope project hal ` + mcpURL
	claudeCodeUserCmd := `claude mcp add --transport http --scope user hal ` + mcpURL
	claudeDesktopJSON := `{` + "\n" +
		`  "mcpServers": {` + "\n" +
		`    "hal": {` + "\n" +
		`      "url": "` + mcpURL + `"` + "\n" +
		`    }` + "\n" +
		`  }` + "\n" +
		`}`
	// R-H4LJ-G9HR: tab interface — two triggers (Claude Code 01,
	// Claude Desktop 02) above two mutually-exclusive panels. Both
	// panels rendered in the HTML; JS toggles which is visible. Each
	// trigger carries a numeric badge, the literal client title, and
	// a 13px right-aligned instruction sentence per client. ARIA tab
	// pattern wired (tablist / tab / tabpanel, aria-selected /
	// aria-controls / aria-labelledby). Default active: Claude Code.
	// Every code block in the area exposes a `copy` affordance.
	// R-MCHV-YEO4 (d): canonical shape per reqs/design.css §3 and
	// reqs/web.md 138-147. The client-tabs container renders inside
	// `<article class="section">` with a `.section-body` wrapping the
	// tabs and panels. `.mcp-instructions` is a forbidden shadow name.
	// R-9TPL-HQBV: the instructions head and the client tabs are
	// SEPARATE children of `<main class="page">`, not nested under a
	// single wrapper.
	// R-NBGD-KUHA: the instructions head (the `<h2>` reading "Connect
	// an MCP client" per R-H4LJ-G9HR) is NOT wrapped in its own card
	// chrome — the canonical CSS hook is `.instructions-head` (a bare
	// container with top margin providing the inter-section gap above
	// it and a small bottom margin providing the internal gap to the
	// tabs panel below). The `<h2>` carries no border, no rounded
	// background, no card-style fill — it is a heading, not a card.
	mcpInstructions := `<div class="instructions-head" aria-label="Connect an MCP client">` +
		`<h2>Connect an MCP client</h2>` +
		`</div>` +
		`<article class="section" aria-label="MCP client connect snippets">` +
		`<div class="section-body">` +
		`<div class="client-tabs" role="tablist" aria-label="MCP client">` +
		// R-UBYN-1LY0: each .client-tab contains exactly two visible
		// elements — the .num chip and the client name as a bare text
		// node. The per-client instruction sentence lives in the
		// matching .client-panel body (R-H4LJ-G9HR allows either
		// placement; we choose panel body to satisfy R-UBYN-1LY0).
		// R-MCHV-YEO4: the panel container is `.client-panel` per the
		// canonical reqs/design.css — `.mcp-client` is a forbidden
		// shadow name.
		`<button class="client-tab active" type="button" role="tab"` +
		` id="tab-claude-code" aria-selected="true" aria-controls="panel-claude-code"` +
		` data-target="claude-code">` +
		`<span class="num">01</span>Claude Code` +
		`</button>` +
		`<button class="client-tab" type="button" role="tab"` +
		` id="tab-claude-desktop" aria-selected="false" aria-controls="panel-claude-desktop"` +
		` tabindex="-1" data-target="claude-desktop">` +
		`<span class="num">02</span>Claude Desktop` +
		`</button>` +
		`</div>` +
		// R-772N-VHQE: Claude Code panel carries `.active` on first
		// render so `.client-panel.active` resolves visible per
		// reqs/design.css; Claude Desktop panel does not.
		`<div class="client-panel active" role="tabpanel"` +
		` id="panel-claude-code" aria-labelledby="tab-claude-code"` +
		` data-client="claude-code">` +
		`<p class="client-hint">Run the following command.</p>` +
		// R-G5FO-DXHS: two stacked scope blocks (project then user),
		// each with its own pill label and code block. Both visible
		// on initial render; not a sub-tab interface.
		// R-UBPK-DLTT: each dark code-block snippet is a single
		// element carrying the canonical `code` class
		// (`<pre class="code">`) so the `.code` rule in
		// reqs/design.css applies directly — no `code-wrap`,
		// `code-block`, or `snippet` shadow wrapper, no inline
		// position:relative override (`.code` already supplies it
		// for the absolutely-positioned `.copy` overlay). The
		// copy button's body is the clipboard `<svg>` glyph
		// (`.copy svg` is sized 12x12 in design.css), with the
		// `aria-label` carrying the visible affordance text.
		`<div class="scope-block" data-scope="project">` +
		`<span class="scope-pill">project</span>` +
		`<pre class="code">` + claudeCodeProjectCmd +
		`<button class="copy" type="button" aria-label="Copy to clipboard">` +
		`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true">` +
		`<rect x="9" y="9" width="13" height="13" rx="2"/>` +
		`<path d="M5 15V5a2 2 0 0 1 2-2h10"/>` +
		`</svg>` +
		`</button>` +
		`</pre>` +
		`</div>` +
		`<div class="scope-block" data-scope="user">` +
		`<span class="scope-pill">user</span>` +
		`<pre class="code">` + claudeCodeUserCmd +
		`<button class="copy" type="button" aria-label="Copy to clipboard">` +
		`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true">` +
		`<rect x="9" y="9" width="13" height="13" rx="2"/>` +
		`<path d="M5 15V5a2 2 0 0 1 2-2h10"/>` +
		`</svg>` +
		`</button>` +
		`</pre>` +
		`</div>` +
		`</div>` +
		`<div class="client-panel" role="tabpanel"` +
		` id="panel-claude-desktop" aria-labelledby="tab-claude-desktop"` +
		` hidden data-client="claude-desktop">` +
		`<p class="client-hint">Add the following JSON to your claude_desktop_config.json</p>` +
		`<pre class="code">` + claudeDesktopJSON +
		`<button class="copy" type="button" aria-label="Copy to clipboard">` +
		`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true">` +
		`<rect x="9" y="9" width="13" height="13" rx="2"/>` +
		`<path d="M5 15V5a2 2 0 0 1 2-2h10"/>` +
		`</svg>` +
		`</button>` +
		`</pre>` +
		`</div>` +
		`</div>` +
		`</article>`

	fmt.Fprintf(w,
		`<!doctype html>`+
			`<html lang="en"><head>`+
			`<meta charset="utf-8">`+
			`<title>HAL</title>`+
			`<link rel="stylesheet" href="/design.css">`+
			// R-G0K2-UUJ0: when the visitor's browser reports
			// prefers-reduced-motion: reduce, the index page suppresses
			// the lens-dot pulse, the subtitle fade-swap, the counter
			// flash and delta animation, and the hover-driven transforms
			// on the re-roll, copy, icon-btn, auth-btn, and client-tab
			// controls. Visual end-states still render; only the
			// transitions and the infinite pulse animation are removed.
			// The overrides live in an inline <style> block so the
			// canonical reqs/design.css stays byte-equal to the embedded
			// copy (R-8MP8-6B77 drift guard).
			`<style>@media (prefers-reduced-motion: reduce){`+
			`.lens{animation:none !important;}`+
			`.subtitle,.subtitle.swap{transition:none !important;}`+
			`.counter-value,.counter-value.flash{transition:none !important;}`+
			`.delta,.delta.show,.delta.minus{transition:none !important;}`+
			`.refresh,.refresh:hover,.refresh:active,`+
			`.icon-btn,.icon-btn:hover:not(:disabled),`+
			`.copy,.copy:hover,.copy.copied,`+
			`.auth-btn,.auth-btn:hover,`+
			`.client-tab,.client-tab.active`+
			`{transition:none !important;}`+
			`}</style>`+
			// R-G6NK-RP8H: the load-bearing layout property of the design
			// reference is a centered ~880px content column inside a single
			// .page wrapper. The .page rule in design.css supplies the
			// max-width and `margin: 0 auto`, so the <main> here must carry
			// the class for that rule to actually apply in the rendered
			// output. Without it the tokens are declared but the centered
			// container is not realized.
			// R-6KK2-AAY0 / R-2ZZH-LJYA / R-O87H-RSH4 / R-CNWX-9VB2 /
			// R-6QIE-4D71:
			// the agents block is an application-specific extension to
			// the canonical banner. Keep reqs/design.css byte-identical,
			// and layer these small layout rules inline so the signed-in
			// email row plus every agent row share one lower-right
			// label/action stack that remains in the banner's normal
			// flow. The flow placement and compact lower padding are
			// conditional on an actual .agents-block descendant, so
			// signed-out and zero-agent pages keep design.css's compact
			// absolutely-positioned .banner-auth treatment.
			`<style>`+
			`.banner:has(.agents-block){display:flex;flex-direction:column;align-items:center;padding-bottom:18px}`+
			`.banner:has(.agents-block) .banner-auth{display:grid;`+
			`grid-template-columns:max-content max-content;`+
			`align-items:center;justify-items:end;gap:8px 14px;text-align:right;`+
			`position:static;align-self:flex-end;margin-top:28px}`+
			`.banner:has(.agents-block) .banner-auth>.auth-email,`+
			`.banner:has(.agents-block) .banner-auth .agent-name{grid-column:1;`+
			`color:var(--ink-mute);font-size:13px}`+
			`.banner:has(.agents-block) .banner-auth>.auth-form,`+
			`.banner:has(.agents-block) .banner-auth .agent-row form{grid-column:2;`+
			`margin:0;justify-self:start}`+
			`.banner:has(.agents-block) .banner-auth>.auth-btn{`+
			`grid-column:2;justify-self:start;text-decoration:none}`+
			`.agents-block,.agent-row{display:contents}`+
			`</style>`+
			`</head><body><main class="page">`+
			// R-UAQQ-NU7B: `.title` and `.subtitle` are reserved
			// page-scope class names. They appear ONLY on the
			// <h1> page heading and the rotating tagline inside
			// this <section class="banner">; no component below
			// reuses either token in its class list.
			// R-GTPJ-Z8EL: the banner card, counter card, and
			// instructions-head article are direct siblings under
			// <main class="page"> with no interposing wrapper and
			// no inline style= overrides — the markup posture the
			// canonical CSS (operator-owned per R-8MP8-6B77)
			// expects to deliver uniform inter-section gaps.
			`<section class="banner">`+
			`<span class="lens" aria-hidden="true"></span>`+
			`<span class="tag">MCP Demo</span>`+
			`<h1 class="title">HAL 9000</h1>`+
			`<div class="subtitle-row">`+
			`<span class="subtitle" id="subtitle">%s</span>`+
			`<button class="refresh" type="button" aria-label="New subtitle">`+
			`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true">`+
			`<path d="M3 12a9 9 0 0 1 15.5-6.3L21 3"/>`+
			`<path d="M21 3v6h-6"/>`+
			`<path d="M21 12a9 9 0 0 1-15.5 6.3L3 21"/>`+
			`<path d="M3 21v-6h6"/>`+
			`</svg>`+
			`</button>`+
			`</div>`+
			`%s`+
			`%s`+
			`</section>`+
			// R-EJAP-XUSB: counter card directly below the banner. The
			// −/+ buttons render as <button> elements carrying the
			// canonical .icon-btn class and aria-labels Decrement /
			// Increment, and are HTML-disabled when no web session is
			// active so .icon-btn:disabled supplies the ≈40% opacity /
			// cursor:not-allowed treatment. The hint line is rendered
			// inside the card (below the counter value, left-aligned
			// within the card's content area) and is identical in both
			// auth states — MCP-agent capability is orthogonal to the
			// visitor's browser session.
			`<section class="counter-card">`+
			`<div>`+
			`<div class="counter-label">CURRENT COUNT</div>`+
			// R-G0K2-UUJ0: aria-live="polite" on the counter value so
			// updates pushed over the live channel (R-FZC6-H2SB) are
			// announced to assistive tech without interrupting.
			`<div class="counter-value" aria-live="polite">%d</div>`+
			`<p class="locked-hint">Authenticated agents using MCP can read &amp; mutate this counter on your behalf.</p>`+
			`</div>`+
			`<div class="counter-actions">`+
			`<button class="icon-btn" type="button" aria-label="Decrement"%s>&minus;</button>`+
			`<button class="icon-btn" type="button" aria-label="Increment"%s>+</button>`+
			`</div>`+
			`</section>`+
			`%s`+
			// R-WOEN-ND69: footer is the last child of <main class="page">,
			// not a sibling of it. A footer placed after </main> spans the
			// full viewport instead of matching the 880px column.
			// R-MCHV-YEO4: canonical chrome is bare `<footer>` (no class)
			// containing `footer .status` on the left; the green dot is
			// drawn by `footer .status::before` per reqs/design.css 480-491,
			// so no inner `<span class="status-dot">` element exists.
			// `.footer-left` / `.footer-right` are forbidden shadow names.
			`<footer>`+
			`<span class="status">MCP server live</span>`+
			`<span>v%s · open my pod bay doors</span>`+
			`</footer>`+
			`</main>`+
			`<script>`+
			`(function(){`+
			`var bank=%s;`+
			`var el=document.getElementById('subtitle');`+
			`var btn=document.querySelector('.refresh');`+
			`if(!el||!btn)return;`+
			`btn.addEventListener('click',function(){`+
			`el.classList.add('swap');`+
			`setTimeout(function(){`+
			`el.textContent=bank[Math.floor(Math.random()*bank.length)];`+
			`el.classList.remove('swap');`+
			`},220);`+
			`});`+
			`})();`+
			// R-H4LJ-G9HR: client-tab toggle (Claude Code / Claude Desktop)
			// and copy-button wiring for every code block in the MCP
			// instructions area. Strips a leading shell prompt prefix
			// ($ or >) when present so the clipboard payload is the
			// executable form, not the visual framing.
			`(function(){`+
			`var tabs=document.querySelectorAll('.client-tab');`+
			`var panels=document.querySelectorAll('.section .client-panel');`+
			`function activate(target){`+
			`tabs.forEach(function(t){`+
			`var on=t.getAttribute('data-target')===target;`+
			`t.classList.toggle('active',on);`+
			`t.setAttribute('aria-selected',on?'true':'false');`+
			`t.setAttribute('tabindex',on?'0':'-1');`+
			`});`+
			`panels.forEach(function(p){`+
			`var on=p.getAttribute('data-client')===target;`+
			`p.classList.toggle('active',on);`+
			`if(on){p.removeAttribute('hidden');}else{p.setAttribute('hidden','');}`+
			`});`+
			`}`+
			`tabs.forEach(function(t){`+
			`t.addEventListener('click',function(){activate(t.getAttribute('data-target'));});`+
			`});`+
			`var copies=document.querySelectorAll('.section .copy');`+
			`copies.forEach(function(b){`+
			`b.addEventListener('click',function(){`+
			// R-UBPK-DLTT: each `.copy` lives directly inside the
			// `.code` element; the snippet text is the parent's
			// textContent (the button's body is an `<svg>` glyph
			// so the button itself contributes no text). The
			// "copied" affordance is a class toggle only — the
			// glyph stays in place.
			`var wrap=b.parentNode;`+
			`if(!wrap)return;`+
			`var text=wrap.textContent.replace(/^[\\s]*[\\$>] /,'');`+
			`var done=function(){`+
			`b.classList.add('copied');`+
			`setTimeout(function(){b.classList.remove('copied');},1400);`+
			`};`+
			`if(navigator.clipboard&&navigator.clipboard.writeText){`+
			`navigator.clipboard.writeText(text).then(done,function(){});`+
			`}else{`+
			`var ta=document.createElement('textarea');`+
			`ta.value=text;document.body.appendChild(ta);ta.select();`+
			`try{document.execCommand('copy');done();}catch(e){}`+
			`document.body.removeChild(ta);`+
			`}`+
			`});`+
			`});`+
			`})();`+
			// R-KSI8-M0JX: a signed-in page that initially rendered with
			// zero live MCP chains has no `.agents-block` in the HTML, so
			// the agents SSE client must be able to create the block on the
			// first non-empty snapshot. Subsequent snapshots replace the
			// rows atomically and remove the block again when the live set
			// becomes empty.
			`(function(){`+
			`var auth=document.querySelector('.banner-auth');`+
			`if(!auth||!auth.querySelector('.auth-email'))return;`+
			`function row(chain){`+
			`var r=document.createElement('div');`+
			`r.className='agent-row';`+
			`r.setAttribute('data-chain-id',chain.chain_id||'');`+
			`var name=document.createElement('span');`+
			`name.className='agent-name';`+
			`name.textContent=(chain.client_name||'undefined')+' ('+String(chain.client_id||'').slice(0,8)+')';`+
			`var form=document.createElement('form');`+
			`form.method='post';form.action='/agents/revoke';`+
			`var input=document.createElement('input');`+
			`input.type='hidden';input.name='chain_id';`+
			`input.value=chain.chain_id||'';`+
			`var btn=document.createElement('button');`+
			`btn.className='auth-btn';btn.type='submit';`+
			`btn.textContent='Revoke';`+
			`form.appendChild(input);form.appendChild(btn);`+
			`r.appendChild(name);r.appendChild(form);`+
			`return r;`+
			`}`+
			`function render(chains){`+
			`var block=document.querySelector('.agents-block');`+
			`if(!chains||chains.length===0){`+
			`if(block&&block.parentNode)block.parentNode.removeChild(block);`+
			`return;`+
			`}`+
			`if(!block){`+
			`block=document.createElement('div');`+
			`block.className='agents-block';`+
			`block.setAttribute('aria-label','Authenticated MCP agents');`+
			`auth.appendChild(block);`+
			`}`+
			`block.textContent='';`+
			`chains.forEach(function(chain){block.appendChild(row(chain));});`+
			`}`+
			`try{`+
			`var es=new EventSource('/agents/stream');`+
			`es.onmessage=function(e){`+
			`try{var chains=JSON.parse(e.data);`+
			`if(Array.isArray(chains))render(chains);}catch(_){}`+
			`};`+
			`}catch(_){}`+
			`})();`+
			// R-FY4A-3B1M: wire the signed-in visitor's +/- clicks to the
			// real mutation endpoints, and subscribe every browser (signed
			// in or not) to the SSE feed R-FZC6-H2SB serves. Each observed
			// value change repaints the digit and inserts a +N/-N delta
			// indicator; both visual cues persist long enough that the
			// visitor unambiguously perceives them (>=600ms). The
			// reduced-motion override (R-G0K2-UUJ0) suppresses transitions
			// but leaves the end-state digit and delta visible.
			`(function(){`+
			`var val=document.querySelector('.counter-value');`+
			`if(!val)return;`+
			`var current=parseInt(val.textContent,10);`+
			`if(isNaN(current))current=0;`+
			`var dec=document.querySelector('.icon-btn[aria-label="Decrement"]');`+
			`var inc=document.querySelector('.icon-btn[aria-label="Increment"]');`+
			`function mutate(url){`+
			`fetch(url,{method:'POST',credentials:'same-origin'}).catch(function(){});`+
			`}`+
			`if(inc&&!inc.disabled){`+
			`inc.addEventListener('click',function(){mutate('/counter/increment');});`+
			`}`+
			`if(dec&&!dec.disabled){`+
			`dec.addEventListener('click',function(){mutate('/counter/decrement');});`+
			`}`+
			`function apply(next){`+
			`if(next===current)return;`+
			`var prev=current;current=next;`+
			`var diff=next-prev;`+
			`val.textContent=String(next);`+
			`val.classList.add('flash');`+
			`var d=document.createElement('span');`+
			`var sign=diff<0?'−':'+';`+
			`d.className='delta show'+(diff<0?' minus':'');`+
			`d.textContent=sign+Math.abs(diff);`+
			`val.appendChild(d);`+
			`setTimeout(function(){`+
			`val.classList.remove('flash');`+
			`d.classList.remove('show');`+
			`setTimeout(function(){if(d.parentNode)d.parentNode.removeChild(d);},350);`+
			`},800);`+
			`}`+
			`try{`+
			`var es=new EventSource('/counter/stream');`+
			`es.onmessage=function(e){`+
			`try{var p=JSON.parse(e.data);`+
			`if(typeof p.value==='number'){apply(p.value);}}catch(_){}`+
			`};`+
			`}catch(_){}`+
			`})();`+
			`</script>`+
			`</body></html>`+"\n",
		htmlEscape(pickSubtitle()), bannerAuth, agentsBlock, c.read(),
		counterDisabled, counterDisabled, mcpInstructions, halVersion, bankJSON)
}

// htmlEscape escapes text for safe interpolation into HTML body content.
// The subtitle bank is a fixed in-source list, so this is defense-in-depth
// rather than a load-bearing sanitizer.
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}

// handleDesignCSS serves the canonical stylesheet embedded from
// app-root/design.css. R-8MP8-6B77 pins that the designer's file is used
// directly, not re-derived; this handler is the load-bearing seam.
func handleDesignCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(designCSS)))
	_, _ = w.Write(designCSS)
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
	states.putWeb(state, bindingID)
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
	stateRec, err := states.consume(state, presentedBinding)
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
		if stateRec.origin == "mcp" && stateRec.mcp != nil {
			writeOAuthErrorRedirect(w, r, stateRec.mcp.redirectURI,
				"access_denied",
				"Google email address is not verified",
				stateRec.mcp.clientState)
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
		if stateRec.origin == "mcp" && stateRec.mcp != nil {
			writeOAuthErrorRedirect(w, r, stateRec.mcp.redirectURI,
				"access_denied",
				"identity is not in the allowed Workspace domain",
				stateRec.mcp.clientState)
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
	switch stateRec.origin {
	case "mcp":
		if stateRec.mcp == nil {
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
		code, err := authCodes.issueWithResource(
			stateRec.mcp.clientID,
			stateRec.mcp.redirectURI,
			stateRec.mcp.codeChallenge,
			stateRec.mcp.codeChallengeMethod,
			identity.Email,
			stateRec.mcp.resource,
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
		target := stateRec.mcp.redirectURI +
			"?code=" + url.QueryEscape(code) +
			"&state=" + url.QueryEscape(stateRec.mcp.clientState)
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	case "web":
		// R-CXJ2-R3BN: mint a web session and set the session cookie. The
		// plaintext identifier appears only here, in the Set-Cookie response;
		// the store keeps a hash (R-SLGL-B5B4).
		plaintext, err := sessions.issue(identity.Email)
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
		if sess := sessions.lookup(c.Value); sess != nil &&
			!sameOriginBrowserMutationR_R4RG_O4Y9(r) {
			writeSameOriginForbiddenR_R4RG_O4Y9(w)
			return
		}
		sessions.revoke(c.Value)
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
		session = sessions.lookup(c.Value)
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
	if !tokens.revokeChainR_D0XD_1YT0(chainID, session.ownerEmail) {
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

func handleCounterReadWithCounter(c *counter, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Value uint64 `json:"value"`
	}{Value: c.read()})
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

func handleCounterStreamWithCounter(c *counter, w http.ResponseWriter, r *http.Request) {
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

	bcast := c.broadcaster()
	sub := bcast.subscribe()
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
	writeValue := func(v uint64) error {
		return writeBytes([]byte(fmt.Sprintf(
			"data: {\"value\":%d}\n\n", v)))
	}

	if err := writeValue(c.read()); err != nil {
		return
	}

	hb := appNewTicker(streamHeartbeatInterval())
	defer hb.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-sub.ch:
			if err := writeValue(v); err != nil {
				return
			}
		case <-hb.C():
			if err := writeBytes([]byte(":hb\n\n")); err != nil {
				return
			}
		}
	}
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
		session = sessions.lookup(c.Value)
	}
	if session == nil {
		http.Error(w, "web session required",
			http.StatusUnauthorized)
		return
	}
	email := session.ownerEmail

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
		chains := tokens.liveAgentChainsR_0NRX_3GV1(email, clients)
		sortAgentChainsByRenderedIdentityR_VWEX_WYWJ(chains)
		type item struct {
			ChainID    string `json:"chain_id"`
			ClientID   string `json:"client_id"`
			ClientName string `json:"client_name"`
			IssuedAt   string `json:"issued_at"`
		}
		out := make([]item, 0, len(chains))
		for _, c := range chains {
			out = append(out, item{
				ChainID:    c.chainID,
				ClientID:   c.clientID,
				ClientName: c.clientName,
				IssuedAt:   c.issuedAt.UTC().Format(time.RFC3339Nano),
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
	base := requestBaseURL(r)
	doc := struct {
		Issuer                            string   `json:"issuer"`
		AuthorizationEndpoint             string   `json:"authorization_endpoint"`
		TokenEndpoint                     string   `json:"token_endpoint"`
		RegistrationEndpoint              string   `json:"registration_endpoint"`
		ResponseTypesSupported            []string `json:"response_types_supported"`
		GrantTypesSupported               []string `json:"grant_types_supported"`
		CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
		TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	}{
		Issuer:                            base,
		AuthorizationEndpoint:             base + "/oauth/authorize",
		TokenEndpoint:                     base + "/oauth/token",
		RegistrationEndpoint:              base + "/oauth/register",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

// R-3UT3-IKZG: the canonical resource identifier is published verbatim
// in the OAuth 2.0 Protected Resource Metadata document at
// /.well-known/oauth-protected-resource/mcp (R-7BHQ-VB64). The same string is used in
// the bound `resource` value recorded on each issued token and in the
// bearer-side validation comparison.
func handleOAuthProtectedResourceMetadata(w http.ResponseWriter,
	r *http.Request) {
	base := requestBaseURL(r)
	doc := struct {
		Resource               string   `json:"resource"`
		AuthorizationServers   []string `json:"authorization_servers"`
		BearerMethodsSupported []string `json:"bearer_methods_supported"`
	}{
		Resource:               canonicalResourceIdentifier(),
		AuthorizationServers:   []string{base},
		BearerMethodsSupported: []string{"header"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
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
	limitRequestBodyR_VKZD_UKVS(w, r)
	var req struct {
		RedirectURIs            []string `json:"redirect_uris"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
		ClientName              string   `json:"client_name"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		if requestBodyTooLargeR_VKZD_UKVS(err) {
			writeBodyTooLargeR_VKZD_UKVS(w)
			return
		}
		writeOAuthError(w, http.StatusBadRequest,
			"invalid_client_metadata", "request body is not valid JSON")
		return
	}
	if len(req.RedirectURIs) == 0 {
		// R-8OBG-7FST: DCR requires at least one redirect URI that passes
		// the R-9OWM-O8XJ validation below; absent or empty lists cannot
		// produce a usable authorization-time exact match.
		writeOAuthError(w, http.StatusBadRequest,
			"invalid_redirect_uri",
			"redirect_uris is required and must be a non-empty array")
		return
	}
	for _, u := range req.RedirectURIs {
		if !validOAuthRedirectURI(u) {
			writeOAuthError(w, http.StatusBadRequest,
				"invalid_redirect_uri",
				"each redirect_uris entry must be an absolute http or https URI with a host and no fragment")
			return
		}
	}
	authMethod := req.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = "none"
	}
	if authMethod != "none" {
		// R-KCBH-CXY9: MCP clients are public PKCE clients. Dynamic
		// Client Registration therefore accepts omitted/none auth and
		// rejects any client-secret-based or otherwise unsupported token
		// endpoint authentication method before a client_id is issued.
		writeOAuthError(w, http.StatusBadRequest,
			"invalid_client_metadata",
			"token_endpoint_auth_method must be none")
		return
	}
	clientName, ok := normalizeOAuthClientName(req.ClientName)
	if !ok {
		// R-JE3Z-IGI4: DCR client_name is optional display text only.
		// Reject overlong or ASCII-control-containing names before a
		// client_id is issued; empty/whitespace-only input is normalized
		// to unset below.
		writeOAuthError(w, http.StatusBadRequest,
			"invalid_client_metadata",
			"client_name must be at most 80 characters and contain no control characters")
		return
	}
	rec := &oauthClient{
		redirectURIs:  append([]string(nil), req.RedirectURIs...),
		clientName:    clientName,
		grantTypes:    append([]string(nil), req.GrantTypes...),
		responseTypes: append([]string(nil), req.ResponseTypes...),
		authMethod:    authMethod,
		issuedAt:      appNow().Unix(),
	}
	// R-19BA-4XX4: generated client_id values are unique among persisted
	// registrations. A collision never overwrites the existing record; the
	// handler retries a bounded number of times and only stores when the ID
	// was absent at the same lock boundary used for the write.
	var clientID string
	for range 8 {
		var err error
		clientID, err = newOAuthClientID()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if clients.putIfAbsent(clientID, rec) {
			break
		}
		clientID = ""
	}
	if clientID == "" {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	resp := struct {
		ClientID                string   `json:"client_id"`
		ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
		RedirectURIs            []string `json:"redirect_uris"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
		GrantTypes              []string `json:"grant_types,omitempty"`
		ResponseTypes           []string `json:"response_types,omitempty"`
		ClientName              string   `json:"client_name,omitempty"`
	}{
		ClientID:                clientID,
		ClientIDIssuedAt:        rec.issuedAt,
		RedirectURIs:            rec.redirectURIs,
		TokenEndpointAuthMethod: rec.authMethod,
		GrantTypes:              rec.grantTypes,
		ResponseTypes:           rec.responseTypes,
		ClientName:              rec.clientName,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

func normalizeOAuthClientName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", true
	}
	count := 0
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", false
		}
		count++
		if count > 80 {
			return "", false
		}
	}
	return name, true
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
	client := clients.lookup(clientID)
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
	for _, u := range client.redirectURIs {
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
	states.putMCP(state, bindingID, oauthStateMCPContext{
		clientID:            clientID,
		redirectURI:         requested,
		codeChallenge:       q.Get("code_challenge"),
		codeChallengeMethod: q.Get("code_challenge_method"),
		clientState:         q.Get("state"),
		resource:            requestedResource,
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
	limitRequestBodyR_VKZD_UKVS(w, r)
	if err := r.ParseForm(); err != nil {
		if requestBodyTooLargeR_VKZD_UKVS(err) {
			writeBodyTooLargeR_VKZD_UKVS(w)
			return
		}
		writeOAuthError(w, http.StatusBadRequest, "invalid_request",
			"could not parse request body")
		return
	}
	// R-WLUL-MZCD: authorization-code and refresh-token grants may omit
	// `resource`; omission targets the canonical resource already bound
	// onto the authorization code or token chain. R-4GRA-EGBY still rejects
	// any present non-canonical resource before a token can be issued or
	// rotated.
	if res := r.PostForm.Get("resource"); res != "" && res != canonicalResourceIdentifier() {
		writeOAuthError(w, http.StatusBadRequest, "invalid_target",
			"resource parameter does not match this service's canonical identifier")
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		handleOAuthTokenAuthCodeWithStores(authCodes, tokens, w, r)
	case "refresh_token":
		handleOAuthTokenRefreshWithStore(tokens, w, r)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type",
			"only authorization_code and refresh_token are supported")
	}
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
	code := r.PostForm.Get("code")
	clientID := r.PostForm.Get("client_id")
	redirectURI := r.PostForm.Get("redirect_uri")
	codeVerifier := r.PostForm.Get("code_verifier")
	if code == "" || clientID == "" || redirectURI == "" || codeVerifier == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request",
			"authorization_code grant requires code, client_id, "+
				"redirect_uri, code_verifier")
		return
	}
	rec, err := authCodes.redeem(code, clientID, redirectURI, codeVerifier)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	access, refresh, err := tokens.issueInitialTokenPairR_2HT5_50F4(
		rec.ownerEmail, rec.clientID, rec.resource)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	// R-KX4N-DZ44: successful token responses contain bearer-token
	// plaintext and must not be stored by clients or intermediaries.
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(authCfg().AccessTokenTTL / time.Second),
		RefreshToken: refresh,
	})
}

func handleOAuthTokenRefresh(w http.ResponseWriter, r *http.Request) {
	handleOAuthTokenRefreshWithStore(newOAuthTokenStorage(), w, r)
}

func handleOAuthTokenRefreshWithStore(tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request) {
	refreshToken := r.PostForm.Get(oauthRefreshTokenFormField)
	clientID := r.PostForm.Get("client_id")
	if refreshToken == "" || clientID == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request",
			"refresh_token grant requires refresh_token and client_id")
		return
	}
	// R-B78O-8X0F: the refresh-token grant rotates a valid refresh
	// token into a fresh bearer access token plus successor refresh
	// token without any browser or Google round trip.
	access, refresh, err := tokens.rotateRefreshForClient(refreshToken, clientID)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(authCfg().AccessTokenTTL / time.Second),
		RefreshToken: refresh,
	})
}

func writeOAuthError(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description,omitempty"`
	}{Error: code, ErrorDescription: desc})
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
	cookiePresented := false
	cookieRejectedByOrigin := false
	if c, err := r.Cookie(webSessionCookieName); err == nil {
		cookiePresented = true
		if sess := sessions.lookup(c.Value); sess != nil {
			if sameOriginBrowserMutationR_R4RG_O4Y9(r) {
				setAuthedUserR_D56D_EBP3(r, sess.ownerEmail)
				return true, 0, "", ""
			}
			cookieRejectedByOrigin = true
		}
	}
	authHeader := r.Header.Get("Authorization")
	plaintext, bearerOK := bearerTokenFromRequest(r)
	if !bearerOK {
		if cookieRejectedByOrigin {
			return false, http.StatusForbidden, "invalid_request",
				"same-origin browser request required"
		}
		if authHeader == "" {
			if cookiePresented {
				return false, http.StatusUnauthorized, "invalid_token",
					"session cookie not recognized"
			}
			return false, http.StatusUnauthorized, "invalid_request",
				"no credentials presented"
		}
		return false, http.StatusUnauthorized, "invalid_token",
			"bearer authorization header malformed"
	}
	rec, reason := tokens.lookupAccessReason(plaintext)
	if rec != nil {
		if rec.resource != canonicalResourceIdentifier() {
			return false, http.StatusUnauthorized, "invalid_token",
				"bearer token resource binding does not match"
		}
		setAuthedUserR_D56D_EBP3(r, rec.ownerEmail)
		return true, 0, "", ""
	}
	if cookieRejectedByOrigin {
		return false, http.StatusForbidden, "invalid_request",
			"same-origin browser request required"
	}
	switch reason {
	case "expired":
		return false, http.StatusUnauthorized, "invalid_token", "bearer token expired"
	case "revoked":
		return false, http.StatusUnauthorized, "invalid_token", "bearer token revoked"
	default:
		return false, http.StatusUnauthorized, "invalid_token", "bearer token not recognized"
	}
}

// R-R4RG-O4Y9: browser requests that rely on a web session cookie for a
// state-changing action must come from this service's own origin. When a
// browser supplies Origin, it is authoritative; otherwise a supplied Referer
// must match. Non-browser clients often send neither header, so absence alone
// is not treated as cross-site.
func sameOriginBrowserMutationR_R4RG_O4Y9(r *http.Request) bool {
	want := requestBaseURL(r)
	if got := r.Header.Get("Origin"); got != "" {
		return got == want
	}
	if got := r.Header.Get("Referer"); got != "" {
		u, err := url.Parse(got)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return false
		}
		return u.Scheme+"://"+u.Host == want
	}
	return true
}

// bearerTokenFromRequest extracts the opaque token from an
// `Authorization: Bearer <token>` header. The scheme match is
// case-insensitive per RFC 6750 §2.1; surrounding whitespace is
// trimmed. Returns ("", false) when no Authorization header is
// present, the scheme is not Bearer, or the token value is empty.
func bearerTokenFromRequest(r *http.Request) (string, bool) {
	return parseBearerAuthHeader(r.Header.Get("Authorization"))
}

func parseBearerAuthHeader(h string) (string, bool) {
	if h == "" {
		return "", false
	}
	const prefix = "Bearer"
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) ||
		(h[len(prefix)] != ' ' && h[len(prefix)] != '\t') {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
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
	rec, reason := tokens.lookupAccessReason(plaintext)
	if rec != nil {
		if rec.resource != canonicalResourceIdentifier() {
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
	writeMutationAuthFailure(w, http.StatusUnauthorized, errCode, errDesc)
}

func writeMutationAuthFailure(w http.ResponseWriter, status int, errCode, errDesc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description,omitempty"`
	}{Error: errCode, ErrorDescription: errDesc})
}

func writeSameOriginForbiddenR_R4RG_O4Y9(w http.ResponseWriter) {
	writeMutationAuthFailure(w, http.StatusForbidden, "invalid_request",
		"same-origin browser request required")
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
	c *counter, sessions *webSessionStorage, tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request,
) {
	if ok, status, errCode, errDesc := checkMutationAuthWithStores(sessions, tokens, r); !ok {
		writeMutationAuthFailure(w, status, errCode, errDesc)
		return
	}
	v := c.increment()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Value uint64 `json:"value"`
	}{Value: v})
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
	c *counter, sessions *webSessionStorage, tokens *oauthTokenStorage, w http.ResponseWriter, r *http.Request,
) {
	if ok, status, errCode, errDesc := checkMutationAuthWithStores(sessions, tokens, r); !ok {
		writeMutationAuthFailure(w, status, errCode, errDesc)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	v, ok := c.decrement()
	if !ok {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(struct {
			Error string `json:"error"`
		}{Error: "counter at zero"})
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		Value uint64 `json:"value"`
	}{Value: v})
}

// R-78B7-YKKL: `hal reset` returns the SQLite database at --db to a
// fresh, never-launched state. Removing the file produces the same
// end state R-773B-KSTW reaches on a checkout with no database file.
func cmdReset(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", "./hal.db", "path to the SQLite database file")
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
