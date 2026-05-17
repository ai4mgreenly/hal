# NEXT — one transformation

## Relocate the OAuth-authorize request-validation tests to live with the federation-flow capability

**Outcome.** With the site-root tests relocated, the next capability
whose behavior tests still sit in the entry-point test monolith is the
OAuth/Google-federation flow. The subset of those tests that pin how
the OAuth authorize endpoint validates an incoming authorization
request — the flow and proof-key rules it enforces, the resource
constraint it enforces, the parameters it does and does not inject, and
that a client re-registered after a process restart still authorizes —
and that exercise the capability purely at the request/response
boundary, are relocated out of the entry-point test monolith into a
test file co-located with the federation-flow capability, constructed
and driven through that capability's existing public injected-input
surface, exactly as the site-root tests were relocated to live with the
site-root capability. They assert byte-for-byte the same observable
properties as before. No production source change and no behavior
change.

**Why.** With the site-root tests relocated, the federation-flow
capability is the next-largest block of behavior tests stranded in the
entry-point test monolith behind entry-point compatibility wrappers.
That capability was already extracted and exposes a public
injected-input surface, so its tests can be relocated by the same
proven pattern with no new production surface. Starting with the
authorize-validation tests — a single coherent behavioral cluster that
exercises the capability purely at the request boundary — establishes
and proves the relocated test seam for this capability while leaving
every observable behavior unchanged, and is the smallest slice that
fully accounts for the authorize-request-validation behavior.

## Scope

- Relocate exactly the federation-flow behavior tests that pin how the
  OAuth authorize endpoint validates an incoming authorization request
  — that it requires the authorization-code flow, requires the strong
  proof-key-for-code-exchange method, rejects a resource indicator that
  does not match the canonical resource, does not inject forced
  authentication parameters, and that a client re-registered after a
  process restart still authorizes — and that exercise the capability
  purely at the request/response boundary, needing nothing beyond the
  capability's existing public injected-input surface, the
  already-public dependency constructors, and the already-public
  identity-provider test double. They move into a test file co-located
  with that capability and assert byte-for-byte the same observable
  properties (the redirect or rejection, the status, the redirect
  location and its query parameters, the error response). Do not
  weaken, rename, skip, or delete any assertion in the move.
- This is the first relocation of this capability's tests. The
  relocated tests are constructed and driven through the capability's
  existing public injected-input surface — the same kind of seam the
  site-root tests use to drive the site-root capability — built only
  from the capability's already-public dependency constructors, the
  already-public identity-provider test double, and test-local
  configuration values. Introduce no new production symbol, no
  test-only seam, and no new cross-capability test double; the surface
  being driven already exists and must not be altered or extended.
- NOT part of this slice, left unchanged where they are: the
  federation-flow tests that drive the fully wired server rather than
  the capability in isolation; the test that constructs a custom
  identity-provider double or that resets or seeds another capability's
  internal state; the test that carries an authorization request
  through into the separate token-issuance endpoint and reaches another
  capability's internal storage; and the login-redirect and
  Google-callback behavior tests (separate later slices). Name in the
  result note exactly which tests moved and which remain.
- The entry-point compatibility wrappers that the not-yet-moved
  federation-flow tests still use stay in place this round; their
  deletion is a later round, gated on the remaining callers being
  resolved; do not delete them now.
- The relocated tests construct and drive the capability through its
  existing injected-input surface and public dependency APIs only — not
  through a global, the request context, application configuration, the
  fully wired server, or any other capability's internal storage.
- This is one coherent behavioral cluster and is expected to move
  whole. Move the largest coherent set that keeps the whole suite
  green; if it must nonetheless be split, move the largest coherent
  green slice and name precisely what moved and what remains so the
  next round continues it.
- This is a test-location change only: do not modify production source,
  reqs/ (the behavioral contract), or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes — the relocated authorize-validation tests asserting the
same observable properties from their new home, every other test
unchanged — the race-detector run passes, gofmt and go vet are clean
across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds; and the relocated tests
drive the capability solely through its existing public injected-input
surface, with no new production symbol or seam introduced and no other
capability's internal storage accessed.

## Result note — 2026-05-17

Completed the relocation of the OAuth authorize request-validation cluster into
`app-root/oauthflow/oauthflow_test.go`. The moved authorize-boundary coverage is:
`TestR_BAXT_SBU9_authorize_requires_code_flow_and_pkce`,
`TestR_JTTZ_CG5J_authorize_pkce_requires_s256`,
`TestR_126C_AM1E_authorize_omits_forced_auth_params`,
`TestR_4GRA_EGBY_authorize_rejects_mismatched_resource_at_issue_time`, and
`TestR_YRMT_B7LZ_restarted_client_store_still_authorizes`. The relocated tests
drive `oauthflow.Surface.HandleOAuthAuthorize` directly through its existing
public injected-input surface, using public `oauth` constructors, public
`googleidp.FakeProvider`, test-local resource/base URL values, and no production
source changes.

Left in `app-root/main_test.go`: the fully wired
`TestR_4SH1_HQGP_authorize_redirects_to_google` and
`TestR_1ERW_YD9G_authorize_rejects_mismatched_redirect_uri`; the token/auth-code
assertions in `TestR_JTTZ_CG5J_pkce_requires_s256`; the token endpoint side of
`TestR_4GRA_EGBY_resource_indicator_mismatch_rejected_at_issue_time`;
`TestR_WLUL_MZCD_oauth_omitted_resource_defaults_to_canonical`; login redirect,
Google callback, state-binding, full round-trip, and server-wired federation
tests; and the DCR persistence assertions in
`TestR_YRMT_B7LZ_dynamic_client_registration_survives_process_restart`.

Files changed: `app-root/main_test.go`, `app-root/oauthflow/oauthflow_test.go`,
and `NEXT.md`.

Verification: `GOROOT=/usr/local/go go test ./oauthflow -run
'TestR_(BAXT_SBU9|JTTZ_CG5J|126C_AM1E|4GRA_EGBY|YRMT_B7LZ)'` passed;
`GOROOT=/usr/local/go go test . -run 'TestR_(JTTZ_CG5J|4GRA_EGBY|YRMT_B7LZ)'`
passed; `GOROOT=/usr/local/go go test -race ./oauthflow -run
'TestR_(BAXT_SBU9|JTTZ_CG5J|126C_AM1E|4GRA_EGBY|YRMT_B7LZ)'` passed;
`GOROOT=/usr/local/go go vet ./...` passed; `awk 'length($0)>120 {print
FILENAME ":" FNR ":" length($0)}' $(git ls-files '*.go' ':!:vendor/*')`
produced no output; and `GOROOT=/usr/local/go CGO_ENABLED=0 go build -o
/tmp/hal-static-test .` passed. `GOROOT=/usr/local/go go test ./...` and
`GOROOT=/usr/local/go go test -race ./...` both failed only at
`TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
`.ralph/requirements-verified.jsonl` is permission denied, which the refactor
prompt identifies as out-of-scope local Ralph state.

Blockers/follow-up risks: broad full-suite and full-race verification remain
locally blocked by unreadable `.ralph/requirements-verified.jsonl`; focused
federation-flow verification for the relocated tests is green.
