package oauthflow_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	googleidppkg "github.com/mgreenly/hal/googleidp"
	jsonapipkg "github.com/mgreenly/hal/jsonapi"
	oauthpkg "github.com/mgreenly/hal/oauth"
	oauthflowpkg "github.com/mgreenly/hal/oauthflow"

	_ "modernc.org/sqlite"
)

const (
	testBaseURL   = "http://127.0.0.1:3000"
	testResource  = testBaseURL + "/mcp"
	testRedirect  = "http://127.0.0.1/cb"
	testOAuthTTL  = 5 * time.Minute
	testClientID  = "oauthflow-test-client"
	googleAuthURL = "accounts.google.com"
)

func TestR_BAXT_SBU9_authorize_requires_code_flow_and_pkce(t *testing.T) {
	f := newAuthorizeFixture(t, testClientID, testRedirect)

	base := f.validAuthorizeValues()
	valid := f.driveAuthorize(base)
	if valid.Code != http.StatusSeeOther {
		t.Fatalf("valid authorize status = %d, want 303; body=%q (R-BAXT-SBU9)",
			valid.Code, valid.Body.String())
	}
	if loc := valid.Header().Get("Location"); !strings.Contains(loc, googleAuthURL) {
		t.Fatalf("valid authorize Location = %q, want Google redirect (R-BAXT-SBU9)",
			loc)
	}

	cases := []struct {
		name   string
		mutate func(url.Values)
	}{
		{"missing response_type", func(v url.Values) { v.Del("response_type") }},
		{"unsupported response_type", func(v url.Values) { v.Set("response_type", "token") }},
		{"missing code_challenge", func(v url.Values) { v.Del("code_challenge") }},
		{"empty code_challenge", func(v url.Values) { v.Set("code_challenge", "") }},
		{"missing code_challenge_method", func(v url.Values) { v.Del("code_challenge_method") }},
		{"empty code_challenge_method", func(v url.Values) { v.Set("code_challenge_method", "") }},
		{"unsupported code_challenge_method", func(v url.Values) { v.Set("code_challenge_method", "bogus") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := f.states.Count()
			v := cloneValues(base)
			tc.mutate(v)
			rec := f.driveAuthorize(v)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%q (R-BAXT-SBU9)",
					rec.Code, rec.Body.String())
			}
			if loc := rec.Header().Get("Location"); loc != "" {
				t.Fatalf("Location = %q, want no redirect (R-BAXT-SBU9)", loc)
			}
			if after := f.states.Count(); after != before {
				t.Fatalf("oauth state records changed from %d to %d on rejection (R-BAXT-SBU9)",
					before, after)
			}
		})
	}
}

func TestR_JTTZ_CG5J_authorize_pkce_requires_s256(t *testing.T) {
	f := newAuthorizeFixture(t, testClientID, testRedirect)
	base := f.validAuthorizeValues()
	base.Set("code_challenge", "R_JTTZ_CG5J_PKCE")

	for _, tc := range []struct {
		name   string
		mutate func(url.Values)
	}{
		{"plain", func(v url.Values) { v.Set("code_challenge_method", "plain") }},
		{"omitted", func(v url.Values) { v.Del("code_challenge_method") }},
		{"empty", func(v url.Values) { v.Set("code_challenge_method", "") }},
		{"other", func(v url.Values) { v.Set("code_challenge_method", "S384") }},
	} {
		t.Run("authorize_rejects_"+tc.name, func(t *testing.T) {
			before := f.states.Count()
			v := cloneValues(base)
			tc.mutate(v)
			rec := f.driveAuthorize(v)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%q (R-JTTZ-CG5J)",
					rec.Code, rec.Body.String())
			}
			if loc := rec.Header().Get("Location"); loc != "" {
				t.Fatalf("Location = %q, want no redirect (R-JTTZ-CG5J)", loc)
			}
			if after := f.states.Count(); after != before {
				t.Fatalf("oauth state records changed from %d to %d on rejection (R-JTTZ-CG5J)",
					before, after)
			}
		})
	}

	valid := f.driveAuthorize(base)
	if valid.Code != http.StatusSeeOther {
		t.Fatalf("S256 authorize status = %d, want 303; body=%q (R-JTTZ-CG5J)",
			valid.Code, valid.Body.String())
	}
}

func TestR_126C_AM1E_authorize_omits_forced_auth_params(t *testing.T) {
	f := newAuthorizeFixture(t, testClientID, testRedirect)
	v := f.validAuthorizeValues()
	v.Set("code_challenge", "R_126C_AM1E_PKCE")

	rec := f.driveAuthorize(v)
	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode < 300 || res.StatusCode >= 400 {
		t.Fatalf("status = %d, want a 3xx redirect (R-126C-AM1E)",
			res.StatusCode)
	}
	loc := res.Header.Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("Location %q unparseable: %v (R-126C-AM1E)", loc, err)
	}
	q := u.Query()
	if got := q.Get("prompt"); got == "login" || got == "consent" {
		t.Errorf("Location query has prompt=%q (R-126C-AM1E); MCP "+
			"authorization flow must omit prompt=login and "+
			"prompt=consent so Google may satisfy the request via "+
			"silent SSO", got)
	}
	if got := q.Get("max_age"); got == "0" {
		t.Errorf("Location query has max_age=0 (R-126C-AM1E); MCP " +
			"authorization flow must omit max_age=0 so Google may " +
			"satisfy the request via silent SSO")
	}
}

func TestR_4GRA_EGBY_authorize_rejects_mismatched_resource_at_issue_time(t *testing.T) {
	f := newAuthorizeFixture(t, testClientID, testRedirect)
	mismatched := testResource + "extra"

	base := f.validAuthorizeValues()
	base.Set("code_challenge", "R_4GRA_EGBY_PKCE")

	bad := cloneValues(base)
	bad.Set("resource", mismatched)
	badRec := f.driveAuthorize(bad)
	badRes := badRec.Result()
	defer badRes.Body.Close()
	if badRes.StatusCode != http.StatusBadRequest {
		t.Errorf("authorize mismatched resource status = %d, want 400 (R-4GRA-EGBY)",
			badRes.StatusCode)
	}
	if loc := badRes.Header.Get("Location"); loc != "" {
		t.Errorf("authorize mismatched resource Location = %q, want empty "+
			"(no redirect using offending value) (R-4GRA-EGBY)", loc)
	}
	var badDoc map[string]any
	_ = json.NewDecoder(badRes.Body).Decode(&badDoc)
	if got, _ := badDoc["error"].(string); got != "invalid_target" {
		t.Errorf("authorize mismatched resource error = %q, want %q (R-4GRA-EGBY)",
			got, "invalid_target")
	}

	ok := cloneValues(base)
	ok.Set("resource", testResource)
	okRec := f.driveAuthorize(ok)
	okRes := okRec.Result()
	defer okRes.Body.Close()
	if okRes.StatusCode < 300 || okRes.StatusCode >= 400 {
		t.Errorf("authorize canonical resource status = %d, want 3xx (R-4GRA-EGBY)",
			okRes.StatusCode)
	}
}

func TestR_YRMT_B7LZ_restarted_client_store_still_authorizes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hal.DB")
	db1 := openOAuthClientTestDB(t, dbPath)
	clients := oauthpkg.NewClientStore()
	if err := clients.Attach(db1); err != nil {
		t.Fatalf("attach first store: %v (R-YRMT-B7LZ)", err)
	}
	clients.Put("ryrmt-persisted-client", oauthpkg.NewClient(oauthpkg.ClientSpec{
		RedirectURIs: []string{"http://127.0.0.1/yrmt-callback"},
		ClientName:   "Restart Durable Client",
		IssuedAt:     1700000000,
	}))
	if err := db1.Close(); err != nil {
		t.Fatalf("close first db: %v (R-YRMT-B7LZ)", err)
	}

	db2 := openOAuthClientTestDB(t, dbPath)
	defer db2.Close()
	restarted := oauthpkg.NewClientStore()
	if err := restarted.Attach(db2); err != nil {
		t.Fatalf("attach restarted store: %v (R-YRMT-B7LZ)", err)
	}
	f := newAuthorizeFixtureWithClients(restarted)
	v := url.Values{}
	v.Set("client_id", "ryrmt-persisted-client")
	v.Set("redirect_uri", "http://127.0.0.1/yrmt-callback")
	v.Set("response_type", "code")
	v.Set("code_challenge", "R_YRMT_B7LZ_PKCE")
	v.Set("code_challenge_method", "S256")
	v.Set("resource", testResource)

	authRec := f.driveAuthorize(v)
	if authRec.Code != http.StatusSeeOther {
		t.Fatalf("authorize after restart status = %d, want 303; body=%q (R-YRMT-B7LZ)",
			authRec.Code, authRec.Body.String())
	}
	if loc := authRec.Header().Get("Location"); !strings.Contains(loc, googleAuthURL) {
		t.Fatalf("authorize after restart Location = %q, want Google redirect (R-YRMT-B7LZ)",
			loc)
	}
}

type authorizeFixture struct {
	surface oauthflowpkg.Surface
	states  *oauthpkg.StateStore
}

func newAuthorizeFixture(t testing.TB, clientID, redirectURI string) authorizeFixture {
	t.Helper()
	clients := oauthpkg.NewClientStore()
	clients.Put(clientID, oauthpkg.NewClient(oauthpkg.ClientSpec{
		RedirectURIs: []string{redirectURI},
		IssuedAt:     1700000000,
	}))
	return newAuthorizeFixtureWithClients(clients)
}

func newAuthorizeFixtureWithClients(clients *oauthpkg.ClientStore) authorizeFixture {
	states := oauthpkg.NewStateStore(oauthpkg.StateOptions{
		TTL: func() time.Duration { return testOAuthTTL },
	})
	return authorizeFixture{
		states: states,
		surface: oauthflowpkg.Surface{
			GoogleIDP:                   googleidppkg.FakeProvider{},
			OAuthStates:                 states,
			OAuthClients:                clients,
			OAuthStateTTL:               func() time.Duration { return testOAuthTTL },
			CanonicalResourceIdentifier: func() string { return testResource },
			RequestBaseURL:              func(*http.Request) string { return testBaseURL },
			WriteOAuthError:             jsonapipkg.WriteOAuthError,
		},
	}
}

func (f authorizeFixture) validAuthorizeValues() url.Values {
	v := url.Values{}
	v.Set("client_id", testClientID)
	v.Set("redirect_uri", testRedirect)
	v.Set("response_type", "code")
	v.Set("code_challenge", "R_BAXT_SBU9_PKCE")
	v.Set("code_challenge_method", "S256")
	v.Set("resource", testResource)
	return v
}

func (f authorizeFixture) driveAuthorize(v url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+v.Encode(), nil)
	rec := httptest.NewRecorder()
	f.surface.HandleOAuthAuthorize(rec, req)
	return rec
}

func cloneValues(in url.Values) url.Values {
	out := url.Values{}
	for key, vals := range in {
		out[key] = append([]string(nil), vals...)
	}
	return out
}

func openOAuthClientTestDB(t testing.TB, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
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
		t.Fatalf("create oauth_clients: %v", err)
	}
	return db
}
