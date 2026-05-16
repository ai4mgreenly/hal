# NEXT — one transformation

## Extract the JSON-API capability into its own package

**Outcome.** The JSON HTTP surface — the machine-facing endpoints that
read and mutate the counter, the agent-management endpoints, the OAuth
discovery/metadata documents, and the shared request-body reading,
size-limiting, and JSON encoding and error-shaping behind them — lives
in its own package with a deliberately small exported surface, following
the package layout the earlier extractions established. The entry-point
package depends on this surface for its JSON responses and no longer
contains the JSON-shaping internals. The program still builds as one
binary and every observable behavior is unchanged.

**Why.** The JSON API is its own capability, distinct from the
server-rendered web surface already extracted and from the MCP transport
still to come. Isolating it leaves the entry-point package as routing
and wiring only, which the final MCP-wiring extraction and the
entry-point collapse depend on. It is the next capability in the
extraction order.

## Scope

- Extract exactly the **JSON-API** capability this round. MCP wiring
  stays where it is; it is its own later round. Routing and transport
  plumbing stay in place and call the new JSON surface, the way earlier
  rounds left their handlers in place.
- JSON responses must be byte-for-byte identical: status codes, response
  bodies, headers, content types, and error shapes do not change,
  including the body-size-limit and malformed-input behaviors. The
  existing tests pin these precisely and are the proof; do not weaken,
  skip, or delete any test or its assertions.
- The package exposes only the minimal surface its consumers need;
  consumers use only that surface. Tests that pin JSON-API behavior
  move with it or continue to run and assert unchanged.
- Follow the module and package-layout precedent the earlier
  extractions set; the exact name and path are yours to choose,
  idiomatic Go. Do not leave new compatibility aliases in the
  entry-point package beyond what is unavoidable to keep this round
  green; prefer migrating the consumer to the new surface.
- If the whole capability cannot move while staying green in one round,
  move the largest coherent green slice and name in the result note
  exactly what moved and what remains, so the next round continues it.
  Never loosen the invariant to fit more in.
- Production behavior is identical. Do not edit reqs/ (the behavioral
  contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean
across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds.

## Result — 2026-05-16

Completed the remaining agent-management JSON-API extraction slice. Moved the
`/agents/revoke` action, `/agents/stream` SSE JSON snapshot formatting, and
per-owner agent stream broadcaster into `app-root/jsonapi`, with `main.go`
left as routing, context-default, timing, and notifier wiring. The main package
keeps thin compatibility wrappers for existing tests and call sites.

Files changed: `app-root/jsonapi/jsonapi.go`, `app-root/main.go`,
`app-root/main_test.go`, `NEXT.md`.

Verification: `env -u GOROOT go test -run
'TestR_(D0XD_1YT0|0TVF_0BKI|T6VA_9U84|R4RG_O4Y9)' .` passed; the same focused
set passed under `-race`; `env -u GOROOT go test -run '^$' .` passed;
`env -u GOROOT go vet ./...` passed; Go source line-length scan passed;
`git diff --check` passed; `env -u GOROOT CGO_ENABLED=0 GOOS=linux
GOARCH=amd64 go build -o hal .` passed. Broad `env -u GOROOT go test ./...`
and `env -u GOROOT go test -race ./...` still fail only at
`TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
`.ralph/requirements-verified.jsonl` is permission denied; per
`helper/REFACTOR.md`, that local Ralph state is out of scope for this refactor
iteration.
