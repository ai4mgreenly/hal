package oauth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"sync"
)

// Client is a registered OAuth client record.
type Client struct {
	redirectURIs  []string
	clientName    string
	grantTypes    []string
	responseTypes []string
	authMethod    string
	issuedAt      int64
}

// ClientSpec contains the values recorded for a registered OAuth client.
type ClientSpec struct {
	RedirectURIs  []string
	ClientName    string
	GrantTypes    []string
	ResponseTypes []string
	AuthMethod    string
	IssuedAt      int64
}

// NewClient returns a registered OAuth client record.
func NewClient(spec ClientSpec) *Client {
	return &Client{
		redirectURIs:  append([]string(nil), spec.RedirectURIs...),
		clientName:    spec.ClientName,
		grantTypes:    append([]string(nil), spec.GrantTypes...),
		responseTypes: append([]string(nil), spec.ResponseTypes...),
		authMethod:    spec.AuthMethod,
		issuedAt:      spec.IssuedAt,
	}
}

// RedirectURIs returns the registered redirect URIs.
func (c *Client) RedirectURIs() []string {
	if c == nil {
		return nil
	}
	return append([]string(nil), c.redirectURIs...)
}

// ClientName returns the registered display name.
func (c *Client) ClientName() string {
	if c == nil {
		return ""
	}
	return c.clientName
}

// GrantTypes returns the registered grant types.
func (c *Client) GrantTypes() []string {
	if c == nil {
		return nil
	}
	return append([]string(nil), c.grantTypes...)
}

// ResponseTypes returns the registered response types.
func (c *Client) ResponseTypes() []string {
	if c == nil {
		return nil
	}
	return append([]string(nil), c.responseTypes...)
}

// AuthMethod returns the registered token endpoint auth method.
func (c *Client) AuthMethod() string {
	if c == nil {
		return ""
	}
	return c.authMethod
}

// IssuedAt returns the registration timestamp.
func (c *Client) IssuedAt() int64 {
	if c == nil {
		return 0
	}
	return c.issuedAt
}

// ClientStore keeps OAuth client registrations, optionally backed by SQLite.
type ClientStore struct {
	mu sync.Mutex
	m  map[string]*Client
	db *sql.DB
}

// NewClientStore returns an in-memory OAuth client registration store.
func NewClientStore() *ClientStore {
	return &ClientStore{m: map[string]*Client{}}
}

func (s *ClientStore) ensureReady() {
	if s.m == nil {
		s.m = map[string]*Client{}
	}
}

// Put records or replaces clientID.
func (s *ClientStore) Put(clientID string, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	if s.db != nil {
		_ = s.insertOrReplaceLocked(clientID, c)
	}
	s.m[clientID] = c
}

// PutIfAbsent records clientID only if it is not already registered.
func (s *ClientStore) PutIfAbsent(clientID string, c *Client) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
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

// Lookup returns the registered client for clientID, if any.
func (s *ClientStore) Lookup(clientID string) *Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	return s.m[clientID]
}

// Count returns the number of registered clients.
func (s *ClientStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureReady()
	return len(s.m)
}

// Attach loads persisted client registrations and binds db as the
// write-through backing store.
func (s *ClientStore) Attach(db *sql.DB) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := db.Query(
		`SELECT client_id, redirect_uris, client_name, grant_types, ` +
			`response_types, auth_method, issued_at FROM oauth_clients`)
	if err != nil {
		return err
	}
	defer rows.Close()

	loaded := map[string]*Client{}
	for rows.Next() {
		var clientID, redirectJSON, grantJSON, responseJSON string
		rec := &Client{}
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

// DetachDBIf clears the backing database if it is currently db.
func (s *ClientStore) DetachDBIf(db *sql.DB) {
	s.mu.Lock()
	if s.db == db {
		s.db = nil
	}
	s.mu.Unlock()
}

func (s *ClientStore) insertOrReplaceLocked(clientID string, c *Client) error {
	redirects, grants, responses, err := marshalClientLists(c)
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

func (s *ClientStore) insertIfAbsentLocked(clientID string, c *Client) (bool, error) {
	redirects, grants, responses, err := marshalClientLists(c)
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

func marshalClientLists(c *Client) (string, string, string, error) {
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

// NewClientID returns a 32-character cryptographically random hex value.
func NewClientID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
