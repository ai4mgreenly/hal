// Package oauth owns OAuth authorization-server state and token capabilities.
package oauth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// StateCookieName is the browser cookie that binds an in-flight OAuth state to
// the user-agent that initiated the redirect.
const StateCookieName = "hal_oauth_state"

// StateOptions supplies host policy for state expiry.
type StateOptions struct {
	Now func() time.Time
	TTL func() time.Duration
}

// StateRecord is an in-flight OAuth state record.
type StateRecord struct {
	bindingID string
	expiresAt time.Time
	consumed  bool
	origin    string
	mcp       *StateMCPContext
}

// StateMCPContext carries the byte-for-byte authorize-request values recorded
// for an MCP-origin OAuth state.
type StateMCPContext struct {
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	ClientState         string
	Resource            string
}

// BindingID returns the browser-binding identifier recorded with the state.
func (r *StateRecord) BindingID() string {
	if r == nil {
		return ""
	}
	return r.bindingID
}

// ExpiresAt returns the instant at which this state ceases to be acceptable.
func (r *StateRecord) ExpiresAt() time.Time {
	if r == nil {
		return time.Time{}
	}
	return r.expiresAt
}

// Consumed reports whether this state has already been accepted once.
func (r *StateRecord) Consumed() bool {
	if r == nil {
		return false
	}
	return r.consumed
}

// Origin returns the producer discriminator: "web" or "mcp".
func (r *StateRecord) Origin() string {
	if r == nil {
		return ""
	}
	return r.origin
}

// MCPContext returns the recorded MCP authorize context, if this is an
// MCP-origin state.
func (r *StateRecord) MCPContext() *StateMCPContext {
	if r == nil || r.mcp == nil {
		return nil
	}
	ctx := *r.mcp
	return &ctx
}

// StateStore keeps single-use OAuth state records in memory.
type StateStore struct {
	mu  sync.Mutex
	m   map[string]*StateRecord
	now func() time.Time
	ttl func() time.Duration
}

// NewStateStore returns an in-memory OAuth state store.
func NewStateStore(opts StateOptions) *StateStore {
	opts = fillStateOptions(opts)
	return &StateStore{
		m:   map[string]*StateRecord{},
		now: opts.Now,
		ttl: opts.TTL,
	}
}

func fillStateOptions(opts StateOptions) StateOptions {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.TTL == nil {
		opts.TTL = func() time.Duration { return 5 * time.Minute }
	}
	return opts
}

func (s *StateStore) ensureReady() {
	opts := fillStateOptions(StateOptions{Now: s.now, TTL: s.ttl})
	s.now = opts.Now
	s.ttl = opts.TTL
	if s.m == nil {
		s.m = map[string]*StateRecord{}
	}
}

// PutWeb records a web-origin state value with its browser-binding ID.
func (s *StateStore) PutWeb(state, bindingID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	s.m[state] = &StateRecord{
		bindingID: bindingID,
		expiresAt: s.now().Add(s.ttl()),
		origin:    "web",
	}
}

// PutMCP records an MCP-origin state value with its browser-binding ID and
// authorize-request context.
func (s *StateStore) PutMCP(state, bindingID string, ctx StateMCPContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	ctxCopy := ctx
	s.m[state] = &StateRecord{
		bindingID: bindingID,
		expiresAt: s.now().Add(s.ttl()),
		origin:    "mcp",
		mcp:       &ctxCopy,
	}
}

var (
	ErrStateMissing      = errors.New("state parameter missing")
	ErrStateUnknown      = errors.New("state value not recognized")
	ErrStateExpired      = errors.New("state value expired")
	ErrStateConsumed     = errors.New("state value already used")
	ErrBindingMissing    = errors.New("session-binding cookie missing")
	ErrBindingMismatched = errors.New("session-binding cookie does not match")
)

// Consume verifies the state and browser binding, then marks the record
// consumed on success.
func (s *StateStore) Consume(state, bindingID string) (*StateRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	rec, ok := s.m[state]
	if !ok {
		return nil, ErrStateUnknown
	}
	if rec.consumed {
		return nil, ErrStateConsumed
	}
	if !s.now().Before(rec.expiresAt) {
		return nil, ErrStateExpired
	}
	if bindingID == "" {
		return nil, ErrBindingMissing
	}
	if rec.bindingID != bindingID {
		return nil, ErrBindingMismatched
	}
	rec.consumed = true
	return rec, nil
}

// Snapshot returns a copy of the record for tests and diagnostic assertions.
func (s *StateStore) Snapshot(state string) (*StateRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	rec, ok := s.m[state]
	if !ok {
		return nil, false
	}
	cp := *rec
	if rec.mcp != nil {
		ctx := *rec.mcp
		cp.mcp = &ctx
	}
	return &cp, true
}

// Count returns the number of state records currently held by the store.
func (s *StateStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	return len(s.m)
}

// NewStateValue returns a 32-character cryptographically random hex value.
func NewStateValue() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
