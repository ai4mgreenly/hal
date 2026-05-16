# NEXT — one transformation

## Extract the web-rendering capability into its own package

**Outcome.** The server-rendered presentation — the index page, the
stylesheet asset, the banner and page chrome, the agent listing and the
HTML fragments streamed to it, and the static presentation data behind
them — lives in its own package with a deliberately small exported
surface, following the package layout the earlier extractions
established. The HTTP handlers depend on rendering only through that
surface; markup assembly, the stylesheet bytes, and the presentation
data no longer live in the entry-point package. The program still builds
as one binary and every observable behavior is unchanged.

**Why.** Presentation is its own capability. Isolating it removes the
markup and templating bulk from the entry-point package and leaves the
HTTP layer thin, which the JSON-API and MCP-wiring extractions that
follow depend on. It is the next capability in the extraction order.

## Scope

- Extract exactly the **web-rendering** capability this round. The JSON
  API and MCP wiring stay where they are; each is its own later round.
  The HTTP handlers stay in place and call the new rendering surface,
  the way earlier rounds left their handlers in place.
- Rendered output must be byte-for-byte identical: the exact HTML of
  every page and streamed fragment, the stylesheet bytes, the response
  headers, and the content types do not change. The existing tests pin
  this markup precisely and are the proof; do not weaken, skip, or
  delete any test or its assertions.
- The package exposes only the minimal surface its consumers need;
  consumers use only that surface. Tests that pin rendering behavior
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

## Result — 2026-05-16 web-rendering extraction

Completed the web-rendering extraction into `app-root/web`. The new package
owns the index-page renderer, stylesheet embed and CSS handler, subtitle bank
and random subtitle selection, rendered agent-row identity rules, and the
agent-chain rendered-identity sort shared by the page and agents stream.
`main` now keeps routing, request/session/token lookup, and response wiring,
then calls the web package's rendering surface.

Files changed: `app-root/web/render.go`, `app-root/web/design.css`,
`app-root/main.go`, `app-root/main_test.go`, `app-root/design.css`, `NEXT.md`.

Verification from `app-root/`:
- `gofmt -l $(find . -path './.ralph' -prune -o -type f -name '*.go' -print)` — no output.
- Go source line-length awk check — no output.
- `env -u GOROOT go vet ./...` — passed.
- `env -u GOROOT CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/hal-static-check .` — passed.
- `env -u GOROOT go test -run 'TestR_8KKV_TDWF|TestR_8MP8_6B77|TestR_G47S_05R3|TestR_VTZ5_5FF5|TestR_VV71_J75U|TestR_6KK2_AAY0|TestR_2ZZH_LJYA|TestR_6QIE_4D71|TestR_CNWX_9VB2|TestR_TS71_XRW4|TestR_O87H_RSH4|TestR_3RL1_IUP6' ./...` — passed.
- `env -u GOROOT go test -skip TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests ./...` — passed.
- `env -u GOROOT go test -race -skip TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests ./...` — passed.
- `env -u GOROOT go test ./...` — blocked only by out-of-scope local Ralph
  state: `.ralph/requirements-verified.jsonl` permission denied.

Blockers / risks: plain `go` commands still inherit a stale `GOROOT` pointing
at a Go 1.23.5 tree while `go` itself is 1.26.2, so verification used
`env -u GOROOT`. No web-rendering follow-up remains from this slice.
