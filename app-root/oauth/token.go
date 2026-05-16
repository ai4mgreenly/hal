package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

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
type Token struct {
	Kind       string // "access" or "refresh"
	OwnerEmail string
	ClientID   string
	// Resource is the canonical Resource identifier (R-3UT3-IKZG) the
	// token is bound to at issue time. Bearer-side validation will
	// compare this byte-for-byte (R-DH2I-28CK) when the protected
	// endpoints land.
	Resource  string
	IssuedAt  time.Time
	ExpiresAt time.Time
	// UsedAt is the consumption stamp for refresh tokens (R-89K0-GH5G's
	// single-use property). Zero on access records; zero on a live
	// refresh; non-zero the moment rotateRefresh consumes it.
	UsedAt    time.Time
	RevokedAt time.Time
	// ChainID groups every (access, refresh) record that descends from a
	// single act of fresh authentication (R-9HGE-87UG). issueRefresh mints
	// a new ChainID; rotateRefresh propagates it onto both successor
	// records so a chain is identifiable across arbitrarily many
	// rotations. Access tokens minted by issueAccess directly (no
	// preceding refresh) carry the zero value — they have no chain
	// affiliation. On reuse detection the rotation primitive walks the
	// store and stamps RevokedAt on every record sharing the replayed
	// refresh's ChainID, killing the live successor refresh and any
	// outstanding access tokens issued under the same chain.
	ChainID string
}

type TokenStore struct {
	Mu         sync.Mutex
	M          map[string]*Token
	DB         *sql.DB
	notify     func(string)
	now        func() time.Time
	accessTTL  func() time.Duration
	refreshTTL func() time.Duration
}

type TokenOptions struct {
	Now        func() time.Time
	AccessTTL  func() time.Duration
	RefreshTTL func() time.Duration
}

func NewTokenStore(opts TokenOptions) *TokenStore {
	opts = fillTokenOptions(opts)
	return &TokenStore{M: map[string]*Token{}, now: opts.Now, accessTTL: opts.AccessTTL, refreshTTL: opts.RefreshTTL}
}

func fillTokenOptions(opts TokenOptions) TokenOptions {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.AccessTTL == nil {
		opts.AccessTTL = func() time.Duration { return time.Hour }
	}
	if opts.RefreshTTL == nil {
		opts.RefreshTTL = func() time.Duration { return 30 * 24 * time.Hour }
	}
	return opts
}

func (s *TokenStore) ensureReady() {
	opts := fillTokenOptions(TokenOptions{Now: s.now, AccessTTL: s.accessTTL, RefreshTTL: s.refreshTTL})
	s.now = opts.Now
	s.accessTTL = opts.AccessTTL
	s.refreshTTL = opts.RefreshTTL
	if s.M == nil {
		s.M = map[string]*Token{}
	}
}

func TokenHash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func tokenUnixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

func tokenTimeFromUnixNano(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(0, v)
}

func (s *TokenStore) SetNotifier(next func(string)) func(string) {
	s.Mu.Lock()
	prev := s.notify
	s.notify = next
	s.Mu.Unlock()
	return prev
}

func (s *TokenStore) notifyAgents(email string) {
	s.Mu.Lock()
	b := s.notify
	s.Mu.Unlock()
	if b != nil {
		b(email)
	}
}

// attach loads HAL-issued OAuth token records from SQLite and makes all
// subsequent token lifecycle writes durable. R-FC5T-WWC2: token records
// survive process restarts because the hash-keyed record map is rebuilt
// from the database before the service accepts requests.
func (s *TokenStore) Attach(DB *sql.DB) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.ensureReady()
	rows, err := DB.Query(
		`SELECT token_hash, kind, owner_email, client_id, resource, ` +
			`issued_at, expires_at, used_at, revoked_at, chain_id FROM oauth_tokens`)
	if err != nil {
		return err
	}
	defer rows.Close()
	loaded := map[string]*Token{}
	for rows.Next() {
		var hash string
		var IssuedAt, ExpiresAt, UsedAt, RevokedAt int64
		rec := &Token{}
		if err := rows.Scan(&hash, &rec.Kind, &rec.OwnerEmail, &rec.ClientID,
			&rec.Resource, &IssuedAt, &ExpiresAt, &UsedAt, &RevokedAt,
			&rec.ChainID); err != nil {
			return err
		}
		rec.IssuedAt = tokenTimeFromUnixNano(IssuedAt)
		rec.ExpiresAt = tokenTimeFromUnixNano(ExpiresAt)
		rec.UsedAt = tokenTimeFromUnixNano(UsedAt)
		rec.RevokedAt = tokenTimeFromUnixNano(RevokedAt)
		loaded[hash] = rec
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.M = loaded
	s.DB = DB
	return nil
}

func (s *TokenStore) persistTokenLocked(hash string, rec *Token) error {
	if s.DB == nil {
		return nil
	}
	return persistTokenRecordLocked(s.DB, hash, rec)
}

type tokenPersister interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func persistTokenRecordLocked(p tokenPersister, hash string, rec *Token) error {
	_, err := p.Exec(
		`INSERT OR REPLACE INTO oauth_tokens (`+
			`token_hash, kind, owner_email, client_id, resource, `+
			`issued_at, expires_at, used_at, revoked_at, chain_id`+
			`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		hash, rec.Kind, rec.OwnerEmail, rec.ClientID, rec.Resource,
		tokenUnixNano(rec.IssuedAt), tokenUnixNano(rec.ExpiresAt),
		tokenUnixNano(rec.UsedAt), tokenUnixNano(rec.RevokedAt),
		rec.ChainID)
	return err
}

// issueAccess mints an opaque access token for the given owner, client,
// and bound Resource, persists a hash-keyed record, and returns the
// plaintext the caller writes to the token response. The expires_at
// stamp is issued_at + AccessTokenTTL exactly, both drawn from a
// single s.now() read so first-use validation cannot trip on
// borderline-clock skew (R-E5GH-PN6G's posture, recorded here so the
// wire path inherits it). The 32 random bytes give 256 bits of
// entropy — well clear of collision concerns and unguessable enough
// that the plaintext alone is the credential.
func (s *TokenStore) IssueAccess(OwnerEmail, ClientID, Resource string) (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	plaintext := hex.EncodeToString(buf[:])
	hash := TokenHash(plaintext)
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.ensureReady()
	now := s.now()
	rec := &Token{
		Kind:       "access",
		OwnerEmail: OwnerEmail,
		ClientID:   ClientID,
		Resource:   Resource,
		IssuedAt:   now,
		ExpiresAt:  now.Add(s.accessTTL()),
	}
	if err := s.persistTokenLocked(hash, rec); err != nil {
		return "", err
	}
	s.M[hash] = rec
	return plaintext, nil
}

func randomTokenSecret() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func randomChainID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// issueInitialTokenPairR_2HT5_50F4 mints the access and refresh token
// returned by one authorization-code exchange into the same MCP token
// chain. Revoking the agents-block row for that chain therefore revokes
// the initial access token even before the refresh token has ever rotated.
func (s *TokenStore) IssueInitialTokenPair(
	OwnerEmail, ClientID, Resource string,
) (string, string, error) {
	accessPlain, err := randomTokenSecret()
	if err != nil {
		return "", "", err
	}
	refreshPlain, err := randomTokenSecret()
	if err != nil {
		return "", "", err
	}
	ChainID, err := randomChainID()
	if err != nil {
		return "", "", err
	}
	accessHash := TokenHash(accessPlain)
	refreshHash := TokenHash(refreshPlain)
	s.Mu.Lock()
	s.ensureReady()
	now := s.now()
	accessRec := &Token{
		Kind:       "access",
		OwnerEmail: OwnerEmail,
		ClientID:   ClientID,
		Resource:   Resource,
		IssuedAt:   now,
		ExpiresAt:  now.Add(s.accessTTL()),
		ChainID:    ChainID,
	}
	refreshRec := &Token{
		Kind:       "refresh",
		OwnerEmail: OwnerEmail,
		ClientID:   ClientID,
		Resource:   Resource,
		IssuedAt:   now,
		ExpiresAt:  now.Add(s.refreshTTL()),
		ChainID:    ChainID,
	}

	if s.DB != nil {
		tx, err := s.DB.Begin()
		if err != nil {
			s.Mu.Unlock()
			return "", "", err
		}
		if err := persistTokenRecordLocked(tx, accessHash, accessRec); err != nil {
			_ = tx.Rollback()
			s.Mu.Unlock()
			return "", "", err
		}
		if err := persistTokenRecordLocked(tx, refreshHash, refreshRec); err != nil {
			_ = tx.Rollback()
			s.Mu.Unlock()
			return "", "", err
		}
		if err := tx.Commit(); err != nil {
			s.Mu.Unlock()
			return "", "", err
		}
	}
	s.M[accessHash] = accessRec
	s.M[refreshHash] = refreshRec
	s.Mu.Unlock()
	// R-0TVF-0BKI: a fresh chain just appeared in this owner's live set.
	s.notifyAgents(OwnerEmail)
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
// ExpiresAt = IssuedAt + RefreshTokenTTL, and rotateRefresh rejects a
// presented refresh past that ceiling. Chain membership (R-9HGE-87UG)
// is established here too — every fresh issueRefresh mints a new
// ChainID; rotateRefresh propagates it onto successors so the chain
// is walkable on reuse detection.
func (s *TokenStore) IssueRefresh(OwnerEmail, ClientID, Resource string) (string, error) {
	plaintext, err := randomTokenSecret()
	if err != nil {
		return "", err
	}
	ChainID, err := randomChainID()
	if err != nil {
		return "", err
	}
	hash := TokenHash(plaintext)
	s.Mu.Lock()
	s.ensureReady()
	now := s.now()
	rec := &Token{
		Kind:       "refresh",
		OwnerEmail: OwnerEmail,
		ClientID:   ClientID,
		Resource:   Resource,
		IssuedAt:   now,
		ExpiresAt:  now.Add(s.refreshTTL()),
		ChainID:    ChainID,
	}
	if err := s.persistTokenLocked(hash, rec); err != nil {
		s.Mu.Unlock()
		return "", err
	}
	s.M[hash] = rec
	s.Mu.Unlock()
	// R-0TVF-0BKI: a fresh chain just appeared in this owner's live set.
	s.notifyAgents(OwnerEmail)
	return plaintext, nil
}

// rotateRefresh atomically consumes the presented refresh-token
// plaintext and issues a new (access, refresh) pair bound to the same
// owner / client / Resource. The consumed record's UsedAt is set in the
// same critical section that stores the successor records — observers
// can never see a window in which the old refresh is still spendable
// after the new one exists, nor one in which the new pair exists
// without the predecessor being marked used. Returns the new access
// plaintext and the new refresh plaintext on success.
func (s *TokenStore) RotateRefresh(plaintext string) (string, string, error) {
	return s.RotateRefreshForClient(plaintext, "")
}

func (s *TokenStore) RotateRefreshForClient(plaintext, ClientID string) (string, string, error) {
	if plaintext == "" {
		return "", "", errors.New("refresh token: empty")
	}
	var accessBuf, refreshBuf [32]byte
	if _, err := rand.Read(accessBuf[:]); err != nil {
		return "", "", err
	}
	if _, err := rand.Read(refreshBuf[:]); err != nil {
		return "", "", err
	}
	newAccess := hex.EncodeToString(accessBuf[:])
	newRefresh := hex.EncodeToString(refreshBuf[:])
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.ensureReady()
	now := s.now()
	presentedHash := TokenHash(plaintext)
	rec, ok := s.M[presentedHash]
	if !ok || rec.Kind != "refresh" {
		return "", "", errors.New("refresh token: unknown")
	}
	if !rec.UsedAt.IsZero() {
		// R-9HGE-87UG: a second presentation of an already-consumed
		// refresh is evidence of compromise. Reject this request and
		// revoke every record sharing the replayed refresh's ChainID —
		// the live successor refresh and any outstanding access tokens
		// issued from this chain. lookupAccess already rejects records
		// with RevokedAt set, so newly arriving requests bearing any
		// chain member are bounced (R-A26O-QBG9).
		revokedOwner := ""
		if rec.ChainID != "" {
			for otherHash, other := range s.M {
				if other.ChainID == rec.ChainID && other.RevokedAt.IsZero() {
					other.RevokedAt = now
					_ = s.persistTokenLocked(otherHash, other)
				}
			}
			revokedOwner = rec.OwnerEmail
		}
		s.Mu.Unlock()
		// R-0TVF-0BKI: the chain just disappeared from this owner's live set.
		if revokedOwner != "" {
			s.notifyAgents(revokedOwner)
		}
		s.Mu.Lock()
		return "", "", errors.New("refresh token: already used")
	}
	if !rec.RevokedAt.IsZero() {
		return "", "", errors.New("refresh token: revoked")
	}
	// R-8UAA-YKR9: a refresh past its thirty-day ceiling is no longer
	// rotatable. Strict Before mirrors the access-token gate's
	// boundary discipline (R-TNXJ-ZWQ0).
	if !now.Before(rec.ExpiresAt) {
		return "", "", errors.New("refresh token: expired")
	}
	// R-5P7B-KY5Z: the wire refresh-token grant must identify the same
	// OAuth client that originally received this refresh token. The
	// store-level primitive takes an optional client binding so existing
	// direct rotation tests can exercise token lifecycle independently,
	// while /oauth/token passes the presented client_id and rejects a
	// mismatch before consuming the refresh.
	if ClientID != "" && rec.ClientID != ClientID {
		return "", "", errors.New("refresh token: client_id mismatch")
	}
	newAccessHash := TokenHash(newAccess)
	newRefreshHash := TokenHash(newRefresh)
	rec.UsedAt = now
	newAccessRec := &Token{
		Kind:       "access",
		OwnerEmail: rec.OwnerEmail,
		ClientID:   rec.ClientID,
		Resource:   rec.Resource,
		IssuedAt:   now,
		ExpiresAt:  now.Add(s.accessTTL()),
		ChainID:    rec.ChainID,
	}
	newRefreshRec := &Token{
		Kind:       "refresh",
		OwnerEmail: rec.OwnerEmail,
		ClientID:   rec.ClientID,
		Resource:   rec.Resource,
		IssuedAt:   now,
		ExpiresAt:  now.Add(s.refreshTTL()),
		ChainID:    rec.ChainID,
	}
	if err := s.persistTokenLocked(presentedHash, rec); err != nil {
		rec.UsedAt = time.Time{}
		return "", "", err
	}
	if err := s.persistTokenLocked(newAccessHash, newAccessRec); err != nil {
		rec.UsedAt = time.Time{}
		return "", "", err
	}
	if err := s.persistTokenLocked(newRefreshHash, newRefreshRec); err != nil {
		rec.UsedAt = time.Time{}
		return "", "", err
	}
	s.M[newAccessHash] = newAccessRec
	s.M[newRefreshHash] = newRefreshRec
	return newAccess, newRefresh, nil
}

// revokeChainR_D0XD_1YT0 atomically marks every record sharing ChainID
// as revoked, scoped to OwnerEmail. Returns true when the chain existed
// and belonged to OwnerEmail; returns false when no record matches the
// ChainID, or when at least one record with that ChainID is owned by a
// different email — the caller surfaces both cases identically so the
// service does not disclose whether such a chain exists.
func (s *TokenStore) RevokeChain(ChainID, OwnerEmail string) bool {
	if ChainID == "" || OwnerEmail == "" {
		return false
	}
	s.Mu.Lock()
	s.ensureReady()
	now := s.now()
	matched := false
	for _, rec := range s.M {
		if rec.ChainID != ChainID {
			continue
		}
		if rec.OwnerEmail != OwnerEmail {
			s.Mu.Unlock()
			return false
		}
		matched = true
	}
	if !matched {
		s.Mu.Unlock()
		return false
	}
	for hash, rec := range s.M {
		if rec.ChainID == ChainID && rec.RevokedAt.IsZero() {
			rec.RevokedAt = now
			_ = s.persistTokenLocked(hash, rec)
		}
	}
	s.Mu.Unlock()
	// R-0TVF-0BKI: the chain just disappeared from this owner's live set.
	s.notifyAgents(OwnerEmail)
	return true
}

// R-0NRX-3GV1: a single live MCP token chain owned by some email, as
// surfaced to the agents block on the index page. The "live" filter is
// applied at collection time (at least one un-revoked, un-expired
// refresh record under the ChainID); rows here are the per-chain
// roll-up used for rendering. Chain initial issuance is the earliest
// refresh record's IssuedAt; refresh rotation does not change the rendered
// row identity used for ordering.
type AgentChain struct {
	ChainID    string
	ClientID   string
	ClientName string
	IssuedAt   time.Time
}

// liveAgentChainsR_0NRX_3GV1 walks the token store under .Mu.Lock(),
// groups un-revoked un-expired refresh records owned by `email` by
// ChainID, and returns one entry per chain. Client name is resolved
// from the supplied OAuth client store after the token-store lock is released to
// keep the two stores' critical sections independent. Order is not pinned
// here; the render and stream seams sort by rendered row identity.
func (s *TokenStore) LiveAgentChains(
	email string, clients *ClientStore,
) []AgentChain {
	if email == "" {
		return nil
	}
	type partial struct {
		ClientID string
		IssuedAt time.Time
	}
	byChain := map[string]*partial{}
	s.Mu.Lock()
	s.ensureReady()
	now := s.now()
	for _, rec := range s.M {
		if rec.Kind != "refresh" {
			continue
		}
		if rec.OwnerEmail != email {
			continue
		}
		if rec.ChainID == "" {
			continue
		}
		if !rec.RevokedAt.IsZero() {
			continue
		}
		if !now.Before(rec.ExpiresAt) {
			continue
		}
		cur, ok := byChain[rec.ChainID]
		if !ok {
			byChain[rec.ChainID] = &partial{
				ClientID: rec.ClientID,
				IssuedAt: rec.IssuedAt,
			}
			continue
		}
		if rec.IssuedAt.Before(cur.IssuedAt) {
			cur.IssuedAt = rec.IssuedAt
		}
	}
	s.Mu.Unlock()
	if len(byChain) == 0 {
		return nil
	}
	out := make([]AgentChain, 0, len(byChain))
	for ChainID, p := range byChain {
		row := AgentChain{
			ChainID:  ChainID,
			ClientID: p.ClientID,
			IssuedAt: p.IssuedAt,
		}
		if clients != nil {
			if c := clients.Lookup(p.ClientID); c != nil {
				row.ClientName = c.ClientName()
			}
		}
		out = append(out, row)
	}
	return out
}

// LookupAccess returns the access-token record for the presented plaintext if
// one exists and is currently valid.
func (s *TokenStore) LookupAccess(plaintext string) *Token {
	rec, _ := s.LookupAccessReason(plaintext)
	return rec
}

// LookupAccessReason is LookupAccess with a stable rejection discriminator:
// "unknown", "expired", or "revoked".
func (s *TokenStore) LookupAccessReason(plaintext string) (*Token, string) {
	if plaintext == "" {
		return nil, "unknown"
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.ensureReady()
	rec, ok := s.M[TokenHash(plaintext)]
	if !ok {
		return nil, "unknown"
	}
	if rec.Kind != "access" {
		return nil, "unknown"
	}
	if !rec.RevokedAt.IsZero() {
		return nil, "revoked"
	}
	if !s.now().Before(rec.ExpiresAt) {
		return nil, "expired"
	}
	return rec, ""
}

// DetachDBIf clears the backing database if it is currently db.
func (s *TokenStore) DetachDBIf(db *sql.DB) {
	s.Mu.Lock()
	if s.DB == db {
		s.DB = nil
	}
	s.Mu.Unlock()
}
