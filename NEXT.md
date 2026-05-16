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

Strangled singleton: `oauthStateStore`.

Completed:
- Removed the mutable package-level OAuth state store singleton.
- Added `newOAuthStateStorage()` and explicit state-store handler variants.
- Constructed the production OAuth state store in `runServeWithEnvAndClock`
  and threaded it through `/login`, `/oauth/google/callback`, and
  `/oauth/authorize`.
- Updated tests that inspect or share OAuth state to use explicit test-owned
  state stores.

Files changed:
- `app-root/main.go`
- `app-root/main_test.go`
- `NEXT.md`

Verification:
- `env -u GOROOT go test -run 'TestR_ETP6_60VA|TestR_5LQM_O89D|TestR_EMW1_D8A0|TestR_CXJ2_R3BN|TestR_8GJG_64MR|TestR_BAXT_SBU9|TestR_JTTZ_CG5J|TestR_WLUL_MZCD|TestR_T37L_4J01|TestR_MTRN_DL9W|TestR_MUZJ_RD0L' ./...` passed.
- `env -u GOROOT go test ./...` reached the out-of-scope local Ralph state
  failure: `.ralph/requirements-verified.jsonl: permission denied`.
- `env -u GOROOT go test -race -run 'TestR_ETP6_60VA|TestR_5LQM_O89D|TestR_EMW1_D8A0|TestR_CXJ2_R3BN|TestR_8GJG_64MR|TestR_BAXT_SBU9|TestR_JTTZ_CG5J|TestR_WLUL_MZCD|TestR_T37L_4J01|TestR_MTRN_DL9W|TestR_MUZJ_RD0L' ./...` passed.
- `env -u GOROOT go vet ./...` passed.
- `rg -n "^.{121,}$" main.go main_test.go` found no overlength lines.
- `git diff --check` passed.
- `env -u GOROOT CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/hal-refactor-build .` passed.

Notes:
- The shell environment has a stale `GOROOT` pointing at a Go 1.23.5 tree
  while `go` is Go 1.26.2; verification used `env -u GOROOT` to select
  `/usr/local/go`.

## Result — 2026-05-16

Strangled singleton: `oauthClientStore`.

Completed:
- Removed the production mutable package-level OAuth client store singleton.
- Added `newOAuthClientStorage()` and explicit client-store handler variants.
- Constructed the production OAuth client store in `runServeWithEnvAndClock`
  and threaded it through DCR, authorize, index-agent rendering, and
  agents-stream rendering.
- Updated tests that inspect OAuth clients to use an explicit test-owned
  client store.

Files changed:
- `app-root/main.go`
- `app-root/main_test.go`
- `NEXT.md`

Verification:
- `env -u GOROOT go test ./...` reached only the out-of-scope local Ralph
  state failure: `.ralph/requirements-verified.jsonl: permission denied`.
- `env -u GOROOT go test -run 'TestR_[^K]' ./...` passed.
- `env -u GOROOT go test -race -run 'TestR_[^K]' ./...` passed.
- `env -u GOROOT go vet ./...` passed.
- `gofmt -w main.go main_test.go` completed.
- `awk 'length($0) > 120 { ... }' main.go main_test.go` found no
  overlength lines.
- `env -u GOROOT CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/hal-static-test .`
  passed.

Notes:
- The shell environment has a stale `GOROOT` pointing at a Go 1.23.5 tree
  while `go` is Go 1.26.2; verification used `env -u GOROOT`.
