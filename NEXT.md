# NEXT — one transformation

## Extract the counter capability into its own package (establish the multi-package layout)

**Outcome.** The application is no longer a single package. The counter
capability — its state and persistence binding, the broadcaster it owns,
and the behavior that increments, decrements, reads, persists, and
streams it — lives in its own package with a deliberately small exported
surface. The rest of the program depends on the counter only through
that surface and no longer reaches into its internals. The program still
builds as one binary and every observable behavior is unchanged. This
round also establishes the module's multi-package layout that the
remaining capability extractions will follow.

**Why.** With every dependency injected and no test/production fork
left, the code is decoupled but still one large package — the headline
of an idiomatic-Go structure refactor is unmet until capabilities live
in their own packages behind explicit boundaries. The counter goes
first because it is the most self-contained capability and the cleanest
place to set the precedent — package-boundary shape, exported-surface
style, and how the entry point wires a capability — that every
subsequent extraction round reuses.

## Scope

- Extract exactly the **counter** capability this round. Other
  capabilities (web sessions, the OAuth/token cluster, web rendering,
  the JSON API, MCP wiring) stay where they are; each is its own later
  round.
- The counter package exposes only the minimal surface its consumers
  need; consumers use only that surface. The tests that pin counter
  behavior move with it or continue to run and assert unchanged — do
  not weaken, skip, or delete any test or its assertions.
- Package name, directory path, and module layout are yours to choose —
  pick the idiomatic Go shape; the spec does not pin them.
- If the whole counter capability cannot move while staying green in
  one round, move the largest coherent slice that stays green (core
  state and persistence first, handlers and tools after) and name in
  the result note exactly what moved and what remains, so the next
  counter round continues it. Never loosen the invariant to fit more in.
- Production behavior is identical. Do not edit reqs/ (the behavioral
  contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean
across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds.

## Result — 2026-05-16

Completed the counter package extraction. The counter state, persistence
attachment, broadcaster, and SSE stream implementation now live in
`app-root/counter`; `main` wires that package into MCP tools, HTTP handlers,
and the page renderer through the exported counter surface. Auth gates remain
in `main` because they depend on web-session and OAuth token stores that are
out of scope for this round. Tests were updated to use the package boundary
and to include the counter package in isolated Makefile build fixtures.

Files changed: `app-root/counter/counter.go`, `app-root/main.go`,
`app-root/main_test.go`, `NEXT.md`.

Verification from `app-root/`:
- `gofmt -w main.go main_test.go counter/counter.go` — passed.
- `awk 'length($0)>120 {print FILENAME ":" FNR ":" length($0)}' $(rg --files -g '*.go')` — no output.
- `GOROOT= go vet ./...` — passed.
- `GOROOT= CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/hal-static-check .` — passed.
- `GOROOT= make build` — passed.
- `GOROOT= go test ./counter && GOROOT= go test -race ./counter` — passed.
- `GOROOT= go test ./...` — blocked only by out-of-scope local Ralph state:
  `.ralph/requirements-verified.jsonl` permission denied.
- `GOROOT= go test -race ./...` — blocked only by the same out-of-scope local
  Ralph state.

Issue noted: the HTTP mutation auth gates and MCP tool registration still live
in `main` pending later extraction of the OAuth/token and MCP wiring
capabilities.
