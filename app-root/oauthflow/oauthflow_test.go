package oauthflow_test

import (
	"database/sql"
	"encoding/json"
	"io"
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
	websessionpkg "github.com/mgreenly/hal/websession"

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

func TestR_MTRN_DL9W_web_origin_records_have_origin_web_and_nil_mcp_context(t *testing.T) {
	f := newAuthorizeFixture(t, testClientID, testRedirect)
	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()
	f.surface.HandleLogin(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode < 300 || res.StatusCode >= 400 {
		t.Fatalf("login status = %d, want 3xx (R-MTRN-DL9W setup)",
			res.StatusCode)
	}
	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v (R-MTRN-DL9W)", err)
	}
	state := loc.Query().Get("state")
	if state == "" {
		t.Fatalf("Location missing state= (R-MTRN-DL9W setup)")
	}
	stateRec, ok := f.states.Snapshot(state)
	if !ok {
		t.Fatalf("state %q not recorded (R-MTRN-DL9W)", state)
	}
	if stateRec.Origin() != "web" {
		t.Fatalf("origin = %q, want %q (R-MTRN-DL9W)", stateRec.Origin(), "web")
	}
	if mcpCtx := stateRec.MCPContext(); mcpCtx != nil {
		t.Fatalf("web-origin record carries non-nil mcp context = %+v "+
			"(R-MTRN-DL9W: web records require no extra context)",
			*mcpCtx)
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

// R-5LQM-O89D: the service is configured at deploy time with the
// single Google Workspace domain whose users are allowed. A Google
// identity whose hosted-domain claim is outside that domain is
// rejected at the federation step with a clear error and no token /
// web session is issued. The fake IDP returns the constant hosted
// domain "example.com"; we drive the callback under a configured
// allow-domain of "allowed.example.org" so the check rejects.
func TestR_5LQM_O89D_callback_rejects_off_domain_identity(t *testing.T) {
	f := newCallbackFixture("allowed.example.org")

	loginReq := httptest.NewRequest("GET", "/login", nil)
	loginRec := httptest.NewRecorder()
	f.surface.HandleLogin(loginRec, loginReq)
	loginRes := loginRec.Result()
	defer loginRes.Body.Close()
	var bindingID string
	for _, c := range loginRes.Cookies() {
		if c.Name == oauthpkg.StateCookieName {
			bindingID = c.Value
		}
	}
	loc, err := url.Parse(loginRes.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	state := loc.Query().Get("state")
	if state == "" || bindingID == "" {
		t.Fatalf("login did not produce state/binding "+
			"(state=%q bindingID=%q)", state, bindingID)
	}

	target := "/oauth/google/callback?state=" + url.QueryEscape(state) +
		"&code=fake"
	cbReq := httptest.NewRequest("GET", target, nil)
	cbReq.AddCookie(&http.Cookie{Name: oauthpkg.StateCookieName, Value: bindingID})
	cbRec := httptest.NewRecorder()
	f.surface.HandleGoogleCallback(cbRec, cbReq)
	cbRes := cbRec.Result()
	defer cbRes.Body.Close()

	if cbRes.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for off-domain identity "+
			"(R-5LQM-O89D)", cbRes.StatusCode)
	}
	body, _ := io.ReadAll(cbRes.Body)
	lb := strings.ToLower(string(body))
	if !strings.Contains(lb, "domain") && !strings.Contains(lb, "workspace") {
		t.Fatalf("error body must name the cause (R-5LQM-O89D): %q", body)
	}
}

// R-5LQM-O89D: when the Google-asserted hosted domain matches the
// configured Workspace domain, the callback proceeds past the
// federation check and produces the placeholder success response.
// (Web-session establishment lands under R-CXJ2-R3BN.) The configured
// domain ("example.com") matches the fake IDP's constant HostedDomain
// claim, so the success path is exercised without any environment
// plumbing.
func TestR_5LQM_O89D_callback_accepts_in_domain_identity(t *testing.T) {
	f := newCallbackFixture("example.com")

	loginReq := httptest.NewRequest("GET", "/login", nil)
	loginRec := httptest.NewRecorder()
	f.surface.HandleLogin(loginRec, loginReq)
	loginRes := loginRec.Result()
	defer loginRes.Body.Close()
	var bindingID string
	for _, c := range loginRes.Cookies() {
		if c.Name == oauthpkg.StateCookieName {
			bindingID = c.Value
		}
	}
	loc, _ := url.Parse(loginRes.Header.Get("Location"))
	state := loc.Query().Get("state")

	target := "/oauth/google/callback?state=" + url.QueryEscape(state) +
		"&code=fake"
	cbReq := httptest.NewRequest("GET", target, nil)
	cbReq.AddCookie(&http.Cookie{Name: oauthpkg.StateCookieName, Value: bindingID})
	cbRec := httptest.NewRecorder()
	f.surface.HandleGoogleCallback(cbRec, cbReq)
	cbRes := cbRec.Result()
	defer cbRes.Body.Close()

	if cbRes.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(cbRes.Body)
		t.Fatalf("status = %d, want 303 for in-domain identity "+
			"(R-5LQM-O89D / R-CXJ2-R3BN); body=%q",
			cbRes.StatusCode, body)
	}
	if loc := cbRes.Header.Get("Location"); loc != "/" {
		t.Fatalf("Location = %q, want %q after in-domain callback "+
			"(R-CXJ2-R3BN)", loc, "/")
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

type callbackFixture struct {
	surface     oauthflowpkg.Surface
	webSessions *websessionpkg.Store
}

func newCallbackFixture(workspaceDomain string) callbackFixture {
	states := oauthpkg.NewStateStore(oauthpkg.StateOptions{
		TTL: func() time.Duration { return testOAuthTTL },
	})
	webSessions := websessionpkg.New(websessionpkg.Options{
		AbsoluteTTL: func() time.Duration { return 12 * time.Hour },
		IdleTTL:     func() time.Duration { return time.Hour },
	})
	return callbackFixture{
		webSessions: webSessions,
		surface: oauthflowpkg.Surface{
			GoogleIDP:             googleidppkg.FakeProvider{},
			OAuthStates:           states,
			OAuthAuthCodes:        oauthpkg.NewAuthCodeStore(oauthpkg.AuthCodeOptions{}),
			WebSessions:           webSessions,
			OAuthStateTTL:         func() time.Duration { return testOAuthTTL },
			WebSessionAbsoluteTTL: func() time.Duration { return 12 * time.Hour },
			WorkspaceDomain:       func() string { return workspaceDomain },
			RequestBaseURL:        func(*http.Request) string { return testBaseURL },
			WriteOAuthError:       jsonapipkg.WriteOAuthError,
		},
	}
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
