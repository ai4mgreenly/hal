# NEXT — one transformation

## Extract the web-session capability into its own package

**Outcome.** The web-session capability — the session record and its
store, the persistence attachment, and the logic that issues, looks up,
validates against idle and absolute lifetimes, and revokes browser
sessions — lives in its own package with a deliberately small exported
surface, following the package layout the counter extraction
established. The rest of the program (login, logout, the mutation-auth
gate, and any browser-authenticated handler) depends on web sessions
only through that surface and no longer reaches into its internals. The
program still builds as one binary and every observable behavior is
unchanged.

**Why.** Per-capability packages behind explicit boundaries are the goal
of the structure refactor; web sessions are the next capability after
the counter in the extraction order. Isolating the session store and
its lifetime rules behind a surface removes another large slice from the
single package and makes the browser-auth boundary explicit, which the
later web-rendering and JSON-API extractions depend on.

## Scope

- Extract exactly the **web-session** capability this round. The
  OAuth/token cluster, web rendering, the JSON API, and MCP wiring stay
  where they are; each is its own later round. Handlers that merely use
  web sessions stay in place and consume the new package's surface, the
  way the counter round left counter handlers in place.
- The package exposes only the minimal surface its consumers need;
  consumers use only that surface. Tests that pin web-session behavior
  move with it or continue to run and assert unchanged — do not weaken,
  skip, or delete any test or its assertions.
- Follow the module and package-layout precedent the counter extraction
  set; the exact name and path are yours to choose, idiomatic Go.
- If the whole capability cannot move while staying green in one round,
  move the largest coherent slice that stays green and name in the
  result note exactly what moved and what remains, so the next round
  continues it. Never loosen the invariant to fit more in.
- Production behavior is identical. Do not edit reqs/ (the behavioral
  contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean
across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds.

## Result — 2026-05-16

Completed the web-session package extraction. The browser session record,
store, hash-keyed issuance/lookup/revocation rules, idle and absolute lifetime
validation, SQLite attachment, and test observation helpers now live in
`app-root/websession`. `main` wires that package through login, logout,
mutation auth, agent revocation/streaming, and index rendering via the exported
store surface; OAuth/token, rendering, JSON API, and MCP wiring remain in
`main` for later rounds.

Files changed: `app-root/websession/session.go`, `app-root/main.go`,
`app-root/main_test.go`, `NEXT.md`.

Verification from `app-root/`:
- `gofmt -w main.go main_test.go websession/session.go` — passed.
- `awk 'length($0)>120 {print FILENAME ":" FNR ":" length($0)}' $(rg --files -g '*.go')` — no output.
- `GOROOT= go vet ./...` — passed.
- `GOROOT= CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/hal-static-check .` — passed.
- `GOROOT= make build` — passed.
- `GOROOT= go test -race ./websession ./counter` — passed.
- `GOROOT= go test ./...` — blocked only by out-of-scope local Ralph state:
  `.ralph/requirements-verified.jsonl` permission denied.
- `GOROOT= go test -race ./...` — blocked only by the same out-of-scope local
  Ralph state.

Issue noted: `main` retains compatibility aliases (`webSession` /
`webSessionStorage`) while later extractions continue to shrink the entry-point
package; the web-session internals themselves have moved behind the new
package boundary.
