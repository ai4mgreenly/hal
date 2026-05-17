# NEXT — one transformation

## Relocate the callback workspace-domain tests to live with the federation-flow capability

**Outcome.** The federation-flow behavior tests that pin how the
Google-callback step enforces the configured workspace domain — that a
Google identity outside the allowed workspace domain is rejected at the
callback with a forbidden response whose body names the cause, and that
an in-domain identity is accepted and a browser session is established
— are relocated out of the entry-point test monolith into the test
file already co-located with the federation-flow capability, driven
through that capability's existing public injected-input surface,
exactly as the authorize-validation tests were. They move whole — every
assertion travels with them — and assert byte-for-byte the same
observable properties. No production source change and no behavior
change.

**Why.** The federation-flow test relocation is underway; the
authorize-validation cluster already moved. The callback
workspace-domain tests are the next coherent slice and, unlike most of
the remaining callback tests, they are wholly self-contained at the
request/response boundary — they exercise only the
login-then-callback path with the public identity-provider test double
and an in-process web-session store, touching no token endpoint, no
fully wired server, and no test-only reset seam. Moving them whole
keeps this a single, low-risk, independently-verifiable claim and
continues emptying the entry-point test monolith toward retiring its
compatibility wrappers.

## Scope

- Relocate exactly the federation-flow behavior tests that pin callback
  workspace-domain enforcement — out-of-domain identity rejected at the
  callback with a forbidden response naming the cause; in-domain
  identity accepted with a browser session established — moving each
  test whole into the test file already co-located with the
  federation-flow capability. Assert byte-for-byte the same observable
  properties (the status, the error body's named cause, the
  established-session signal). Do not weaken, rename, skip, or delete
  any assertion; this is a whole move, not a split.
- Construct and drive the capability through its existing public
  injected-input surface only — the same seam the authorize-validation
  tests already use — built from the capability's already-public
  dependency constructors (including a public in-process web-session
  store), the already-public identity-provider test double, and
  test-local configuration values. Introduce no new production symbol,
  no test-only seam, and no new cross-capability test double; the
  surface being driven already exists and must not be altered or
  extended.
- NOT part of this slice, left unchanged where they are: the callback
  and state-binding tests that additionally span client registration,
  the authorize endpoint, the token-issuance endpoint, or another
  capability's internal state; the tests that drive the fully wired
  server or use a test-only reset or record seam; the login-redirect
  tests; and any multi-capability or full round-trip test. Name in the
  result note exactly which tests moved and which remain.
- The entry-point compatibility wrappers that the not-yet-moved
  federation-flow tests still use stay in place this round; their
  deletion is a later round, gated on the remaining callers being
  resolved; do not delete them now.
- Acceptance properties that must hold (they apply to this whole move
  defensively, and always thereafter):
  - Assertion preservation. Across the suite, the complete set of
    assertions and their expected values is unchanged: every assertion
    that existed before still runs somewhere afterward, byte-identical
    in expectation and behaviorally-equivalent in setup, none weakened,
    skipped, renamed-away, or deleted, and none duplicated in a way
    that changes meaning. Any material reproduced rather than moved
    must be an inert fixture, not a re-executed assertion.
  - Requirement-name traceability. Every requirement whose verified
    record names a specific test must continue to have a test of
    exactly that name in the suite. When a test is relocated or split,
    the function carrying the recorded name must continue to exist and
    remain a coherent, non-vacuous test; any new companion test takes a
    new name and never displaces the recorded one.
- This is one coherent behavioral slice and is expected to move whole.
  Move the largest coherent set that keeps the whole suite green; if it
  must nonetheless be split, move the largest coherent green slice,
  preserve every assertion across the split per the property above,
  keep the recorded-name function intact, and name precisely what moved
  and what remains.
- This is a test-location change only: do not modify production source,
  reqs/ (the behavioral contract), or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes — the relocated callback workspace-domain tests asserting
the same observable properties from their new home, every other test
unchanged — the race-detector run passes, gofmt and go vet are clean
across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds; the relocated tests drive
the capability solely through its existing public injected-input
surface with no new production symbol or seam and no other capability's
internal storage accessed; and both acceptance properties above hold.

## Result note — 2026-05-17

Completed the callback workspace-domain relocation into
`app-root/oauthflow/oauthflow_test.go`. The moved tests are
`TestR_5LQM_O89D_callback_rejects_off_domain_identity` and
`TestR_5LQM_O89D_callback_accepts_in_domain_identity`; both kept their
recorded names and still drive the login-then-callback path through
`oauthflow.Surface.HandleLogin` and `oauthflow.Surface.HandleGoogleCallback`
using public `oauth`, `websession`, and `googleidp` constructors. No production
source changed, no new production symbol or seam was introduced, and the
entry-point compatibility wrappers remain in place.

Left in `app-root/main_test.go`: callback/state-binding tests that also span
client registration, OAuth authorize, token issuance, fully wired server paths,
test-only reset/record seams, login redirects, origin dispatch, unverified
email rejection, web-session establishment, and multi-capability full
round-trips.

Files changed: `app-root/main_test.go`,
`app-root/oauthflow/oauthflow_test.go`, and `NEXT.md`.

Verification: `GOROOT=/usr/local/go /usr/local/go/bin/go test ./oauthflow -run
'TestR_5LQM_O89D_callback_(rejects_off_domain_identity|accepts_in_domain_identity)'`
passed; an adjacent focused root-package command covering `TestR_ETP6_60VA`,
`TestR_EMW1_D8A0`, `TestR_CXJ2_R3BN`, and `TestR_MUZJ_RD0L` passed; the same
two focused commands with `-race` passed; `GOROOT=/usr/local/go
/usr/local/go/bin/go vet ./...` passed; gofmt was clean; the Go source
line-length scan produced no output; and `GOROOT=/usr/local/go CGO_ENABLED=0
/usr/local/go/bin/go build -o /tmp/hal-static .` passed. `GOROOT=/usr/local/go
/usr/local/go/bin/go test ./...` failed only at
`TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
`.ralph/requirements-verified.jsonl` is permission denied, which the refactor
prompt identifies as out-of-scope local Ralph state; all non-root packages
passed, and `GOROOT=/usr/local/go /usr/local/go/bin/go test -race` across
non-root packages passed.

Blockers/follow-up risks: broad root-package full-suite verification remains
locally blocked by unreadable `.ralph/requirements-verified.jsonl`; focused
federation-flow relocation checks and adjacent callback behavior checks are
green.
