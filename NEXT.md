# NEXT — one transformation

## Move the MCP capability's tests into the MCP package

**Outcome.** The tests that pin the MCP capability's behavior live in
the MCP package itself, in that package's own test file, and depend
only on that package's exported surface and other already-extracted
packages' exported surfaces — not on any entry-point-package internal.
The entry-point package's test file no longer contains MCP-capability
tests. Every assertion that existed is preserved exactly; the full
test suite still passes and no production behavior changes.

**Why.** All test coverage is still funneled through one monolithic
entry-point test file. That file is the single thing keeping the
transitional compatibility aliases and the entry-point forwarding
wrappers alive — they exist only because that test file still
references entry-point-package symbols. A capability's tests cannot be
removed from the monolith until they depend solely on that
capability's public surface. Relocating them per capability is the
prerequisite that unblocks deleting each capability's compatibility
shims and, finally, collapsing the entry-point wrappers. The MCP
capability is the cleanest to do first: the smallest, most
self-contained surface, with no remaining shims of its own, so this
step is purely additive.

## Scope

- Move exactly the **MCP capability's** tests this round: the tests
  exercising the MCP server, its tools, the bearer/authorization gate
  and challenge, the prompt-signal behavior, the transport handler,
  and the structural checks specific to the MCP package's source.
  Other capabilities' tests stay where they are; they are their own
  later rounds.
- This is a pure relocation. Every test that moves keeps every
  assertion it had, unchanged — same inputs, same expected outputs,
  same behavioral checks. Do not weaken, skip, delete, or loosen any
  test or assertion to make it fit the new location. The relocated
  tests, plus the unchanged remainder of the suite, are the proof
  that nothing changed.
- After the move, the MCP tests must compile and pass as part of the
  MCP package, depending only on that package's exported surface and
  other already-extracted packages' exported surfaces — not on any
  entry-point-package symbol. Resolve residual dependence on
  entry-point-package helpers by using the equivalent already-public
  surface; do not export new entry-point internals to satisfy a test.
- Net effect on the entry-point test file is a reduction: the moved
  MCP tests are gone from it, not duplicated. No new compatibility
  alias or shim is introduced anywhere to keep this round green.
- If a particular MCP test cannot be relocated while preserving every
  assertion exactly, leave that one in place, move the rest, and name
  in the result note precisely which tests moved, which remained, and
  why, so the next round can continue it. Never loosen the invariant
  to move more.
- Production behavior is identical; production source need not change
  for this round. Do not edit reqs/ (the behavioral contract) or
  helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes (every relocated assertion included), the race-detector
run passes, gofmt and go vet are clean across the whole module, no
source line in the module exceeds 120 columns, and the static binary
still builds.

## Result — 2026-05-16

Moved the MCP package-owned tests out of the entry-point test file into
`app-root/mcpwire/mcpwire_test.go`. The relocated tests cover the official SDK
structural check, Streamable HTTP handler, counter read/increment/decrement
tools, bearer-required and invalid-bearer gates, no-credentials prompt signal,
tool listing/descriptions, unauthenticated read, access-token increment grant,
and the legacy-SSE structural check. The new MCP test harness constructs
`mcpwire.Surface` directly with exported `counter` and `oauth` package surfaces;
`app-root/main_test.go` no longer contains those MCP-capability tests.

Left the mixed entry-point tests in place where they also assert main-package
routing, OAuth metadata, HTTP counter endpoints, agent revocation, or the full
OAuth round trip rather than only the MCP package surface.

Files changed: `app-root/mcpwire/mcpwire_test.go`, `app-root/main_test.go`,
`NEXT.md`.

Verification: `env -u GOROOT go test ./mcpwire` passed; focused root compile
and `TestR_76M5_C87C_byte_equal_resource_match_at_presentation_time` passed
with `env -u GOROOT go test -run
'TestR_76M5_C87C_byte_equal_resource_match_at_presentation_time|TestR_74NI_T9CI_usage_lists_three_subcommands'
.`; `env -u GOROOT go vet ./...` passed; `git diff --check` passed; Go source
line-length scan passed; `env -u GOROOT CGO_ENABLED=0 GOOS=linux GOARCH=amd64
go build -o /tmp/hal-static .` passed. Broad `env -u GOROOT go test ./...` and
`env -u GOROOT go test -race ./...` still fail only at
`TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
`.ralph/requirements-verified.jsonl` is permission denied; per
`helper/REFACTOR.md`, that local Ralph state is out of scope for this refactor
iteration.
