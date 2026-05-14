# Authentication and authorization

The service is the OAuth authorization server its MCP and HTTP API
clients talk to. Google Workspace sits upstream as the actual identity
provider; clients never see Google directly.

## Posture

- R-1KML-5J0Q: the service exposes its own OAuth 2.1 authorization
  endpoints on the same origin as the MCP server
  (https://hal.ai.metaspot.org). Clients are configured with
  only this origin.
- R-27SO-F63X: the service mints and signs its own access tokens.
  Tokens issued by Google are not propagated to MCP clients; clients
  receive tokens issued by this service.
- R-2XEK-GCOI: the service publishes the OAuth authorization-server
  metadata document required by the MCP authorization spec, so
  conformant clients can discover endpoints from the base URL alone.
- R-3JCR-C810: the service supports Dynamic Client Registration
  (RFC 7591) so MCP clients can self-register without manual
  per-client setup.
- R-25DN-9PUR: the DCR endpoint accepts registration requests from
  anyone, unauthenticated, by design. The service does not require
  an initial access token, an admin allowlist, or any other gating
  on registration. A successful registration returns only a
  `client_id` (and, where the spec calls for one, a `client_secret`)
  usable as the starting point of an OAuth flow; it confers no
  access to the counter on its own. The actual access gate is the
  Workspace-domain check at the federation step (R-5LQM-O89D),
  which runs before any token is issued. This is named explicitly
  so it reads as a deliberate posture rather than an oversight: an
  open DCR endpoint is what lets MCP clients self-onboard against
  the published metadata document with no out-of-band credentials,
  per R-VVRG-W2G2.
- R-42V5-GJW4: the service supports the Authorization Code flow with
  PKCE (RFC 7636). It does not support the implicit flow or any
  password grant.
- R-1ERW-YD9G: the authorize endpoint rejects any request whose
  `redirect_uri` is not a byte-for-byte exact match against one of
  the redirect URIs the requesting client registered (via DCR per
  R-3JCR-C810). There is no prefix matching, no wildcard, no
  scheme/host/port normalization, and no trailing-slash tolerance.
  A request with an unregistered or mismatched `redirect_uri` is
  refused at the authorize endpoint itself — the user-agent is not
  redirected anywhere using the supplied value, so a mismatched URI
  cannot be used as an open redirect or to exfiltrate an
  authorization code.
- R-ZPE1-0DV8: an authorization code is single-use and short-lived,
  expiring no more than a few minutes after issue. At issue time the
  code is bound to three values from the originating authorize
  request: the `client_id`, the PKCE code challenge, and the
  `redirect_uri`. The token endpoint accepts a code only when the
  presenting client's `client_id` matches the bound value, the
  presented PKCE verifier hashes to the bound challenge under the
  bound method, and the presented `redirect_uri` is byte-equal to
  the bound one. A second presentation of an already-redeemed code
  is rejected, and any access and refresh tokens that were issued
  from that code are revoked along with the rest of their chain —
  the same posture R-9HGE-87UG defines for refresh-token reuse. An
  expired code is likewise rejected. Without these bindings, PKCE
  is decorative and a leaked code is exchangeable by an attacker.

## Authorization ordering

- R-FFOQ-Y4JG: every endpoint and MCP tool whose specification
  requires authentication evaluates the authentication check
  **before** any input validation, business-logic gate,
  state-dependent rejection, or transport-level transformation
  that could produce its own error response. An unauthenticated
  request to a gated endpoint receives the unauthenticated-
  rejection response and nothing else — HTTP 401 on the JSON API
  (R-53Z2-DNB1), the standard unauthenticated-tool signal on the
  MCP transport (R-0YOE-9NO8) — regardless of the request's
  payload, the system's current state, or what other rejections
  the request would have hit had it been authenticated.

  Concretely: an unauthenticated `POST /counter/decrement`
  (R-H3FE-QFC0) presented while the counter is zero returns 401,
  **not** the "counter cannot go below zero" 4xx that
  R-F5X4-XI2F pins for an authenticated decrement against a zero
  counter. The auth check terminates the request before the
  zero-counter check is reached, and the body of the response
  is the same one any other unauthenticated mutation request
  would receive — it does not leak the counter's value, its
  position relative to zero, or any other state. The same
  ordering applies to the MCP decrement tool (R-GG9B-GS8T,
  R-ZQS0-HWZ8): an unauthenticated decrement against a
  zero-valued counter produces the unauthenticated-tool signal,
  not the "below zero" tool error.

  The intent has two parts. First, an unauthenticated caller
  cannot use response-shape differences to probe system state:
  the only thing a request's rejection can tell an unauth caller
  is "you are not authenticated", never anything about the world
  the gated endpoint operates on. Second, the order of checks
  is observable, so getting it right is testable: a test that
  drives an unauthenticated mutation request against a state in
  which a non-auth rejection would also apply, and asserts that
  the response is exactly the unauthenticated rejection, fences
  this ordering for every gated endpoint the spec names.

## Configuration surface

- R-LWCN-ZBXO: every numeric and string value that governs the
  service's authentication posture — including (but not limited to)
  the token lifetimes named in R-TNXJ-ZWQ0 and R-8UAA-YKR9, the
  web-session ceilings in R-KJ15-9P17, the authorization-code TTL
  in R-ZPE1-0DV8, the Google OIDC scopes named in R-W3K0-QD0E, the
  forced-authentication posture in R-3BKZ-L7R4 and the deliberate
  non-use of forced-authentication parameters in R-126C-AM1E, the
  configured workspace domain in R-5LQM-O89D, the canonical resource
  identifier in R-75E8-YGGN, and the HSTS max-age in R-ID5L-BSJM —
  is sourced from a single named configuration surface inside the
  service. Secrets (the Google client credentials R-68WP-XVCK names)
  are read from environment variables by that same configuration
  surface; startup fails loudly (refuses to begin serving and
  surfaces a clear error) if a required environment variable is
  missing, rather than substituting a default or a sentinel. Code
  that consumes any of these values reads it from the central
  configuration surface and does not duplicate the literal anywhere
  else in the codebase: changing the access-token lifetime, the web
  idle ceiling, the Google scope list, etc., is a single-location
  edit. The intent: an operator reading the auth posture finds every
  value that governs it in one place; a magic number buried in a
  handler or model is itself a defect.

## Google federation

- R-4SH1-HQGP: when a user reaches the service's authorize endpoint,
  the service redirects them to Google so that Google performs the
  actual login.
- R-126C-AM1E: the redirect R-4SH1-HQGP sends to Google for the MCP
  authorization flow does **not** include `prompt=login`,
  `prompt=consent`, `max_age=0`, or any other parameter that demands
  fresh re-authentication. MCP federation uses Google's default
  behavior, which permits silent SSO when Google has an active
  session for the user. The forced-authentication posture
  R-3BKZ-L7R4 establishes is the web-flow analog and does not
  extend to MCP — the two contexts have asymmetric trust postures
  by deliberate choice. The intent: MCP refresh is expensive (it
  requires a browser pop and a human present), so an MCP token
  chain is allowed to ride a long-lived Google session within its
  R-8UAA-YKR9 refresh-chain ceiling rather than being forced to
  collect credentials on each refresh's federation step. Browsers
  are the higher-risk vector and have their own tight cadence
  pinned in R-KJ15-9P17 / R-3BKZ-L7R4; MCP clients are local,
  longer-lived, and operate with the looser cadence pinned in
  R-TNXJ-ZWQ0 / R-8UAA-YKR9.
- R-5LQM-O89D: the service is configured at deploy time with the
  single Workspace domain whose users are allowed. A user whose
  Google identity is outside that domain is rejected with a clear
  error message and no token is issued.
- R-ANRQ-04PK: the allowed Workspace domain (R-5LQM-O89D) is
  supplied via the environment variable `GOOGLE_WORKSPACE_DOMAIN`,
  matching the bare-`GOOGLE_*` convention R-68WP-XVCK and the
  Google federation seam already use for `GOOGLE_CLIENT_ID` and
  `GOOGLE_CLIENT_SECRET`. The service reads this exact name — not
  a `HAL_`-prefixed variant — and surfaces a clear error at startup
  when the variable is unset or empty, consistent with the
  fail-loudly contract R-LWCN-ZBXO pins for required configuration.
  The same value flows to the two places it governs: the `hd`
  parameter on the Google authorization URL (R-W3K0-QD0E), and the
  `hosted_domain` claim check the callback applies (R-5LQM-O89D).
- R-68WP-XVCK: Google client credentials (client ID and secret) are
  supplied via environment configuration. They are never committed to
  the repository.
- R-T37L-4J01: this requirement governs **every code path in the
  service that redirects the user-agent to Google for federated
  login** — without exception. The enumerated paths are:

  1. The **web `/login`** redirect to Google (the entry point the
     human visitor reaches per R-9PNQ-BN2G, R-8GJG-64MR,
     R-3BKZ-L7R4).
  2. The **MCP `/oauth/authorize`** redirect to Google (the entry
     point a registered MCP client's authorize request reaches
     per R-4SH1-HQGP, R-126C-AM1E).

  Any future code path the service introduces that hands the
  user-agent off to Google for federation is governed by this
  requirement as well; the enumeration above names what exists
  today, not a closed set.

  **Property** (applies to every enumerated path): when the
  service redirects the user-agent to Google for federated
  login, it generates a fresh unguessable `state` value and
  records it server-side, bound both to the in-flight authorize
  request it represents and to the originating browser session
  (the `bindingID` written to the browser as the
  `hal_oauth_state` cookie, or any equivalent browser-session
  binding the service uses for this purpose). The Google
  callback is accepted only when the returned `state` is
  recognized, unexpired, has not been consumed before, and was
  generated for the same browser session that is presenting the
  callback. A `state` that is missing, unknown, expired,
  already-consumed, or tied to a different session causes the
  callback to be rejected with no token chain issued and no web
  session established. Used `state` values are single-use.

  **The property is per-path, not aggregate.** Satisfying the
  state-binding contract on one enumerated path while skipping
  it on another does **not** satisfy this requirement. A
  redirect-to-Google code path that generates a `state` value
  but does **not** record it server-side, does **not** set the
  browser-session binding cookie, or does **not** validate both
  on the callback is in violation regardless of which other
  paths are correctly wired. Concretely: an MCP authorization
  flow whose `/oauth/authorize` handler redirects to Google
  with a `state` value the callback then rejects as "state
  value not recognized" — because the handler never recorded
  the state in the server-side store — is the exact failure
  mode this requirement forbids.

  **Acceptance criteria the test suite exercises both paths:**
  the verification of this requirement names tests that drive a
  redirect-to-Google through the web `/login` entry point and
  separately drive a redirect-to-Google through the MCP
  `/oauth/authorize` entry point, and asserts in both cases
  that (a) a state-binding cookie is set on the redirect
  response, (b) the corresponding callback succeeds when the
  cookie and `state` value are presented together, and (c) the
  callback fails with no token chain issued when the cookie is
  missing, the `state` is unknown, the `state` is expired, the
  `state` has already been consumed, or the cookie's value
  differs from the binding recorded server-side. A test suite
  that exercises only one of the two entry points does not
  verify this requirement.

  This closes the login-CSRF window where an attacker
  pre-initiates a flow and induces a victim to complete the
  Google step on their behalf — and closes the
  per-path-omission gap where one code path correctly enforces
  the binding while another silently skips it.

- R-MTRN-DL9W: every state record (R-T37L-4J01) carries enough
  of the originating authorize-request context that the
  Google-callback handler can complete its work without
  consulting any other source. The record stores, at minimum:

  - **Origin discriminator**: which code path created this
    record. Exactly two values exist today: `web` (the
    `/login` redirect to Google) and `mcp` (the
    `/oauth/authorize` redirect to Google). The
    discriminator is recorded at state-creation time and
    never mutates.
  - **For `mcp`-origin records, the full MCP authorize-
    request context needed to complete the post-callback
    action**: the requesting MCP client's `client_id`, the
    `redirect_uri` it presented (already verified at
    authorize time against its DCR-registered values per
    R-1ERW-YD9G — the recorded value is the byte-for-byte
    request value, not a normalized form), the PKCE
    `code_challenge` and `code_challenge_method`, the
    original `state` value the MCP client supplied on its
    `/oauth/authorize` request (to be echoed back to the MCP
    client on the eventual redirect to its callback URL),
    and any `resource` parameter the request carried (which
    must have already passed the R-4GRA-EGBY check; the
    recorded value is the byte-for-byte request value).
  - **For `web`-origin records**: no extra context beyond
    the discriminator is required, because the post-callback
    action (establish a web session, redirect to `/`) needs
    no further values from the originating request.

  The record's session-binding ID, expiry, and consumed flag
  pinned by R-T37L-4J01 are unchanged; this requirement adds
  to that store, it does not replace any field there. A
  state record that lacks the origin discriminator, or that
  lacks any of the enumerated `mcp`-origin fields when its
  origin is `mcp`, is in violation; the build agent cannot
  satisfy R-MUZJ-RD0L without the data this requirement pins.

- R-MUZJ-RD0L: the Google-callback handler dispatches on the
  recorded origin discriminator (R-MTRN-DL9W) after — and
  only after — the state-binding check (R-T37L-4J01) and the
  workspace-domain check (R-5LQM-O89D) have both succeeded.
  No HAL-issued token, no HAL-issued authorization code, and
  no web session is created on any path where either check
  failed. On the success path the dispatch is:

  - **`web` origin** (federation initiated from `/login`):
    the handler establishes a web session for the
    Google-asserted email per R-CXJ2-R3BN (with cookie
    attributes per R-AYLJ-8SYX), writes the web-session
    record per R-SLGL-B5B4, and redirects the user-agent to
    `/` via HTTP 303. No HAL authorization code is minted on
    this path; no MCP client is involved.
  - **`mcp` origin** (federation initiated from
    `/oauth/authorize`): the handler **does NOT establish a
    web session**, **does NOT redirect to `/`**, and **does
    NOT touch the web-session store**. Instead, it:

    1. Mints a fresh HAL authorization code per R-ZPE1-0DV8,
       bound at issue time to the state record's recorded
       `client_id`, `code_challenge`, `code_challenge_method`,
       and `redirect_uri` — these come from the state
       record, not from the callback request's query
       parameters. The code is also tied to the
       Google-asserted email (the identity it represents),
       so the eventual token issuance (R-ZQS0-HWZ8) can
       attach owner information; and to the `resource` value
       if one was recorded.
    2. Builds the redirect target as the state record's
       recorded `redirect_uri`, with the freshly-minted HAL
       authorization code in the `code` query parameter and
       the MCP client's **originally-supplied** `state`
       value (recorded at authorize time, NOT the HAL
       internal state value used for the Google round-trip)
       echoed in the `state` query parameter.
    3. Issues HTTP 303 to that target.

    A rendering in which an `mcp`-origin Google callback
    success terminates with the user-agent at HAL's
    `/` landing page — the exact failure observable in the
    server access log as the sequence
    `GET /oauth/authorize → 303` ··· `GET /oauth/google/callback → 303` ··· `GET / → 200`,
    with no intervening `GET <mcp-client-redirect_uri>?code=…&state=…`
    — does **not** satisfy this requirement. The
    user-agent's final HTTP destination on the success path
    of an `mcp`-origin federation is the MCP client's
    registered callback, with the HAL authorization code in
    hand; anything else is a defect.

    A rendering in which an `mcp`-origin success
    inadvertently also establishes a HAL web session (so
    the human running the MCP-auth flow finds themselves
    signed in to the HAL web UI as a side effect) is a
    separate defect: per R-0XJ4-5MSL, web sessions and MCP
    token chains are independent identity contexts;
    completing an MCP authorize flow does not — under any
    code path — produce a web session.

  When the workspace-domain check (R-5LQM-O89D) rejects an
  out-of-domain identity, the dispatch path is suspended for
  both origins; the handler returns the appropriate
  rejection response and writes neither a web session nor a
  HAL authorization code. The origin discriminator is still
  used to choose the rejection surface (an in-browser error
  page for `web` origin, the standard OAuth `error=…`
  redirect to the MCP client's `redirect_uri` for `mcp`
  origin where that is feasible), but no authenticated
  artifact is produced on either path.

- R-77U1-PZY1: the test suite contains at least one
  end-to-end test that drives a complete MCP OAuth round
  trip against the running service (using the Google
  identity-provider test double per R-VF61-2Y6I) and asserts
  that the round trip terminates with the simulated MCP
  client holding a usable bearer access token. The test
  exercises, in order, every leg of the documented MCP
  authorization flow:

  1. Challenge: an unauthenticated request to the MCP
     transport endpoint (`POST /mcp` per R-UK7D-Z0IZ /
     R-7A9U-HJFF) responds with the unauthenticated-tool
     signal R-0YOE-9NO8 names and carries a
     `WWW-Authenticate: Bearer ..., resource_metadata="..."`
     header per R-7BHQ-VB64. The `resource_metadata` URL the
     header advertises is the protected-resource metadata
     document's URL (`/.well-known/oauth-protected-resource/mcp`
     per R-75E8-YGGN).
  2. Discovery: `GET /.well-known/oauth-authorization-server`
     and `GET /.well-known/oauth-protected-resource/mcp`
     return the documents R-2XEK-GCOI / R-75E8-YGGN pin.
     The `resource` field in the protected-resource metadata
     document is byte-equal to the configured canonical
     identifier (`HAL_RESOURCE_IDENTIFIER` per R-791Y-3ROQ).
  3. Dynamic Client Registration: `POST /oauth/register`
     returns a fresh `client_id` per R-3JCR-C810 /
     R-25DN-9PUR.
  4. Authorize: `GET /oauth/authorize` with the registered
     `client_id`, a PKCE challenge, a `redirect_uri`, and a
     `resource` parameter equal to the canonical identifier
     R-75E8-YGGN publishes responds with a 303 to Google and
     sets the state-binding cookie per R-T37L-4J01.
  5. Google round trip: the test double satisfies the
     authorization-URL/code-exchange seam R-T0B2-A4E5
     defines, simulating a callback to
     `/oauth/google/callback` with a workspace-domain
     identity that passes R-5LQM-O89D.
  6. Origin dispatch: the callback responds with a 303 to
     the MCP client's registered `redirect_uri` carrying a
     HAL `code` and the MCP client's original `state` per
     R-MUZJ-RD0L.
  7. Token exchange: `POST /oauth/token` with the HAL
     authorization code, the PKCE verifier, the `client_id`,
     the registered `redirect_uri`, and the same `resource`
     value sent in step 4 returns a bearer access token and
     a refresh token per R-42V5-GJW4 / R-ZPE1-0DV8 /
     R-WRDD-TR27.
  8. Bearer use: presenting the issued access token at the
     MCP transport endpoint (R-UK7D-Z0IZ / R-7A9U-HJFF) for
     an authenticated tool — increment per R-ZQS0-HWZ8 — is
     accepted and the call succeeds.

  All eight legs must pass within the single test for the
  property to hold. A test suite that passes every leg in
  isolation but does **not** demonstrate that one MCP
  client can carry context across all eight legs in a
  single end-to-end run does not verify this requirement.
  The failure modes the test prevents include the
  challenge-discovery gap (an unauthenticated MCP request
  fails without a `WWW-Authenticate` header pointing at the
  metadata document, so a real client cannot bootstrap into
  the flow), the per-path-omission gap (state-binding
  skipped on the MCP authorize path), the origin-dispatch
  gap (callback terminates at `/` instead of the MCP
  client's redirect_uri), the code-binding gap (token
  exchange succeeds with a wrong PKCE verifier or a wrong
  `redirect_uri`), and the bearer-binding gap (an issued
  token is rejected at the MCP endpoint due to a
  resource-binding mismatch the test would have caught at
  issue time — including the host-form mismatch the
  R-791Y-3ROQ / R-76M5-C87C pair closes).

  Because R-VF61-2Y6I forbids outbound network calls in
  tests, the Google leg is driven through the test double.
  The build agent does **not** mark R-MTRN-DL9W,
  R-MUZJ-RD0L, or R-77U1-PZY1 verified by a test that
  exercises only a subset of these legs; a per-leg unit
  test is fine for the per-leg requirements it verifies,
  but the end-to-end test this requirement names is
  separately required.

## Web sessions

- R-8GJG-64MR: the service offers a browser-facing login flow distinct
  from the MCP authorization flow. A human reaches it through a stable
  web entry point and is taken through the same Google Workspace
  federation Google federation defines — including the same workspace-
  domain check (R-5LQM-O89D); a Google identity outside the configured
  domain is rejected with a clear error and no web session is
  established. On successful federation the service records a web
  session that identifies the human by their Google email; that email
  is the identity the rest of the application sees for the signed-in
  visitor.
- R-CXJ2-R3BN: the only code path that establishes a web session is
  the successful completion of the Google federation round-trip
  R-8GJG-64MR defines: the visitor's user-agent must reach Google's
  authorization endpoint, the human must complete the Google login
  screen, and the service must accept Google's callback (with `state`
  validated per R-T37L-4J01 and the workspace-domain check applied per
  R-5LQM-O89D) before any web session exists. The service does not
  synthesize a web session by any other means — not from an active MCP
  token chain belonging to the same email, not from a development-mode
  auto-sign-in, not from a "remember me" cookie that revives a session
  without re-running federation. The observable property: in any
  environment that uses the real Google identity provider
  (R-W3K0-QD0E), every transition from "not signed in" to "signed in"
  produces a network round-trip to Google's authorization endpoint. A
  login flow that completes without ever reaching Google is a defect.
- R-SLGL-B5B4: web sessions are persisted in a dedicated store
  distinct from the OAuth token store R-WRDD-TR27 defines. Each
  persisted session records at minimum: the owner (the Google email
  recorded at sign-in time per R-8GJG-64MR), a cryptographic hash of
  the opaque session identifier the cookie carries, issued-at,
  expires-at, and revoked-at. The plaintext session identifier
  appears in exactly one place outside the user-agent's cookie store:
  the `Set-Cookie` response that established the session; the service
  never persists it — mirroring the posture R-CUUP-REQT defines for
  OAuth token plaintext. Validation of an inbound session cookie is a
  single lookup against this store: hash the presented value, find
  the record, accept iff the record is un-expired and un-revoked.
  Logout (R-AE1P-Z1WC) is the act of writing revoked-at on the
  matching record; once revoked, the same cookie value cannot be
  redeemed again. The web-session store and the OAuth-token store
  are independent — they do not share records and have no referential
  relationship; lifecycle operations on one (issue, revoke, rotate,
  expire) do not read or write records in the other, reinforcing
  R-0XJ4-5MSL at the storage level.
- R-KJ15-9P17: a web session is bounded by two ceilings beyond
  explicit revocation. (1) **Idle:** a session that has gone 1 hour
  without a successful authenticated request from its bearer is
  treated as expired; the 1-hour clock restarts on each successful
  authenticated request that presents the session cookie. (2)
  **Absolute:** regardless of activity, a session is treated as
  expired 12 hours after its issued-at timestamp. The earlier of
  the two bounds governs at any given moment. An expired session
  validates as expired per R-SLGL-B5B4 and the user must complete a
  fresh federation round-trip per R-CXJ2-R3BN — and re-enter
  credentials at Google per R-3BKZ-L7R4 — to obtain a new session.
  How the two bounds are stored and combined is HOW; the observable
  property is the two ceilings — 1 hour idle, 12 hours absolute.
  Logout still terminates a session immediately by writing
  revoked-at; these ceilings are upper bounds on how long a session
  can survive without explicit termination. The intent: an active
  user enjoys a workday-length session up to 12 hours; an abandoned
  tab dies after 1 hour; neither path lets a web session linger past
  the bounds.
- R-3BKZ-L7R4: every web /login redirect (R-9PNQ-BN2G) to Google's
  authorization endpoint includes whatever parameter Google's OIDC
  contract requires to demand a fresh authentication of the user —
  Google must actually re-authenticate the human (collect
  credentials, complete MFA if configured) rather than satisfy the
  request from an existing Google session via silent SSO. The exact
  parameter (today: `prompt=login`, `max_age=0`, or both) is HOW;
  the observable property is that on every web sign-in the user
  must complete Google's authentication UI, regardless of any
  active Google cookie in their browser. This applies uniformly to
  every web sign-in: the first sign-in, the sign-in after a HAL
  logout (R-AE1P-Z1WC), the sign-in after a HAL session has
  expired by the idle or absolute ceiling (R-KJ15-9P17), and the
  sign-in after any HAL-side revocation. A web sign-in flow that
  completes without the user having entered credentials at Google is
  a defect. The intent: a HAL web session can only be established by
  a deliberate act of human authentication at Google, not by a
  transparent cookie handshake. This is the web-side analog of the
  defense-in-depth posture R-CXJ2-R3BN establishes for the federation
  round-trip itself; together they say a web session requires both a
  network round-trip to Google and a fresh credential collection on
  every issuance.
- R-0XJ4-5MSL: a web session and an MCP token chain are independent
  identity contexts that do not share lifetime or revocation. A human
  who is signed in to the web UI and also has live MCP tokens issued
  for the same email is in two separate states: ending the web session
  (logout) does not revoke any MCP token, and revoking or expiring an
  MCP token chain does not end the web session. There is exactly one
  user-initiated cross-action permitted by this spec: a signed-in
  visitor may revoke an MCP token chain owned by their own email
  through the action R-0SNI-MJTT defines on the index page's agents
  block. This direction is one-way only — revoking a chain through
  that action does not affect the web session that issued the revoke,
  and the inverse (an MCP-token-chain action ending a web session) is
  still forbidden. No other cross-actions exist in the current spec,
  and the build agent must not invent any.

## Provider wiring

- R-VF61-2Y6I: in the test environment, the Google identity provider
  is a test double, and the automated test suite makes no outbound
  network calls to Google. The double returns payloads whose shape
  matches Google's documented OAuth/OIDC responses, so service code
  under test exercises the same code paths it uses against real
  Google. The property is verified by checking the configured
  provider in the test environment and by exercising the double's
  behavior; it is **not** verified by asserting that any "not yet
  implemented" sentinel exists in the real-Google code path. A test
  that pins the real provider's operations to a sentinel couples the
  test to a transient implementation state and will break in lockstep
  with R-W3K0-QD0E being satisfied — that coupling is itself a defect.
- R-T0B2-A4E5: the seam between the service's Google client and
  Google is narrow — exactly two operations: one that produces the
  authorization URL the user-agent should be redirected to, and one
  that exchanges an authorization code for an identity value. The
  two implementations (test double, real-Google) return values of
  identical shape, so callers of the seam do not branch on which is
  in use. The authorization-URL operation returns the URL string the
  user-agent should be redirected to. The code-exchange operation
  returns an identity value carrying the four claims the callback
  consumes — `sub`, `email`, `hosted_domain`, `email_verified` —
  drawn from the resulting OIDC ID token. Callers depend only on
  this contract; they do not look for extras one implementation may
  incidentally expose (e.g. a raw OAuth token-endpoint hash, the
  unparsed ID token JWT, or a pre-parsed claims map), because the
  other implementation may not surface them. There is no automated
  test tier that exercises real Google; end-to-end verification
  against real Google is performed manually by running the service
  in development and driving the flow through a browser.
- R-W3K0-QD0E: in development and production environments, the
  Google identity provider is the real implementation (not the test
  double). Both seam operations defined in R-T0B2-A4E5 are fully
  implemented — neither raises a "not yet implemented" sentinel of
  any form, and neither returns a fixture or stub value. The
  authorization-URL operation builds a URL on Google's documented
  OAuth 2.0 / OIDC authorization endpoint, parameterized with the
  client ID from `GOOGLE_CLIENT_ID`, the supplied redirect URI, the
  supplied state, the OIDC scopes the service needs
  (`openid email profile`), and the `hd` parameter set to the
  configured Workspace domain (R-5LQM-O89D). The code-exchange
  operation performs an HTTPS POST to Google's documented token
  endpoint, authenticating with `GOOGLE_CLIENT_ID` and
  `GOOGLE_CLIENT_SECRET` per R-68WP-XVCK, and returns an identity
  value carrying the `sub`, `email`, `hosted_domain`, and
  `email_verified` claims from the resulting ID token. The
  implementations satisfy the surrounding Google-federation
  requirements (R-4SH1-HQGP, R-5LQM-O89D, R-68WP-XVCK, R-T37L-4J01)
  when reached in deployed environments.

## Tokens

- R-WRDD-TR27: tokens (both access and refresh) are opaque
  cryptographically-random strings. Each issued token has a
  corresponding server-side record capturing at minimum: the token's
  kind (access or refresh), its owner, its chain membership (so
  rotation and revocation can act on siblings), issued-at,
  expires-at, used-at (refresh tokens only, for reuse detection),
  and revoked-at. Validation of an inbound bearer token is a lookup
  against this store — the string itself carries no information —
  and the lookup accepts the token **only when** the record exists,
  its `expires-at` is in the future, its `revoked-at` is unset, and
  (for refresh tokens) its `used-at` is unset. If any of those
  conditions fails the token is rejected, regardless of which
  condition produced the failure or which other condition might
  also have applied. The `expires-at` field is the mechanism that
  enforces R-TNXJ-ZWQ0 and R-8UAA-YKR9; the `used-at` field is the
  mechanism for R-89K0-GH5G and R-9HGE-87UG; the `revoked-at` field
  is the mechanism for **every** revocation path the spec names —
  reuse-detection-triggered chain revocation (R-9HGE-87UG /
  R-A26O-QBG9), user-initiated chain revocation through the agents
  block (R-0SNI-MJTT), the revocation that R-ZPE1-0DV8 performs on
  the access and refresh tokens of a code-reuse chain, and any
  future revocation path the spec introduces. A token-validation
  code path that consults `expires-at` but not `revoked-at`, or
  that honors `revoked-at` for some revocation origins but not
  others, is in violation. The property is observable at every
  bearer-token validation site — the MCP transport endpoint
  (R-UK7D-Z0IZ) and the HTTP API mutation endpoints
  (R-340Z-T6K2 / R-H3FE-QFC0 / R-4ED6-CGQG) — and the failure mode
  this requirement fences is the one where a revoked chain's
  outstanding access token continues to satisfy validation because
  the validation lookup never reads `revoked-at`.
- R-CUUP-REQT: the token record stores a cryptographic hash of the
  token string, not the plaintext. The plaintext is returned to the
  client exactly once, at issue time, and is never persisted by the
  service. Inbound bearer tokens are validated by hashing the
  presented string with the same algorithm and looking up the record
  by that hash. A database leak therefore does not give an attacker
  any usable token.
- R-6UUW-TQP2: an issued access token grants the holder permission to
  call the increment tool. There are no finer-grained scopes in the
  current spec.
- R-7GT3-PM1K: access tokens have a finite lifetime. The service
  issues refresh tokens so well-behaved clients can stay logged in
  without re-prompting on every expiry.
- R-TNXJ-ZWQ0: an issued access token expires one hour after issue.
- R-89K0-GH5G: each successful refresh-token use issues a new
  refresh token alongside the new access token. A refresh token is
  single-use: it is invalidated the moment its successor is issued.
- R-8UAA-YKR9: a refresh token expires thirty days after its own
  issue time. An active client that keeps refreshing stays logged in
  indefinitely; a client that goes thirty days without refreshing
  must re-authenticate through Google.
- R-9HGE-87UG: presenting a refresh token that has already been used
  (or has otherwise been invalidated) is treated as evidence of
  compromise. The server rejects the request and revokes the entire
  token chain it belongs to — the current live refresh token and any
  outstanding access tokens issued from that chain. The user must
  re-authenticate through Google to obtain a fresh chain.
- R-A26O-QBG9: revocation triggered by reuse detection takes effect
  immediately for newly arriving requests. Any access token from the
  revoked chain that is presented after revocation is rejected.
- R-75E8-YGGN: the service has a single configured resource
  identifier — the MCP transport endpoint URL the service is
  reached at, including its path component (R-7A9U-HJFF pins the
  path as `/mcp`; in development the full identifier is
  `http://localhost:3000/mcp`, in production
  `https://hal.ai.metaspot.org/mcp`). The identifier is sourced
  exclusively from the environment per R-791Y-3ROQ. The service
  honors the `resource` parameter (RFC 8707) on its authorize and
  token endpoints. The canonical identifier is published verbatim,
  byte-for-byte, as the value of the `resource` field in the
  OAuth 2.0 Protected Resource Metadata document (RFC 9728). Per
  RFC 9728 §3.1, because the identifier carries a non-root path,
  the metadata document is served at
  `/.well-known/oauth-protected-resource/mcp` — the path component
  of the resource identifier is appended to
  `/.well-known/oauth-protected-resource`. The same string appears
  in the metadata document, in the bound `resource` value recorded
  on each issued token, and in the validation comparison. Each
  issued access token's server-side record captures the resource
  identifier the token was bound to at issue time. The MCP transport
  endpoint and the HTTP API mutation endpoints accept a presented
  bearer token only when its recorded resource binding equals this
  service's identifier; a token whose binding is for any other
  resource, or that has no recorded binding, is rejected. This
  holds even though only one resource currently exists, so a token
  minted for some future second resource cannot be replayed against
  this one (and vice versa) the moment such a resource is
  introduced.
- R-4GRA-EGBY: the authorize endpoint and the token endpoint
  reject any request whose `resource` parameter is present and is
  not byte-equal to the configured canonical resource identifier
  R-75E8-YGGN defines. Rejection happens at the endpoint itself,
  before any code or token is issued, and uses the standard OAuth
  signaling RFC 8707 §3.2 prescribes (`error="invalid_target"`).
  The user-agent is not redirected anywhere using the offending
  `resource` value. This is the issue-time mirror of the
  presentation-time check R-76M5-C87C defines: without it, a
  client that sends a `resource` value the service cannot accept
  (a different trailing-slash discipline, a different scheme, an
  extra path segment, etc.) silently obtains a token that every
  subsequent presentation will reject for resource-binding
  mismatch, with no obvious signal of what went wrong. Closing
  the loop at issuance turns that silent failure into a loud,
  diagnosable one.
- R-76M5-C87C: "matches" in R-75E8-YGGN means byte-for-byte
  equality against the one configured resource-identifier string
  the service was started with — the value R-791Y-3ROQ reads from
  `HAL_RESOURCE_IDENTIFIER`. The protected endpoints — the MCP
  transport endpoint (R-UK7D-Z0IZ) and the counter-mutation
  endpoints `POST /counter/increment` (R-340Z-T6K2) and
  `POST /counter/decrement` (R-H3FE-QFC0) — share that single
  resource identifier; neither endpoint derives its own resource
  identifier from the request URL, the request's `Host` header,
  the bound interface (`hal serve --ip`), the bound port
  (`--port`), or any per-endpoint convention. A token bound to
  the configured resource is therefore valid for every protected
  endpoint of this service simultaneously. Conversely, a token
  whose recorded resource string differs from the configured one
  in any way — trailing slash, scheme, host, port, path — is
  rejected. This closes the failure mode where the resource check
  rejects every token (and protected-resource metadata advertises
  a mismatched identifier) because the build agent derived the
  identifier from the request URL or the bind address (e.g.
  publishing `http://127.0.0.1:3000/mcp` because `--ip` defaults
  to `127.0.0.1`, while clients reach the service through
  `http://localhost:3000/mcp` and reject the mismatch) rather
  than from the single configured value R-791Y-3ROQ pins.
- R-791Y-3ROQ: the canonical resource identifier R-75E8-YGGN
  defines is sourced from the environment variable
  `HAL_RESOURCE_IDENTIFIER`. The variable is required: when it is
  unset or empty, `hal serve` fails loudly per R-LWCN-ZBXO with a
  clear error and never begins accepting requests. The service
  does not provide a default value, does not derive the identifier
  from the bound interface (`--ip`), the bound port (`--port`),
  the request's `Host` header, the request URL, or any other
  runtime signal — every place the spec calls for the canonical
  identifier reads exactly the string the operator supplied. The
  value must include the MCP endpoint path component R-7A9U-HJFF
  pins (`/mcp`); a value whose path component is absent, empty,
  or differs from `/mcp` is rejected at startup with the same
  fail-loudly contract. The operator picks the scheme, host, and
  port that match how clients externally reach the service: a
  developer running `hal serve` bound to `127.0.0.1:3000` but
  reaching the service through `http://localhost:3000` sets
  `HAL_RESOURCE_IDENTIFIER=http://localhost:3000/mcp`, not
  `http://127.0.0.1:3000/mcp`. The variable is not classified as
  a secret; the startup banner R-NQ3G-K0CQ pins reports its
  effective value verbatim.
- R-E5GH-PN6G: at the moment a token record is written, the
  difference between the record's `expires_at` and `issued_at` equals
  the lifetime defined for that token kind exactly: one hour for an
  access token (R-TNXJ-ZWQ0), thirty days for a refresh token
  (R-8UAA-YKR9). Both timestamps are taken from a single clock
  source within the issuance code path. A token must therefore
  validate as un-expired by the same `expires_at` rule the very
  first time it is presented after issue; a token that validates as
  already-expired on first use is a bug, not a borderline-clock
  anomaly. This closes the failure mode where `expires_at` is
  misset (wrong timezone, set to `issued_at`, set to a past
  instant) and every freshly issued token is rejected as expired.
- R-EV2D-QTR1: when the service rejects a request because the
  presented bearer token failed validation, the response uses the
  standard OAuth signaling — `error="invalid_token"` for a token
  that was presented but did not validate, `error="invalid_request"`
  for a request that should have carried a token but did not — and
  the `error_description` field discriminates among the distinct
  failure causes the spec separately defines: no token presented,
  token malformed, token not found in the store, token expired
  (R-TNXJ-ZWQ0), token chain revoked (R-9HGE-87UG / R-A26O-QBG9),
  and token's recorded resource binding does not match (R-75E8-YGGN
  / R-76M5-C87C). Each cause yields its own distinct
  `error_description` string; the service does not collapse two or
  more causes into a single placeholder reason such as `"expired"`.
  The exact wording of each description is HOW; the property is
  that a debugger reading the response can tell which of the named
  causes fired, and a conformant client can decide between
  refreshing (genuinely expired) and restarting the auth flow
  (revoked, wrong resource, unknown).
- R-AYLJ-8SYX: any cookie the service uses to identify a browser
  session — including the one R-T37L-4J01 binds `state` against —
  is set with `HttpOnly` and `SameSite=Lax`. The cookie additionally
  carries `Secure` when the response is served over HTTPS — detected
  via the same forwarded-protocol signal R-ID5L-BSJM uses to gate
  HSTS. The local-development service, which speaks plain HTTP per
  R-PVA6-Q6OB and is not reached through the production TLS-
  terminating proxy, omits `Secure` so the session cookie can survive
  the OAuth round-trip; without this dispensation, modern browsers
  refuse to store a `Secure` cookie set over `http://`, the session
  evaporates between the authorize redirect and the Google callback,
  and the state-binding check (R-T37L-4J01) rejects every callback
  in dev. The `Secure` property is therefore conditional on having
  actually been served over HTTPS, exactly the way R-ID5L-BSJM
  treats HSTS: present in production, absent locally. `SameSite=Lax`
  (rather than `Strict`) is required because the callback from
  Google is a cross-site top-level navigation that must carry the
  session cookie for the state-binding check to succeed. The
  session identifier is rotated when authentication state changes,
  in particular on successful completion of the federated Google
  login, so a session ID an attacker may have planted in the
  victim's browser before the flow began is no longer valid
  afterwards. Without these attributes, the browser-session premise
  R-T37L-4J01 depends on does not hold: a non-`HttpOnly` cookie is
  reachable from page-level script; under HTTPS, a non-`Secure`
  cookie can travel in plaintext; a missing `SameSite` lets
  cross-site requests ride the session.
- R-SAK8-WB9W: bearer token plaintext (access and refresh tokens)
  appears in exactly one place outside the client's own memory: the
  response body of the token endpoint that issued it. The service
  does not write token plaintext to application logs, request logs,
  error reports, traces, metrics, or any other diagnostic sink. The
  service does not accept tokens presented in URL query strings or
  path segments — bearer tokens are accepted only in the
  `Authorization: Bearer` request header. Any logging that captures
  request URLs or headers must redact `Authorization` values before
  emission. Together with R-CUUP-REQT (no plaintext in the database),
  this preserves the property that no static artifact the service
  produces or retains contains a usable token.

## Cross-origin posture

- R-MHYT-TIF7: protected endpoints — the MCP transport endpoint
  (R-UK7D-Z0IZ) and `POST /counter/increment` (R-340Z-T6K2) — reject
  cross-origin browser requests. Their responses carry no
  `Access-Control-Allow-Origin` header, and the service never sets
  `Access-Control-Allow-Credentials: true` on any response. Public
  read surfaces — `GET /counter` (R-2I2S-XB7K) and the index page
  (R-QY5R-PYDH) — may be served cross-origin since they only
  disclose the counter value, which is already public. The OAuth
  endpoints — authorization-server metadata (R-2XEK-GCOI), the
  authorize endpoint, the token endpoint, and dynamic client
  registration (R-3JCR-C810) — are reachable cross-origin to the
  extent the OAuth 2.1 / MCP authorization specs require for browser-
  based clients to discover and use them; in particular, the
  metadata document and the token endpoint are readable from any
  origin. The intent: a malicious page in a victim's browser cannot
  reach a protected endpoint from an unrelated origin, regardless
  of how that page might have obtained an access token, while
  spec-conformant browser-based discovery and token exchange
  continue to work.

## Transport security headers

- R-ID5L-BSJM: when the service detects (via the standard forwarded-
  protocol signal already honored by R-DA34-WX9P) that a request
  arrived through the production TLS-terminating proxy, every
  response carries `Strict-Transport-Security` with `max-age` of at
  least one year and `includeSubDomains`. Every response —
  regardless of whether it was served through the proxy — carries
  `X-Content-Type-Options: nosniff`. The local-development service,
  which speaks plain HTTP and is not reached through the production
  proxy, does not need to emit `Strict-Transport-Security`; the
  HSTS property is conditional on having actually been reached over
  HTTPS. HSTS pins browsers to HTTPS for this host across the
  `max-age` window so a user who once reached the site over HTTPS
  cannot be downgraded to plaintext on a later visit; `nosniff`
  prevents browsers from reinterpreting a response as a more
  dangerous content type than the service declared.
