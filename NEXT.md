# NEXT — one transformation

## Strangle one global singleton into an owned, injected instance

**Outcome.** Exactly one of the application's mutable package-level
singletons stops being a global and becomes an instance constructed at
the serve entry point and threaded to the code that uses it — the same
ownership-from-the-entry-point shape steps 1–3 established for the
environment lookup, the clock, and the lifecycle context. After this
round that one singleton no longer exists as package-global mutable
state; every consumer reaches it through the threaded instance. Every
other singleton remains exactly as it is today — each is strangled in
its own later round, one per round.

**Why.** Package-level singletons are shared mutable state. They force
the `testing.Testing()` special-casing and they block the per-capability
package extraction that follows: a global cannot move into its own
package without dragging every other consumer with it. Converting them
to entry-owned injected instances, one at a time so every round stays
independently green, removes that coupling incrementally and is the
precondition the extraction steps depend on.

## Scope

- Strangle exactly **one** singleton. Choose one that is still a mutable
  package-level global and whose set of consumers is the smallest and
  most self-contained available. Do not touch any singleton already
  converted in a prior round.
- The result note must name the singleton strangled, so the remaining
  globals stay trackable from one round to the next.
- Production behavior is identical: the entry point constructs the same
  instance the package variable held, threaded to the same consumers.
  Existing test substitution points stay exactly as they are.
- Do **not** remove, collapse, or alter any `testing.Testing()` branch
  or test hook in this round. Retiring that branch is reserved for the
  final strangle round only — once the singleton being strangled is the
  last global standing.
- Do **only** this one strangle. Do not also extract packages, change
  routing or handler behavior, or strangle a second singleton.
- Do not modify, weaken, or skip any test. Do not edit reqs/ (the
  behavioral contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean,
no source line exceeds 120 columns, and the static binary still builds.

## Result - 2026-05-16

Completed one singleton strangle: `mcpServer`. The package-level
`mcpServer` variable was removed, and `runServeWithEnvAndClock` now
constructs the same `newMCPServer()` instance at serve startup and
threads it directly into the Streamable HTTP handler mounted at `/mcp`.
No other singleton was changed.

Files changed:
- `app-root/main.go`
- `NEXT.md`

Verification:
- `gofmt -w main.go main_test.go` completed.
- `env -u GOROOT go test -run 'TestR_325I_TX6C|TestR_UK7D_Z0IZ|TestR_7A9U_HJFF|TestR_Z3LX_89W1|TestR_XS1U_B7YY|TestR_YHNQ_CEJJ|TestR_ZQS0_HWZ8'` passed.
- `env -u GOROOT go test -race -run 'TestR_325I_TX6C|TestR_UK7D_Z0IZ|TestR_7A9U_HJFF|TestR_Z3LX_89W1|TestR_XS1U_B7YY|TestR_YHNQ_CEJJ|TestR_ZQS0_HWZ8'` passed.
- `env -u GOROOT go vet ./...` passed.
- `awk 'length($0) > 120 { print FILENAME ":" FNR ":" length($0) }' $(rg --files -g '*.go')` passed with no output.
- `env -u GOROOT CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hal ./...` passed.
- `env -u GOROOT go test ./...` and `env -u GOROOT go test -race ./...` both failed only at `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because local `.ralph/requirements-verified.jsonl` is permission-denied; this is out-of-scope Ralph state per `helper/REFACTOR.md`.

Blockers / follow-up risks:
- Broad full-suite and broad race commands remain blocked by local `.ralph` permission state, not by this refactor.
