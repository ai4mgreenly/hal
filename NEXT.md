# NEXT — one transformation

## Retire the last test/production fork: run real startup for every caller

**Outcome.** The serve startup performs the same work for every caller —
open the database through the injected opener, attach every store to it,
read required configuration through the injected environment lookup, and
validate the required resource identifier — with no `testing.Testing()`
branch anywhere. The real Google identity provider is constructed from
the required environment only when no identity provider was supplied
through the existing serving seam; when one was supplied (as tests do),
it is used unchanged and the real one is not constructed. After this
round no non-test code calls `testing.Testing()`: production and test
drive the identical startup path, differing only by what each injects
through the database-opener, environment-lookup, and identity-provider
seams established in earlier rounds.

**Why.** Every dependency the old guard protected is now injectable: the
database via the opener seam, configuration and required variables via
the environment-lookup seam, the identity provider via the serving
seam. The guard is the last vestige of the program behaving differently
merely because a test binary is running. Removing it makes the startup
path single and real for everyone — the central aim of the seam work —
and lets serve tests exercise the actual open/attach/validate path
instead of skipping it.

## Scope

- Remove the `testing.Testing()` startup guard entirely. The database
  open, every store attach, the required-environment reads, and the
  resource-identifier validation run unconditionally for every caller.
- The real identity provider is built from the required environment
  only when none was injected through the existing serving seam; an
  injected provider is used as-is and suppresses real construction.
  Production injects none, so it still builds the real provider and
  still fails loudly when a required variable is missing or the
  resource identifier is invalid — that fail-loudly contract is
  unchanged.
- Serve tests drive this path through the existing seams: a
  database-opener returning a throwaway database (no stray on-disk
  file), an environment lookup carrying the required variables, and
  their identity-provider double through the serving seam. Adjust test
  wiring as needed; do not weaken, skip, or delete any test or its
  assertions, and do not reintroduce any runtime test check or global.
- This is the final retirement. Do not extract packages or change
  routing or handler behavior. Do not edit reqs/ (the behavioral
  contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean,
no source line exceeds 120 columns, and the static binary still builds.
Additionally, a search for `testing.Testing()` across non-test source
returns nothing — the production/test fork is gone.

## Result — 2026-05-16

Completed the final startup-fork retirement. `runServe` now always opens
the database through the injected opener, attaches all stores, reads
required startup configuration through the injected lookup, and validates
`HAL_RESOURCE_IDENTIFIER`. The real Google identity provider is only built
when no provider was injected. Serve tests now enter this same startup path
through a test helper that supplies a temp database opener, required env
lookup, and the existing fake identity provider.

Files changed: `app-root/main.go`, `app-root/main_test.go`, `NEXT.md`.

Verification from `app-root/`:
- `gofmt -w main.go main_test.go` — passed.
- `awk 'length($0)>120 {print FILENAME ":" FNR ":" length($0)}' main.go main_test.go` — no output.
- `rg -n "testing\\.Testing\\(" app-root --glob '!**/.ralph/**'` — no output.
- `GOROOT= go test ./...` — blocked only by out-of-scope local Ralph state:
  `.ralph/requirements-verified.jsonl` permission denied.
- `GOROOT= go test -run 'TestR_(FA71_BAO6|NQ3G_K0CQ|791Y_3ROQ|7A9U_HJFF|77U1_PZY1|ANRQ_04PK)' -count=1 -v` — passed.
- `GOROOT= go test -race -run 'TestR_(FA71_BAO6|NQ3G_K0CQ|791Y_3ROQ|7A9U_HJFF|77U1_PZY1|ANRQ_04PK)' -count=1` — passed.
- `GOROOT= go vet ./...` — passed.
- `GOROOT= CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/hal-static-check .` — passed.

Issue noted: the default shell environment had `GOROOT` pointing at a
Go 1.23.5 tree while `go` was Go 1.26.2; verification used `GOROOT=` to
select the matching `/usr/local/go` toolchain. Broad test verification was
otherwise blocked only by the local `.ralph/` permission error, which this
iteration did not inspect or modify.
