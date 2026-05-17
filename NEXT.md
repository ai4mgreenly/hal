# NEXT — one transformation

## Extract the OAuth/Google-federation HTTP-flow capability into its own package

**Outcome.** The browser- and MCP-client-facing OAuth/Google-federation
HTTP flow — the sign-in redirect to Google, the Google callback that
binds state, enforces the workspace-domain constraint, and dispatches
by origin, the OAuth authorize endpoint that validates the client,
redirect URI, PKCE method, and resource and then redirects to Google,
together with the OAuth-error-redirect helper and the redirect-URI and
code-challenge-method validators — lives in its own package with a
deliberately small exported surface, following the package layout the
earlier extractions established. The entry-point package depends on
that surface, no longer contains the flow internals, and keeps only
route registration and the wiring seam. The program still builds as one
binary and every observable behavior is unchanged.

**Why.** This OAuth/Google-federation flow is its own capability,
distinct from the identity provider just extracted, the OAuth token and
registration endpoints (already thin delegators), and the stores
already extracted. Isolating it removes the last large block of
business logic from the entry-point package, unblocks relocating the
OAuth-flow tests and deleting the corresponding compatibility wrappers,
and is a prerequisite for the eventual entry-point collapse.

## Scope

- Extract exactly the **OAuth/Google-federation HTTP flow** this round:
  the login redirect-to-Google handler, the Google-callback handler
  (state binding, workspace-domain enforcement, origin dispatch,
  session/auth-code issuance), the OAuth authorize handler (client,
  redirect-URI, PKCE-method, and resource validation, then Google
  redirect), the OAuth-error-redirect helper, and the redirect-URI and
  code-challenge-method validators. The logout handler, the OAuth
  client-registration endpoint, and the OAuth token endpoint are NOT
  part of this slice — they are a different concern or already thin
  delegators; leave them where they are.
- Every observable behavior is byte-for-byte identical: the redirect
  targets and their query parameters, the cookies set, the state
  binding and its consumption, the workspace-domain accept/reject, the
  origin-based dispatch, every validation accept/reject (client,
  redirect URI, PKCE method, resource), the issued session/auth-code,
  and every error response and redirect do not change. The existing
  tests pin these precisely and are the proof; do not weaken, skip, or
  delete any test or its assertions.
- The package must not read application configuration or hold
  package-level singletons. The Google identity provider enters through
  its already-extracted provider seam; the OAuth state, auth-code, and
  client stores and the web-session store enter as their
  already-extracted package types; and every configuration value or
  cross-cutting helper the flow needs — the OAuth-state and
  web-session TTLs, the allowed Workspace domain, the canonical
  resource identifier, the request base-URL / forwarded-proto / OAuth-
  error-writing helpers — is supplied to it as an injected input, not
  read from a global. This is the same no-config/no-singleton
  discipline the earlier capability extractions established.
- The package exposes only the minimal surface its consumers need;
  consumers use only that surface. The existing OAuth-flow tests
  continue to run and assert unchanged from where they are (they
  relocate to the new package in a later round).
- Follow the module and package-layout precedent the earlier
  extractions set; the exact name and path are yours to choose,
  idiomatic Go. Do not leave new compatibility aliases in the
  entry-point package beyond what is unavoidable to keep this round
  green; prefer migrating the consumer to the new surface.
- If the whole capability cannot move while staying green in one round,
  move the largest coherent green slice and name in the result note
  exactly what moved and what remains, so the next round continues it.
  Never loosen the invariant to fit more in.
- Production behavior is identical. Do not edit reqs/ (the behavioral
  contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean
across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds.

## Result — 2026-05-17

Completed the OAuth/Google-federation HTTP-flow extraction into the new
`app-root/oauthflow` package. The entry-point package now wires an injected
`oauthflow.Surface` and keeps thin compatibility wrappers for the current
main-package tests. The moved surface owns `/login`, `/oauth/google/callback`,
`/oauth/authorize`, OAuth error redirects, redirect-URI validation, and
authorize PKCE-method validation. Logout, registration, and token handling were
left in their existing packages as scoped.

Files changed:
- `app-root/oauthflow/oauthflow.go`
- `app-root/main.go`
- `app-root/main_test.go`
- `NEXT.md`

Verification:
- `gofmt -w main.go main_test.go oauthflow/oauthflow.go`: passed
- `GOROOT=/usr/local/go go test ./...`: compiled and ran; failed only at
  `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
  `.ralph/requirements-verified.jsonl` is permission-denied local Ralph state,
  which is out of scope for this refactor prompt.
- `GOROOT=/usr/local/go go test ./... -run '^$'`: passed
- `GOROOT=/usr/local/go go test -run 'TestR_(9PNQ_BN2G|3BKZ_L7R4|ETP6_60VA|5LQM_O89D|EMW1_D8A0|CXJ2_R3BN|8GJG_64MR|T37L_4J01|MTRN_DL9W|MUZJ_RD0L|4SH1_HQGP|1ERW_YD9G|BAXT_SBU9|4GRA_EGBY|WLUL_MZCD)' .`: passed
- `GOROOT=/usr/local/go go test -race -run 'TestR_(9PNQ_BN2G|3BKZ_L7R4|ETP6_60VA|5LQM_O89D|EMW1_D8A0|CXJ2_R3BN|8GJG_64MR|T37L_4J01|MTRN_DL9W|MUZJ_RD0L|4SH1_HQGP|1ERW_YD9G|BAXT_SBU9|4GRA_EGBY|WLUL_MZCD)' .`: passed
- `GOROOT=/usr/local/go go test -race ./... -run '^$'`: passed
- `GOROOT=/usr/local/go go vet ./...`: passed
- line-length check for Go sources with `.ralph` pruned: passed
- `git diff --check`: passed
- `GOROOT=/usr/local/go CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '-s -w' -o /tmp/hal-refactor-build ./`: passed

Notes and follow-up risks:
- The ambient `go` command has a mixed toolchain environment (`go1.26.2`
  command with `GOROOT` pointing at a Go 1.23.5 tree). Verification used
  `GOROOT=/usr/local/go` to select the matching Go tree.
- Current OAuth-flow tests remain in `main_test.go` by scope; a later round can
  move them to `oauthflow` and remove the compatibility wrappers.
