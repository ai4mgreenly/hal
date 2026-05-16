# NEXT — one transformation

## Introduce an injected database-opener seam (startup guard unchanged)

**Outcome.** The serve startup path obtains its database by calling a
database-opener supplied from the program entry point as a parameter —
the same ownership-from-the-entry-point shape already established for the
environment lookup and the clock — instead of calling the concrete
open-database function directly. In production the injected opener *is*
that concrete function, so the database opened, the schema applied, and
every observable behavior are identical. The existing `testing.Testing()`
startup guard is left exactly as it is: this round only adds the seam,
it does not move any code through it and does not change which code runs
under test.

**Why.** The final `testing.Testing()` retirement needs the serve-test
path to run the real database open-and-attach code against a substitute
database rather than skipping it, which requires the database opener to
be injectable first. Per seams-before-moves, this round introduces only
the seam — production behavior unchanged, guard still in place — so it
is independently green; the next round moves the open/attach work
through this seam and removes the guard.

## Scope

- Add a database-opener seam threaded from the entry point exactly like
  the environment-lookup and clock seams (same default-to-real,
  install-at-entry shape). The serve startup calls the injected opener
  rather than the concrete open function directly.
- The default opener used by the real program entry point is the
  existing concrete open function — production behavior is byte-for-byte
  identical.
- Do **not** remove, weaken, move, or alter the `testing.Testing()`
  startup guard or any code inside it. Do not move database
  open/attach, required-environment, or identity-provider construction
  out of the guard — that is the next round.
- No test should need to change this round. Do not weaken, skip, or
  delete any test. Do not extract packages or change routing or handler
  behavior. Do not edit reqs/ (the behavioral contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean,
no source line exceeds 120 columns, and the static binary still builds.

## Result — 2026-05-16

Completed the injected database-opener seam. The program entry point now
passes the production `openCounterDB` function through the same startup
ownership chain used for the environment lookup and clock, and the serve
startup path calls the injected opener inside the existing
`!testing.Testing()` guard. The guard and all guarded startup work were left
in place.

Files changed: `app-root/main.go`, `NEXT.md`.

Verification from `app-root/`:
- `gofmt -w main.go` — passed.
- `awk 'length($0)>120 {print FILENAME ":" FNR ":" length($0) ":" $0}' main.go main_test.go` — no output.
- `go test ./...` — blocked by local Go toolchain mismatch until rerun with
  `GOROOT` unset.
- `go vet ./...` — blocked by local Go toolchain mismatch until rerun with
  `GOROOT` unset.
- `env -u GOROOT go test ./...` — blocked only by out-of-scope local Ralph
  state: `.ralph/requirements-verified.jsonl` permission denied.
- `env -u GOROOT go vet ./...` — passed.
- `env -u GOROOT go test -race ./...` — blocked only by out-of-scope local
  Ralph state: `.ralph/requirements-verified.jsonl` permission denied.
- `env -u GOROOT CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hal .` —
  passed.
- `env -u GOROOT go test -skip TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests ./...` — passed.
- `env -u GOROOT go test -race -skip TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests ./...` — passed.

Follow-up risk: the broad suite still depends on readable local `.ralph/`
state; this iteration did not inspect or modify `.ralph/` per the refactor
prompt.
