# Development environment

Properties of the dev / CI / runtime environment that the build agent
must satisfy. The "current" Go toolchain version is stated here
explicitly so the spec has a single source of truth. When upstream
releases a new line we want to adopt, edit these requirements (and
mint fresh IDs for material changes).

## Versions

- R-35T7-Z8EF: the project pins one specific Go toolchain version in
  `go.mod` (via the `go` and `toolchain` directives). That file is
  the single source of truth for the exact toolchain version in use.
- R-3714-D054: the pinned Go toolchain is exactly `go1.26.2`.

## Smoke tests for the runtime

These tests exist so that "I built this with the wrong Go" or "this
machine doesn't have Go installed" surfaces as a loud, specific
failure instead of confusing downstream errors.

- R-3890-QRVT: an automated test fails when the version reported by
  the Go runtime that built the binary under test
  (`runtime.Version()` at the call site of the test) does not match
  the toolchain version pinned in `go.mod` per R-35T7-Z8EF.

## Bootstrap

- R-SDDJ-SBIN: a fresh checkout, given a working Go toolchain of the
  correct version, can be brought to a passing test suite by running
  a single documented command. Any further manual steps are a bug.

## Testing

- R-727Q-1PV4: the project's tests are written using Go's standard
  `testing` package and live in `*_test.go` files alongside the code
  they exercise. `go test ./...` runs the entire suite. When a test
  function verifies one or more specific requirements, each verified
  requirement's ID appears in the test function's name in the form
  `R_XXXX_XXXX` (underscores in place of the canonical dashes, as
  required by Go identifier syntax), giving function names of the
  shape
  `TestR_XXXX_XXXX_descriptive_name(t *testing.T)`.
  Multiple IDs may appear in the same function name, each in the
  same form, separated from each other and from the descriptive
  suffix by single underscores. A `grep` for any single requirement
  ID across the source tree (with the dashes either preserved or
  swapped for underscores, at the grepper's discretion) returns
  every test that verifies that requirement.
- R-H74C-7WFF: not every test function must carry a requirement ID.
  Helper tests, fixture builders, and exploratory tests are allowed
  to be un-tagged. The trace is one-way: every R-XXXX-XXXX claim
  that has been verified in code is locatable by ID, but the test
  suite may also contain functions that don't map to any single
  requirement.
- R-70ZT-NY4F: `go test ./...` invoked at the repo root runs the
  project's full test suite and exits non-zero if any test fails.
  The test suite makes no outbound network calls — every test that
  would otherwise reach Google is satisfied by the test double
  R-VF61-2Y6I defines.
- R-195O-JBGX: `go test -race ./...` invoked at the repo root
  runs the same full test suite under Go's race detector and exits
  zero — no test reports `DATA RACE`, no test reports
  `race detected during execution of test`, and no test fails. A
  data race the race detector surfaces is a **defect** regardless
  of where it surfaces or how it got there: which test function
  the detector happened to fire inside of does not matter; whether
  the race is "pre-existing" from an earlier iteration does not
  matter; whether the iteration currently being worked on
  introduced the race does not matter; whether the race is
  considered "benign" by inspection does not matter. The property
  is a single binary outcome — the race-detector run exits clean —
  and that property must hold on every iteration. A build agent
  encountering a race-detector failure in any test, in any
  package, must fix the race before marking any requirement
  verified; a race-detector failure is not grandfathered, is not
  carried over in handoff notes as acceptable baggage, and is not
  excused by the iteration's scope. The intent: a service whose
  concurrent-request property R-T4FH-IAQQ pins and whose
  concurrent-counter property R-TOI0-0Z8X pins cannot meet either
  property in the presence of unsynchronized access — a
  race-detector failure is direct evidence that one of those
  promises is at risk, and shipping past it silently is a defect
  the spec now names explicitly.

## Binary

The deliverable is a single binary named `hal`. The repo does not
ship wrapper shell scripts for routine operations; the binary's
subcommands are the operational surface.

- R-74NI-T9CI: the `hal` binary exposes exactly three subcommands:
  `serve`, `reset`, and `version`. Invoked with no subcommand, or
  with an unknown subcommand, the binary prints a short usage
  summary listing these three subcommands and exits non-zero.
- R-75VF-7137: `hal serve` starts the HTTP server. The subcommand
  accepts three flags, each with a default:
  - `--port` (default `3000`) — the TCP port to listen on.
  - `--ip` (default `127.0.0.1`) — the local interface to bind to.
  - `--db` (default `./hal.db`) — the path to the SQLite database
    file the service uses for the counter, OAuth tokens, web
    sessions, and any other persistent state.
  When invoked with defaults, the running service listens on
  `127.0.0.1:3000` (consistent with R-FA71-BAO6) and uses
  `./hal.db` relative to the current working directory. The
  observable property: a developer running `./hal serve` in a fresh
  checkout, with no environment variables set and no flags passed,
  reaches a running service against `./hal.db`.
- R-FA71-BAO6: when started via `hal serve` with defaults (no
  `--port` flag overriding it), the service listens on TCP port
  3000.
- R-PVA6-Q6OB: the locally-launched service speaks plain HTTP, not
  HTTPS. TLS termination is a deployment concern handled in front
  of the service at https://hal.ai.metaspot.org; the application
  process itself does not terminate TLS, locally or in production.
  The test suite does not depend on TLS being available.
- R-773B-KSTW: when `hal serve` brings the service up, the database
  schema is current before the service begins accepting requests.
  A fresh checkout (no database file present), a checkout whose
  database file already exists with the current schema, and a
  checkout whose database file already exists with an older schema
  shape that is a subset of the current one all reach the same end
  state: a running service whose first inbound request is served
  against an up-to-date schema. The mechanism is `CREATE TABLE IF
  NOT EXISTS` (and analogous `CREATE INDEX IF NOT EXISTS`)
  executed at every start; the project does not maintain a
  migrations directory, a schema-version table, or any other
  schema-evolution machinery. When a column or table needs to
  change shape in a way `IF NOT EXISTS` cannot achieve, the dev
  loop is "edit the schema in code, run `hal reset`, restart"
  per R-78B7-YKKL — not an in-place migration.
- R-78B7-YKKL: `hal reset` brings the database the local service
  uses (per the same `--db` path R-75VF-7137 names; default
  `./hal.db`) back to the state of a fresh, never-launched
  checkout: every persisted record gone, no schema present. The
  first `hal serve` after `hal reset` reaches the same end state
  it reaches on a never-launched checkout per R-773B-KSTW: schema
  created from scratch, no application rows. The subcommand's
  scope is the file at `--db` only; it never targets any deployed
  environment. The choice between deleting the file outright and
  truncating it in place is HOW; the property is that the next
  `hal serve` sees the same starting state a never-touched
  checkout sees.
- R-79J4-CCBA: `hal version` prints version information for the
  binary — at minimum the project's own version string — to
  standard output and exits zero. The subcommand makes no network
  calls and does not require the database file to exist. Operators
  use this subcommand as an `ExecStartPre`-style sanity check
  before invoking `hal serve`, and as a triage signal when a
  deployed binary's identity is in doubt.

## Makefile

The repo ships a Makefile at the application root as the
developer-facing build interface. It is a convenience over the
underlying Go toolchain, not a substitute for it — `go build` and
`go test ./...` continue to work directly. The Makefile exists so a
new contributor can build, test, and install the binary without
memorizing the toolchain's exact invocations.

- R-8LUR-X0YH: the application root contains a file named
  `Makefile` exposing exactly three targets — `build`, `test`, and
  `install` — with `build` as the default. Running `make` with no
  argument from the application root runs the `build` target. Each
  target is reproducible: invoking it twice in a row against the
  same source tree succeeds twice and produces equivalent results.
  Targets the developer does not name (`clean`, `lint`, `release`,
  etc.) are out of scope; adding one is a spec edit, not an
  implementation choice. The Makefile surfaces the underlying
  tool's exit code, so a failing `go build` or `go test` produces
  a failing `make` invocation.

- R-8OAK-OKFV: `make build` produces the `hal` binary at the
  application root, with the deliverable properties R-34LB-LGNQ
  pins (single statically-linked binary, `CGO_ENABLED=0`, no
  shared-library / C-runtime / language-runtime dependency, target
  `linux/amd64`). `make test` runs the full test suite that
  R-70ZT-NY4F names — equivalent to `go test ./...` at the repo
  root — and exits non-zero if any test fails. Neither target
  reaches the network beyond what the Go toolchain itself needs
  for module resolution (already satisfied on a fresh checkout
  per R-7AR0-Q41Z), and the test target makes no outbound calls
  per R-70ZT-NY4F's no-network contract.

- R-8PIH-2C6K: `make install` places the `hal` binary at
  `~/.local/bin/hal` — `$HOME/.local/bin/hal`, following the XDG
  user-binary convention — with execute permission set so the
  invoking user can run it directly. The target creates
  `~/.local/bin/` if it does not already exist (the `mkdir -p`
  semantics; an existing directory is not an error). When the
  build artifact is out of date or absent, the install target
  rebuilds it first — equivalently, `make install` from a fresh
  checkout produces a working `~/.local/bin/hal` without a
  separate prior `make build`. The install target does not write
  anywhere outside `~/.local/bin/`; in particular it does not
  touch `/usr/local/bin`, system paths, or any other directory
  on the developer's machine.

## Operational baseline

These properties apply to the running service and to the build
agent's iteration discipline as a whole, not to any single feature.
They close the class of "the spec didn't spell it out, so the build
didn't do it" defects that would otherwise live in the seams between
feature requirements.

- R-K7DK-LSJ6: when the locally-launched service receives SIGINT
  or SIGTERM, the process exits within 1 second, regardless of
  what requests are in flight. Open responses — including any
  long-lived connection the live-update channel R-FZC6-H2SB
  maintains — are dropped; the client's connection simply closes.
  Any transport the build agent picks for any feature must remain
  compatible with this deadline; a transport whose handler blocks
  process exit on an in-flight request is disqualified. The
  operator experience: Ctrl-C in the launching terminal returns a
  prompt promptly, every time. The intent: a graceful shutdown
  that waits for long-lived connections cannot complete on its
  own — the system explicitly opts out of that posture in favor
  of fast termination, on the understanding that dropped clients
  reconnect on their next attempt.

- R-VKZD-UKVS: every endpoint that reads a client-supplied request
  body enforces a fixed maximum body size before parsing. Requests
  whose bodies exceed the endpoint's limit are rejected with an
  error response and no state change. The limit is high enough for
  normal OAuth, Dynamic Client Registration, token, counter, and
  web-session form payloads, and low enough that one request cannot
  force the service to buffer unbounded input. This applies at
  minimum to Dynamic Client Registration, the OAuth token endpoint,
  web-session-authenticated revoke actions, and any future endpoint
  that parses JSON or form data.

- R-8IPO-FZ7T: every documented HTTP endpoint accepts only the HTTP
  method or methods named for that endpoint. A request to a
  documented path with any other method is rejected with HTTP 405
  Method Not Allowed, does not perform the endpoint's action, and
  does not fall through to the index page, OAuth flow, MCP
  transport, or any other handler. The rejection includes an
  `Allow` header naming the accepted method or methods for that
  path. This applies to the counter API, OAuth endpoints, metadata
  endpoints, web-session actions, live-update streams, static
  stylesheet, and MCP transport path.

- R-7MLK-O6I5: browser-facing actions that change authenticated
  state use POST, never GET. This applies to logout/session
  revocation, MCP token-chain revocation from the agents block,
  counter mutations, and any future browser action that creates,
  revokes, mutates, or deletes server-side state. A GET request to
  one of these action paths does not perform the action; it is
  rejected according to the documented method-rejection behavior.
  Navigational GETs may still render pages or start OAuth redirects,
  but they do not terminate sessions, revoke token chains, or mutate
  the counter.

- R-X0O1-BJ2H: an HTTP request whose path does not match a
  documented endpoint returns HTTP 404 Not Found and does not
  perform any action. Unknown paths do not fall through to the
  index page, OAuth endpoints, MCP transport, static stylesheet,
  live-update streams, or counter API behavior. The root path `/`
  remains the only path that renders the browser index page.

- R-K9TD-DC0K: every R-XXXX-XXXX requirement in the spec is
  satisfied by at least one automated test that fails when the
  requirement is violated, identified per R-727Q-1PV4. A
  requirement is considered complete only when such a test
  exists, runs against the currently-built service, and passes.
  Once a requirement is complete, no subsequent iteration is
  allowed to land code that causes its test to fail again; an
  iteration whose changes red any previously-green
  requirement-tagged test is rolled back on that iteration, not
  committed. The intent: the test suite is the spec's enforcement
  surface. "I added a feature but broke another" is a defect, not
  an acceptable trade. This extends R-727Q-1PV4 and R-H74C-7WFF —
  the ID-tagging convention they pin remains, and helper /
  fixture / exploratory tests are still allowed to be un-tagged;
  this requirement adds coverage and no-regression on top of the
  one-way trace.

## Startup banner

When the service starts, the operator wants a quick, visible
confirmation of which configuration the process is actually running
with — not the configuration the operator's shell thinks it set, the
configuration the process itself sees. The startup banner is that
confirmation; it makes "I set the env var but hal doesn't see it"
self-diagnosing.

- R-NQ3G-K0CQ: when `hal serve` starts, before the listener begins
  accepting requests, the service prints to standard error a
  startup banner that lists every environment variable hal
  consults. Each variable appears on its own line in the form
  `NAME=value`. A variable whose effective value came from an
  operator-supplied environment entry shows that value (subject to
  redaction per R-NRBC-XS3F). A variable that was not set and is
  honored as "unset" with a built-in default shows the default
  value followed by ` (default)`. A required variable that was not
  set never reaches the banner — startup has already failed with
  the fail-loudly contract R-LWCN-ZBXO pins. The banner is written
  to stderr rather than stdout because stdout is reserved for
  access log lines per R-D1IO-90H0.

  The variables in scope today: `GOOGLE_CLIENT_ID`,
  `GOOGLE_CLIENT_SECRET`, `GOOGLE_WORKSPACE_DOMAIN` (R-ANRQ-04PK),
  and `HAL_RESOURCE_IDENTIFIER` (R-791Y-3ROQ). Any future
  environment variable the service begins consulting is added to
  the banner in the same pass that introduces it.

- R-NRBC-XS3F: env vars the spec classifies as secret are redacted
  in the banner so only the last three characters of the value
  appear, preceded by the literal prefix `***` — e.g. a secret
  whose value is `GOCSPX-abcXYZ` prints as
  `GOOGLE_CLIENT_SECRET=***XYZ`.
  A secret whose value is shorter than eight characters is printed
  as `***` with no trailing characters, so an accidentally-short
  secret cannot be substantially reconstructed from the banner.
  The set of secrets today is exactly one variable:
  `GOOGLE_CLIENT_SECRET`. `GOOGLE_CLIENT_ID` is not a secret per
  the OAuth 2.0 convention (it appears in browser-visible
  authorization URLs) and prints verbatim. `GOOGLE_WORKSPACE_DOMAIN`
  and `HAL_RESOURCE_IDENTIFIER` are configuration identifiers, not
  secrets, and print verbatim. Adding a new secret to the service
  means adding the variable name to this requirement's secret
  list rather than carrying the secret/non-secret distinction in
  scattered code.

- R-PLTU-G0FD: the startup banner also includes a line naming the
  database file `hal serve` is using — the file at the path
  R-75VF-7137 names (the `--db` flag's effective value; default
  `./hal.db`). The line is `db=<path>`, with the path shown in its
  resolved absolute form so the operator sees exactly which file
  on disk the running process is bound to (a relative default
  resolves against the working directory `hal serve` was launched
  from). The line sits inside the banner R-NQ3G-K0CQ pins —
  written to stderr, emitted before the listener begins accepting
  requests — between the env-var lines and the trailing blank
  line R-NSJ9-BJU4 names. The database path is not classified as
  a secret and is not redacted; it is operational configuration,
  not credential material.

- R-NSJ9-BJU4: the banner ends with a single blank line that
  separates it from the access log lines R-D0AR-V8QB will begin
  emitting once the listener is up. The banner is emitted exactly
  once per `hal serve` invocation, at startup; reconfiguration at
  runtime is out of scope, so the banner does not reappear when
  the operator changes an env var without restarting.

## Access logging

The running service emits one access log line per accepted HTTP
request, formatted the way an operator already knows how to read.
This is the only diagnostic logging surface the spec pins; structured
event emission, debug-level application logging, and metrics are out
of scope until a requirement adds them.

- R-D0AR-V8QB: every HTTP request the service accepts produces
  exactly one access log line. Requests that the service answered
  with an error status count; requests whose response closed
  mid-stream count. The acceptance bar is "the listener handed an
  HTTP-framed request to the service"; raw-TCP events that never
  produced a request line do not produce access log lines.

- R-D1IO-90H0: access log lines are written to the process's
  standard output stream. The application process does not open or
  rotate its own log file; operators redirect stdout to land the
  lines wherever they want (a file, the systemd journal, a log
  shipper). At steady state — after `hal serve` has reported its
  bind and before the process begins shutdown — every line written
  to stdout is an access log line conforming to the format
  R-D2QK-MS7P pins, so a downstream consumer that parses one access
  log line per stdout line is correct by construction.

- R-D2QK-MS7P: each access log line is in NCSA Combined Log Format,
  with fields in this order, separated by single ASCII spaces:
  (1) client host, (2) RFC 1413 ident, (3) authenticated user,
  (4) bracketed timestamp, (5) double-quoted request line,
  (6) HTTP status code, (7) response body byte count,
  (8) double-quoted referer, (9) double-quoted user-agent. The
  RFC 1413 ident field is always the single character `-` (the
  service does not consult ident). Unquoted fields whose value is
  genuinely absent appear as `-`; the quoted fields (5, 8, 9) are
  always quoted and carry `-` between the quotes when absent.

- R-D3YH-0JYE: the timestamp field records the moment the service
  began handling the request — the wall-clock instant the request
  line was parsed — formatted as `[%d/%b/%Y:%H:%M:%S %z]` using the
  C-locale English month abbreviations, e.g.
  `[12/May/2026:14:03:21 -0500]`. The line itself is emitted after
  the response handler completes so the status and byte-count
  fields reflect the response actually sent, but the timestamp
  does not shift to emission time.

- R-D56D-EBP3: the authenticated-user field carries the email
  address bound to the request when the service authenticated it,
  and `-` otherwise. A request authenticated by a web session
  (R-CXJ2-R3BN, R-SLGL-B5B4) logs the email recorded on that
  session. A request authenticated by a bearer access token
  (R-SAK8-WB9W) logs the email recorded on the token's underlying
  identity. A request that reached an unauthenticated route, or
  reached an authenticated route without satisfying its auth bar,
  logs `-`. The field never contains an ASCII space; an email
  containing whitespace (which Google Workspace email addresses
  cannot, but the field is defined for the general case) logs as
  `-` rather than splitting the line.

- R-D6E9-S3FS: the client-host field is the external IP address
  the request appears to originate from. When the request carries
  an `X-Forwarded-For` header — the production posture per
  R-PVA6-Q6OB places a TLS-terminating proxy in front of the
  service — the field is the first comma-separated token of that
  header, stripped of surrounding whitespace. With no
  `X-Forwarded-For`, the field is the remote address of the TCP
  peer the listener accepted. Absent peer information logs as `-`;
  the field is never empty and never the literal string `unknown`.

- R-D8U2-JMX6: the three double-quoted fields (request line,
  referer, user-agent) are emitted with Apache's escaping
  discipline: an embedded double-quote becomes `\"`, an embedded
  backslash becomes `\\`, and bytes that are not printable ASCII
  become `\xHH` hex escapes. The fields are always quoted, even
  when their value is `-`. The request-line field is the verbatim
  HTTP request line the service received (method, request-URI,
  protocol), subject only to the redactions R-DA1Y-XENV pins.

- R-DA1Y-XENV: access log lines never contain a usable credential.
  The `Authorization` request header is not logged — this is the
  no-token-in-logs property R-SAK8-WB9W already pins, restated
  here so the access log is named explicitly in the surface that
  rule governs. In addition, the OAuth authorization-response
  query parameters that arrive at the Google callback — `code`
  and `state` — appear in the logged request line with their
  values replaced by the literal string `REDACTED`; the parameter
  names remain so the path is recognizable, e.g.
  `"GET /oauth/google/callback?code=REDACTED&state=REDACTED HTTP/1.1"`.
  Any future query parameter the spec marks as sensitive is
  added to this redaction set rather than logged raw.

- R-DB9V-B6EK: a long-lived streaming response — the live-update
  channel R-FZC6-H2SB is the only such response the spec currently
  names — produces its access log line when the connection closes
  (whether the client disconnected, the response handler returned,
  or the process tore the connection down on shutdown per
  R-K7DK-LSJ6). The status field carries the HTTP status the
  service sent (typically 200); the byte-count field carries the
  total response body bytes streamed before close.

## Dependencies and tooling

- R-7AR0-Q41Z: Go module dependencies are managed via `go.mod`
  and `go.sum`. Both files are committed to the repository. A
  fresh checkout produces identical dependency versions on every
  machine until someone intentionally updates a dependency
  (e.g. via `go get`). Every direct dependency in `go.mod` resolves
  to one specific version; the `go.sum` file records cryptographic
  checksums so a tampered or substituted dependency fails the build.
- R-73FM-FHLT: lint discipline is enforced by the standard Go
  toolchain. The project's full source tree satisfies two checks
  on every commit: (1) `gofmt -l ./...` produces empty output —
  every Go file is already in canonical `gofmt` form; and (2)
  `go vet ./...` exits zero — no vet warnings. Either check
  failing is a lint failure. No third-party linter is required to
  pass the build; if the project later adopts one
  (e.g. `golangci-lint`), this requirement is replaced with a new
  ID rather than amended in place.
- R-7NWT-PODV: source-file line length is capped at 120
  characters. A Go source file containing any line longer than 120
  characters is a lint failure. (`gofmt` does not enforce a line
  length, so this is enforced by a separate check — e.g. a small
  custom test, or a linter the project explicitly opts into; the
  enforcement mechanism is HOW.)
- R-7BYX-3VSO: development tooling that is not invoked by the
  running service — the formatter (`gofmt`), the static checker
  (`go vet`), and any other Go-toolchain-shipped tools the build
  uses — is not separately version-pinned. It travels with whatever
  Go toolchain R-35T7-Z8EF selects. A toolchain release that
  legitimately tightens a `vet` check or a `gofmt` rule, and so
  causes a previously-green source tree to fail R-73FM-FHLT, is
  acceptable and expected; the response is to fix the source, not
  to pin the tooling.
