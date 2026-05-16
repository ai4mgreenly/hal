// Package web owns HAL's server-rendered presentation.
package web

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"sort"
	"strconv"
	"strings"

	oauthpkg "github.com/mgreenly/hal/oauth"
)

// R-8MP8-6B77: the canonical stylesheet is embedded from a checked-in
// package-local copy and served directly so the rendered page is styled
// by the designer's file rather than a re-derived parallel. `//go:embed`
// cannot traverse above the module root, so the canonical source-of-truth
// at ../reqs/design.css is mirrored here; a drift test reads
// ../reqs/design.css and asserts byte-equality with this embed.
//
//go:embed design.css
var designCSS []byte

// CSSBytes returns the embedded canonical stylesheet bytes.
func CSSBytes() []byte {
	return designCSS
}

// R-G47S-05R3: the index page's subtitle is one entry chosen uniformly at
// random per page render from this fixed bank of acronym expansions. The
// list is canonical: prose elsewhere in the spec defers to this slice as
// the source of truth for which expansions are reachable. Order is the
// spec's listing order; selection is uniform random (not round-robin),
// and `PickSubtitle` is the only path through which a renderer obtains a
// value, so every entry is reachable.
var subtitleBank = []string{
	"Holistic Access Layer",
	"Human Augmentation Layer",
	"Heuristic Agent Liaison",
	"Home, APIs, Library",
	"Heuristically programmed ALgorithm",
	"Heuristically programmed ALgorithmic computer",
	"Helpful Autonomous Liaison",
	"Hyperlocal Agent Layer",
	"Host Agent Liaison",
	"Has Always Listened",
	"House Always Loses",
	"Hardware Abstraction Layer",
	"Hyperdimensional Access Layer",
	"Holistic Application Logic",
	"Highly Adaptive Listener",
	"Headless Agent Loop",
	"Hosted Action Library",
	"Hermetic Authorization Layer",
	"Hypertext Application Language",
	"High-Availability Lambda",
	"Heretical Automation Layer",
	"Hyper-tuned Agent Logic",
	"Handy Autoresponse Layer",
	"Hallucination Avoidance Layer",
	"Honest Assistant, Lately",
	"Halfway Awake Loop",
	"Homemade Agent Lab",
	"Heuristic Argument Linker",
}

// SubtitleBank returns the canonical subtitle bank.
func SubtitleBank() []string {
	return append([]string(nil), subtitleBank...)
}

// PickSubtitle returns one entry from subtitleBank chosen uniformly at
// random. math/rand/v2's default Source is seeded per-process from the
// runtime entropy pool, so successive process renders produce independent
// draws without explicit seeding.
func PickSubtitle() string {
	return subtitleBank[rand.IntN(len(subtitleBank))]
}

// IndexData is the presentation input for the server-rendered index page.
type IndexData struct {
	Count       uint64
	SignedIn    bool
	OwnerEmail  string
	AgentChains []oauthpkg.AgentChain
	BaseURL     string
	Version     string
}

func agentChainRenderedName(ch oauthpkg.AgentChain) string {
	if ch.ClientName == "" {
		return "undefined"
	}
	return ch.ClientName
}

func agentChainRenderedIDPrefix(ch oauthpkg.AgentChain) string {
	prefix := ch.ClientID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	return prefix
}

// SortAgentChainsByRenderedIdentity orders agent chains as rendered in
// the index page and agents stream.
func SortAgentChainsByRenderedIdentity(chains []oauthpkg.AgentChain) {
	// R-VWEX-WYWJ: agent rows below the signed-in web-session row sort for
	// scanning by rendered identity, case-insensitive, with the rendered
	// first-8 client_id prefix as the tie-breaker. Refresh rotations leave
	// both values unchanged, so they cannot move a row.
	sort.SliceStable(chains, func(i, j int) bool {
		leftName := strings.ToLower(agentChainRenderedName(chains[i]))
		rightName := strings.ToLower(agentChainRenderedName(chains[j]))
		if leftName != rightName {
			return leftName < rightName
		}
		leftPrefix := agentChainRenderedIDPrefix(chains[i])
		rightPrefix := agentChainRenderedIDPrefix(chains[j])
		if leftPrefix != rightPrefix {
			return leftPrefix < rightPrefix
		}
		return chains[i].ChainID < chains[j].ChainID
	})
}

// WriteIndex renders the current count and presentation chrome as HTML.
func WriteIndex(w http.ResponseWriter, data IndexData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// bankJSON is the subtitle bank embedded in the page so the re-roll
	// button can pick a fresh entry client-side without a page reload
	// (R-8KKV-TDWF's no-`GET /`, preserve-page-state property). The
	// canonical bank is the Go-side subtitleBank; this is the same
	// slice, serialized once per render.
	bankJSON, err := json.Marshal(subtitleBank)
	if err != nil {
		http.Error(w, "subtitle bank serialization failed",
			http.StatusInternalServerError)
		return
	}
	// R-GUEU-LKL1 / R-0WB7-RV1W: the index page reflects web-session
	// state. If the hal_session cookie resolves to a live record, the
	// bottom-right of the banner shows the visitor's email + a separate
	// Sign out control (R-0WB7-RV1W pins email as inert non-interactive
	// text, no avatar element, sign-out as a distinct pill-chrome
	// affordance reaching /logout); otherwise it shows a single pill-
	// chrome Sign in affordance reaching /login. The auth area lives
	// inside the banner card per R-0WB7-RV1W. The counter card's -/+
	// buttons drop their HTML `disabled` attribute when signed in
	// (R-GVMQ-ZCBQ pins the no-session "disabled" treatment).
	var bannerAuth, counterDisabled string
	if data.SignedIn {
		bannerAuth = `<div class="banner-auth">` +
			// R-TEP7-Q6UT: the Google email is externally sourced
			// identity text, so it is escaped before interpolation.
			`<span class="auth-email">` + htmlEscape(data.OwnerEmail) + `</span>` +
			// R-A2L2-1NA1: Sign out is a real form POST, not a JS-only
			// click handler or href, so it works when scripts are absent.
			`<form method="post" action="/logout" class="auth-form">` +
			`<button class="auth-btn" type="submit">Sign out</button>` +
			`</form>` +
			`</div>`
	} else {
		bannerAuth = `<div class="banner-auth">` +
			`<a class="auth-btn" href="/login">Sign in</a>` +
			`</div>`
		counterDisabled = " disabled"
	}
	// R-VTZ5-5FF5: when the signed-in visitor owns one or more live MCP
	// token chains, the index page renders an agents block inside the
	// banner card, immediately below the auth row. A "live" chain is
	// one with at least one un-revoked, un-expired refresh token; the
	// store-side helper filters by owner email so a chain owned by any
	// other email cannot surface here regardless of how the underlying
	// records are stored. R-TS71-XRW4: the block is omitted entirely for
	// signed-out visitors and for signed-in visitors whose live-chain
	// count is zero, so the banner card collapses to its compact auth
	// row instead of reserving vertical space for absent agent rows.
	var agentsBlock string
	if data.SignedIn {
		chains := append([]oauthpkg.AgentChain(nil), data.AgentChains...)
		SortAgentChainsByRenderedIdentity(chains)
		if len(chains) > 0 {
			var b strings.Builder
			b.WriteString(`<div class="agents-block" aria-label="Authenticated MCP agents">`)
			for _, ch := range chains {
				// R-VV71-J75U: per-row content is exactly two visible
				// elements left to right — one inert identity label
				// combining client_name (literal `undefined` when the DCR
				// client supplied none) with the parenthesised client_id
				// 8-char prefix, and a Revoke pill. The button form-submits
				// to R-D0XD-1YT0's chain-revoke endpoint scoped to this row.
				name := agentChainRenderedName(ch)
				idPrefix := agentChainRenderedIDPrefix(ch)
				b.WriteString(`<div class="agent-row" data-chain-id="`)
				b.WriteString(htmlEscape(ch.ChainID))
				// R-VV71-J75U: identity label is inert text with client_name
				// followed by parenthesised 8-char client_id prefix; Revoke
				// button carries class="auth-btn" for matching pill chrome.
				b.WriteString(`"><span class="agent-name">`)
				// R-10ZV-8OFH: DCR client metadata is untrusted; render
				// client_name as escaped inert text inside the agent row.
				b.WriteString(htmlEscape(name))
				b.WriteString(` (`)
				b.WriteString(htmlEscape(idPrefix))
				b.WriteString(`)</span><form method="post" action="/agents/revoke">`)
				b.WriteString(`<input type="hidden" name="chain_id" value="`)
				b.WriteString(htmlEscape(ch.ChainID))
				b.WriteString(`"><button class="auth-btn" type="submit">Revoke</button></form></div>`)
			}
			b.WriteString(`</div>`)
			agentsBlock = b.String()
		}
	}
	// R-6KK2-AAY0 / R-3RL1-IUP6: when live agent rows exist, they live
	// inside the same bottom-right auth grid as the visitor row, not in a
	// separate banner sibling. This lets the email label and every agent
	// label share one right-aligned label column, while Sign out and all
	// Revoke buttons share one action column.
	if data.SignedIn && agentsBlock != "" {
		bannerAuth = strings.TrimSuffix(bannerAuth, `</div>`) + agentsBlock + `</div>`
		agentsBlock = ""
	}
	// R-BZQY-DN3B: the index page displays MCP client configuration
	// for two clients (Claude Code and Claude Desktop), each with its
	// own copy-pasteable instructions. The base URL is request-
	// derived (R-CO4Y-11X7 / R-DA34-WX9P) so a visitor reaching the
	// service through a TLS-terminating proxy sees the public origin,
	// not the local plain-HTTP origin. No Google details, no client
	// credentials, and no service-internal paths beyond the base URL
	// + `/mcp` transport endpoint (consistent with R-VVRG-W2G2). The
	// tab-interface presentation pinned by R-H4LJ-G9HR and the scope-
	// block structure pinned by R-G5FO-DXHS are not implemented here
	// yet — both clients' panels are rendered side-by-side as a
	// minimal first step.
	//
	// R-5GQZ-KWCD: each snippet is in the format the client itself
	// documents for adding an HTTP-transport MCP server. Claude Code
	// uses the `claude mcp add --transport http [--scope <scope>]
	// <name> <url>` CLI form (verbatim from `claude mcp add --help`);
	// Claude Desktop uses the `claude_desktop_config.json`
	// `mcpServers` block. Both are paste-and-go without translation.
	mcpURL := htmlEscape(data.BaseURL + "/mcp")
	claudeCodeProjectCmd := `claude mcp add --transport http --scope project hal ` + mcpURL
	claudeCodeUserCmd := `claude mcp add --transport http --scope user hal ` + mcpURL
	claudeDesktopJSON := `{` + "\n" +
		`  "mcpServers": {` + "\n" +
		`    "hal": {` + "\n" +
		`      "url": "` + mcpURL + `"` + "\n" +
		`    }` + "\n" +
		`  }` + "\n" +
		`}`
	// R-H4LJ-G9HR: tab interface — two triggers (Claude Code 01,
	// Claude Desktop 02) above two mutually-exclusive panels. Both
	// panels rendered in the HTML; JS toggles which is visible. Each
	// trigger carries a numeric badge, the literal client title, and
	// a 13px right-aligned instruction sentence per client. ARIA tab
	// pattern wired (tablist / tab / tabpanel, aria-selected /
	// aria-controls / aria-labelledby). Default active: Claude Code.
	// Every code block in the area exposes a `copy` affordance.
	// R-MCHV-YEO4 (d): canonical shape per reqs/design.css §3 and
	// reqs/web.md 138-147. The client-tabs container renders inside
	// `<article class="section">` with a `.section-body` wrapping the
	// tabs and panels. `.mcp-instructions` is a forbidden shadow name.
	// R-9TPL-HQBV: the instructions head and the client tabs are
	// SEPARATE children of `<main class="page">`, not nested under a
	// single wrapper.
	// R-NBGD-KUHA: the instructions head (the `<h2>` reading "Connect
	// an MCP client" per R-H4LJ-G9HR) is NOT wrapped in its own card
	// chrome — the canonical CSS hook is `.instructions-head` (a bare
	// container with top margin providing the inter-section gap above
	// it and a small bottom margin providing the internal gap to the
	// tabs panel below). The `<h2>` carries no border, no rounded
	// background, no card-style fill — it is a heading, not a card.
	mcpInstructions := `<div class="instructions-head" aria-label="Connect an MCP client">` +
		`<h2>Connect an MCP client</h2>` +
		`</div>` +
		`<article class="section" aria-label="MCP client connect snippets">` +
		`<div class="section-body">` +
		`<div class="client-tabs" role="tablist" aria-label="MCP client">` +
		// R-UBYN-1LY0: each .client-tab contains exactly two visible
		// elements — the .num chip and the client name as a bare text
		// node. The per-client instruction sentence lives in the
		// matching .client-panel body (R-H4LJ-G9HR allows either
		// placement; we choose panel body to satisfy R-UBYN-1LY0).
		// R-MCHV-YEO4: the panel container is `.client-panel` per the
		// canonical reqs/design.css — `.mcp-client` is a forbidden
		// shadow name.
		`<button class="client-tab active" type="button" role="tab"` +
		` id="tab-claude-code" aria-selected="true" aria-controls="panel-claude-code"` +
		` data-target="claude-code">` +
		`<span class="num">01</span>Claude Code` +
		`</button>` +
		`<button class="client-tab" type="button" role="tab"` +
		` id="tab-claude-desktop" aria-selected="false" aria-controls="panel-claude-desktop"` +
		` tabindex="-1" data-target="claude-desktop">` +
		`<span class="num">02</span>Claude Desktop` +
		`</button>` +
		`</div>` +
		// R-772N-VHQE: Claude Code panel carries `.active` on first
		// render so `.client-panel.active` resolves visible per
		// reqs/design.css; Claude Desktop panel does not.
		`<div class="client-panel active" role="tabpanel"` +
		` id="panel-claude-code" aria-labelledby="tab-claude-code"` +
		` data-client="claude-code">` +
		`<p class="client-hint">Run the following command.</p>` +
		// R-G5FO-DXHS: two stacked scope blocks (project then user),
		// each with its own pill label and code block. Both visible
		// on initial render; not a sub-tab interface.
		// R-UBPK-DLTT: each dark code-block snippet is a single
		// element carrying the canonical `code` class
		// (`<pre class="code">`) so the `.code` rule in
		// reqs/design.css applies directly — no `code-wrap`,
		// `code-block`, or `snippet` shadow wrapper, no inline
		// position:relative override (`.code` already supplies it
		// for the absolutely-positioned `.copy` overlay). The
		// copy button's body is the clipboard `<svg>` glyph
		// (`.copy svg` is sized 12x12 in design.css), with the
		// `aria-label` carrying the visible affordance text.
		`<div class="scope-block" data-scope="project">` +
		`<span class="scope-pill">project</span>` +
		`<pre class="code">` + claudeCodeProjectCmd +
		`<button class="copy" type="button" aria-label="Copy to clipboard">` +
		`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true">` +
		`<rect x="9" y="9" width="13" height="13" rx="2"/>` +
		`<path d="M5 15V5a2 2 0 0 1 2-2h10"/>` +
		`</svg>` +
		`</button>` +
		`</pre>` +
		`</div>` +
		`<div class="scope-block" data-scope="user">` +
		`<span class="scope-pill">user</span>` +
		`<pre class="code">` + claudeCodeUserCmd +
		`<button class="copy" type="button" aria-label="Copy to clipboard">` +
		`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true">` +
		`<rect x="9" y="9" width="13" height="13" rx="2"/>` +
		`<path d="M5 15V5a2 2 0 0 1 2-2h10"/>` +
		`</svg>` +
		`</button>` +
		`</pre>` +
		`</div>` +
		`</div>` +
		`<div class="client-panel" role="tabpanel"` +
		` id="panel-claude-desktop" aria-labelledby="tab-claude-desktop"` +
		` hidden data-client="claude-desktop">` +
		`<p class="client-hint">Add the following JSON to your claude_desktop_config.json</p>` +
		`<pre class="code">` + claudeDesktopJSON +
		`<button class="copy" type="button" aria-label="Copy to clipboard">` +
		`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true">` +
		`<rect x="9" y="9" width="13" height="13" rx="2"/>` +
		`<path d="M5 15V5a2 2 0 0 1 2-2h10"/>` +
		`</svg>` +
		`</button>` +
		`</pre>` +
		`</div>` +
		`</div>` +
		`</article>`

	fmt.Fprintf(w,
		`<!doctype html>`+
			`<html lang="en"><head>`+
			`<meta charset="utf-8">`+
			`<title>HAL</title>`+
			`<link rel="stylesheet" href="/design.css">`+
			// R-G0K2-UUJ0: when the visitor's browser reports
			// prefers-reduced-motion: reduce, the index page suppresses
			// the lens-dot pulse, the subtitle fade-swap, the counter
			// flash and delta animation, and the hover-driven transforms
			// on the re-roll, copy, icon-btn, auth-btn, and client-tab
			// controls. Visual end-states still render; only the
			// transitions and the infinite pulse animation are removed.
			// The overrides live in an inline <style> block so the
			// canonical reqs/design.css stays byte-equal to the embedded
			// copy (R-8MP8-6B77 drift guard).
			`<style>@media (prefers-reduced-motion: reduce){`+
			`.lens{animation:none !important;}`+
			`.subtitle,.subtitle.swap{transition:none !important;}`+
			`.counter-value,.counter-value.flash{transition:none !important;}`+
			`.delta,.delta.show,.delta.minus{transition:none !important;}`+
			`.refresh,.refresh:hover,.refresh:active,`+
			`.icon-btn,.icon-btn:hover:not(:disabled),`+
			`.copy,.copy:hover,.copy.copied,`+
			`.auth-btn,.auth-btn:hover,`+
			`.client-tab,.client-tab.active`+
			`{transition:none !important;}`+
			`}</style>`+
			// R-G6NK-RP8H: the load-bearing layout property of the design
			// reference is a centered ~880px content column inside a single
			// .page wrapper. The .page rule in design.css supplies the
			// max-width and `margin: 0 auto`, so the <main> here must carry
			// the class for that rule to actually apply in the rendered
			// output. Without it the tokens are declared but the centered
			// container is not realized.
			// R-6KK2-AAY0 / R-2ZZH-LJYA / R-O87H-RSH4 / R-CNWX-9VB2 /
			// R-6QIE-4D71:
			// the agents block is an application-specific extension to
			// the canonical banner. Keep reqs/design.css byte-identical,
			// and layer these small layout rules inline so the signed-in
			// email row plus every agent row share one lower-right
			// label/action stack that remains in the banner's normal
			// flow. The flow placement and compact lower padding are
			// conditional on an actual .agents-block descendant, so
			// signed-out and zero-agent pages keep design.css's compact
			// absolutely-positioned .banner-auth treatment.
			`<style>`+
			`.banner:has(.agents-block){display:flex;flex-direction:column;align-items:center;padding-bottom:18px}`+
			`.banner:has(.agents-block) .banner-auth{display:grid;`+
			`grid-template-columns:max-content max-content;`+
			`align-items:center;justify-items:end;gap:8px 14px;text-align:right;`+
			`position:static;align-self:flex-end;margin-top:28px}`+
			`.banner:has(.agents-block) .banner-auth>.auth-email,`+
			`.banner:has(.agents-block) .banner-auth .agent-name{grid-column:1;`+
			`color:var(--ink-mute);font-size:13px}`+
			`.banner:has(.agents-block) .banner-auth>.auth-form,`+
			`.banner:has(.agents-block) .banner-auth .agent-row form{grid-column:2;`+
			`margin:0;justify-self:start}`+
			`.banner:has(.agents-block) .banner-auth>.auth-btn{`+
			`grid-column:2;justify-self:start;text-decoration:none}`+
			`.agents-block,.agent-row{display:contents}`+
			`</style>`+
			`</head><body><main class="page">`+
			// R-UAQQ-NU7B: `.title` and `.subtitle` are reserved
			// page-scope class names. They appear ONLY on the
			// <h1> page heading and the rotating tagline inside
			// this <section class="banner">; no component below
			// reuses either token in its class list.
			// R-GTPJ-Z8EL: the banner card, counter card, and
			// instructions-head article are direct siblings under
			// <main class="page"> with no interposing wrapper and
			// no inline style= overrides — the markup posture the
			// canonical CSS (operator-owned per R-8MP8-6B77)
			// expects to deliver uniform inter-section gaps.
			`<section class="banner">`+
			`<span class="lens" aria-hidden="true"></span>`+
			`<span class="tag">MCP Demo</span>`+
			`<h1 class="title">HAL 9000</h1>`+
			`<div class="subtitle-row">`+
			`<span class="subtitle" id="subtitle">%s</span>`+
			`<button class="refresh" type="button" aria-label="New subtitle">`+
			`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true">`+
			`<path d="M3 12a9 9 0 0 1 15.5-6.3L21 3"/>`+
			`<path d="M21 3v6h-6"/>`+
			`<path d="M21 12a9 9 0 0 1-15.5 6.3L3 21"/>`+
			`<path d="M3 21v-6h6"/>`+
			`</svg>`+
			`</button>`+
			`</div>`+
			`%s`+
			`%s`+
			`</section>`+
			// R-EJAP-XUSB: counter card directly below the banner. The
			// -/+ buttons render as <button> elements carrying the
			// canonical .icon-btn class and aria-labels Decrement /
			// Increment, and are HTML-disabled when no web session is
			// active so .icon-btn:disabled supplies the ~40% opacity /
			// cursor:not-allowed treatment. The hint line is rendered
			// inside the card (below the counter value, left-aligned
			// within the card's content area) and is identical in both
			// auth states — MCP-agent capability is orthogonal to the
			// visitor's browser session.
			`<section class="counter-card">`+
			`<div>`+
			`<div class="counter-label">CURRENT COUNT</div>`+
			// R-G0K2-UUJ0: aria-live="polite" on the counter value so
			// updates pushed over the live channel (R-FZC6-H2SB) are
			// announced to assistive tech without interrupting.
			`<div class="counter-value" aria-live="polite">%d</div>`+
			`<p class="locked-hint">Authenticated agents using MCP can read &amp; mutate this counter on your behalf.</p>`+
			`</div>`+
			`<div class="counter-actions">`+
			`<button class="icon-btn" type="button" aria-label="Decrement"%s>&minus;</button>`+
			`<button class="icon-btn" type="button" aria-label="Increment"%s>+</button>`+
			`</div>`+
			`</section>`+
			`%s`+
			// R-WOEN-ND69: footer is the last child of <main class="page">,
			// not a sibling of it. A footer placed after </main> spans the
			// full viewport instead of matching the 880px column.
			// R-MCHV-YEO4: canonical chrome is bare `<footer>` (no class)
			// containing `footer .status` on the left; the green dot is
			// drawn by `footer .status::before` per reqs/design.css 480-491,
			// so no inner `<span class="status-dot">` element exists.
			// `.footer-left` / `.footer-right` are forbidden shadow names.
			`<footer>`+
			`<span class="status">MCP server live</span>`+
			`<span>v%s · open my pod bay doors</span>`+
			`</footer>`+
			`</main>`+
			`<script>`+
			`(function(){`+
			`var bank=%s;`+
			`var el=document.getElementById('subtitle');`+
			`var btn=document.querySelector('.refresh');`+
			`if(!el||!btn)return;`+
			`btn.addEventListener('click',function(){`+
			`el.classList.add('swap');`+
			`setTimeout(function(){`+
			`el.textContent=bank[Math.floor(Math.random()*bank.length)];`+
			`el.classList.remove('swap');`+
			`},220);`+
			`});`+
			`})();`+
			// R-H4LJ-G9HR: client-tab toggle (Claude Code / Claude Desktop)
			// and copy-button wiring for every code block in the MCP
			// instructions area. Strips a leading shell prompt prefix
			// ($ or >) when present so the clipboard payload is the
			// executable form, not the visual framing.
			`(function(){`+
			`var tabs=document.querySelectorAll('.client-tab');`+
			`var panels=document.querySelectorAll('.section .client-panel');`+
			`function activate(target){`+
			`tabs.forEach(function(t){`+
			`var on=t.getAttribute('data-target')===target;`+
			`t.classList.toggle('active',on);`+
			`t.setAttribute('aria-selected',on?'true':'false');`+
			`t.setAttribute('tabindex',on?'0':'-1');`+
			`});`+
			`panels.forEach(function(p){`+
			`var on=p.getAttribute('data-client')===target;`+
			`p.classList.toggle('active',on);`+
			`if(on){p.removeAttribute('hidden');}else{p.setAttribute('hidden','');}`+
			`});`+
			`}`+
			`tabs.forEach(function(t){`+
			`t.addEventListener('click',function(){activate(t.getAttribute('data-target'));});`+
			`});`+
			`var copies=document.querySelectorAll('.section .copy');`+
			`copies.forEach(function(b){`+
			`b.addEventListener('click',function(){`+
			// R-UBPK-DLTT: each `.copy` lives directly inside the
			// `.code` element; the snippet text is the parent's
			// textContent (the button's body is an `<svg>` glyph
			// so the button itself contributes no text). The
			// "copied" affordance is a class toggle only — the
			// glyph stays in place.
			`var wrap=b.parentNode;`+
			`if(!wrap)return;`+
			`var text=wrap.textContent.replace(/^[\\s]*[\\$>] /,'');`+
			`var done=function(){`+
			`b.classList.add('copied');`+
			`setTimeout(function(){b.classList.remove('copied');},1400);`+
			`};`+
			`if(navigator.clipboard&&navigator.clipboard.writeText){`+
			`navigator.clipboard.writeText(text).then(done,function(){});`+
			`}else{`+
			`var ta=document.createElement('textarea');`+
			`ta.value=text;document.body.appendChild(ta);ta.select();`+
			`try{document.execCommand('copy');done();}catch(e){}`+
			`document.body.removeChild(ta);`+
			`}`+
			`});`+
			`});`+
			`})();`+
			// R-KSI8-M0JX: a signed-in page that initially rendered with
			// zero live MCP chains has no `.agents-block` in the HTML, so
			// the agents SSE client must be able to create the block on the
			// first non-empty snapshot. Subsequent snapshots replace the
			// rows atomically and remove the block again when the live set
			// becomes empty.
			`(function(){`+
			`var auth=document.querySelector('.banner-auth');`+
			`if(!auth||!auth.querySelector('.auth-email'))return;`+
			`function row(chain){`+
			`var r=document.createElement('div');`+
			`r.className='agent-row';`+
			`r.setAttribute('data-chain-id',chain.chain_id||'');`+
			`var name=document.createElement('span');`+
			`name.className='agent-name';`+
			`name.textContent=(chain.client_name||'undefined')+' ('+String(chain.client_id||'').slice(0,8)+')';`+
			`var form=document.createElement('form');`+
			`form.method='post';form.action='/agents/revoke';`+
			`var input=document.createElement('input');`+
			`input.type='hidden';input.name='chain_id';`+
			`input.value=chain.chain_id||'';`+
			`var btn=document.createElement('button');`+
			`btn.className='auth-btn';btn.type='submit';`+
			`btn.textContent='Revoke';`+
			`form.appendChild(input);form.appendChild(btn);`+
			`r.appendChild(name);r.appendChild(form);`+
			`return r;`+
			`}`+
			`function render(chains){`+
			`var block=document.querySelector('.agents-block');`+
			`if(!chains||chains.length===0){`+
			`if(block&&block.parentNode)block.parentNode.removeChild(block);`+
			`return;`+
			`}`+
			`if(!block){`+
			`block=document.createElement('div');`+
			`block.className='agents-block';`+
			`block.setAttribute('aria-label','Authenticated MCP agents');`+
			`auth.appendChild(block);`+
			`}`+
			`block.textContent='';`+
			`chains.forEach(function(chain){block.appendChild(row(chain));});`+
			`}`+
			`try{`+
			`var es=new EventSource('/agents/stream');`+
			`es.onmessage=function(e){`+
			`try{var chains=JSON.parse(e.data);`+
			`if(Array.isArray(chains))render(chains);}catch(_){}`+
			`};`+
			`}catch(_){}`+
			`})();`+
			// R-FY4A-3B1M: wire the signed-in visitor's +/- clicks to the
			// real mutation endpoints, and subscribe every browser (signed
			// in or not) to the SSE feed R-FZC6-H2SB serves. Each observed
			// value change repaints the digit and inserts a +N/-N delta
			// indicator; both visual cues persist long enough that the
			// visitor unambiguously perceives them (>=600ms). The
			// reduced-motion override (R-G0K2-UUJ0) suppresses transitions
			// but leaves the end-state digit and delta visible.
			`(function(){`+
			`var val=document.querySelector('.counter-value');`+
			`if(!val)return;`+
			`var current=parseInt(val.textContent,10);`+
			`if(isNaN(current))current=0;`+
			`var dec=document.querySelector('.icon-btn[aria-label="Decrement"]');`+
			`var inc=document.querySelector('.icon-btn[aria-label="Increment"]');`+
			`function mutate(url){`+
			`fetch(url,{method:'POST',credentials:'same-origin'}).catch(function(){});`+
			`}`+
			`if(inc&&!inc.disabled){`+
			`inc.addEventListener('click',function(){mutate('/counter/increment');});`+
			`}`+
			`if(dec&&!dec.disabled){`+
			`dec.addEventListener('click',function(){mutate('/counter/decrement');});`+
			`}`+
			`function apply(next){`+
			`if(next===current)return;`+
			`var prev=current;current=next;`+
			`var diff=next-prev;`+
			`val.textContent=String(next);`+
			`val.classList.add('flash');`+
			`var d=document.createElement('span');`+
			`var sign=diff<0?'−':'+';`+
			`d.className='delta show'+(diff<0?' minus':'');`+
			`d.textContent=sign+Math.abs(diff);`+
			`val.appendChild(d);`+
			`setTimeout(function(){`+
			`val.classList.remove('flash');`+
			`d.classList.remove('show');`+
			`setTimeout(function(){if(d.parentNode)d.parentNode.removeChild(d);},350);`+
			`},800);`+
			`}`+
			`try{`+
			`var es=new EventSource('/counter/stream');`+
			`es.onmessage=function(e){`+
			`try{var p=JSON.parse(e.data);`+
			`if(typeof p.value==='number'){apply(p.value);}}catch(_){}`+
			`};`+
			`}catch(_){}`+
			`})();`+
			`</script>`+
			`</body></html>`+"\n",
		htmlEscape(PickSubtitle()), bannerAuth, agentsBlock, data.Count,
		counterDisabled, counterDisabled, mcpInstructions, data.Version, bankJSON)
}

// htmlEscape escapes text for safe interpolation into HTML body content.
// The subtitle bank is a fixed in-source list, so this is defense-in-depth
// rather than a load-bearing sanitizer.
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}

// HandleDesignCSS serves the canonical stylesheet embedded from
// web/design.css. R-8MP8-6B77 pins that the designer's file is used
// directly, not re-derived; this handler is the load-bearing seam.
func HandleDesignCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(designCSS)))
	_, _ = w.Write(designCSS)
}
