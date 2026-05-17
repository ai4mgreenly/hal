# NEXT — one transformation

## Extract the MCP wiring capability into its own package

**Outcome.** The Model Context Protocol surface — the MCP server
construction, the tool handlers it registers, the bearer-token gate and
unauthorized challenge that protect them, the prompt-signal behavior,
and the HTTP transport that mounts the MCP endpoint — lives in its own
package with a deliberately small exported surface, following the
package layout the earlier extractions established. The entry-point
package depends on that surface for its MCP endpoint and no longer
contains the MCP server, tool, gating, or transport internals. The
program still builds as one binary and every observable behavior is
unchanged.

**Why.** The MCP transport is its own capability, distinct from the
JSON API and the server-rendered web surface already extracted. It is
the last per-capability extraction in the order; isolating it leaves
the entry-point package as routing and wiring only, which the final
entry-point collapse depends on.

## Scope

- Extract exactly the **MCP wiring** capability this round: the MCP
  server and the tools it exposes, the bearer/authorization gate and
  the unauthorized challenge response, the prompt-signal behavior, and
  the HTTP handler that serves the MCP endpoint. Route registration may
  stay in the entry-point package and call the new surface, the way
  earlier rounds left their routes in place.
- Every MCP-observable behavior is byte-for-byte identical: the JSON-RPC
  request and response bodies, tool results, status codes, the
  unauthorized challenge including its WWW-Authenticate header, which
  requests are gated versus allowed, the prompt-signal effect, the
  endpoint's accepted HTTP methods, and content types do not change.
  The existing tests pin these precisely and are the proof; do not
  weaken, skip, or delete any test or its assertions.
- The capability depends on the counter and the OAuth token store
  through their already-extracted package surfaces, and on the
  canonical resource identifier / request base URL it needs to build
  the challenge. Any configuration the gate needs must be passed in
  explicitly — do not introduce a new package-level global or hidden
  singleton for it; this preserves the decoupling the earlier rounds
  established.
- The package exposes only the minimal surface its consumers need;
  consumers use only that surface. Tests that pin MCP behavior move
  with it or continue to run and assert unchanged.
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

Extracted the MCP wiring capability into `app-root/mcpwire`. The new package
owns MCP server construction, counter tool handlers, bearer-token validation,
the HTTP prompt-signal challenge, and the Streamable HTTP handler. `main.go`
now mounts `/mcp` by constructing `mcpwire.Surface` with the serving counter,
OAuth token store, canonical resource identifier, and version. MCP structural
and focused behavior tests were updated to target the extracted package, and
the Makefile source-copy tests include the new package file.

Files changed: `app-root/mcpwire/mcpwire.go`, `app-root/main.go`,
`app-root/main_test.go`, `NEXT.md`.

Verification: `env -u GOROOT go test ./mcpwire` passed; focused MCP/counter
tests passed with `env -u GOROOT go test -run
'TestR_(VKZD_UKVS|UC3P_Z0IX|OBU9_0WFI|UK7D_Z0IZ|7A9U_HJFF|7BHQ_VB64|51PZ_MEQR|77U1_PZY1|7E4W_K6HL)'
.`; structural/Makefile tests passed with `env -u GOROOT go test -run
'TestR_325I_TX6C|TestR_8OAK_OKFV|TestR_8PIH_2C6K' .`; the focused set passed
under `-race`; `env -u GOROOT go vet ./...` passed; `git diff --check`
passed; Go source line-length scan passed; `env -u GOROOT CGO_ENABLED=0
GOOS=linux GOARCH=amd64 go build -o hal .` passed. Broad `env -u GOROOT go
test ./...` and `env -u GOROOT go test -race ./...` still fail only at
`TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
`.ralph/requirements-verified.jsonl` is permission denied; per
`helper/REFACTOR.md`, that local Ralph state is out of scope for this refactor
iteration.
