package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// AuthCodeOptions supplies host policy for authorization-code expiry.
type AuthCodeOptions struct {
	Now func() time.Time
	TTL func() time.Duration
}

// AuthCode is a single-use, short-lived OAuth authorization code record.
type AuthCode struct {
	clientID            string
	redirectURI         string
	codeChallenge       string
	codeChallengeMethod string
	ownerEmail          string
	resource            string
	expiresAt           time.Time
	consumed            bool
}

func (c *AuthCode) ClientID() string {
	if c == nil {
		return ""
	}
	return c.clientID
}

func (c *AuthCode) RedirectURI() string {
	if c == nil {
		return ""
	}
	return c.redirectURI
}

func (c *AuthCode) CodeChallenge() string {
	if c == nil {
		return ""
	}
	return c.codeChallenge
}

func (c *AuthCode) CodeChallengeMethod() string {
	if c == nil {
		return ""
	}
	return c.codeChallengeMethod
}

func (c *AuthCode) OwnerEmail() string {
	if c == nil {
		return ""
	}
	return c.ownerEmail
}

func (c *AuthCode) Resource() string {
	if c == nil {
		return ""
	}
	return c.resource
}

func (c *AuthCode) ExpiresAt() time.Time {
	if c == nil {
		return time.Time{}
	}
	return c.expiresAt
}

func (c *AuthCode) Consumed() bool {
	if c == nil {
		return false
	}
	return c.consumed
}

// AuthCodeStore keeps single-use authorization code records in memory.
type AuthCodeStore struct {
	mu  sync.Mutex
	m   map[string]*AuthCode
	now func() time.Time
	ttl func() time.Duration
}

// NewAuthCodeStore returns an in-memory OAuth authorization-code store.
func NewAuthCodeStore(opts AuthCodeOptions) *AuthCodeStore {
	opts = fillAuthCodeOptions(opts)
	return &AuthCodeStore{
		m:   map[string]*AuthCode{},
		now: opts.Now,
		ttl: opts.TTL,
	}
}

func fillAuthCodeOptions(opts AuthCodeOptions) AuthCodeOptions {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.TTL == nil {
		opts.TTL = func() time.Duration { return time.Minute }
	}
	return opts
}

func (s *AuthCodeStore) ensureReady() {
	opts := fillAuthCodeOptions(AuthCodeOptions{Now: s.now, TTL: s.ttl})
	s.now = opts.Now
	s.ttl = opts.TTL
	if s.m == nil {
		s.m = map[string]*AuthCode{}
	}
}

var (
	ErrAuthCodeUnknown          = errors.New("authorization code not recognized")
	ErrAuthCodeExpired          = errors.New("authorization code expired")
	ErrAuthCodeConsumed         = errors.New("authorization code already redeemed")
	ErrAuthCodeClientMismatch   = errors.New("client_id does not match the bound value")
	ErrAuthCodeRedirectMismatch = errors.New("redirect_uri does not match the bound value")
	ErrAuthCodePKCEMismatch     = errors.New("code_verifier does not satisfy the bound code_challenge")
	ErrAuthCodePKCEMethod       = errors.New("unsupported code_challenge_method")
)

// Issue records a freshly generated authorization code with its client,
// redirect, PKCE, owner, and empty resource bindings.
func (s *AuthCodeStore) Issue(
	clientID, redirectURI, codeChallenge, codeChallengeMethod, ownerEmail string,
) (string, error) {
	return s.IssueWithResource(
		clientID, redirectURI, codeChallenge, codeChallengeMethod,
		ownerEmail, "")
}

// IssueWithResource records a freshly generated authorization code with its
// client, redirect, PKCE, owner, and resource bindings.
func (s *AuthCodeStore) IssueWithResource(
	clientID, redirectURI, codeChallenge, codeChallengeMethod, ownerEmail, resource string,
) (string, error) {
	if codeChallengeMethod != "S256" {
		return "", ErrAuthCodePKCEMethod
	}
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	code := hex.EncodeToString(buf[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	s.m[code] = &AuthCode{
		clientID:            clientID,
		redirectURI:         redirectURI,
		codeChallenge:       codeChallenge,
		codeChallengeMethod: codeChallengeMethod,
		ownerEmail:          ownerEmail,
		resource:            resource,
		expiresAt:           s.now().Add(s.ttl()),
	}
	return code, nil
}

// Redeem validates the presented bindings and atomically marks the code
// consumed on success.
func (s *AuthCodeStore) Redeem(
	code, clientID, redirectURI, codeVerifier string,
) (*AuthCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	rec, ok := s.m[code]
	if !ok {
		return nil, ErrAuthCodeUnknown
	}
	if rec.consumed {
		return rec, ErrAuthCodeConsumed
	}
	if !s.now().Before(rec.expiresAt) {
		return nil, ErrAuthCodeExpired
	}
	if rec.clientID != clientID {
		return nil, ErrAuthCodeClientMismatch
	}
	if rec.redirectURI != redirectURI {
		return nil, ErrAuthCodeRedirectMismatch
	}
	if !PKCEVerifierMatches(rec.codeChallengeMethod, rec.codeChallenge, codeVerifier) {
		return nil, ErrAuthCodePKCEMismatch
	}
	rec.consumed = true
	return rec, nil
}

// Snapshot returns a copy of a record for tests and diagnostic assertions.
func (s *AuthCodeStore) Snapshot(code string) (*AuthCode, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	rec, ok := s.m[code]
	if !ok {
		return nil, false
	}
	cp := *rec
	return &cp, true
}

// Count returns the number of authorization code records held by the store.
func (s *AuthCodeStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	return len(s.m)
}

// ResetForTest clears authorization code records from the store.
func (s *AuthCodeStore) ResetForTest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = map[string]*AuthCode{}
}

// PKCEVerifierMatches reports whether the presented verifier satisfies the
// bound challenge under the only supported method: S256.
func PKCEVerifierMatches(method, challenge, verifier string) bool {
	switch method {
	case "S256":
		sum := sha256.Sum256([]byte(verifier))
		return challenge == base64.RawURLEncoding.EncodeToString(sum[:])
	default:
		return false
	}
}
