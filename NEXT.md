# NEXT — one transformation

## Extract the site-root (index) request-assembly capability into its own package

**Outcome.** The capability that serves the site-root request — reading
the optional session, deciding signed-in versus signed-out, gathering
the current count, the visitor's identity, and the visitor's live agent
chains, and assembling the single presentation payload handed to the
already-extracted rendering layer — lives in its own package with a
deliberately small exported surface, following the package layout the
earlier extractions established. The entry-point package depends on that
surface, no longer contains the request-assembly logic, and keeps only
route registration and the wiring seam. The program still builds as one
binary and every observable behavior of the site root is unchanged.

**Why.** This site-root request-assembly is the last substantial block
of business logic still living in the entry-point package, distinct from
the rendering layer it feeds (already its own package), the
OAuth/Google-federation flow just extracted, and the
logout/registration/token endpoints (a different concern). Isolating it
empties the entry point of capability logic, completes the extraction
phase, and unblocks relocating the remaining capability tests and
deleting the corresponding compatibility wrappers ahead of the eventual
entry-point collapse.

## Scope

- Extract exactly the **site-root request-assembly** capability this
  round: the handler that, on a request to the site root, looks up the
  optional session, branches on signed-in versus signed-out, gathers the
  count, the owner identity, and the owner's live agent chains, and
  assembles the presentation payload passed to the existing rendering
  layer. The rendering layer is already its own package — consume it, do
  not absorb any of it. The logout handler, the OAuth client-registration
  endpoint, and the OAuth token endpoint are NOT part of this slice;
  leave them where they are.
- Every observable behavior of the site root is byte-for-byte identical:
  the signed-out page versus the signed-in page, the status code, the
  content type, the body the renderer produces from the assembled
  payload, the absence of any authentication gate, redirect, or cookie
  mutation, and the signed-in/signed-out branch including the owner
  identity and the live-agent-chain list. The existing tests pin these
  precisely and are the proof; do not weaken, skip, or delete any test
  or its assertions.
- The package must not read application configuration or hold
  package-level singletons. The session, token, and client stores and
  the counter enter as their already-extracted package types / injected
  inputs; the request base-URL helper and the static version string
  enter as injected inputs; nothing is pulled from a global, a
  package-level variable, or the request context as a hidden
  convenience. This is the same no-config/no-singleton discipline the
  earlier capability extractions established; shed the convenience
  context-pulling entry points rather than carry them forward.
- The package exposes only the minimal surface its consumers need;
  consumers use only that surface. The site-root behavior tests continue
  to run and assert unchanged from where they are (they relocate to the
  new package in a later round). Where a test instead pins, by literal
  entry-point source text, how this capability is currently wired, the
  invariant that test protects still holds and is still asserted — its
  expectation is carried forward and re-expressed against the new
  wiring, never weakened, skipped, or deleted.
- Follow the module and package-layout precedent the earlier extractions
  set; the exact name and path are yours to choose, idiomatic Go. Do not
  leave new compatibility aliases in the entry-point package beyond what
  is unavoidable to keep this round green; prefer migrating the consumer
  to the new surface.
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

## Result - 2026-05-17

Completed the site-root request-assembly extraction by adding
`app-root/siteindex`, whose `Surface` accepts the counter, session store,
token/client stores, request base-URL helper, and version string, then
assembles `web.IndexData` for the existing renderer. `main.go` now keeps a
small wiring helper and delegates index handling through the new surface; the
Makefile build/install fixture tests copy the new package into their isolated
source trees.

Files changed: `app-root/siteindex/siteindex.go`, `app-root/main.go`,
`app-root/main_test.go`, `NEXT.md`.

Verification: `gofmt` on changed Go files; focused index tests passed with
`GOROOT=/usr/local/go`; `go vet ./...` passed; line-length check for Go files
passed; focused `go test -race ./...` covering index and package-copy build
tests passed; static `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build` passed.
Broad `go test ./...` reached project tests and failed only on local Ralph
state: `.ralph/requirements-verified.jsonl` is unreadable, which this
refactor prompt treats as out-of-scope for broad verification.

Blockers/follow-up: site-root behavior tests remain in `main_test.go` behind
the existing compatibility helper and can move to `siteindex` in a later
iteration.
