# NEXT — one transformation

## Retire the last test-only hook: pass the token-exchange context as a parameter

**Outcome.** The Google token-exchange code obtains the context it uses
from its caller — passed in as an ordinary argument, the way Go
conventionally threads a context — rather than reading a package-level
variable that tests assign. The test-only package variable, and the
production branch that consults it, no longer exist. Production callers
pass the context they already hold (the request's context, or a
background context where none applies); tests that must redirect the
exchange's HTTPS POST at a loopback server supply their client-bearing
context through that same parameter. No non-test code reads any
test-assigned global.

**Why.** This is the last place production code behaves differently
because a test assigned a global — the final test/production coupling
left after the `testing.Testing()` retirement. Threading the context as
a parameter is both the idiomatic Go shape (a context flows as an
explicit argument, not ambient global state) and what lets tests inject
their HTTP client without a shared mutable hook. With this gone the
codebase has one path for every caller — the precondition for the
package split that follows, since a package boundary cannot straddle a
global that production and tests both mutate.

## Scope

- Remove the test-only exchange-context package variable and the
  production branch that reads it. The exchange context becomes an
  explicit parameter supplied by the caller.
- Production behavior is identical: callers pass the context they
  already hold; with no test client present the exchange uses the same
  effective context it does today.
- Tests inject their client-bearing context through the new parameter
  (directly, or via the request context that reaches the exchange) — not
  through a reinstated global. Adjust test wiring as needed; do not
  weaken, skip, or delete any test or its assertions.
- If the exchange method is part of an interface, update the interface
  and every implementer and caller consistently. Do not change routing
  or handler behavior, do not extract packages this round, and do not
  edit reqs/ (the behavioral contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean,
no source line exceeds 120 columns, and the static binary still builds.
Additionally, a search across non-test source for the retired hook name
returns nothing.

## Result — 2026-05-16

Completed the token-exchange context refactor. `googleIDP.ExchangeCode`
now accepts a `context.Context`; the real Google provider uses that context
for both the OAuth token POST and ID-token JWK fetch, with nil preserving the
previous background-context fallback. The Google callback passes the request
context, and tests that need the loopback HTTPS client pass an
`oauth2.HTTPClient` context directly. Removed `testHookGoogleExchangeContext`
and the production branch that read it.

Files changed: `app-root/main.go`, `app-root/main_test.go`, `NEXT.md`.

Verification from `app-root/`:
- `gofmt -w main.go main_test.go` — passed.
- `awk 'length($0)>120 {print FILENAME ":" FNR ":" length($0)}' main.go main_test.go` — no output.
- `rg -n "testHookGoogleExchangeContext" app-root --glob '!**/.ralph/**'` — no output.
- `GOROOT= go vet ./...` — passed.
- `GOROOT= CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/hal-static-check .` — passed.
- `GOROOT= go test -run 'TestR_(T0B2_A4E5|VF61_2Y6I|W3K0_QD0E|ZBV4_KEJ6|EMW1_D8A0)' -count=1 -v` — passed.
- `GOROOT= go test -race -run 'TestR_(T0B2_A4E5|VF61_2Y6I|W3K0_QD0E|ZBV4_KEJ6|EMW1_D8A0)' -count=1` — passed.
- `GOROOT= go test ./...` — blocked only by out-of-scope local Ralph state:
  `.ralph/requirements-verified.jsonl` permission denied.
- `GOROOT= go test -race ./...` — blocked only by the same out-of-scope local
  Ralph state.

Issue noted: the default shell environment still has `GOROOT` pointing at a
Go 1.23.5 tree while `go` is Go 1.26.2; verification used `GOROOT=` to select
the matching `/usr/local/go` toolchain.
