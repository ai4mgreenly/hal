# NEXT — one transformation

## Split the origin/MCP-context state-record test: relocate its web-origin scenario to the federation-flow capability

**Outcome.** One federation-flow test today verifies, in a single
function spanning two scenarios, that the OAuth state record persists
the originating request's origin discriminator and its MCP
authorize-context. Its web-origin scenario — a sign-in request arriving
from the web origin records an OAuth state whose origin discriminator
is the web value and which carries no MCP authorize-context — is split
out and relocated into the test file co-located with the
federation-flow capability, driven through that capability's existing
public injected-input surface and asserted against the capability's
public dependency APIs on the test's own injected stores. The
MCP-origin scenario, which additionally spans dynamic client
registration and the authorize endpoint, stays where it is, coherent
and self-contained. Every assertion is preserved byte-for-byte across
the split. No production source change and no behavior change.

**Why.** The federation-flow test relocation is underway; the
whole-movable callback and authorize tests have already moved. The
remaining callback/state tests each bundle more than one concern, so
they are untangled by within-test splitting — moving the portion that
is self-contained at the request boundary and leaving the portion that
depends on other capabilities. This test is the lowest-risk first such
split: its web-origin scenario is fully self-contained at the sign-in
boundary, while its MCP-origin scenario depends on client registration
and the authorize endpoint. Splitting cleanly separates two
independently-verifiable claims, continues emptying the entry-point
test monolith toward retiring its compatibility wrappers, and proves
the within-test-split pattern under explicit, checkable acceptance
properties.

## Scope

- Split exactly the federation-flow test that verifies the OAuth state
  record persists the originating request's origin discriminator and
  its MCP authorize-context. Relocate its web-origin scenario — a
  sign-in request arriving from the web origin records a state whose
  origin discriminator is the web value and which carries no MCP
  authorize-context — into the test file co-located with the
  federation-flow capability. Leave its MCP-origin scenario unchanged
  where it is; that scenario additionally exercises dynamic client
  registration and the authorize endpoint and must remain coherent and
  non-vacuous on its own setup and assertions.
- Drive the relocated scenario through the capability's existing public
  injected-input surface only — the same seam the already-relocated
  federation-flow tests use — and assert the recorded state against the
  capability's public dependency APIs on the test's own injected
  stores. Introduce no new production symbol and no test-only seam. The
  relocated scenario must not reach a test-only reset or record seam, a
  process clock reachable only as an entry-point global, the fully
  wired server, or another capability's internal (unexported) storage.
  (Inspecting a public dependency store's public API on the test's own
  injected instance is ordinary permitted use, not a violation.)
- This round is a within-test split governed by the acceptance
  properties below: the two scenarios are independently-verifiable
  claims. Relocate the web-origin scenario as the coherent green slice
  and keep the residual coherent and non-vacuous; do not attempt to
  also move the MCP-origin scenario.
- Acceptance properties that must hold:
  - Assertion preservation. Across the suite, the complete set of
    assertions and their expected values is unchanged: every assertion
    that existed before still runs somewhere afterward, byte-identical
    in expectation and behaviorally-equivalent in setup, none weakened,
    skipped, renamed-away, or deleted, and none duplicated in a way
    that changes meaning. Any material reproduced rather than moved
    must be an inert fixture, not a re-executed assertion.
  - Requirement-name traceability. Every requirement whose verified
    record names a specific test must continue to have a test of
    exactly that name in the suite. When a test is split or relocated,
    the function carrying the recorded name must continue to exist and
    remain a coherent, non-vacuous test; any new companion test takes a
    new name and never displaces the recorded one.
- NOT part of this slice, left unchanged where they are: the other
  callback/state tests whose movable portion would first require
  decoupling a test-only reset or record seam, or a process clock
  reachable only as an entry-point global, or that drive the fully
  wired server, or that bundle large multi-path fixtures; and the
  login-redirect tests (which require a separate decouple-rewrite off
  the fully wired server first). Name in the result note exactly which
  test was split, which scenario moved, and what remained.
- The entry-point compatibility wrappers that the not-yet-moved
  federation-flow tests still use stay in place this round; their
  deletion is a later round, gated on the remaining callers being
  resolved; do not delete them now.
- This is a test-location and test-structure change only: do not modify
  production source, reqs/ (the behavioral contract), or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes — the relocated web-origin scenario asserting the same
observable properties from its new home, the residual MCP-origin
scenario unchanged in place and still coherent, every other test
unchanged — the race-detector run passes, gofmt and go vet are clean
across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds; the relocated scenario
drives the capability solely through its existing public injected-input
surface and public dependency APIs, with no new production symbol or
seam and no test-only seam, entry-point global, fully wired server, or
other-capability internal storage reached; and both acceptance
properties above hold.

## Result note — 2026-05-17

- Completed one split of `TestR_MTRN_DL9W_state_record_carries_origin_and_mcp_context`.
  Moved the `web_origin_records_have_origin_web_and_nil_mcp_context` scenario to
  `oauthflow` as
  `TestR_MTRN_DL9W_web_origin_records_have_origin_web_and_nil_mcp_context`.
  The original named test remains in `main_test.go` with the
  `mcp_origin_records_carry_byte_for_byte_authorize_context` scenario.
- Files changed: `app-root/oauthflow/oauthflow_test.go`,
  `app-root/main_test.go`, `NEXT.md`.
- Verification from `app-root/` used `GOROOT=/usr/local/go` because the shell
  had `GOROOT` set to an older Go tree while `go` itself is 1.26.2.
  `go test ./oauthflow -run TestR_MTRN_DL9W_web_origin_records_have_origin_web_and_nil_mcp_context`
  passed. `go test . -run TestR_MTRN_DL9W_state_record_carries_origin_and_mcp_context`
  passed. `gofmt`/`go vet ./...` passed. Tracked Go-source line-length scan
  found no lines over 120 columns. `CGO_ENABLED=0 go build -o bin/hal .`
  passed; the generated `bin/` artifact was removed afterward.
- `go test ./...` and `go test -race ./...` passed for non-root packages but
  the root package run stopped at
  `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
  `.ralph/requirements-verified.jsonl` is permission denied. Per the refactor
  prompt, that is out-of-scope local Ralph state rather than a refactor
  failure.
