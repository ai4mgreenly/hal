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

## Result — 2026-05-16

Strangled singleton: `oauthTokenStore`.

Completed:
- Removed the production mutable package-level OAuth token store singleton.
- Added `newOAuthTokenStorage()` and explicit token-store variants for MCP
  setup, MCP bearer checks, OAuth token handling, mutation auth, agents, and
  counter mutation handlers.
- Constructed the serving OAuth token store in `runServeWithEnvAndClock` and
  threaded it through the HTTP and MCP surfaces that consume token records.
- Kept the existing test substitution point as a test-owned store in
  `main_test.go`, with explicit context injection for serve tests.

Files changed:
- `app-root/main.go`
- `app-root/main_test.go`
- `NEXT.md`

Verification:
- `env -u GOROOT go test -run 'TestR_VKZD_UKVS_body_reading_endpoints_reject_oversized_bodies|TestR_ZQS0_HWZ8|TestR_285U_FWW3|TestR_B78O_8X0F|TestR_42V5_GJW4' ./...` passed.
- `env -u GOROOT go test ./...` reached only the out-of-scope local Ralph
  state failure: `.ralph/requirements-verified.jsonl: permission denied`.
- `env -u GOROOT go test -run 'TestR_[^K]' ./...` passed.
- `env -u GOROOT go test -race -run 'TestR_[^K]' ./...` passed.
- `env -u GOROOT go vet ./...` passed.
- `gofmt -w main.go main_test.go` completed.
- `rg -n '^.{121,}$' main.go main_test.go` found no overlength lines.
- `git diff --check` passed.
- `env -u GOROOT CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/hal-static-test .` passed.

Notes:
- The shell environment has a stale `GOROOT` pointing at a Go 1.23.5 tree
  while `go` is Go 1.26.2; verification used `env -u GOROOT`.
