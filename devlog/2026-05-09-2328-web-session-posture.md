# 2026-05-09 23:28 — Web sessions get a deliberately stricter posture than MCP

## What

Added a Web sessions section to `reqs/auth.md` and `/login` `/logout` /
index identity-display rules to `reqs/web.md`. Web sessions live in a
dedicated `sessions` table (separate from the OAuth token store), are
established only by completing a Google federation round-trip, are
bounded by a 1h idle ceiling and a 12h absolute ceiling, and require a
fresh credential entry at Google on every sign-in. MCP keeps its
existing 1h-access / 30d-refresh chain and its existing willingness to
ride Google's silent SSO.

Key requirements:

- Web: `R-8GJG-64MR`, `R-CXJ2-R3BN`, `R-SLGL-B5B4`, `R-KJ15-9P17`,
  `R-3BKZ-L7R4`, `R-93PJ-FRPY`, plus `R-9PNQ-BN2G` / `R-AE1P-Z1WC` /
  `R-AZZW-UX8U` for the routes and index display.
- MCP guard-rail: `R-126C-AM1E` (no forced-prompt parameters on the
  MCP authorize redirect to Google).

## Why these choices

**Web and MCP have asymmetric trust postures by design.** Browsers
are the higher-risk vector right now — a stolen laptop, a malicious
extension, a phished session can all surface through the browser
surface much more readily than through an MCP client running locally
in someone's terminal. MCP refresh, conversely, is *expensive* — it
requires a browser pop and a human present at the right moment to
complete the federation. Holding both contexts to the same cadence
either makes the web surface dangerously loose or makes the MCP
surface intolerably annoying. So they're tuned independently.

**Web: 1h idle / 12h absolute, forced credentials every sign-in.**
The idle ceiling is short because abandoned tabs are a real exposure;
the absolute ceiling caps a long workday so even a continuously
active user re-authenticates daily. Forced credential entry on every
sign-in (`R-3BKZ-L7R4`) — not just protocol-level federation, but
actual UI at Google with a fresh password (and MFA, where configured)
— is the load-bearing posture. The default OIDC behavior is silent
SSO when Google has an active cookie, and HAL's first iteration
inherited that default. We rejected it: a web session should only
come into existence as a deliberate human act, never as a transparent
cookie handshake. The stronger posture is requested via standard
OIDC parameters (`prompt=login` / `max_age=0`) on the redirect to
Google.

**MCP: 1h access / 30d refresh, silent SSO permitted.** Refresh
chains rotate on every use (`R-89K0-GH5G`) with reuse-detection
(`R-9HGE-87UG`), so a leaked refresh token destroys its own chain
the moment an attacker tries it. The 30-day window keeps real users
from being yanked back through a browser pop unnecessarily.
`R-126C-AM1E` explicitly forecloses a future build agent reaching
for symmetry with the web posture and dragging `prompt=login` into
the MCP flow — that would force a credential prompt on every refresh
chain, which is exactly the friction the MCP timing is designed to
avoid.

**Web sessions live in their own table, not in the OAuth token
store.** The OAuth token schema (`R-Z955-CD0I`: kind, owner, chain,
issued-at, expires-at, used-at, revoked-at, resource binding) was
designed for refresh-rotated bearer tokens with per-resource scoping;
a web session has none of those properties (no chain, no resource,
no single-use semantics). Sharing the table would require scoping
half the existing token rules with "applies to kind ∈ {access,
refresh} only" footnotes. Separate tables also make the independence
property `R-93PJ-FRPY` declares physically true rather than just
disciplinarily true.

**Cookie carries an opaque random string; database stores its hash.**
Same posture as `R-CUUP-REQT` for OAuth tokens — the plaintext
exists in exactly one place outside the user-agent's cookie store
(the `Set-Cookie` response that established the session) and is
never persisted by the service.

## Things deliberately deferred

- A "permission system" or "active sessions" UI that lets a signed-in
  user enumerate and force-expire their own MCP token chains and web
  sessions. `R-93PJ-FRPY` reserves the right to introduce cross-
  context actions; nothing about that surface is committed yet.
- Per-DCR client metadata (human-readable client name, software
  identifier) that the future active-sessions UI will need to render
  meaningful labels. Worth pinning when the surface itself is
  designed, not before.
- An `expires-at` semantic for an "informational future bound" on
  web sessions distinct from the active idle/absolute ceiling.
  Today's `expires-at` is just the materialized min of those two.
