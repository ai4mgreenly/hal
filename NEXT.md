# NEXT — one transformation

## Move the web-session capability's tests into the websession package

**Outcome.** The tests that pin the web-session store's behavior live
in the websession package itself, in that package's own test file(s),
and depend only on that package's surface and other already-extracted
packages' exported surfaces — not on any entry-point-package internal.
The entry-point package's test file no longer contains those
web-session-store tests. Every assertion that existed is preserved
exactly; the full test suite still passes and no production behavior
changes.

**Why.** All test coverage was funneled through one monolithic
entry-point test file; that file is the single thing keeping the
transitional compatibility aliases and the entry-point forwarding
wrappers alive, because it still references entry-point-package
symbols. The MCP and counter capabilities' tests were relocated in the
prior two rounds, proving the pattern. The web-session store is the
cleanest one to do next: it already exposes the construction-time and
introspection seams its store tests need, and it has no remaining
entry-point shims of its own, so this step is purely additive and
continues unblocking the eventual shim deletion and entry-point
collapse.

## Scope

- Move exactly the **web-session store's package-owned** tests this
  round: the tests that exercise the session store itself — issuing,
  hashing, lookup, revocation, the idle and absolute expiry ceilings,
  store properties, and persistence across attach/restart — as
  exercised through the websession package's own surface. Other
  capabilities' tests, and any session-touching test that also asserts
  the entry-point's login/callback/authorize/logout handlers, cookie
  emission, the index page, routing, or the full serve harness, stay
  where they are; those are later rounds.
- This is a pure relocation. Every test that moves keeps every
  assertion it had, unchanged — same inputs, same expected outputs,
  same behavioral checks. Do not weaken, skip, delete, or loosen any
  test or assertion to make it fit the new location. The relocated
  tests, plus the unchanged remainder of the suite, are the proof that
  nothing changed.
- After the move, the relocated tests must compile and pass as part of
  the websession package, depending only on that package's surface and
  other already-extracted packages' exported surfaces — not on any
  entry-point-package symbol. Any seam a relocated test needs — its
  storage backing, its time source, its fixtures — must be obtained
  through the websession package's own surface, not by reaching back
  into the entry-point package.
- Do not widen any package's exported surface to satisfy a relocated
  test, and do not introduce any new compatibility alias or shim
  anywhere. A test that genuinely requires a package's internals
  belongs in that package's own internal (white-box) test scope, not
  in a forced new public API.
- Net effect on the entry-point test file is a reduction: the moved
  web-session tests are gone from it, not duplicated.
- If a particular web-session test cannot be relocated while preserving
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

Moved the web-session store-owned tests into `app-root/websession/session_test.go`:
`TestR_KJ15_9P17_session_expires_at_idle_and_absolute_ceilings`,
the store restart/revocation portions of `TestR_8CBQ_IKKA_web_sessions_survive_restart`,
and `TestR_SLGL_B5B4_web_session_store_properties`. The entry-point package now
keeps only `TestR_8CBQ_IKKA_hal_reset_clears_web_sessions`, because the `hal reset`
assertion depends on the command entry point rather than the websession package.

Files changed:
`app-root/main_test.go`, `app-root/websession/session_test.go`, `NEXT.md`.

Verification:
`GOROOT=/usr/local/go go test ./websession` passed.
`GOROOT=/usr/local/go go test . -run TestR_8CBQ_IKKA_hal_reset_clears_web_sessions` passed.
`GOROOT=/usr/local/go go test ./...` ran, with only the out-of-scope local Ralph
ledger permission failure at `.ralph/requirements-verified.jsonl`.
`GOROOT=/usr/local/go go test -race ./websession ./counter ./mcpwire ./jsonapi ./oauth ./web` passed.
`GOROOT=/usr/local/go go test -race ./...` ran, with only the same out-of-scope
local Ralph ledger permission failure.
`GOROOT=/usr/local/go go vet ./...` passed.
`gofmt -w main_test.go websession/session_test.go` completed.
The Go source line-length check found no lines over 120 columns.
`GOROOT=/usr/local/go CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hal .` passed.

Issue noted:
the default Go environment has a mismatched GOROOT (`go1.26.2` tool with
`go1.23.5` standard-library artifacts), so verification used `GOROOT=/usr/local/go`.
