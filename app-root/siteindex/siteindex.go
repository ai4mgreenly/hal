// Package siteindex owns the request assembly for HAL's site-root page.
package siteindex

import (
	"net/http"

	counterpkg "github.com/mgreenly/hal/counter"
	oauthpkg "github.com/mgreenly/hal/oauth"
	webpkg "github.com/mgreenly/hal/web"
	websessionpkg "github.com/mgreenly/hal/websession"
)

// Surface carries the dependencies needed to assemble and render the site root.
type Surface struct {
	Counter        *counterpkg.Counter
	WebSessions    *websessionpkg.Store
	OAuthTokens    *oauthpkg.TokenStore
	OAuthClients   *oauthpkg.ClientStore
	RequestBaseURL func(*http.Request) string
	Version        string
}

func (s Surface) requestBaseURL(r *http.Request) string {
	if s.RequestBaseURL != nil {
		return s.RequestBaseURL(r)
	}
	return ""
}

// HandleIndex serves the site root without imposing an authentication gate.
func (s Surface) HandleIndex(w http.ResponseWriter, r *http.Request) {
	var session *websessionpkg.Session
	if c, err := r.Cookie(websessionpkg.CookieName); err == nil {
		session = s.WebSessions.Lookup(c.Value)
	}
	var ownerEmail string
	var chains []oauthpkg.AgentChain
	if session != nil {
		ownerEmail = session.OwnerEmail()
		chains = s.OAuthTokens.LiveAgentChains(ownerEmail, s.OAuthClients)
	}
	webpkg.WriteIndex(w, webpkg.IndexData{
		Count:       s.Counter.Read(),
		SignedIn:    session != nil,
		OwnerEmail:  ownerEmail,
		AgentChains: chains,
		BaseURL:     s.requestBaseURL(r),
		Version:     s.Version,
	})
}
