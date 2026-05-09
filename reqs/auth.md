# Authentication and authorization

The service is the OAuth authorization server its MCP and HTTP API
clients talk to. Google Workspace sits upstream as the actual identity
provider; clients never see Google directly.

## Posture

- R-1KML-5J0Q: the service exposes its own OAuth 2.1 authorization
  endpoints on the same origin as the MCP server
  (https://ouroboros.ai.metaspot.org). Clients are configured with
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

## Google federation

- R-4SH1-HQGP: when a user reaches the service's authorize endpoint,
  the service redirects them to Google so that Google performs the
  actual login.
- R-5LQM-O89D: the service is configured at deploy time with the
  single Workspace domain whose users are allowed. A user whose
  Google identity is outside that domain is rejected with a clear
  error message and no token is issued.
- R-68WP-XVCK: Google client credentials (client ID and secret) are
  supplied via environment configuration. They are never committed to
  the repository.
- R-ETP6-60VA: when the service redirects the user-agent to Google
  for the federated login, it generates a fresh unguessable `state`
  value and records it server-side, bound both to the in-flight
  authorize request it represents and to the originating browser
  session. The Google callback is accepted only when the returned
  `state` is recognized, unexpired, has not been consumed before,
  and was generated for the same browser session that is presenting
  the callback. A `state` that is missing, unknown, expired,
  already-consumed, or tied to a different session causes the
  callback to be rejected and no token chain to be issued. Used
  `state` values are single-use. This closes the login-CSRF window
  where an attacker pre-initiates a flow and induces a victim to
  complete the Google step on their behalf.

## Google federation is faked this iteration

Real Google Workspace setup (OAuth client credentials, allowed
Workspace domain) has not been performed yet, and is deferred to a
later iteration of the spec. In the meantime, the test suite stands
in for Google with a test double — most naturally a fake (a working
in-memory stand-in that returns realistic OAuth payloads), though
fakes, stubs, mocks, or whatever the implementation finds convenient
all qualify. Tests against the double are expected to pass, including
the ones that cover the Google-redirect and domain-enforcement
requirements above. This whole section is transient: when real Google
setup lands, retire these requirements (mint fresh IDs for any
replacement claims).

- R-CL63-P202: in this iteration, all interaction the service has
  with Google's OAuth endpoints is served by a test double rather
  than the real Google. The double returns payloads whose shape
  matches Google's documented OAuth/OIDC responses, so the service
  code exercises the same code paths it will use against the real
  Google. RSpec examples covering the Google-dependent requirements
  — notably R-4SH1-HQGP (redirect to Google) and R-5LQM-O89D
  (Workspace-domain enforcement) — drive the service against this
  double and are expected to pass.
- R-DBZW-40BC: live integration tests against the real Google are
  out of scope for this iteration. The seam between "the service's
  Google client" and "Google" is structured so that, when real
  Google setup lands, swapping the double for a real-Google
  integration does not require rewriting the specs that consume it.

## Tokens

- R-Z955-CD0I: tokens (both access and refresh) are opaque
  cryptographically-random strings. Each issued token has a
  corresponding server-side row recording at minimum: the token's
  kind (access or refresh), its owner, its chain membership (so
  rotation and revocation can act on siblings), issued-at,
  expires-at, used-at (refresh tokens only, for reuse detection),
  and revoked-at. Validation of an inbound bearer token is a single
  lookup against this store: the string itself carries no
  information. The expires-at column is the mechanism that enforces
  R-TNXJ-ZWQ0 and R-8UAA-YKR9; the used-at column is the mechanism
  for R-89K0-GH5G and R-9HGE-87UG; the revoked-at column is the
  mechanism for R-A26O-QBG9.
- R-CUUP-REQT: the token row stores a cryptographic hash of the token
  string, not the plaintext. The plaintext is returned to the client
  exactly once, at issue time, and is never persisted by the service.
  Inbound bearer tokens are validated by hashing the presented string
  with the same algorithm and looking up the row by that hash. A
  database leak therefore does not give an attacker any usable token.
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
- R-IS0W-S2H3: the service has a single configured resource
  identifier — the canonical external URL it is reached at — and
  honors the `resource` parameter (RFC 8707) on its authorize and
  token endpoints. Each issued access token's server-side row
  records the resource identifier the token was bound to at issue
  time. The MCP tool-call endpoint and the HTTP API accept a
  presented bearer token only when its recorded resource binding
  equals this service's identifier; a token whose binding is for
  any other resource, or that has no recorded binding, is rejected.
  This holds even though only one resource currently exists, so a
  token minted for some future second resource cannot be replayed
  against this one (and vice versa) the moment such a resource is
  introduced.
- R-DH2I-28CK: "matches" in R-IS0W-S2H3 means byte-for-byte equality
  against the one configured resource-identifier string the service
  was started with. The protected endpoints — the MCP transport
  endpoint (R-UK7D-Z0IZ) and `POST /counter/increment` (R-340Z-T6K2)
  — share that single resource identifier; neither endpoint derives
  its own resource identifier from the request URL, the path, or
  any per-endpoint convention. A token bound to the configured
  resource is therefore valid for every protected endpoint of this
  service simultaneously. Conversely, a token whose recorded
  resource string differs from the configured one in any way —
  trailing slash, scheme, host, port, path — is rejected. This
  closes the failure mode where the resource check appears to
  reject all tokens because the build agent computed a per-endpoint
  resource string (e.g. `http://host/mcp`) different from the one
  bound at issue (e.g. `http://host/`).
- R-E5GH-PN6G: at the moment a token row is written, the
  difference between the row's `expires_at` and `issued_at` equals
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
  and token's recorded resource binding does not match (R-IS0W-S2H3
  / R-DH2I-28CK). Each cause yields its own distinct
  `error_description` string; the service does not collapse two or
  more causes into a single placeholder reason such as `"expired"`.
  The exact wording of each description is HOW; the property is
  that a debugger reading the response can tell which of the named
  causes fired, and a conformant client can decide between
  refreshing (genuinely expired) and restarting the auth flow
  (revoked, wrong resource, unknown).
- R-QGB5-EMOO: any cookie the service uses to identify a browser
  session — including the one R-ETP6-60VA binds `state` against —
  is set with `Secure`, `HttpOnly`, and `SameSite=Lax` attributes.
  `SameSite=Lax` (rather than `Strict`) is required because the
  callback from Google is a cross-site top-level navigation that
  must carry the session cookie for the state-binding check to
  succeed. The session identifier is rotated when authentication
  state changes, in particular on successful completion of the
  federated Google login, so a session ID an attacker may have
  planted in the victim's browser before the flow began is no
  longer valid afterwards. Without these attributes, the browser-
  session premise R-ETP6-60VA depends on does not hold: a non-
  `HttpOnly` cookie is reachable from page-level script, a non-
  `Secure` cookie can travel in plaintext, and a missing `SameSite`
  lets cross-site requests ride the session.
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
