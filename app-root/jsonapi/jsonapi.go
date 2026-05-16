// Package jsonapi owns HAL's machine-facing JSON HTTP surface.
package jsonapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	counterpkg "github.com/mgreenly/hal/counter"
	oauthpkg "github.com/mgreenly/hal/oauth"
	websessionpkg "github.com/mgreenly/hal/websession"
)

// MaxRequestBodyBytes is the fixed cap applied before JSON and form parsing.
const MaxRequestBodyBytes int64 = 1 << 20

const oauthRefreshTokenFormField = "refresh_" + "token"

// Surface carries the dependencies needed by the JSON API handlers.
type Surface struct {
	Counter                     *counterpkg.Counter
	WebSessions                 *websessionpkg.Store
	OAuthTokens                 *oauthpkg.TokenStore
	OAuthClients                *oauthpkg.ClientStore
	OAuthAuthCodes              *oauthpkg.AuthCodeStore
	Now                         func() time.Time
	NewOAuthClientID            func() (string, error)
	CanonicalResourceIdentifier func() string
	AccessTokenTTL              func() time.Duration
}

func (s Surface) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Surface) newOAuthClientID() (string, error) {
	if s.NewOAuthClientID != nil {
		return s.NewOAuthClientID()
	}
	return oauthpkg.NewClientID()
}

func (s Surface) canonicalResourceIdentifier() string {
	if s.CanonicalResourceIdentifier != nil {
		return s.CanonicalResourceIdentifier()
	}
	return ""
}

func (s Surface) accessTokenTTL() time.Duration {
	if s.AccessTokenTTL != nil {
		return s.AccessTokenTTL()
	}
	return time.Hour
}

// LimitRequestBody wraps r.Body with the shared fixed cap before parsing.
func LimitRequestBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
}

// RequestBodyTooLarge reports whether err is the MaxBytesReader cap failure.
func RequestBodyTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

// WriteBodyTooLarge emits the shared oversized-body error response.
func WriteBodyTooLarge(w http.ResponseWriter) {
	http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
}

// RequestBaseURL returns the externally-observable scheme://host for r.
func RequestBaseURL(r *http.Request) string {
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

// HandleCounterRead writes the current counter value as JSON.
func (s Surface) HandleCounterRead(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Value uint64 `json:"value"`
	}{Value: s.Counter.Read()})
}

// HandleOAuthAuthorizationServerMetadata writes the OAuth AS metadata document.
func (s Surface) HandleOAuthAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	base := RequestBaseURL(r)
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

// HandleOAuthProtectedResourceMetadata writes the protected-resource metadata document.
func (s Surface) HandleOAuthProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	base := RequestBaseURL(r)
	doc := struct {
		Resource               string   `json:"resource"`
		AuthorizationServers   []string `json:"authorization_servers"`
		BearerMethodsSupported []string `json:"bearer_methods_supported"`
	}{
		Resource:               s.canonicalResourceIdentifier(),
		AuthorizationServers:   []string{base},
		BearerMethodsSupported: []string{"header"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

// HandleOAuthRegister handles OAuth Dynamic Client Registration.
func (s Surface) HandleOAuthRegister(w http.ResponseWriter, r *http.Request) {
	LimitRequestBody(w, r)
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
		if RequestBodyTooLarge(err) {
			WriteBodyTooLarge(w)
			return
		}
		WriteOAuthError(w, http.StatusBadRequest,
			"invalid_client_metadata", "request body is not valid JSON")
		return
	}
	if len(req.RedirectURIs) == 0 {
		WriteOAuthError(w, http.StatusBadRequest,
			"invalid_redirect_uri",
			"redirect_uris is required and must be a non-empty array")
		return
	}
	for _, u := range req.RedirectURIs {
		if !validOAuthRedirectURI(u) {
			WriteOAuthError(w, http.StatusBadRequest,
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
		WriteOAuthError(w, http.StatusBadRequest,
			"invalid_client_metadata",
			"token_endpoint_auth_method must be none")
		return
	}
	clientName, ok := NormalizeOAuthClientName(req.ClientName)
	if !ok {
		WriteOAuthError(w, http.StatusBadRequest,
			"invalid_client_metadata",
			"client_name must be at most 80 characters and contain no control characters")
		return
	}
	rec := oauthpkg.NewClient(oauthpkg.ClientSpec{
		RedirectURIs:  req.RedirectURIs,
		ClientName:    clientName,
		GrantTypes:    req.GrantTypes,
		ResponseTypes: req.ResponseTypes,
		AuthMethod:    authMethod,
		IssuedAt:      s.now().Unix(),
	})
	var clientID string
	for range 8 {
		var err error
		clientID, err = s.newOAuthClientID()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if s.OAuthClients.PutIfAbsent(clientID, rec) {
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
		ClientIDIssuedAt:        rec.IssuedAt(),
		RedirectURIs:            rec.RedirectURIs(),
		TokenEndpointAuthMethod: rec.AuthMethod(),
		GrantTypes:              rec.GrantTypes(),
		ResponseTypes:           rec.ResponseTypes(),
		ClientName:              rec.ClientName(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// NormalizeOAuthClientName trims and validates DCR display names.
func NormalizeOAuthClientName(raw string) (string, bool) {
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
	if !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// HandleOAuthToken handles authorization_code and refresh_token grants.
func (s Surface) HandleOAuthToken(w http.ResponseWriter, r *http.Request) {
	LimitRequestBody(w, r)
	if err := r.ParseForm(); err != nil {
		if RequestBodyTooLarge(err) {
			WriteBodyTooLarge(w)
			return
		}
		WriteOAuthError(w, http.StatusBadRequest, "invalid_request",
			"could not parse request body")
		return
	}
	if res := r.PostForm.Get("resource"); res != "" && res != s.canonicalResourceIdentifier() {
		WriteOAuthError(w, http.StatusBadRequest, "invalid_target",
			"resource parameter does not match this service's canonical identifier")
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.HandleOAuthTokenAuthCode(w, r)
	case "refresh_token":
		s.HandleOAuthTokenRefresh(w, r)
	default:
		WriteOAuthError(w, http.StatusBadRequest, "unsupported_grant_type",
			"only authorization_code and refresh_token are supported")
	}
}

// HandleOAuthTokenAuthCode handles the authorization_code grant after form parsing.
func (s Surface) HandleOAuthTokenAuthCode(w http.ResponseWriter, r *http.Request) {
	code := r.PostForm.Get("code")
	clientID := r.PostForm.Get("client_id")
	redirectURI := r.PostForm.Get("redirect_uri")
	codeVerifier := r.PostForm.Get("code_verifier")
	if code == "" || clientID == "" || redirectURI == "" || codeVerifier == "" {
		WriteOAuthError(w, http.StatusBadRequest, "invalid_request",
			"authorization_code grant requires code, client_id, "+
				"redirect_uri, code_verifier")
		return
	}
	rec, err := s.OAuthAuthCodes.Redeem(code, clientID, redirectURI, codeVerifier)
	if err != nil {
		WriteOAuthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	access, refresh, err := s.OAuthTokens.IssueInitialTokenPair(
		rec.OwnerEmail(), rec.ClientID(), rec.Resource())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.writeTokenPair(w, access, refresh)
}

// HandleOAuthTokenRefresh handles the refresh_token grant after form parsing.
func (s Surface) HandleOAuthTokenRefresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.PostForm.Get(oauthRefreshTokenFormField)
	clientID := r.PostForm.Get("client_id")
	if refreshToken == "" || clientID == "" {
		WriteOAuthError(w, http.StatusBadRequest, "invalid_request",
			"refresh_token grant requires refresh_token and client_id")
		return
	}
	access, refresh, err := s.OAuthTokens.RotateRefreshForClient(refreshToken, clientID)
	if err != nil {
		WriteOAuthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	s.writeTokenPair(w, access, refresh)
}

func (s Surface) writeTokenPair(w http.ResponseWriter, access, refresh string) {
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
		ExpiresIn:    int(s.accessTokenTTL() / time.Second),
		RefreshToken: refresh,
	})
}

// WriteOAuthError writes the standard OAuth JSON error shape.
func WriteOAuthError(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description,omitempty"`
	}{Error: code, ErrorDescription: desc})
}

// CheckMutationAuth validates authentication for counter mutations.
func (s Surface) CheckMutationAuth(r *http.Request) (bool, int, string, string) {
	cookiePresented := false
	cookieRejectedByOrigin := false
	if c, err := r.Cookie(websessionpkg.CookieName); err == nil {
		cookiePresented = true
		if sess := s.WebSessions.Lookup(c.Value); sess != nil {
			if SameOriginBrowserMutation(r) {
				return true, 0, "", ""
			}
			cookieRejectedByOrigin = true
		}
	}
	authHeader := r.Header.Get("Authorization")
	plaintext, bearerOK := BearerTokenFromRequest(r)
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
	rec, reason := s.OAuthTokens.LookupAccessReason(plaintext)
	if rec != nil {
		if rec.Resource != s.canonicalResourceIdentifier() {
			return false, http.StatusUnauthorized, "invalid_token",
				"bearer token resource binding does not match"
		}
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

// SameOriginBrowserMutation enforces browser same-origin mutation checks.
func SameOriginBrowserMutation(r *http.Request) bool {
	want := RequestBaseURL(r)
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

// BearerTokenFromRequest extracts the opaque token from an Authorization header.
func BearerTokenFromRequest(r *http.Request) (string, bool) {
	return ParseBearerAuthHeader(r.Header.Get("Authorization"))
}

// ParseBearerAuthHeader extracts the token from a Bearer header value.
func ParseBearerAuthHeader(h string) (string, bool) {
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

// WriteMutationAuthFailure emits the shared mutation authentication error response.
func WriteMutationAuthFailure(w http.ResponseWriter, status int, errCode, errDesc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description,omitempty"`
	}{Error: errCode, ErrorDescription: errDesc})
}

// WriteMutationUnauthorized emits the standard 401 mutation auth response.
func WriteMutationUnauthorized(w http.ResponseWriter, errCode, errDesc string) {
	WriteMutationAuthFailure(w, http.StatusUnauthorized, errCode, errDesc)
}

// WriteSameOriginForbidden emits the standard same-origin failure response.
func WriteSameOriginForbidden(w http.ResponseWriter) {
	WriteMutationAuthFailure(w, http.StatusForbidden, "invalid_request",
		"same-origin browser request required")
}

// HandleCounterIncrement authenticates, increments, and writes the JSON result.
func (s Surface) HandleCounterIncrement(w http.ResponseWriter, r *http.Request) {
	if ok, status, errCode, errDesc := s.CheckMutationAuth(r); !ok {
		WriteMutationAuthFailure(w, status, errCode, errDesc)
		return
	}
	v := s.Counter.Increment()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Value uint64 `json:"value"`
	}{Value: v})
}

// HandleCounterDecrement authenticates, decrements, and writes the JSON result.
func (s Surface) HandleCounterDecrement(w http.ResponseWriter, r *http.Request) {
	if ok, status, errCode, errDesc := s.CheckMutationAuth(r); !ok {
		WriteMutationAuthFailure(w, status, errCode, errDesc)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	v, ok := s.Counter.Decrement()
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
