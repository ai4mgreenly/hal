# NEXT — one transformation

## Move the counter capability's tests into the counter package

**Outcome.** The tests that pin the counter capability's behavior live
in the counter package itself, in that package's own test file(s), and
depend only on that package's surface and other already-extracted
packages' exported surfaces — not on any entry-point-package internal.
The entry-point package's test file no longer contains those
counter-package tests. Every assertion that existed is preserved
exactly; the full test suite still passes and no production behavior
changes.

**Why.** All test coverage was funneled through one monolithic
entry-point test file; that file is the single thing keeping the
transitional compatibility aliases and the entry-point forwarding
wrappers alive, because it still references entry-point-package
symbols. The MCP capability's tests were relocated last round, proving
the pattern. Counter is the cleanest one to do next: a small exported
surface and no remaining entry-point shims of its own, so this step is
purely additive and continues unblocking the eventual shim deletion and
entry-point collapse.

## Scope

- Move exactly the **counter capability's package-owned** tests this
  round: the tests that exercise the counter itself — its read,
  increment, decrement, non-negative/zero-floor behavior, persistence
  across attach/restart, and the counter stream and broadcaster — as
  exercised through the counter package's own surface. Other
  capabilities' tests, and any counter-touching test that also asserts
  the entry-point's HTTP endpoints, routing, auth gating, or full
  serve harness, stay where they are; those are later rounds.
- This is a pure relocation. Every test that moves keeps every
  assertion it had, unchanged — same inputs, same expected outputs,
  same behavioral checks. Do not weaken, skip, delete, or loosen any
  test or assertion to make it fit the new location. The relocated
  tests, plus the unchanged remainder of the suite, are the proof that
  nothing changed.
- After the move, the relocated tests must compile and pass as part of
  the counter package, depending only on that package's surface and
  other already-extracted packages' exported surfaces — not on any
  entry-point-package symbol. Where a relocated test needed an
  entry-point helper only to set up storage or fixtures, it must
  establish that state through the counter package's own surface
  instead of reaching back into the entry-point package.
- Do not widen any package's exported surface to satisfy a relocated
  test, and do not introduce any new compatibility alias or shim
  anywhere. A test that genuinely requires a package's internals
  belongs in that package's own internal (white-box) test scope, not
  in a forced new public API.
- Net effect on the entry-point test file is a reduction: the moved
  counter tests are gone from it, not duplicated.
- If a particular counter test cannot be relocated while preserving
  every assertion exactly, leave that one in place, move the rest, and
  name in the result note precisely which tests moved, which remained,
  and why, so the next round can continue it. Never loosen the
  invariant to move more.
- Production behavior is identical; production source need not change
  for this round. Do not edit reqs/ (the behavioral contract) or
  helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes (every relocated assertion included), the race-detector
run passes, gofmt and go vet are clean across the whole module, no
source line in the module exceeds 120 columns, and the static binary
still builds.

## Result — 2026-05-16

Moved the counter package-owned tests out of `app-root/main_test.go` into
`app-root/counter/counter_test.go`. The relocated tests cover the counter
operation structural fences, non-negative storage/read behavior, increment and
decrement semantics including the zero floor, concurrent increment/decrement
updates, fresh in-memory state, SQLite attach/persistence across reopen, and
the counter-owned SSE stream/broadcaster fan-out and dead-subscriber release.

Left mixed entry-point tests in place where they also assert HTTP endpoints,
routing, auth ordering/gating, schema startup wiring, serve responsiveness, web
rendering, MCP/OAuth flows, or access-log behavior rather than only the counter
package surface.

Files changed: `app-root/counter/counter_test.go`, `app-root/main_test.go`,
`NEXT.md`.

Verification: `env -u GOROOT go test ./counter` passed; `env -u GOROOT go test
-race ./counter` passed; `env -u GOROOT go test ./counter ./mcpwire` passed;
`env -u GOROOT go vet ./...` passed; `git diff --check` passed; Go source
line-length scan passed; `env -u GOROOT CGO_ENABLED=0 GOOS=linux GOARCH=amd64
go build -o /tmp/hal-static .` passed. Broad `env -u GOROOT go test .`,
`env -u GOROOT go test ./...`, and `env -u GOROOT go test -race ./...` still
fail only at `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests`
because `.ralph/requirements-verified.jsonl` is permission denied; per
`helper/REFACTOR.md`, that local Ralph state is out of scope for this refactor
iteration.
