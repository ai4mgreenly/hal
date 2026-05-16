// Package websession owns browser session issuance, validation, revocation,
// and persistence.
package websession

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"sync"
	"time"
)

// CookieName carries the plaintext browser session identifier.
const CookieName = "hal_session"

// Options supplies the host application's clock and lifetime policy.
type Options struct {
	Now         func() time.Time
	AbsoluteTTL func() time.Duration
	IdleTTL     func() time.Duration
}

// Session is a live browser session record.
type Session struct {
	ownerEmail string
	issuedAt   time.Time
	expiresAt  time.Time
	// lastSeenAt advances on each successful lookup and feeds the idle
	// ceiling per R-KJ15-9P17.
	lastSeenAt time.Time
	revokedAt  time.Time
}

// OwnerEmail returns the Google-asserted owner email for the session.
func (s *Session) OwnerEmail() string {
	if s == nil {
		return ""
	}
	return s.ownerEmail
}

// IssuedAt returns when the session was issued.
func (s *Session) IssuedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.issuedAt
}

// ExpiresAt returns the absolute expiry timestamp.
func (s *Session) ExpiresAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.expiresAt
}

// RevokedAt returns the revocation timestamp, or zero when not revoked.
func (s *Session) RevokedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.revokedAt
}

// Store keeps browser session records, optionally backed by SQLite.
type Store struct {
	mu sync.Mutex
	m  map[string]*Session
	db *sql.DB

	now         func() time.Time
	absoluteTTL func() time.Duration
	idleTTL     func() time.Duration
}

// New returns an in-memory session store.
func New(opts Options) *Store {
	opts = fillOptions(opts)
	return &Store{
		m:           map[string]*Session{},
		now:         opts.Now,
		absoluteTTL: opts.AbsoluteTTL,
		idleTTL:     opts.IdleTTL,
	}
}

func fillOptions(opts Options) Options {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.AbsoluteTTL == nil {
		opts.AbsoluteTTL = func() time.Duration { return 12 * time.Hour }
	}
	if opts.IdleTTL == nil {
		opts.IdleTTL = func() time.Duration { return time.Hour }
	}
	return opts
}

func (s *Store) ensureReady() {
	opts := fillOptions(Options{
		Now:         s.now,
		AbsoluteTTL: s.absoluteTTL,
		IdleTTL:     s.idleTTL,
	})
	s.now = opts.Now
	s.absoluteTTL = opts.AbsoluteTTL
	s.idleTTL = opts.IdleTTL
	if s.m == nil {
		s.m = map[string]*Session{}
	}
}

// Hash returns the SHA-256 hex lookup key for a plaintext session id.
func Hash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// Issue records a session for ownerEmail and returns the plaintext identifier
// the caller writes to the user-agent cookie store. The plaintext is not
// retained by the service (R-SLGL-B5B4).
func (s *Store) Issue(ownerEmail string) (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	plaintext := hex.EncodeToString(buf[:])
	hash := Hash(plaintext)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	now := s.now()
	rec := &Session{
		ownerEmail: ownerEmail,
		issuedAt:   now,
		expiresAt:  now.Add(s.absoluteTTL()),
		lastSeenAt: now,
	}
	if s.db != nil {
		if err := s.upsert(hash, rec); err != nil {
			return "", err
		}
	}
	s.m[hash] = rec
	return plaintext, nil
}

// Lookup returns the session record for plaintext if it exists and is
// currently active: not revoked, not past absolute expiry, and not idle-expired.
func (s *Store) Lookup(plaintext string) *Session {
	if plaintext == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	rec, ok := s.m[Hash(plaintext)]
	if !ok {
		return nil
	}
	if !rec.revokedAt.IsZero() {
		return nil
	}
	now := s.now()
	if !now.Before(rec.expiresAt) {
		return nil
	}
	if !now.Before(rec.lastSeenAt.Add(s.idleTTL())) {
		return nil
	}
	rec.lastSeenAt = now
	if s.db != nil {
		_, _ = s.db.Exec(
			`UPDATE web_sessions SET last_seen_at = ? WHERE session_hash = ?`,
			rec.lastSeenAt.UnixNano(), Hash(plaintext))
	}
	return rec
}

// Revoke marks the session matching plaintext as revoked. Missing and already
// revoked entries are no-ops.
func (s *Store) Revoke(plaintext string) {
	if plaintext == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	rec, ok := s.m[Hash(plaintext)]
	if !ok {
		return
	}
	if rec.revokedAt.IsZero() {
		rec.revokedAt = s.now()
		if s.db != nil {
			_, _ = s.db.Exec(
				`UPDATE web_sessions SET revoked_at = ? WHERE session_hash = ?`,
				rec.revokedAt.UnixNano(), Hash(plaintext))
		}
	}
}

// Attach loads persisted session records and binds db as the write-through
// backing store.
func (s *Store) Attach(db *sql.DB) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	rows, err := db.Query(
		`SELECT session_hash, owner_email, issued_at, expires_at, ` +
			`last_seen_at, revoked_at FROM web_sessions`)
	if err != nil {
		return err
	}
	defer rows.Close()

	loaded := map[string]*Session{}
	for rows.Next() {
		var hash, owner string
		var issued, expires, lastSeen int64
		var revoked sql.NullInt64
		if err := rows.Scan(&hash, &owner, &issued, &expires,
			&lastSeen, &revoked); err != nil {
			return err
		}
		rec := &Session{
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

// DetachDBIf clears the backing database if it is currently db.
func (s *Store) DetachDBIf(db *sql.DB) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	if s.db == db {
		s.db = nil
	}
}

// Count returns the number of records currently held by the store.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	return len(s.m)
}

// ResetForTest clears all records. It exists so package-boundary tests can
// preserve their existing assertions without reaching into Store internals.
func (s *Store) ResetForTest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	s.m = map[string]*Session{}
}

// RecordForPlaintextForTest returns the raw record for a plaintext id.
func (s *Store) RecordForPlaintextForTest(plaintext string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	return s.m[Hash(plaintext)]
}

// HasPlaintextKeyForTest reports whether the raw map is keyed by plaintext.
func (s *Store) HasPlaintextKeyForTest(plaintext string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	_, ok := s.m[plaintext]
	return ok
}

// HasHashKeyForTest reports whether the raw map is keyed by Hash(plaintext).
func (s *Store) HasHashKeyForTest(plaintext string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	_, ok := s.m[Hash(plaintext)]
	return ok
}

// PlaintextLeakedForTest reports whether any record stores plaintext in a
// string field.
func (s *Store) PlaintextLeakedForTest(plaintext string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	for _, rec := range s.m {
		if rec.ownerEmail == plaintext {
			return true
		}
	}
	return false
}

func (s *Store) upsert(hash string, rec *Session) error {
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
