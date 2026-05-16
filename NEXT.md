# NEXT — one transformation

## Retire the test-only Google-IDP short-circuit; inject the fake through the existing seam

**Outcome.** The function that selects the Google identity provider for a
request no longer consults `testing.Testing()` and no longer reads a
test-only package-level provider hook. It returns the identity provider
threaded to it through the serving seam steps 1–4 already established —
the same parameter the production startup path supplies the real
provider through. Tests that need a fake, or a claim-rejection-driving
double, supply that double through that same serving seam exactly as
production supplies the real one. The runtime `testing.Testing()` branch
and the test-only provider package variable no longer exist.

**Why.** With every dependency now owned and threaded, the
`testing.Testing()` runtime fork is the last thing making production
code behave differently merely because a test binary is running. It is
being retired one isolated branch per round. This branch is the most
self-contained: the provider is already a threaded seam, so the only
change is to stop overriding it at runtime and let tests inject through
the seam they already have — no database, configuration, or startup
path is touched.

## Scope

- Retire **only** the Google-IDP `testing.Testing()` short-circuit and
  its associated test-only provider package variable. Provider selection
  must come solely from the value threaded through the existing serving
  seam.
- Production behavior is identical: outside tests the function already
  returns the threaded serving provider; that path does not change.
- Tests obtain the fake or rejection-driving double by injecting it
  through the existing serving-provider seam — not through any runtime
  test check and not through a reinstated global. Adjust test wiring as
  needed; do not weaken, skip, or delete any test or its assertions.
- Do **not** touch the other three `testing.Testing()` sites: the
  config/env-lookup test branch, its serve-startup toggle, and the
  database/required-environment startup gate. Each is retired in its
  own later round.
- Do not extract packages or change routing or handler behavior. Do not
  edit reqs/ (the behavioral contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean,
no source line exceeds 120 columns, and the static binary still builds.

## Result — 2026-05-16

Completed the Google-IDP provider-selection refactor: `configuredGoogleIDP`
now returns only the threaded serving provider, the test-only Google-IDP hook
was removed, and tests inject `googleFakeIDP{}` or the unverified-email double
through the existing serving seams. Live serve tests receive the fake through
the test context provider value used by `runServe`.

Files changed: `app-root/main.go`, `app-root/main_test.go`, `NEXT.md`.

Verification from `app-root/` with `GOROOT` unset:
- `go test -run 'TestR_(VF61_2Y6I|9PNQ_BN2G|3BKZ_L7R4|ETP6_60VA|5LQM_O89D|EMW1_D8A0|CXJ2_R3BN|8GJG_64MR|MUZJ_RD0L)' .` — passed.
- `go test ./...` — blocked only by out-of-scope local Ralph state:
  `.ralph/requirements-verified.jsonl` permission denied.
- `go test -skip TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests ./...` — passed.
- `go test -race -skip TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests ./...` — passed.
- `go vet ./...` — passed.
- `awk 'length($0)>120 {print}' main.go main_test.go` — no output.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hal .` — passed.

Follow-up risk: the broad suite still depends on local `.ralph/` permissions;
this iteration did not inspect or modify `.ralph/` per the refactor prompt.
