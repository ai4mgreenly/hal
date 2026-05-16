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

Completed a coherent JSON-API extraction slice into `app-root/jsonapi`.
Moved the shared request body cap/error helpers, request-base URL helper,
OAuth metadata JSON documents, OAuth DCR JSON endpoint, OAuth token JSON
endpoint, bearer/mutation auth error shaping, and counter read/mutation
JSON response logic behind `jsonapi.Surface`. The main package now wires
that surface through thin adapters so existing routing, test seams, access-log
user attribution, browser flows, and MCP transport behavior remain unchanged.
Agent SSE JSON remains in `main.go` because it is still coupled to the
main-owned broadcaster and stream timing state.

Files changed: `app-root/jsonapi/jsonapi.go`, `app-root/main.go`,
`app-root/main_test.go`, `NEXT.md`.

Verification: `env -u GOROOT go test -run '^$' .` passed; focused
JSON/API/build tests passed with `env -u GOROOT go test -run
'TestR_(SAK8_WB9W|8OAK_OKFV|8PIH_2C6K|2I2S_XB7K|340Z_T6K2|H3FE_QFC0|3JCR_C810|2XEK_GCOI|3UT3_IKZG|27SO_F63X|B78O_8X0F)' .`;
focused race run passed with the same `-run` set; `env -u GOROOT go vet
./...` passed; line-length scan for Go files passed; `env -u GOROOT
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hal .` passed.
Broad `env -u GOROOT go test ./...` and `env -u GOROOT go test -race
./...` still fail only at `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests`
because `.ralph/requirements-verified.jsonl` is permission denied; per
`helper/REFACTOR.md`, that local Ralph state is out of scope for this
refactor iteration. Running Go commands without unsetting the inherited
`GOROOT` also fails before project compilation because it points at a
Go 1.23.5 tree while the active tool is Go 1.26.2.
