# Spec layout

The behavioral contract for hal lives in [`../reqs/`](../reqs/), split by
topic — one file per coherent area. Every requirement carries a stable
`R-XXXX-XXXX` ID; specs and tests name the IDs they verify, so you can
`grep` any ID across the tree to find both the requirement and the
examples that cover it.

## Files

- `OVERVIEW.md` — scope, stack constraints, out-of-scope.
- `auth.md` — OAuth posture, Google federation, tokens, browser session
  cookies, cross-origin posture, transport security headers.
- `mcp.md` — MCP server transport, advertised tools, and the auth
  boundary at the transport.
- `api.md` — the JSON HTTP API surface that mirrors the MCP tools.
- `web.md` — the browser-facing index page and its derived MCP client
  configuration.
- `counter.md` — the shared counter itself (value, operations,
  concurrency).
- `environment.md` — the binary's subcommand surface, persistence
  workflow, testing conventions, and deliverable properties.
- `design.md` — the visual contract for the index page, sourcing the
  canonical stylesheet at `design.css`.

## Cross-references

Requirements reference each other by `R-XXXX-XXXX` ID directly and span
files freely. A concern that touches multiple surfaces (auth is the
obvious one) lives in the file that owns it, with other files pointing
at it by ID rather than restating the rule.
