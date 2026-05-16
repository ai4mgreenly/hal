# NEXT — one transformation

## Extract the OAuth / token capability into its own package

**Outcome.** The OAuth and token capability — the OAuth state, client
registration, authorization-code, and access/refresh-token stores
together with their persistence and the rules that issue, look up,
rotate, detect reuse of, expire, and revoke them — lives in its own
package with a deliberately small exported surface, following the
package layout the earlier extractions established. The rest of the
program (the authorize, token, registration, and callback handlers, and
the agents notification path) depends on this capability only through
that surface and no longer reaches into its internals. The program still
builds as one binary and every observable behavior is unchanged.

**Why.** This is the next capability in the extraction order and the
largest remaining one. Isolating the OAuth/token stores and their
lifetime and reuse-detection rules behind a single surface removes the
biggest slice from the entry-point package and makes the
authorization-server boundary explicit, which the later web-rendering,
JSON-API, and MCP-wiring extractions depend on.

## Scope

- Extract the **OAuth/token** capability this round. Web rendering, the
  JSON API, and MCP wiring stay where they are; each is its own later
  round. Handlers that merely use these stores stay in place and consume
  the new package's surface, as the counter and web-session rounds left
  their handlers in place.
- The package exposes only the minimal surface its consumers need;
  consumers use only that surface. Tests that pin OAuth/token behavior
  move with it or continue to run and assert unchanged — do not weaken,
  skip, or delete any test or its assertions.
- This capability is large. If it cannot all move while staying green in
  one round, move the largest coherent green slice — a natural cut is
  one store together with its own rules per round (for example the
  state store, then client registration, then authorization codes, then
  the access/refresh tokens) — and name in the result note exactly what
  moved and what remains, so the next round continues the cluster.
  Never loosen the invariant to fit more in.
- Follow the module and package-layout precedent the earlier
  extractions set; the exact name and path are yours to choose,
  idiomatic Go. Do not leave new compatibility aliases in the
  entry-point package beyond what is unavoidable to keep this round
  green; prefer migrating the consumer to the new surface.
- Production behavior is identical. Do not edit reqs/ (the behavioral
  contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean
across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds.

## Result — 2026-05-16 client-registration slice

Completed the next coherent OAuth/token extraction slice: OAuth dynamic-client
registration records and their store now live in `app-root/oauth`, including
client ID generation, registration lookup, collision-safe insert, replacement
for test/setup callers, count/detach diagnostics, and SQLite persistence.
`main` now uses the package surface for registration, authorization redirect
matching, startup DB attach, and agents client-name lookup. Access/refresh-token
storage and agents notification remain in `main` for later slices.

Files changed: `app-root/oauth/client.go`, `app-root/main.go`,
`app-root/main_test.go`, `NEXT.md`.

Verification from `app-root/`:
- `gofmt -w main.go main_test.go oauth/client.go oauth/state.go oauth/authcode.go` — passed.
- Go source line-length awk check — no output.
- `GOROOT=/usr/local/go go vet ./...` — passed.
- `GOROOT=/usr/local/go CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/hal-static-check .` — passed.
- `GOROOT=/usr/local/go go test -run 'TestR_(8OBG_7FST|19BA_4XX4|YRMT_B7LZ|JE3Z_IGI4|KCBH_CXY9|8OAK_OKFV|8PIH_2C6K)' .` — passed.
- `GOROOT=/usr/local/go go test -race -run 'TestR_(8OBG_7FST|19BA_4XX4|YRMT_B7LZ|JE3Z_IGI4|KCBH_CXY9)' .` — passed.
- `GOROOT=/usr/local/go go test -race ./oauth ./counter ./websession` — passed.
- `GOROOT=/usr/local/go go test ./...` — blocked only by out-of-scope local
  Ralph state: `.ralph/requirements-verified.jsonl` permission denied.
- `GOROOT=/usr/local/go go test -race ./...` — blocked only by the same
  out-of-scope local Ralph state.

Local toolchain note: plain `go test ./...` without `GOROOT=/usr/local/go`
failed before project compilation because the active `go1.26.2` binary was
reading a stale `go1.23.5` GOROOT from the environment.
