# NEXT — one transformation

## Complete shutdown so it drains in-flight requests and releases the database

**Outcome.** When the lifecycle context the entry point already owns and
threads inward is cancelled (operator signal, or a test cancelling its
own context), the running server stops accepting new connections, lets
in-flight requests finish within a single bounded grace period, and only
then forces any still-open connections closed — instead of severing
connections abruptly the instant the context is done. As part of the
same teardown, the database opened at serve startup is closed on every
path the serve routine returns through, including the normal
signal-driven shutdown that today returns without closing it. The
signal-derived cancellation context remains the single lifecycle source
the entry point supplies and threads inward; this transformation routes
teardown *through that existing seam* and does not introduce a new one.

**Why.** The cancellation seam is already in place — context is threaded
from the entry point through the serve wrappers. But the teardown riding
on it is incomplete: shutdown severs live connections mid-request, and
the startup-opened database handle is leaked on the normal shutdown path
(it is closed only on start-up attach-error paths). Idiomatic Go drains
then releases. Every owned resource having exactly one open/close
lifecycle is the precondition the singleton-strangling and
package-extraction steps that follow depend on; completing teardown now,
on the existing seam, establishes it.

## Scope

- The grace period is bounded: if in-flight requests do not finish
  within it, fall back to forcing connections closed, so a slow or hung
  request cannot wedge shutdown. Context cancellation must still return
  promptly — the existing tests that cancel the context and expect a
  clean, timely return depend on that.
- Close exactly the database the serve routine opened, on every path
  that routine returns — and only that. Tests do not open it on this
  path (it is opened only outside `testing.Testing()`), and their
  behavior must remain unaffected.
- This is a move through the existing context seam, not a new seam. Do
  **only** the shutdown-drain and database-release completion. Do
  **not** also strangle singletons, extract packages, remove any
  under-test branch, or change request handling, routing, or the entry
  point's signal set.
- Do not modify, weaken, or skip any test. Do not edit reqs/ (the
  behavioral contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean,
no source line exceeds 120 columns, and the static binary still builds.

## Result - 2026-05-16

Completed the shutdown-drain/database-release iteration. `runServe` now
uses the existing lifecycle context to stop accepting new connections,
wait through a single 500ms graceful shutdown window, and force-close any
remaining connections if that window expires. The database opened by the
non-test serve startup path is now closed by one deferred owner path on
every return after open, including startup configuration failures and
normal context-driven shutdown.

Files changed:
- `app-root/main.go`
- `NEXT.md`

Verification:
- `env -u GOROOT go test -run 'TestR_K7DK_LSJ6_serve_exits_within_1s_on_signal|TestR_FZC6_H2SB_counter_stream_live_updates|TestR_T4FH_IAQQ_service_responsive_with_many_streams|TestR_T6VA_9U84_agents_stream_resource_budget'` passed.
- `env -u GOROOT go test -run 'TestR_K7DK_LSJ6_serve_exits_within_1s_on_signal|TestR_FZC6_H2SB_counter_stream_live_updates|TestR_T4FH_IAQQ_service_responsive_with_many_streams|TestR_T6VA_9U84_agents_stream_resource_budget|TestR_195O_JBGX_access_log_concurrent_writes_race_free'` passed.
- `env -u GOROOT go test -race -run 'TestR_K7DK_LSJ6_serve_exits_within_1s_on_signal|TestR_FZC6_H2SB_counter_stream_live_updates|TestR_T4FH_IAQQ_service_responsive_with_many_streams|TestR_T6VA_9U84_agents_stream_resource_budget|TestR_195O_JBGX_access_log_concurrent_writes_race_free'` passed.
- `env -u GOROOT go vet ./...` passed.
- `gofmt -w main.go` completed; line-length scan over Go sources found no lines over 120 columns.
- `env -u GOROOT CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hal ./...` passed.
- `env -u GOROOT go test ./...` and `env -u GOROOT go test -race ./...` both failed only at `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because local `.ralph/requirements-verified.jsonl` is permission-denied; this is the out-of-scope Ralph-state failure covered by `helper/REFACTOR.md`.

Blockers / follow-up risks:
- The shell has `GOROOT` set to `/home/mgreenly/.local/go1.23.5.linux-amd64` while `go` reports `go1.26.2`; verification required unsetting `GOROOT`.
- Broad full-suite and broad race commands remain blocked by local `.ralph` permission state, not by this refactor.
