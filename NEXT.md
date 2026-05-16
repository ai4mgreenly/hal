# NEXT — one transformation

## Route all time access through one injected clock

**Outcome.** Every reading of the current time and every timer or ticker
the application creates resolves through a single clock abstraction that
the program entry point supplies as a parameter — not through direct
`time.Now`, `time.NewTicker`, `time.NewTimer`, `time.After`, or
`time.Sleep` calls spread through the code. The clock provides at least
the current instant and the creation of tickers; deadlines currently
derived from the current time (write deadlines and the like) are computed
from that injected clock's notion of now. Duration values themselves
(heartbeat interval, tick interval, write timeout) keep coming from
configuration as they do today — this seam governs *when it is now* and
*who issues timers*, not how long the intervals are.

**Why.** The entry point must be able to run the entire application
against a controllable clock, so time-dependent behavior — expiry
checks, heartbeats, stream ticks, deadlines — is exercisable
deterministically without real sleeps or dependence on wall-clock
progression. This mirrors the environment seam already in place and
establishes the time seam the lifecycle/cancellation transformation
that follows will build on.

## Scope

- In production the injected clock *is* the real time source, so
  observable behavior is identical and every existing test continues to
  pass unchanged.
- Thread the clock the same way the environment lookup is threaded: the
  entry point owns the real clock and passes it inward; the existing
  test-facing wrappers remain the substitution point.
- Do **only** the clock seam. Do **not** also introduce a
  lifecycle/cancellation context, graceful shutdown, database-close
  handling, singleton strangling, package extraction, or removal of any
  under-test branch — each of those is a separate later iteration.
- Do not modify, weaken, or skip any test. Do not edit reqs/ (the
  behavioral contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean,
no source line exceeds 120 columns, and the static binary still builds.

## Result - 2026-05-16

Completed the clock-seam iteration. Added a single `appClock` abstraction
with `Now` and ticker creation, installed by the serve entry path, and routed
production time reads/tickers/write-deadline calculations through it while
leaving the existing test-facing `*Now` function variables as substitution
points.

Files changed:
- `app-root/main.go`
- `NEXT.md`

Verification:
- `GOROOT=/usr/local/go go test ./...` compiled and ran, but failed only at
  `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because local
  `.ralph/requirements-verified.jsonl` is permission-denied; this is the
  out-of-scope Ralph-state failure covered by `helper/REFACTOR.md`.
- `GOROOT=/usr/local/go go test -race ./...` reached the same `.ralph`
  permission-denied failure.
- `GOROOT=/usr/local/go go test -run 'TestR_(ZBV4_KEJ6|TNXJ_ZWQ0|E5GH_PN6G|KJ15_9P17|3JCR_C810|19BA_4XX4|FZC6_H2SB|T4FH_IAQQ|T5ND_W2HF|D3YH_0JYE|DB9V_B6EK|0TVF_0BKI|T6VA_9U84|195O_JBGX|MTRN_DL9W|B78O_8X0F|8UAA_YKR9)' ./...` passed.
- `GOROOT=/usr/local/go go test -race -run 'TestR_(FZC6_H2SB|T5ND_W2HF|D3YH_0JYE|0TVF_0BKI|195O_JBGX|TNXJ_ZWQ0|KJ15_9P17)' ./...` passed.
- `GOROOT=/usr/local/go go vet ./...` passed.
- `gofmt -w main.go main_test.go` completed; line-length scan over
  `main.go` and `main_test.go` found no lines over 120 columns.
- `GOROOT=/usr/local/go CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hal .` passed.

Blockers / follow-up risks:
- The shell environment has `GOROOT` set to an older Go tree
  (`/home/mgreenly/.local/go1.23.5.linux-amd64`) while `go` reports
  `go1.26.2`; verification required `GOROOT=/usr/local/go`.
- Broad full-suite and race commands remain blocked by local `.ralph`
  permission state, not by this refactor.
