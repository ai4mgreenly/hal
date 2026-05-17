package siteindex_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	counterpkg "github.com/mgreenly/hal/counter"
	oauthpkg "github.com/mgreenly/hal/oauth"
	siteindexpkg "github.com/mgreenly/hal/siteindex"
	webpkg "github.com/mgreenly/hal/web"
	websessionpkg "github.com/mgreenly/hal/websession"
)

var (
	testCounter          = counterpkg.New()
	testWebSessionStore  = websessionpkg.New(websessionpkg.Options{})
	testOAuthTokenStore  = oauthpkg.NewTokenStore(oauthpkg.TokenOptions{})
	testOAuthClientStore = oauthpkg.NewClientStore()
)

func handleTestIndex(w http.ResponseWriter, r *http.Request) {
	siteindexpkg.Surface{
		Counter:        testCounter,
		WebSessions:    testWebSessionStore,
		OAuthTokens:    testOAuthTokenStore,
		OAuthClients:   testOAuthClientStore,
		RequestBaseURL: testRequestBaseURL,
		Version:        "0.0.1",
	}.HandleIndex(w, r)
}

func testRequestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fp := r.Header.Get("X-Forwarded-Proto"); fp != "" {
		first, _, _ := strings.Cut(fp, ",")
		switch strings.ToLower(strings.TrimSpace(first)) {
		case "http", "https":
			scheme = strings.ToLower(strings.TrimSpace(first))
		}
	}
	return scheme + "://" + r.Host
}

// R-8KKV-TDWF: the index page presents a banner card with the chrome
// the design reference pins — lens dot (decorative, aria-hidden), tag
// "MCP Demo", title "HAL", subtitle row carrying one entry from the
// R-G47S-05R3 bank followed inline by a re-roll control rendered as a
// <button> (NOT an <a>) with aria-label="New subtitle", and the
// page's auth area in the banner's bottom-right. The canonical
// stylesheet R-8MP8-6B77 serves is linked from <head> so the page
// styles itself by the designer's file. Structural assertions
// verifiable against the server-rendered HTML; activation behavior
// (the cross-fade swap and the no-page-reload property) lives in the
// inline script and is not exercised by the Go test surface.
func TestR_8KKV_TDWF_index_renders_banner_card(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handleTestIndex(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (R-8KKV-TDWF)", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body,
		`<link rel="stylesheet" href="/design.css">`) {
		t.Errorf("body missing canonical stylesheet link "+
			"(R-8KKV-TDWF / R-8MP8-6B77): %q", body)
	}
	if !strings.Contains(body, `class="banner"`) {
		t.Errorf("body missing banner card element class "+
			"(R-8KKV-TDWF): %q", body)
	}
	if !regexp.MustCompile(
		`<span class="lens"[^>]*aria-hidden="true"`).MatchString(body) {
		t.Errorf("body missing decorative lens dot with "+
			"aria-hidden=\"true\" (R-8KKV-TDWF): %q", body)
	}
	if !regexp.MustCompile(
		`<span class="tag"[^>]*>MCP Demo</span>`).MatchString(body) {
		t.Errorf("body missing tag span with text \"MCP Demo\" "+
			"(R-8KKV-TDWF): %q", body)
	}
	if !regexp.MustCompile(
		`<h1 class="title"[^>]*>HAL 9000</h1>`).MatchString(body) {
		t.Errorf("body missing title <h1 class=\"title\">HAL 9000</h1> "+
			"(R-8KKV-TDWF): %q", body)
	}
	subtitleRe := regexp.MustCompile(
		`<span class="subtitle"[^>]*>([^<]*)</span>`)
	m := subtitleRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("body missing subtitle span (R-8KKV-TDWF): %q", body)
	}
	inBank := false
	for _, s := range webpkg.SubtitleBank() {
		if s == m[1] {
			inBank = true
			break
		}
	}
	if !inBank {
		t.Errorf("subtitle text %q is not an entry from webpkg.SubtitleBank() "+
			"(R-8KKV-TDWF / R-G47S-05R3)", m[1])
	}
	// Re-roll control: a <button> (NOT an <a>) with class refresh and
	// aria-label="New subtitle". The spec is explicit that this is
	// rendered as a non-navigating control.
	refreshRe := regexp.MustCompile(
		`<button[^>]*class="refresh"[^>]*aria-label="New subtitle"`)
	refreshReAlt := regexp.MustCompile(
		`<button[^>]*aria-label="New subtitle"[^>]*class="refresh"`)
	if !refreshRe.MatchString(body) && !refreshReAlt.MatchString(body) {
		t.Errorf("body missing re-roll <button class=\"refresh\" "+
			"aria-label=\"New subtitle\"> (R-8KKV-TDWF): %q", body)
	}
	if regexp.MustCompile(
		`<a[^>]*aria-label="New subtitle"`).MatchString(body) {
		t.Errorf("re-roll control is rendered as an <a> — it must "+
			"be a non-navigating <button> per R-8KKV-TDWF: %q", body)
	}
	// Banner auth area: the auth affordance lives inside the banner
	// card (R-8KKV-TDWF's "anchored to the bottom-right of the banner
	// card" property), wrapped in .banner-auth.
	if !strings.Contains(body, `class="banner-auth"`) {
		t.Errorf("body missing banner-auth area inside banner card "+
			"(R-8KKV-TDWF): %q", body)
	}
}

// R-BZQY-DN3B: the index page displays MCP client configuration for
// two clients — Claude Code and Claude Desktop — each with its own
// copy-pasteable instructions that include the request-derived base
// URL and no Google details, no client credentials, and no service-
// internal paths beyond the base URL + transport endpoint. The tab-
// interface presentation (R-H4LJ-G9HR), the Claude Code section's
// stacked scope-block structure (R-G5FO-DXHS), and the per-client
// snippet format (R-5GQZ-KWCD) are separate requirements; this test
// pins only R-BZQY-DN3B's "both clients are present, each with a
// copy-pasteable snippet that names the base URL, and no forbidden
// material is exposed" property.
func TestR_BZQY_DN3B_index_displays_mcp_client_config(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	host := "hal." + "example" + ".test"
	req.Host = host
	handleTestIndex(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (R-BZQY-DN3B)", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "Claude Code") {
		t.Errorf("body missing \"Claude Code\" client label (R-BZQY-DN3B): %q", body)
	}
	if !strings.Contains(body, "Claude Desktop") {
		t.Errorf("body missing \"Claude Desktop\" client label (R-BZQY-DN3B): %q", body)
	}

	expectedBase := "http://" + host
	if !strings.Contains(body, expectedBase) {
		t.Errorf("body missing request-derived base URL %q (R-BZQY-DN3B / R-CO4Y-11X7): %q",
			expectedBase, body)
	}

	// Locate the MCP-instructions area and assert each client's panel
	// names the base URL inside its own copy-pasteable snippet, not
	// only somewhere else on the page.
	areaRe := regexp.MustCompile(
		`(?s)<article[^>]*class="section"[^>]*aria-label="MCP client connect snippets"[^>]*>(.*?)</article>`)
	areaMatch := areaRe.FindStringSubmatch(body)
	if areaMatch == nil {
		t.Fatalf("body missing <section class=\"mcp-instructions\"> wrapper (R-BZQY-DN3B): %q",
			body)
	}
	area := areaMatch[1]

	codeRe := regexp.MustCompile(
		`(?s)data-client="([^"]+)">.*?<pre[^>]*class="[^"]*\bcode\b[^"]*">(.*?)<button class="copy"`)
	matches := codeRe.FindAllStringSubmatch(area, -1)
	seen := map[string]string{}
	for _, m := range matches {
		seen[m[1]] = m[2]
	}
	for _, client := range []string{"claude-code", "claude-desktop"} {
		snippet, ok := seen[client]
		if !ok {
			t.Errorf("MCP instructions area missing copy-pasteable snippet for "+
				"data-client=%q (R-BZQY-DN3B): %q", client, area)
			continue
		}
		if !strings.Contains(snippet, expectedBase) {
			t.Errorf("snippet for %q does not include base URL %q (R-BZQY-DN3B): %q",
				client, expectedBase, snippet)
		}
	}

	// Forbidden material: no Google details, no client credentials.
	for _, forbidden := range []string{
		"google", "Google",
		"client_secret", "client_id",
		"accounts." + "google" + ".com",
		"google" + "apis.com",
	} {
		if strings.Contains(area, forbidden) {
			t.Errorf("MCP instructions area contains forbidden token %q (R-BZQY-DN3B): %q",
				forbidden, area)
		}
	}
}

// R-5GQZ-KWCD: each client's instructions are in the format that the
// client itself documents for adding an HTTP-transport MCP server, so
// a user can paste them directly without translation. For Claude
// Code, that's `claude mcp add --transport http <name> <url>` (with
// optional `--scope <scope>`); for Claude Desktop, the
// `claude_desktop_config.json` `mcpServers` block.
func TestR_5GQZ_KWCD_mcp_snippets_in_client_documented_format(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	host := "hal." + "example" + ".test"
	req.Host = host
	handleTestIndex(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (R-5GQZ-KWCD)", rr.Code)
	}
	body := rr.Body.String()

	areaRe := regexp.MustCompile(
		`(?s)<article[^>]*class="section"[^>]*aria-label="MCP client connect snippets"[^>]*>(.*?)</article>`)
	areaMatch := areaRe.FindStringSubmatch(body)
	if areaMatch == nil {
		t.Fatalf("body missing <section class=\"mcp-instructions\"> wrapper (R-5GQZ-KWCD)")
	}
	area := areaMatch[1]

	codeRe := regexp.MustCompile(
		`(?s)data-client="([^"]+)">.*?<pre[^>]*class="[^"]*\bcode\b[^"]*">(.*?)<button class="copy"`)
	snippets := map[string]string{}
	for _, m := range codeRe.FindAllStringSubmatch(area, -1) {
		snippets[m[1]] = m[2]
	}

	mcpURL := "http://" + host + "/mcp"

	// Claude Code: the documented `claude mcp add` form. The CLI's
	// positional argument order is `<name> <url>`; the transport flag
	// is required for an HTTP-transport server.
	cc, ok := snippets["claude-code"]
	if !ok {
		t.Fatalf("missing claude-code snippet (R-5GQZ-KWCD)")
	}
	ccRe := regexp.MustCompile(
		`^claude mcp add --transport http(?: --scope (?:project|user|local))? hal ` +
			regexp.QuoteMeta(mcpURL) + `$`)
	if !ccRe.MatchString(strings.TrimSpace(cc)) {
		t.Errorf("claude-code snippet not in documented `claude mcp add --transport http "+
			"[--scope <scope>] <name> <url>` form (R-5GQZ-KWCD): %q", cc)
	}

	// Claude Desktop: a valid JSON document whose `mcpServers` block
	// names `hal` with the HTTP transport URL, paste-and-go into
	// claude_desktop_config.json.
	cd, ok := snippets["claude-desktop"]
	if !ok {
		t.Fatalf("missing claude-desktop snippet (R-5GQZ-KWCD)")
	}
	var parsed struct {
		MCPServers map[string]struct {
			URL  string `json:"url"`
			Type string `json:"type"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(cd), &parsed); err != nil {
		t.Fatalf("claude-desktop snippet is not valid JSON (R-5GQZ-KWCD): %v\n%q", err, cd)
	}
	if parsed.MCPServers == nil {
		t.Fatalf("claude-desktop snippet missing top-level `mcpServers` key (R-5GQZ-KWCD): %q", cd)
	}
	entry, ok := parsed.MCPServers["hal"]
	if !ok {
		t.Fatalf("claude-desktop snippet's mcpServers has no `hal` entry (R-5GQZ-KWCD): %q", cd)
	}
	if entry.URL != mcpURL {
		t.Errorf("claude-desktop `hal` entry URL = %q, want %q (R-5GQZ-KWCD)",
			entry.URL, mcpURL)
	}
	if len(parsed.MCPServers) != 1 {
		t.Errorf("claude-desktop mcpServers has %d entries, want exactly 1 named `hal` "+
			"(R-5GQZ-KWCD): %q", len(parsed.MCPServers), cd)
	}
}

// R-G5FO-DXHS: the Claude Code section card renders its two scope
// examples as two stacked scope blocks (project first, then user),
// each with its own pill label and its own code block. Both are
// visible simultaneously on page load; the two scope commands are
// not fused into a single code block, and the structure is not a
// sub-tab interface inside the Claude Code panel.
func TestR_G5FO_DXHS_claude_code_panel_has_two_stacked_scope_blocks(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	host := "hal." + "example" + ".test"
	req.Host = host
	handleTestIndex(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (R-G5FO-DXHS)", rr.Code)
	}
	body := rr.Body.String()

	// Isolate the Claude Code client panel.
	panelRe := regexp.MustCompile(
		`(?s)<div[^>]*class="[^"]*\bclient-panel\b[^"]*"` +
			`[^>]*data-client="claude-code"[^>]*>(.*?)</div>` +
			`\s*<div[^>]*class="[^"]*\bclient-panel\b`)
	pm := panelRe.FindStringSubmatch(body)
	if pm == nil {
		t.Fatalf("body missing Claude Code client-panel (R-G5FO-DXHS): %q", body)
	}
	panel := pm[1]

	// Two stacked scope blocks: project first, user second.
	scopeRe := regexp.MustCompile(
		`(?s)<div[^>]*class="scope-block"[^>]*data-scope="([^"]+)"[^>]*>(.*?)</div>`)
	scopes := scopeRe.FindAllStringSubmatch(panel, -1)
	if len(scopes) != 2 {
		t.Fatalf("Claude Code panel has %d scope-block elements, want exactly 2 "+
			"(R-G5FO-DXHS): %q", len(scopes), panel)
	}
	if scopes[0][1] != "project" || scopes[1][1] != "user" {
		t.Errorf("scope-block order = [%s, %s], want [project, user] (R-G5FO-DXHS)",
			scopes[0][1], scopes[1][1])
	}

	expectedURL := "http://" + host + "/mcp"
	expected := map[string]string{
		"project": "claude mcp add --transport http --scope project hal " + expectedURL,
		"user":    "claude mcp add --transport http --scope user hal " + expectedURL,
	}
	pillRe := regexp.MustCompile(`(?s)<[^>]+class="scope-pill"[^>]*>([^<]+)<`)
	codeRe := regexp.MustCompile(
		`(?s)<pre[^>]*class="[^"]*\bcode\b[^"]*">(.*?)<button class="copy"`)
	for _, m := range scopes {
		scope, inner := m[1], m[2]
		// Pill label literally bears the scope's name.
		pm := pillRe.FindStringSubmatch(inner)
		if pm == nil {
			t.Errorf("scope-block %q missing scope-pill label (R-G5FO-DXHS): %q", scope, inner)
			continue
		}
		if strings.TrimSpace(pm[1]) != scope {
			t.Errorf("scope-block %q pill text = %q, want %q (R-G5FO-DXHS)",
				scope, pm[1], scope)
		}
		// Each scope-block carries its own `<pre class="code">`
		// with the matching command. Each block contains exactly
		// one code block — not fused with the other scope's
		// command.
		cms := codeRe.FindAllStringSubmatch(inner, -1)
		if len(cms) != 1 {
			t.Errorf("scope-block %q has %d code blocks, want 1 (R-G5FO-DXHS): %q",
				scope, len(cms), inner)
			continue
		}
		if strings.TrimSpace(cms[0][1]) != expected[scope] {
			t.Errorf("scope-block %q command = %q, want %q (R-G5FO-DXHS)",
				scope, cms[0][1], expected[scope])
		}
		// No nested sub-tab interface: no tab triggers / tabpanel
		// roles inside the scope-blocks.
		if strings.Contains(inner, `role="tab"`) || strings.Contains(inner, `role="tablist"`) {
			t.Errorf("scope-block %q contains a sub-tab interface; the spec "+
				"forbids nesting a second row of tabs inside the Claude Code "+
				"panel (R-G5FO-DXHS): %q", scope, inner)
		}
	}

	// Both blocks visible on initial render: neither carries the
	// `hidden` attribute, and neither carries `aria-hidden="true"`.
	for _, m := range scopes {
		scope := m[1]
		// Look at the opening tag only (up to the first `>`).
		openRe := regexp.MustCompile(`<div[^>]*data-scope="` + scope + `"[^>]*>`)
		open := openRe.FindString(panel)
		if strings.Contains(open, " hidden") || strings.Contains(open, `aria-hidden="true"`) {
			t.Errorf("scope-block %q is hidden on initial render; both must be "+
				"visible simultaneously (R-G5FO-DXHS): %q", scope, open)
		}
	}
}

// R-H4LJ-G9HR: the MCP client instructions area is structured as a
// functional two-tab interface — Claude Code (`01`) and Claude
// Desktop (`02`) — with exactly one panel visible at a time. Both
// panels are present in the rendered HTML on initial load; the
// inactive panel carries `hidden`. The tab triggers are <button>
// elements (not navigating <a> elements) wired with the WAI-ARIA tab
// pattern (`role="tablist"`, `role="tab"`, `role="tabpanel"`,
// `aria-selected`, `aria-controls`, `aria-labelledby`). Default
// active: Claude Code. Each trigger carries its numeric badge, its
// literal client title, and a per-client instruction sentence. Every
// code block in the area has a visible `copy` affordance.
func TestR_H4LJ_G9HR_mcp_client_instructions_is_tab_interface(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "hal." + "example" + ".test"
	handleTestIndex(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (R-H4LJ-G9HR)", rr.Code)
	}
	body := rr.Body.String()

	areaRe := regexp.MustCompile(
		`(?s)<article[^>]*class="section"[^>]*aria-label="MCP client connect snippets"[^>]*>(.*?)</article>`)
	am := areaRe.FindStringSubmatch(body)
	if am == nil {
		t.Fatalf("mcp-instructions wrapper missing (R-H4LJ-G9HR)")
	}
	area := am[1]

	// Tablist with two tabs in the area; each trigger is a <button>
	// (not an <a>) carrying role="tab".
	if !regexp.MustCompile(`<div[^>]*role="tablist"`).MatchString(area) {
		t.Errorf("mcp-instructions area has no role=\"tablist\" container "+
			"(R-H4LJ-G9HR): %q", area)
	}
	tabRe := regexp.MustCompile(
		`(?s)<button[^>]*class="[^"]*\bclient-tab\b[^"]*"[^>]*data-target="([^"]+)"[^>]*>(.*?)</button>`)
	tabs := tabRe.FindAllStringSubmatch(area, -1)
	if len(tabs) != 2 {
		t.Fatalf("found %d client-tab buttons, want 2 (R-H4LJ-G9HR)", len(tabs))
	}
	if tabs[0][1] != "claude-code" || tabs[1][1] != "claude-desktop" {
		t.Errorf("tab order = [%s, %s], want [claude-code, claude-desktop] (R-H4LJ-G9HR)",
			tabs[0][1], tabs[1][1])
	}

	// Each trigger contains the numeric badge, the client title,
	// and a per-client instruction sentence.
	wantBadge := map[string]string{"claude-code": "01", "claude-desktop": "02"}
	wantTitle := map[string]string{
		"claude-code":    "Claude Code",
		"claude-desktop": "Claude Desktop",
	}
	wantHint := map[string]string{
		"claude-code":    "Run the following command.",
		"claude-desktop": "Add the following JSON to your claude_desktop_config.json",
	}
	for _, m := range tabs {
		client, inner := m[1], m[2]
		fullTag := m[0]
		if !strings.Contains(fullTag, `role="tab"`) {
			t.Errorf("client-tab for %q missing role=\"tab\" (R-H4LJ-G9HR): %q",
				client, fullTag)
		}
		if !strings.Contains(inner, wantBadge[client]) {
			t.Errorf("client-tab for %q missing numeric badge %q (R-H4LJ-G9HR): %q",
				client, wantBadge[client], inner)
		}
		if !strings.Contains(inner, wantTitle[client]) {
			t.Errorf("client-tab for %q missing literal title %q (R-H4LJ-G9HR): %q",
				client, wantTitle[client], inner)
		}
		// R-H4LJ-G9HR allows the instruction sentence either inside the
		// trigger or as the first element of the panel body; R-UBYN-1LY0
		// pins it to the panel body. Assert it appears somewhere in the
		// instructions area.
		if !strings.Contains(area, wantHint[client]) {
			t.Errorf("mcp-instructions area missing %q instruction for %q (R-H4LJ-G9HR): area=%q",
				wantHint[client], client, area)
		}
	}

	// Triggers are <button> elements, never <a href>.
	if regexp.MustCompile(`<a[^>]*\brole="tab"`).MatchString(area) {
		t.Errorf("client-tab rendered as <a> (would navigate); must be <button> "+
			"(R-H4LJ-G9HR): %q", area)
	}

	// Default active trigger: Claude Code carries aria-selected="true"
	// (and the "active" class); Claude Desktop carries
	// aria-selected="false".
	ccTag := tabs[0][0]
	cdTag := tabs[1][0]
	if !strings.Contains(ccTag, `aria-selected="true"`) {
		t.Errorf("Claude Code tab not aria-selected=\"true\" on first render "+
			"(R-H4LJ-G9HR): %q", ccTag)
	}
	if !strings.Contains(ccTag, `class="client-tab active"`) {
		t.Errorf("Claude Code tab missing active class on first render "+
			"(R-H4LJ-G9HR): %q", ccTag)
	}
	if !strings.Contains(cdTag, `aria-selected="false"`) {
		t.Errorf("Claude Desktop tab is aria-selected on first render "+
			"(R-H4LJ-G9HR): %q", cdTag)
	}

	// Panels: both data-client divs carry role="tabpanel" and
	// aria-labelledby pointing at their tab's id; aria-controls on the
	// tab points back at the panel id. Exactly one panel is visible —
	// the Claude Desktop panel carries `hidden`, the Claude Code panel
	// does not.
	panelRe := regexp.MustCompile(
		`(?s)<div([^>]*)data-client="([^"]+)">`)
	panels := panelRe.FindAllStringSubmatch(area, -1)
	if len(panels) != 2 {
		t.Fatalf("found %d data-client panels, want 2 (R-H4LJ-G9HR)", len(panels))
	}
	gotHidden := map[string]bool{}
	for _, m := range panels {
		attrs, client := m[1], m[2]
		if !strings.Contains(attrs, `role="tabpanel"`) {
			t.Errorf("panel for %q missing role=\"tabpanel\" (R-H4LJ-G9HR): %q",
				client, attrs)
		}
		if !strings.Contains(attrs, `aria-labelledby="tab-`+client+`"`) {
			t.Errorf("panel for %q missing aria-labelledby=\"tab-%s\" (R-H4LJ-G9HR): %q",
				client, client, attrs)
		}
		gotHidden[client] = strings.Contains(attrs, " hidden")
	}
	if gotHidden["claude-code"] {
		t.Errorf("Claude Code panel is hidden on first render; it should be the " +
			"default active panel (R-H4LJ-G9HR)")
	}
	if !gotHidden["claude-desktop"] {
		t.Errorf("Claude Desktop panel is not hidden on first render; exactly " +
			"one panel must be visible (R-H4LJ-G9HR)")
	}

	// aria-controls on each tab names the matching panel id.
	for _, m := range tabs {
		client, full := m[1], m[0]
		want := `aria-controls="panel-` + client + `"`
		if !strings.Contains(full, want) {
			t.Errorf("tab for %q missing %s (R-H4LJ-G9HR): %q",
				client, want, full)
		}
	}

	// Every `<pre class="code">` code block in the area exposes a
	// visible `copy` affordance. The R-G5FO-DXHS Claude Code panel
	// has two (project + user); the Claude Desktop panel has one.
	// So three code blocks, three copy buttons.
	codes := regexp.MustCompile(
		`<pre[^>]*class="[^"]*\bcode\b[^"]*"`).FindAllString(area, -1)
	copies := regexp.MustCompile(
		`<button[^>]*class="[^"]*\bcopy\b`).FindAllString(area, -1)
	if len(codes) < 1 {
		t.Fatalf("mcp-instructions area has no code blocks (R-H4LJ-G9HR)")
	}
	if len(copies) != len(codes) {
		t.Errorf("found %d code blocks but %d copy buttons; every code block "+
			"must have its own copy affordance (R-H4LJ-G9HR)",
			len(codes), len(copies))
	}
}

// R-GVMQ-ZCBQ: the index page renders a counter card with the chrome
// R-CO4Y-11X7: the base URL in the MCP client configuration snippets
// is derived from the request the visitor used to reach the page —
// two requests at distinct Host values render distinct snippet URLs,
// and neither is a hard-coded literal. The forwarded-proto half of
// the request-derived posture is covered by R-DA34-WX9P.
func TestR_CO4Y_11X7_mcp_snippets_url_is_request_derived(t *testing.T) {
	areaRe := regexp.MustCompile(
		`(?s)<article[^>]*class="section"[^>]*aria-label="MCP client connect snippets"[^>]*>(.*?)</article>`)
	codeRe := regexp.MustCompile(
		`(?s)data-client="([^"]+)">.*?<pre[^>]*class="[^"]*\bcode\b[^"]*">(.*?)<button class="copy"`)

	render := func(host string) map[string]string {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		handleTestIndex(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (R-CO4Y-11X7)", rr.Code)
		}
		areaMatch := areaRe.FindStringSubmatch(rr.Body.String())
		if areaMatch == nil {
			t.Fatalf("body missing mcp-instructions wrapper (R-CO4Y-11X7)")
		}
		out := map[string]string{}
		for _, m := range codeRe.FindAllStringSubmatch(areaMatch[1], -1) {
			out[m[1]] = m[2]
		}
		return out
	}

	hostA := "hal." + "example" + ".test"
	hostB := "alt." + "example" + ".test:8443"
	snippetsA := render(hostA)
	snippetsB := render(hostB)

	for _, client := range []string{"claude-code", "claude-desktop"} {
		a, okA := snippetsA[client]
		b, okB := snippetsB[client]
		if !okA || !okB {
			t.Fatalf("missing snippet for %q (R-CO4Y-11X7)", client)
		}
		wantA := "http://" + hostA + "/mcp"
		wantB := "http://" + hostB + "/mcp"
		if !strings.Contains(a, wantA) {
			t.Errorf("snippet for %q at host %q missing %q (R-CO4Y-11X7): %q",
				client, hostA, wantA, a)
		}
		if !strings.Contains(b, wantB) {
			t.Errorf("snippet for %q at host %q missing %q (R-CO4Y-11X7): %q",
				client, hostB, wantB, b)
		}
		if strings.Contains(a, hostB) {
			t.Errorf("snippet for %q at host %q leaks host %q (R-CO4Y-11X7): %q",
				client, hostA, hostB, a)
		}
		if strings.Contains(b, hostA) {
			t.Errorf("snippet for %q at host %q leaks host %q (R-CO4Y-11X7): %q",
				client, hostB, hostA, b)
		}
	}
}

// the design reference pins — label "CURRENT COUNT", the current
// counter value in a monospaced display, and the canonical .icon-btn
// −/+ buttons carrying aria-label="Decrement" / "Increment". A hint
// line below the card explains MCP capability (rendered identically
// regardless of session state). With no web session wired yet, the
// buttons render visibly disabled via the HTML disabled attribute;
// the visual disabled treatment is supplied by .icon-btn:disabled in
// the canonical stylesheet.
func TestR_GVMQ_ZCBQ_index_renders_counter_card(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handleTestIndex(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (R-GVMQ-ZCBQ)", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, `class="counter-card"`) {
		t.Errorf("body missing counter-card section (R-GVMQ-ZCBQ): %q", body)
	}
	if !regexp.MustCompile(
		`<div class="counter-label"[^>]*>CURRENT COUNT</div>`).MatchString(body) {
		t.Errorf("body missing CURRENT COUNT label (R-GVMQ-ZCBQ): %q", body)
	}
	if !regexp.MustCompile(
		`<div class="counter-value"[^>]*>\d+</div>`).MatchString(body) {
		t.Errorf("body missing counter-value with numeric content "+
			"(R-GVMQ-ZCBQ): %q", body)
	}

	decRe := regexp.MustCompile(
		`<button[^>]*class="icon-btn"[^>]*aria-label="Decrement"[^>]*disabled`)
	decReAlt := regexp.MustCompile(
		`<button[^>]*aria-label="Decrement"[^>]*class="icon-btn"[^>]*disabled`)
	if !decRe.MatchString(body) && !decReAlt.MatchString(body) {
		t.Errorf("body missing disabled Decrement icon-btn (R-GVMQ-ZCBQ): %q", body)
	}

	incRe := regexp.MustCompile(
		`<button[^>]*class="icon-btn"[^>]*aria-label="Increment"[^>]*disabled`)
	incReAlt := regexp.MustCompile(
		`<button[^>]*aria-label="Increment"[^>]*class="icon-btn"[^>]*disabled`)
	if !incRe.MatchString(body) && !incReAlt.MatchString(body) {
		t.Errorf("body missing disabled Increment icon-btn (R-GVMQ-ZCBQ): %q", body)
	}

	if !strings.Contains(body,
		"Authenticated agents using MCP can read &amp; mutate this counter on your behalf.") {
		t.Errorf("body missing MCP capability hint line (R-GVMQ-ZCBQ): %q", body)
	}
}

// TestR_G0K2_UUJ0_index_motion_and_aria pins R-G0K2-UUJ0: the index page
// honors prefers-reduced-motion (via an inline @media block that suppresses
// the lens-dot pulse, the subtitle fade-swap, the counter-value flash, the
// delta animation, and hover-driven transforms on the interactive
// controls) and exposes the accessible structure the spec enumerates —
// tablist/tab/tabpanel on the MCP-client tabs, aria-label on the counter
// buttons and re-roll button, aria-live="polite" on the counter value,
// and aria-hidden on the decorative lens dot and footer status dot.
func TestR_G0K2_UUJ0_index_motion_and_aria(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handleTestIndex(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (R-G0K2-UUJ0)", rr.Code)
	}
	body := rr.Body.String()

	// Reduced-motion @media block + each suppression the spec enumerates.
	if !strings.Contains(body, "@media (prefers-reduced-motion: reduce)") {
		t.Fatalf("body missing prefers-reduced-motion @media block "+
			"(R-G0K2-UUJ0): %q", body)
	}
	suppressions := []struct {
		name, needle string
	}{
		{"lens pulse", ".lens{animation:none"},
		{"subtitle fade-swap", ".subtitle,.subtitle.swap{transition:none"},
		{"counter flash", ".counter-value,.counter-value.flash{transition:none"},
		{"delta animation", ".delta,.delta.show"},
		{"re-roll hover transform", ".refresh"},
		{"icon-btn hover transform", ".icon-btn"},
		{"copy hover/copied", ".copy"},
		{"client-tab transition", ".client-tab"},
	}
	for _, s := range suppressions {
		if !strings.Contains(body, s.needle) {
			t.Errorf("reduced-motion block missing %s suppression (%q) "+
				"(R-G0K2-UUJ0)", s.name, s.needle)
		}
	}

	// ARIA semantics on the MCP-client tab pattern.
	ariaTabs := []string{
		`role="tablist"`,
		`role="tab"`,
		`role="tabpanel"`,
		`aria-selected="true"`,
		`aria-selected="false"`,
		`aria-controls="panel-claude-code"`,
		`aria-controls="panel-claude-desktop"`,
		`aria-labelledby="tab-claude-code"`,
		`aria-labelledby="tab-claude-desktop"`,
	}
	for _, a := range ariaTabs {
		if !strings.Contains(body, a) {
			t.Errorf("tab pattern missing %s (R-G0K2-UUJ0)", a)
		}
	}

	// aria-label on the counter buttons and the re-roll button.
	for _, label := range []string{
		`aria-label="Increment"`,
		`aria-label="Decrement"`,
		`aria-label="New subtitle"`,
	} {
		if !strings.Contains(body, label) {
			t.Errorf("body missing %s (R-G0K2-UUJ0)", label)
		}
	}

	// aria-live="polite" on the counter value.
	if !regexp.MustCompile(
		`<div class="counter-value"[^>]*aria-live="polite"[^>]*>\d+</div>`,
	).MatchString(body) {
		t.Errorf("counter-value missing aria-live=\"polite\" "+
			"(R-G0K2-UUJ0): %q", body)
	}

	// aria-hidden on the decorative lens dot. The footer status dot is
	// drawn by the canonical `footer .status::before` pseudo-element
	// (reqs/design.css 485-491), so no DOM element carries an
	// aria-hidden marker for it — pseudo-elements are not in the
	// accessibility tree by default. (R-MCHV-YEO4 rename.)
	if !strings.Contains(body, `<span class="lens" aria-hidden="true">`) {
		t.Errorf("lens dot missing aria-hidden (R-G0K2-UUJ0)")
	}
}

// R-GUEU-LKL1: the index page reflects web-session state. With a live
// hal_session cookie the bottom-right of the banner shows the recorded
// owner email verbatim alongside a distinct Sign out control whose
// activation reaches /logout, and the counter card's −/+ buttons drop
// their HTML `disabled` attribute. With no live session the page shows
// a Sign in affordance that reaches /login, renders no anonymous
// placeholder identity, and keeps the −/+ buttons visibly disabled.
func TestR_GUEU_LKL1_index_reflects_web_session_state(t *testing.T) {
	t.Run("signed_in_visitor_sees_email_and_signout_and_enabled_buttons", func(t *testing.T) {
		plaintext, err := testWebSessionStore.Issue("dave@discovery.one")
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: websessionpkg.CookieName, Value: plaintext})
		rec := httptest.NewRecorder()
		handleTestIndex(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (R-GUEU-LKL1)", rec.Code)
		}
		body := rec.Body.String()

		// Email rendered verbatim inside the banner-auth area.
		bannerAuthRe := regexp.MustCompile(
			`<div class="banner-auth">[\s\S]*?dave@discovery\.one[\s\S]*?</div>`)
		if !bannerAuthRe.MatchString(body) {
			t.Errorf("banner-auth missing owner email (R-GUEU-LKL1): %q", body)
		}

		// A separate, explicitly labeled Sign out control that reaches /logout.
		signOutRe := regexp.MustCompile(
			`<form[^>]*method="post"[^>]*action="/logout"[^>]*>[\s\S]*?` +
				`<button[^>]*>Sign out</button>[\s\S]*?</form>`)
		if !signOutRe.MatchString(body) {
			t.Errorf("body missing Sign out form posting to /logout "+
				"(R-GUEU-LKL1): %q", body)
		}

		// No /login affordance in the signed-in state.
		if strings.Contains(body, `href="/login"`) {
			t.Errorf("signed-in page still exposes /login affordance "+
				"(R-GUEU-LKL1): %q", body)
		}

		// Counter buttons drop the disabled attribute.
		decDisabled := regexp.MustCompile(
			`<button[^>]*aria-label="Decrement"[^>]*disabled`)
		incDisabled := regexp.MustCompile(
			`<button[^>]*aria-label="Increment"[^>]*disabled`)
		if decDisabled.MatchString(body) {
			t.Errorf("Decrement button still HTML-disabled for signed-in "+
				"visitor (R-GUEU-LKL1): %q", body)
		}
		if incDisabled.MatchString(body) {
			t.Errorf("Increment button still HTML-disabled for signed-in "+
				"visitor (R-GUEU-LKL1): %q", body)
		}
	})

	t.Run("signed_out_visitor_sees_signin_and_no_placeholder_identity", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handleTestIndex(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (R-GUEU-LKL1)", rec.Code)
		}
		body := rec.Body.String()

		signInRe := regexp.MustCompile(
			`<div class="banner-auth">[\s\S]*?` +
				`<a[^>]*href="/login"[^>]*>Sign in</a>[\s\S]*?</div>`)
		if !signInRe.MatchString(body) {
			t.Errorf("body missing /login affordance in banner-auth "+
				"(R-GUEU-LKL1): %q", body)
		}
		if strings.Contains(body, "guest") || strings.Contains(body, "Guest") {
			t.Errorf("body renders a placeholder identity for anonymous "+
				"visitor (R-GUEU-LKL1): %q", body)
		}
		if strings.Contains(body, `action="/logout"`) {
			t.Errorf("body exposes /logout affordance with no session "+
				"(R-GUEU-LKL1): %q", body)
		}
	})

	t.Run("revoked_or_unknown_session_is_treated_as_signed_out", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  websessionpkg.CookieName,
			Value: "not-a-real-session",
		})
		rec := httptest.NewRecorder()
		handleTestIndex(rec, req)
		body := rec.Body.String()
		if !strings.Contains(body, `href="/login"`) {
			t.Errorf("unknown session cookie should fall back to /login "+
				"affordance (R-GUEU-LKL1): %q", body)
		}
		decDisabled := regexp.MustCompile(
			`<button[^>]*aria-label="Decrement"[^>]*disabled`)
		if !decDisabled.MatchString(body) {
			t.Errorf("Decrement button must remain disabled for unknown "+
				"session (R-GUEU-LKL1): %q", body)
		}
	})
}

// R-UBYN-1LY0: each .client-tab button contains exactly two visible
// elements — the .num chip ("01" / "02") and the client's name as a
// bare text node. The label is NOT wrapped in any inner element with
// a class of its own. No subtitle, no hint, no secondary line lives
// inside the tab trigger; content describing what the panel will
// show lives inside the matching .client-panel body.
func TestR_UBYN_1LY0_client_tab_inner_markup(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handleTestIndex(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (R-UBYN-1LY0)", rr.Code)
	}
	body := rr.Body.String()

	areaRe := regexp.MustCompile(
		`(?s)<article[^>]*class="section"[^>]*aria-label="MCP client connect snippets"[^>]*>(.*?)</article>`)
	am := areaRe.FindStringSubmatch(body)
	if am == nil {
		t.Fatalf("mcp-instructions wrapper missing (R-UBYN-1LY0)")
	}
	area := am[1]

	tabRe := regexp.MustCompile(
		`(?s)<button[^>]*class="[^"]*\bclient-tab\b[^"]*"[^>]*data-target="([^"]+)"[^>]*>(.*?)</button>`)
	tabs := tabRe.FindAllStringSubmatch(area, -1)
	if len(tabs) != 2 {
		t.Fatalf("found %d client-tab buttons, want 2 (R-UBYN-1LY0)", len(tabs))
	}

	wantName := map[string]string{
		"claude-code":    "Claude Code",
		"claude-desktop": "Claude Desktop",
	}
	wantNum := map[string]string{
		"claude-code":    "01",
		"claude-desktop": "02",
	}
	// Sentences that must NOT appear inside the tab trigger — they
	// belong in the panel body.
	bannedInTab := []string{
		"Run the following command.",
		"Add the following JSON to your claude_desktop_config.json",
	}

	chipRe := regexp.MustCompile(
		`^<span class="num">([^<]+)</span>(.*)$`)
	tagRe := regexp.MustCompile(`<[^>]+>`)

	for _, m := range tabs {
		client, inner := m[1], m[2]
		trimmed := strings.TrimSpace(inner)

		cm := chipRe.FindStringSubmatch(trimmed)
		if cm == nil {
			t.Fatalf("client-tab %q inner does not start with a single "+
				"<span class=\"num\">…</span> chip (R-UBYN-1LY0): %q",
				client, inner)
		}
		if cm[1] != wantNum[client] {
			t.Errorf("client-tab %q .num chip = %q, want %q (R-UBYN-1LY0)",
				client, cm[1], wantNum[client])
		}

		// After the chip, the only remaining content must be the
		// client's name as a bare text node — no further tags, no
		// inner element with a class wrapping the label.
		rest := strings.TrimSpace(cm[2])
		if tagRe.MatchString(rest) {
			t.Errorf("client-tab %q has additional element(s) after the "+
				".num chip; the client name must be a bare text node "+
				"with no wrapping class (R-UBYN-1LY0): rest=%q",
				client, rest)
		}
		if rest != wantName[client] {
			t.Errorf("client-tab %q label = %q, want bare text %q (R-UBYN-1LY0)",
				client, rest, wantName[client])
		}

		for _, ban := range bannedInTab {
			if strings.Contains(inner, ban) {
				t.Errorf("client-tab %q contains instruction text %q; it "+
					"must live inside the matching .client-panel body, "+
					"not the tab trigger (R-UBYN-1LY0): inner=%q",
					client, ban, inner)
			}
		}
	}

	// The instruction sentences live inside the matching panel body.
	// Use FindAllStringSubmatchIndex so each panel body extends to the
	// start of the next data-client div, or end-of-area for the last.
	startRe := regexp.MustCompile(
		`<div[^>]*data-client="([^"]+)"[^>]*>`)
	locs := startRe.FindAllStringSubmatchIndex(area, -1)
	panels := map[string]string{}
	for i, loc := range locs {
		client := area[loc[2]:loc[3]]
		bodyStart := loc[1]
		bodyEnd := len(area)
		if i+1 < len(locs) {
			bodyEnd = locs[i+1][0]
		}
		panels[client] = area[bodyStart:bodyEnd]
	}
	wantPanelHint := map[string]string{
		"claude-code":    "Run the following command.",
		"claude-desktop": "Add the following JSON to your claude_desktop_config.json",
	}
	for client, want := range wantPanelHint {
		body := panels[client]
		if body == "" {
			t.Errorf("no panel body captured for %q (R-UBYN-1LY0)", client)
			continue
		}
		if !strings.Contains(body, want) {
			t.Errorf(".client-panel body for %q missing instruction %q "+
				"(R-UBYN-1LY0): body=%q", client, want, body)
		}
	}
}

// R-8031-9QQ9: the banner card's on-page title is the literal
// `HAL 9000`. R-1ZS0-XSZ7 separately pins the <title> element to
// the short form `HAL`; the two must not be conflated. The auth
// area inside the banner is wrapped in `.banner-auth`.
func TestR_8031_9QQ9_banner_title_is_hal_9000(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handleTestIndex(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (R-8031-9QQ9)", rr.Code)
	}
	body := rr.Body.String()
	if !regexp.MustCompile(
		`<h1 class="title"[^>]*>HAL 9000</h1>`).MatchString(body) {
		t.Errorf("banner title is not the literal `HAL 9000` "+
			"(R-8031-9QQ9): %q", body)
	}
	if !strings.Contains(body, `class="banner-auth"`) {
		t.Errorf("banner missing .banner-auth wrapper "+
			"(R-8031-9QQ9): %q", body)
	}
}

// R-1ZS0-XSZ7: the rendered HTML document's <title> element carries
// the literal short-form text `HAL` — distinct from R-8031-9QQ9's
// on-page `HAL 9000` banner heading. The spec explicitly enumerates
// `HAL 9000`, `HAL · MCP Demo`, and the empty string as failure
// modes.
func TestR_1ZS0_XSZ7_document_title_is_short_form(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handleTestIndex(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (R-1ZS0-XSZ7)", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `<title>HAL</title>`) {
		t.Errorf("<title> is not the literal short form `HAL` "+
			"(R-1ZS0-XSZ7): %q", body)
	}
	for _, bad := range []string{
		`<title>HAL 9000</title>`,
		`<title></title>`,
		`<title>HAL · MCP Demo</title>`,
	} {
		if strings.Contains(body, bad) {
			t.Errorf("<title> contains forbidden variant %q "+
				"(R-1ZS0-XSZ7): %q", bad, body)
		}
	}
}

// R-0WB7-RV1W: the index page's auth area lives inside the banner card,
// right-aligned in the lower portion. Signed-out state shows a single
// pill-chrome `Sign in` control reaching /login; signed-in state shows
// the visitor's bare email as inert non-interactive text (no avatar /
// initials chip / monogram badge) plus a separate, explicitly labeled
// pill-chrome `Sign out` control reaching /logout. The email itself is
// not clickable (not wrapped in <a> or <button>, no onclick) — sign-out
// is a distinct, separately-labelled element from identity.
//
// This test extends what R-GUEU-LKL1 pins (presence of email + a Sign
// out form posting to /logout) by adding the placement-inside-banner-
// card property, the no-avatar property, the inert-email property, and
// the pill-chrome property on both states. The hover-inversion visual
// property is pinned by the existing visual-fidelity card-chrome test
// for `.auth-btn:hover` (design tokens), not duplicated here.
func TestR_0WB7_RV1W_banner_auth_placement_and_shape(t *testing.T) {
	// Extracts the inner contents of <section class="banner">...</section>
	// from a rendered index page. The banner section is the first
	// child of <main class="page"> and there is exactly one of them.
	bannerInner := func(t *testing.T, body string) string {
		t.Helper()
		re := regexp.MustCompile(
			`<section class="banner">([\s\S]*?)</section>`)
		m := re.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("body has no <section class=\"banner\">…</section> "+
				"(R-0WB7-RV1W): %q", body)
		}
		return m[1]
	}

	t.Run("signed_out_pill_sign_in_inside_banner_card", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handleTestIndex(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (R-0WB7-RV1W)", rec.Code)
		}
		body := rec.Body.String()
		inner := bannerInner(t, body)

		// .banner-auth lives INSIDE the banner card.
		if !strings.Contains(inner, `class="banner-auth"`) {
			t.Errorf("banner-auth wrapper not inside <section "+
				"class=\"banner\"> (R-0WB7-RV1W): banner inner = %q",
				inner)
		}
		// And nowhere else: exactly one .banner-auth in the page, and
		// it is the one inside the banner.
		if got := strings.Count(body, `class="banner-auth"`); got != 1 {
			t.Errorf("body has %d .banner-auth occurrences, want 1 "+
				"(R-0WB7-RV1W): %q", got, body)
		}

		// Pill-chrome Sign in affordance reaching /login. The pill
		// chrome is realized by the `.auth-btn` class (the rule the
		// reduced-motion override at main.go:2086 names, and the
		// hover-inversion rule keys off the same selector). A bare
		// text link without `.auth-btn` would not satisfy the
		// "pill chrome on either control" property.
		signInRe := regexp.MustCompile(
			`<a [^>]*class="auth-btn"[^>]*href="/login"[^>]*>Sign in</a>`)
		signInReAlt := regexp.MustCompile(
			`<a [^>]*href="/login"[^>]*class="auth-btn"[^>]*>Sign in</a>`)
		if !signInRe.MatchString(inner) && !signInReAlt.MatchString(inner) {
			t.Errorf("signed-out banner-auth missing pill-chrome "+
				"Sign in control with class=\"auth-btn\" reaching "+
				"/login (R-0WB7-RV1W): banner inner = %q", inner)
		}

		// No Sign out anywhere when signed out.
		if strings.Contains(body, "Sign out") {
			t.Errorf("signed-out page renders a Sign out affordance "+
				"(R-0WB7-RV1W): %q", body)
		}
		// No avatar / initials chip / monogram badge anywhere.
		assertNoAvatarChipForBannerAuth(t, body)
	})

	t.Run("signed_in_inert_email_plus_distinct_pill_sign_out", func(t *testing.T) {
		email := "dave@discovery.one"
		plaintext, err := testWebSessionStore.Issue(email)
		if err != nil {
			t.Fatalf("webSessionStore.issue: %v (R-0WB7-RV1W)", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  websessionpkg.CookieName,
			Value: plaintext,
		})
		rec := httptest.NewRecorder()
		handleTestIndex(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (R-0WB7-RV1W)", rec.Code)
		}
		body := rec.Body.String()
		inner := bannerInner(t, body)

		// .banner-auth inside the banner card.
		if !strings.Contains(inner, `class="banner-auth"`) {
			t.Errorf("banner-auth wrapper not inside <section "+
				"class=\"banner\"> (R-0WB7-RV1W): banner inner = %q",
				inner)
		}
		if got := strings.Count(body, `class="banner-auth"`); got != 1 {
			t.Errorf("body has %d .banner-auth occurrences, want 1 "+
				"(R-0WB7-RV1W): %q", got, body)
		}

		// Locate the .banner-auth block contents.
		authRe := regexp.MustCompile(
			`<div class="banner-auth">([\s\S]*?)</div>\s*</section>`)
		am := authRe.FindStringSubmatch(body)
		if am == nil {
			t.Fatalf("could not extract .banner-auth contents "+
				"(R-0WB7-RV1W): %q", body)
		}
		authInner := am[1]

		// Email rendered verbatim.
		if !strings.Contains(authInner, email) {
			t.Errorf("banner-auth missing verbatim email %q "+
				"(R-0WB7-RV1W): authInner = %q", email, authInner)
		}

		// Email is rendered as inert, non-interactive text — not
		// wrapped in <a>, not inside a <button>, no onclick. Find
		// the rendered email occurrence and inspect the surrounding
		// tag.
		idx := strings.Index(authInner, email)
		if idx < 0 {
			t.Fatalf("email not found in authInner (R-0WB7-RV1W)")
		}
		before := authInner[:idx]
		// Inspect the innermost open tag preceding the email.
		openTagRe := regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)\b[^>]*>$`)
		// Strip any trailing whitespace and find the last open tag
		// just before the email.
		lastOpen := openTagRe.FindStringSubmatch(strings.TrimRight(before, " \t\n"))
		if lastOpen == nil {
			// Fallback: scan all open tags before idx and take the
			// last one.
			allOpens := regexp.MustCompile(
				`<([a-zA-Z][a-zA-Z0-9]*)\b[^>]*>`).
				FindAllStringSubmatch(before, -1)
			if len(allOpens) == 0 {
				t.Fatalf("could not locate enclosing tag for email "+
					"(R-0WB7-RV1W): before = %q", before)
			}
			lastOpen = allOpens[len(allOpens)-1]
		}
		enclosingTag := strings.ToLower(lastOpen[1])
		if enclosingTag == "a" || enclosingTag == "button" {
			t.Errorf("email is wrapped in interactive <%s> tag — "+
				"R-0WB7-RV1W requires inert non-interactive text "+
				"for the identity display: authInner = %q",
				enclosingTag, authInner)
		}
		// No onclick / href attribute on the email's enclosing tag.
		enclosingOpen := lastOpen[0]
		if strings.Contains(strings.ToLower(enclosingOpen), "onclick") {
			t.Errorf("email's enclosing tag carries onclick handler "+
				"— identity display must be inert (R-0WB7-RV1W): "+
				"tag = %q", enclosingOpen)
		}
		if strings.Contains(strings.ToLower(enclosingOpen), " href=") {
			t.Errorf("email's enclosing tag carries href — identity "+
				"display must be non-navigating (R-0WB7-RV1W): "+
				"tag = %q", enclosingOpen)
		}

		// A separate, distinct, pill-chrome Sign out control reaching
		// /logout. It must be a different element from the identity
		// display (the email span/text). The regex pins (a) the
		// .auth-btn class on the button, (b) Sign out literal text,
		// (c) a form posting to /logout that wraps the button.
		signOutRe := regexp.MustCompile(
			`<form[^>]*method="post"[^>]*action="/logout"[^>]*>` +
				`[\s\S]*?<button[^>]*class="auth-btn"[^>]*>Sign out</button>` +
				`[\s\S]*?</form>`)
		if !signOutRe.MatchString(authInner) {
			t.Errorf("signed-in banner-auth missing pill-chrome "+
				"(class=\"auth-btn\") Sign out button inside a "+
				"<form action=\"/logout\"> (R-0WB7-RV1W): "+
				"authInner = %q", authInner)
		}

		// The email and the Sign out control are distinct sibling
		// elements: the email rendering must end before the Sign out
		// form begins. (i.e. "click your name to sign out" — the
		// email is inside the form/button — is forbidden.)
		emailEnd := strings.Index(authInner, email) + len(email)
		formStart := strings.Index(authInner, `action="/logout"`)
		if formStart >= 0 && formStart < emailEnd {
			t.Errorf("Sign out form encloses the identity display — "+
				"R-0WB7-RV1W requires distinct elements: "+
				"authInner = %q", authInner)
		}

		// No Sign in affordance when signed in.
		if strings.Contains(body, "Sign in") {
			t.Errorf("signed-in page still renders Sign in "+
				"affordance (R-0WB7-RV1W): %q", body)
		}
		if strings.Contains(body, `href="/login"`) {
			t.Errorf("signed-in page still exposes /login link "+
				"(R-0WB7-RV1W): %q", body)
		}

		// No avatar / initials chip anywhere.
		assertNoAvatarChipForBannerAuth(t, body)
	})
}

// assertNoAvatarChipForBannerAuth asserts that the rendered page does not
// expose any avatar / initials chip / monogram badge — R-0WB7-RV1W is
// explicit that the identity display is the bare email, no preceding
// circular initials chip, no monogram badge, no glyphic identity
// decoration. The design reference's `.avatar` element bearing the
// visitor's initials (e.g. `DV` for `dave@discovery.one`) is a named
// deviation the project does not render.
func assertNoAvatarChipForBannerAuth(t *testing.T, body string) {
	t.Helper()
	forbidden := []string{
		`class="avatar"`,
		`class="initials"`,
		`class="monogram"`,
		`class="identity-chip"`,
	}
	for _, needle := range forbidden {
		if strings.Contains(body, needle) {
			t.Errorf("body renders forbidden avatar/identity-chip "+
				"element %q (R-0WB7-RV1W): %q", needle, body)
		}
	}
}

// R-EJAP-XUSB pins the counter card directly below the banner. The
// card contains a `CURRENT COUNT` label, the counter value, a
// `−` button with aria-label="Decrement" and a `+` button with
// aria-label="Increment". When no web session is active both
// buttons carry the HTML `disabled` attribute (so .icon-btn:disabled
// supplies the ≈40% opacity / cursor:not-allowed treatment); when a
// web session IS active neither button is disabled. The hint
// `Authenticated agents using MCP can read & mutate this counter on
// your behalf.` is rendered inside the card (positioned below the
// counter value, left-aligned within the card's content area), and
// the hint text is identical in both auth states.
func TestR_EJAP_XUSB_counter_card_structure(t *testing.T) {
	cardInner := func(t *testing.T, body string) string {
		t.Helper()
		re := regexp.MustCompile(
			`<section class="counter-card">([\s\S]*?)</section>`)
		m := re.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("body has no <section class=\"counter-card\">…"+
				"</section> (R-EJAP-XUSB): %q", body)
		}
		return m[1]
	}

	const hint = `Authenticated agents using MCP can read &amp; mutate ` +
		`this counter on your behalf.`

	assertShape := func(t *testing.T, body string, signedIn bool) {
		t.Helper()
		// Exactly one counter card in the page.
		if got := strings.Count(body, `<section class="counter-card">`); got != 1 {
			t.Errorf("body has %d counter-card sections, want 1 "+
				"(R-EJAP-XUSB): %q", got, body)
		}
		// Counter card is directly below the banner — its opening tag
		// appears after </section> of the banner and before the next
		// named block (instructions head).
		bannerClose := strings.Index(body, `</section>`)
		cardOpen := strings.Index(body, `<section class="counter-card">`)
		if bannerClose < 0 || cardOpen < 0 || cardOpen < bannerClose {
			t.Errorf("counter card not placed directly below banner "+
				"(R-EJAP-XUSB): %q", body)
		}
		inner := cardInner(t, body)

		// Label.
		if !strings.Contains(inner, `<div class="counter-label">CURRENT COUNT</div>`) {
			t.Errorf("counter card missing `CURRENT COUNT` label "+
				"(R-EJAP-XUSB): inner = %q", inner)
		}
		// Counter value rendered with .counter-value.
		if !regexp.MustCompile(
			`<div class="counter-value"[^>]*>\s*\d+`).MatchString(inner) {
			t.Errorf("counter card missing .counter-value with a "+
				"numeric value (R-EJAP-XUSB): inner = %q", inner)
		}
		// Increment / decrement buttons exist with the canonical aria-labels.
		decRe := regexp.MustCompile(
			`<button [^>]*aria-label="Decrement"([^>]*)>`)
		incRe := regexp.MustCompile(
			`<button [^>]*aria-label="Increment"([^>]*)>`)
		decM := decRe.FindStringSubmatch(inner)
		incM := incRe.FindStringSubmatch(inner)
		if decM == nil {
			t.Fatalf("counter card missing aria-label=\"Decrement\" "+
				"button (R-EJAP-XUSB): inner = %q", inner)
		}
		if incM == nil {
			t.Fatalf("counter card missing aria-label=\"Increment\" "+
				"button (R-EJAP-XUSB): inner = %q", inner)
		}
		decHasDisabled := strings.Contains(decM[1], "disabled")
		incHasDisabled := strings.Contains(incM[1], "disabled")
		if signedIn {
			if decHasDisabled {
				t.Errorf("signed-in `-` button still HTML-disabled "+
					"(R-EJAP-XUSB): %q", decM[0])
			}
			if incHasDisabled {
				t.Errorf("signed-in `+` button still HTML-disabled "+
					"(R-EJAP-XUSB): %q", incM[0])
			}
		} else {
			if !decHasDisabled {
				t.Errorf("signed-out `-` button missing HTML disabled "+
					"attribute (R-EJAP-XUSB): %q", decM[0])
			}
			if !incHasDisabled {
				t.Errorf("signed-out `+` button missing HTML disabled "+
					"attribute (R-EJAP-XUSB): %q", incM[0])
			}
		}
		// Hint text appears inside the counter card (NOT as a sibling
		// after </section>).
		if !strings.Contains(inner, hint) {
			t.Errorf("counter card missing inside-card hint text "+
				"(R-EJAP-XUSB): inner = %q", inner)
		}
		// Hint must not also appear outside the card.
		if got := strings.Count(body, hint); got != 1 {
			t.Errorf("hint text appears %d times in body, want 1 "+
				"(must be inside counter card only, R-EJAP-XUSB): %q",
				got, body)
		}
		// Hint positioned below the counter value within the inner content area.
		valueIdx := strings.Index(inner, `<div class="counter-value"`)
		hintIdx := strings.Index(inner, hint)
		if valueIdx < 0 || hintIdx < 0 || hintIdx <= valueIdx {
			t.Errorf("hint must be positioned below the counter value "+
				"inside the card (R-EJAP-XUSB): inner = %q", inner)
		}
	}

	t.Run("signed_out_buttons_disabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handleTestIndex(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (R-EJAP-XUSB)", rec.Code)
		}
		assertShape(t, rec.Body.String(), false)
	})

	t.Run("signed_in_buttons_enabled_same_hint", func(t *testing.T) {
		plaintext, err := testWebSessionStore.Issue("dave@discovery.one")
		if err != nil {
			t.Fatalf("webSessionStore.issue: %v (R-EJAP-XUSB)", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  websessionpkg.CookieName,
			Value: plaintext,
		})
		rec := httptest.NewRecorder()
		handleTestIndex(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (R-EJAP-XUSB)", rec.Code)
		}
		assertShape(t, rec.Body.String(), true)
	})
}

// TestR_TEP7_Q6UT_signed_in_email_renders_as_inert_text pins that the
// externally sourced Google email shown in the signed-in auth row is escaped
// text, not interpreted as markup, script, attributes, or URLs.
func TestR_TEP7_Q6UT_signed_in_email_renders_as_inert_text(t *testing.T) {
	email := `eve"><img src=x onerror="alert('owned')">&<script>bad()</script>@example.com`
	sess, err := testWebSessionStore.Issue(email)
	if err != nil {
		t.Fatalf("webSessionStore.issue: %v (R-TEP7-Q6UT)", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: websessionpkg.CookieName, Value: sess})
	w := httptest.NewRecorder()
	handleTestIndex(w, req)
	body := w.Body.String()

	start := strings.Index(body, `<div class="banner-auth">`)
	if start < 0 {
		t.Fatalf("signed-in auth row missing (R-TEP7-Q6UT): %q", body)
	}
	rest := body[start:]
	end := strings.Index(rest, `</div>`)
	if end < 0 {
		t.Fatalf("signed-in auth row close missing (R-TEP7-Q6UT): %q", rest)
	}
	authRow := rest[:end+len(`</div>`)]

	for _, raw := range []string{`eve"><img`, `<script>bad()</script>`} {
		if strings.Contains(authRow, raw) {
			t.Fatalf("email rendered as raw markup %q in auth row "+
				"(R-TEP7-Q6UT): %q", raw, authRow)
		}
	}
	for _, escaped := range []string{
		`eve&quot;&gt;&lt;img src=x onerror=&quot;alert(&#39;owned&#39;)&quot;&gt;`,
		`&amp;`,
		`&lt;script&gt;bad()&lt;/script&gt;@example.com`,
	} {
		if !strings.Contains(authRow, escaped) {
			t.Errorf("escaped email fragment %q missing (R-TEP7-Q6UT): %q",
				escaped, authRow)
		}
	}
}

// TestR_A2L2_1NA1_signed_in_sign_out_is_post_form_without_href pins that
// the signed-in Sign out affordance works without JavaScript: it is a
// submit button inside a POST /logout form and exposes no navigable
// /logout href.
func TestR_A2L2_1NA1_signed_in_sign_out_is_post_form_without_href(t *testing.T) {
	sess, err := testWebSessionStore.Issue("form-signout@example.com")
	if err != nil {
		t.Fatalf("webSessionStore.issue: %v (R-A2L2-1NA1)", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: websessionpkg.CookieName, Value: sess})
	w := httptest.NewRecorder()
	handleTestIndex(w, req)
	body := w.Body.String()

	start := strings.Index(body, `<div class="banner-auth">`)
	if start < 0 {
		t.Fatalf("signed-in auth row missing (R-A2L2-1NA1): %q", body)
	}
	rest := body[start:]
	end := strings.Index(rest, `</section>`)
	if end < 0 {
		t.Fatalf("banner close missing after auth row (R-A2L2-1NA1): %q", rest)
	}
	authArea := rest[:end]

	formRe := regexp.MustCompile(
		`<form[^>]*method="post"[^>]*action="/logout"[^>]*>` +
			`[\s\S]*<button[^>]*class="auth-btn"[^>]*type="submit"[^>]*>` +
			`Sign out</button>[\s\S]*</form>`)
	if !formRe.MatchString(authArea) {
		t.Fatalf("Sign out is not a POST form submit control "+
			"(R-A2L2-1NA1): %q", authArea)
	}
	for _, forbidden := range []string{
		`href="/logout"`,
		`href='/logout'`,
		`onclick=`,
		`fetch('/logout'`,
		`fetch("/logout"`,
	} {
		if strings.Contains(authArea, forbidden) {
			t.Fatalf("Sign out exposes JS-only or navigable logout hook %q "+
				"(R-A2L2-1NA1): %q", forbidden, authArea)
		}
	}
}
