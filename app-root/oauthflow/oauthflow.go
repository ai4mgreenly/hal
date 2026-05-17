// Package oauthflow owns the browser- and MCP-facing Google federation
// redirect flow for HAL's OAuth authorization server.
package oauthflow

import (
	"net/http"
	"net/url"
	"time"

	googleidppkg "github.com/mgreenly/hal/googleidp"
	jsonapipkg "github.com/mgreenly/hal/jsonapi"
	oauthpkg "github.com/mgreenly/hal/oauth"
	websessionpkg "github.com/mgreenly/hal/websession"
)

// Surface carries the dependencies needed by the OAuth/Google federation flow.
type Surface struct {
	GoogleIDP                   googleidppkg.Provider
	OAuthStates                 *oauthpkg.StateStore
	OAuthAuthCodes              *oauthpkg.AuthCodeStore
	OAuthClients                *oauthpkg.ClientStore
	WebSessions                 *websessionpkg.Store
	OAuthStateTTL               func() time.Duration
	WebSessionAbsoluteTTL       func() time.Duration
	WorkspaceDomain             func() string
	CanonicalResourceIdentifier func() string
	RequestBaseURL              func(*http.Request) string
	ForwardedProtoHTTPS         func(*http.Request) bool
	WriteOAuthError             func(http.ResponseWriter, int, string, string)
	NewStateValue               func() (string, error)
}

func (s Surface) oauthStateTTL() time.Duration {
	if s.OAuthStateTTL != nil {
		return s.OAuthStateTTL()
	}
	return 5 * time.Minute
}

func (s Surface) webSessionAbsoluteTTL() time.Duration {
	if s.WebSessionAbsoluteTTL != nil {
		return s.WebSessionAbsoluteTTL()
	}
	return 12 * time.Hour
}

func (s Surface) workspaceDomain() string {
	if s.WorkspaceDomain != nil {
		return s.WorkspaceDomain()
	}
	return ""
}

func (s Surface) canonicalResourceIdentifier() string {
	if s.CanonicalResourceIdentifier != nil {
		return s.CanonicalResourceIdentifier()
	}
	return ""
}

func (s Surface) requestBaseURL(r *http.Request) string {
	if s.RequestBaseURL != nil {
		return s.RequestBaseURL(r)
	}
	return jsonapipkg.RequestBaseURL(r)
}

func (s Surface) forwardedProtoHTTPS(r *http.Request) bool {
	if s.ForwardedProtoHTTPS != nil {
		return s.ForwardedProtoHTTPS(r)
	}
	return false
}

func (s Surface) writeOAuthError(w http.ResponseWriter, status int, code, desc string) {
	if s.WriteOAuthError != nil {
		s.WriteOAuthError(w, status, code, desc)
		return
	}
	jsonapipkg.WriteOAuthError(w, status, code, desc)
}

func (s Surface) newStateValue() (string, error) {
	if s.NewStateValue != nil {
		return s.NewStateValue()
	}
	return oauthpkg.NewStateValue()
}

// HandleLogin initiates the web-origin Google federation flow.
func (s Surface) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if s.GoogleIDP == nil {
		http.Error(w, "google identity provider not configured",
			http.StatusServiceUnavailable)
		return
	}
	state, err := s.newStateValue()
	if err != nil {
		http.Error(w, "state generation failed",
			http.StatusInternalServerError)
		return
	}
	bindingID, err := s.newStateValue()
	if err != nil {
		http.Error(w, "state generation failed",
			http.StatusInternalServerError)
		return
	}
	s.OAuthStates.PutWeb(state, bindingID)
	http.SetCookie(w, &http.Cookie{
		Name:     oauthpkg.StateCookieName,
		Value:    bindingID,
		Path:     "/",
		MaxAge:   int(s.oauthStateTTL() / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.forwardedProtoHTTPS(r),
	})
	redirectURI := s.requestBaseURL(r) + "/oauth/google/callback"
	http.Redirect(w, r, s.GoogleIDP.AuthorizationURL(redirectURI, state, true),
		http.StatusSeeOther)
}

// HandleGoogleCallback completes web- and MCP-origin Google federation flows.
func (s Surface) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, oauthpkg.ErrStateMissing.Error(), http.StatusBadRequest)
		return
	}
	var presentedBinding string
	if c, err := r.Cookie(oauthpkg.StateCookieName); err == nil {
		presentedBinding = c.Value
	}
	stateRec, err := s.OAuthStates.Consume(state, presentedBinding)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthpkg.StateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.forwardedProtoHTTPS(r),
	})
	if s.GoogleIDP == nil {
		http.Error(w, "google identity provider not configured",
			http.StatusServiceUnavailable)
		return
	}
	identity, err := s.GoogleIDP.ExchangeCode(
		r.Context(),
		r.URL.Query().Get("code"),
		s.requestBaseURL(r)+"/oauth/google/callback",
	)
	if err != nil {
		http.Error(w, "google code exchange failed", http.StatusBadGateway)
		return
	}
	if !identity.EmailVerified {
		if mcpCtx := stateRec.MCPContext(); stateRec.Origin() == "mcp" && mcpCtx != nil {
			WriteOAuthErrorRedirect(w, r, mcpCtx.RedirectURI,
				"access_denied",
				"Google email address is not verified",
				mcpCtx.ClientState)
			return
		}
		http.Error(w, "Google email address is not verified",
			http.StatusForbidden)
		return
	}
	if identity.HostedDomain != s.workspaceDomain() {
		if mcpCtx := stateRec.MCPContext(); stateRec.Origin() == "mcp" && mcpCtx != nil {
			WriteOAuthErrorRedirect(w, r, mcpCtx.RedirectURI,
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
	switch stateRec.Origin() {
	case "mcp":
		s.handleMCPCallback(w, r, stateRec, identity)
	case "web":
		s.handleWebCallback(w, r, identity)
	default:
		http.Error(w, "state record carries unknown origin",
			http.StatusInternalServerError)
	}
}

func (s Surface) handleMCPCallback(
	w http.ResponseWriter, r *http.Request,
	stateRec *oauthpkg.StateRecord, identity googleidppkg.Identity,
) {
	mcpCtx := stateRec.MCPContext()
	if mcpCtx == nil {
		http.Error(w, "mcp state record missing context",
			http.StatusInternalServerError)
		return
	}
	code, err := s.OAuthAuthCodes.IssueWithResource(
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
	target := mcpCtx.RedirectURI +
		"?code=" + url.QueryEscape(code) +
		"&state=" + url.QueryEscape(mcpCtx.ClientState)
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s Surface) handleWebCallback(
	w http.ResponseWriter, r *http.Request, identity googleidppkg.Identity,
) {
	plaintext, err := s.WebSessions.Issue(identity.Email)
	if err != nil {
		http.Error(w, "session issuance failed",
			http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     websessionpkg.CookieName,
		Value:    plaintext,
		Path:     "/",
		MaxAge:   int(s.webSessionAbsoluteTTL() / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.forwardedProtoHTTPS(r),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// WriteOAuthErrorRedirect issues a 303 OAuth error redirect to a registered
// MCP redirect_uri.
func WriteOAuthErrorRedirect(
	w http.ResponseWriter, r *http.Request,
	redirectURI, errCode, errDesc, clientState string,
) {
	target := redirectURI +
		"?error=" + url.QueryEscape(errCode) +
		"&error_description=" + url.QueryEscape(errDesc) +
		"&state=" + url.QueryEscape(clientState)
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// ValidRedirectURI reports whether raw is an absolute http(s) redirect URI
// with a host and no fragment.
func ValidRedirectURI(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// SupportedAuthorizeCodeChallengeMethod reports whether method is accepted at
// the OAuth authorize endpoint.
func SupportedAuthorizeCodeChallengeMethod(method string) bool {
	return method == "S256"
}

// HandleOAuthAuthorize initiates the MCP-origin Google federation flow.
func (s Surface) HandleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if s.GoogleIDP == nil {
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
	client := s.OAuthClients.Lookup(clientID)
	if client == nil {
		http.Error(w, "unknown client_id", http.StatusBadRequest)
		return
	}
	requested := q.Get("redirect_uri")
	if requested == "" {
		http.Error(w, "redirect_uri is required", http.StatusBadRequest)
		return
	}
	if !clientAllowsRedirect(client, requested) {
		http.Error(w, "redirect_uri does not match a registered value",
			http.StatusBadRequest)
		return
	}
	if q.Get("response_type") != "code" {
		http.Error(w, "response_type must be code", http.StatusBadRequest)
		return
	}
	if q.Get("code_challenge") == "" {
		http.Error(w, "code_challenge is required", http.StatusBadRequest)
		return
	}
	if !SupportedAuthorizeCodeChallengeMethod(q.Get("code_challenge_method")) {
		http.Error(w, "unsupported code_challenge_method", http.StatusBadRequest)
		return
	}
	requestedResource := q.Get("resource")
	if requestedResource == "" {
		requestedResource = s.canonicalResourceIdentifier()
	} else if requestedResource != s.canonicalResourceIdentifier() {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_target",
			"resource parameter does not match this service's canonical identifier")
		return
	}
	state, err := s.newStateValue()
	if err != nil {
		http.Error(w, "state generation failed",
			http.StatusInternalServerError)
		return
	}
	bindingID, err := s.newStateValue()
	if err != nil {
		http.Error(w, "state generation failed",
			http.StatusInternalServerError)
		return
	}
	s.OAuthStates.PutMCP(state, bindingID, oauthpkg.StateMCPContext{
		ClientID:            clientID,
		RedirectURI:         requested,
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
		ClientState:         q.Get("state"),
		Resource:            requestedResource,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oauthpkg.StateCookieName,
		Value:    bindingID,
		Path:     "/",
		MaxAge:   int(s.oauthStateTTL() / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.forwardedProtoHTTPS(r),
	})
	redirectURI := s.requestBaseURL(r) + "/oauth/google/callback"
	http.Redirect(w, r, s.GoogleIDP.AuthorizationURL(redirectURI, state, false),
		http.StatusSeeOther)
}

func clientAllowsRedirect(client *oauthpkg.Client, requested string) bool {
	for _, u := range client.RedirectURIs() {
		if u == requested {
			return true
		}
	}
	return false
}
