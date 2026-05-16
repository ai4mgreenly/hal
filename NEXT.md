# NEXT — one transformation

## Retire the config re-read test branch; tests install config through the existing seam

**Outcome.** The authentication-configuration accessor returns only the
configuration installed through the existing startup seam — the single
value parsed once from the injected environment lookup and held by the
config setter. It no longer consults `testing.Testing()` and no longer
consults the test-only flag that today makes it re-read the live process
environment on every call under the test binary. That flag, the
initializer that enables it, and the serve-startup code that toggles it
off for the duration of a full serve are all removed. Tests that need
particular configuration install it through the same
configuration/environment-lookup seam the serving path uses, rather than
relying on a runtime re-read of mutated process state.

**Why.** This is the second of the three `testing.Testing()`
retirements. The configuration seam from the environment-seam step
already lets any caller — production or test — install an exact
configuration value; the runtime re-read branch is redundant scaffolding
that exists only because tests historically mutated the live
environment. Removing it deletes a whole production/test fork — the
accessor branch and the serve-startup toggle that compensates for it —
and leaves configuration flowing through one path for every caller.

## Scope

- Retire the configuration accessor's `testing.Testing()` re-read
  branch, the test-only flag it reads, the initializer that enables that
  flag, and the serve-startup block that toggles the flag off and back.
  After this round the accessor has one path: return the installed
  configuration.
- Production behavior is identical: production already returned the
  installed startup-parsed configuration; that path does not change.
- Tests that depend on specific configuration install it through the
  existing configuration/environment-lookup seam before exercising the
  code under test — not through live-environment mutation observed by a
  runtime test check, and not through a reinstated flag. Adjust test
  wiring as needed; do not weaken, skip, or delete any test or its
  assertions.
- Do **not** touch the remaining `testing.Testing()` site: the
  database / required-environment startup gate. It is retired in the
  final round.
- Do not extract packages or change routing or handler behavior. Do not
  edit reqs/ (the behavioral contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean,
no source line exceeds 120 columns, and the static binary still builds.

## Result — 2026-05-16

Completed the auth configuration re-read retirement. `authCfg()` now only
returns the installed configuration, the test-only live-environment re-read
flag and initializer were removed, and `runServe` no longer toggles that flag
while serving. Tests that need specific auth configuration now install it
through `loadAuthConfig` plus `setAuthCfg` using the same env-name lookup seam
as startup.

Files changed: `app-root/main.go`, `app-root/main_test.go`, `NEXT.md`.

Verification from `app-root/` with `GOROOT` unset:
- `go test -run 'TestR_(3UT3_IKZG|LWCN_ZBXO|ANRQ_04PK|5LQM_O89D|W3K0_QD0E|ETP6_60VA|EMW1_D8A0|CXJ2_R3BN|8GJG_64MR|WLUL_MZCD|MUZJ_RD0L)' .` — passed.
- `go test ./...` — blocked only by out-of-scope local Ralph state:
  `.ralph/requirements-verified.jsonl` permission denied.
- `go test -skip TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests ./...` — passed.
- `go test -race -skip TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests ./...` — passed.
- `go vet ./...` — passed.
- `awk 'length($0)>120 {print}' main.go main_test.go` — no output.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hal .` — passed.

Follow-up risk: the broad suite still depends on local `.ralph/` permissions;
this iteration did not inspect or modify `.ralph/` per the refactor prompt.
