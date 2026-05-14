# hal

A small demo of an HTTP-transport MCP server with real OAuth, deployed
at https://hal.ai.metaspot.org. The point is to be readable
end-to-end in one sitting: one shared counter, three MCP tools (read,
increment, decrement), and OAuth-with-Google sitting in front of
writes. The counter is a placeholder for "a tool that mutates state"
so the moving parts on display are the MCP transport, the auth flow,
and the wiring between them — not a domain model.

Audience: developers learning to build MCP servers, or to wire
agent-style clients (Claude Desktop, Claude Code, GPT desktop apps) to
a service of their own.

## Where things live

The spec is split by topic, one file per coherent area:

- `OVERVIEW.md` — scope, stack constraints, out-of-scope.
- `auth.md` — OAuth posture, Google federation, tokens, browser
  session cookies, cross-origin posture, transport security headers.
- `mcp.md` — MCP server transport, advertised tools, and the auth
  boundary at the transport.
- `api.md` — the JSON HTTP API surface that mirrors the MCP tools.
- `web.md` — the browser-facing index page and its derived MCP
  client configuration.
- `counter.md` — the shared counter itself (value, operations,
  concurrency).
- `environment.md` — the binary's subcommand surface, persistence
  workflow, testing conventions, and deliverable properties.
- `design.md` — the visual contract for the index page, sourcing
  the canonical stylesheet at `design.css`.

Cross-references between requirements use `R-XXXX-XXXX` IDs directly
and span files freely; topics that touch multiple surfaces (auth is
the obvious one) live in the file that owns the concern, with other
files pointing at it by ID rather than restating the rule.

## Stack constraints

- R-2YHT-OLY9: the implementation language is Go. The exact toolchain
  version is pinned in `environment.md`.
- R-30XM-G5FN: persistence is SQLite, reached through a pure-Go
  driver (`modernc.org/sqlite`) and the standard library's
  `database/sql` package. No ORM, no migration tooling, no
  schema-version table. Other databases are out of scope unless this
  requirement is replaced.
- R-325I-TX6C: the MCP server is built on the official MCP Go SDK
  (`github.com/modelcontextprotocol/go-sdk`). JSON-RPC and transport
  framing are not hand-rolled.
- R-33DF-7OX1: the upstream-OAuth client (the service's own
  Google-federation leg) is built on `golang.org/x/oauth2`.
- R-34LB-LGNQ: the deliverable is a single statically-linked binary,
  built with `CGO_ENABLED=0`. It has no shared-library dependencies,
  no C runtime dependency, and no language-runtime dependency beyond
  the kernel. The target architecture is `linux/amd64`. The binary
  runs on a bare Linux host without any pre-installed tooling.
- R-JBSD-NKJ8: the deployment target is a single instance reachable
  at https://hal.ai.metaspot.org. Local development is also
  supported.
- R-K1E9-OR3T: the upstream identity provider is Google (Workspace).
  Access is restricted to a single Workspace domain, configured at
  deploy time, never hard-coded.

## Out of scope

- R-KPS9-C5XP: per-user counters or any namespacing. Exactly one
  counter, shared by every caller.
- R-I219-0C8A: history, audit log, or reset operations on the
  counter. The counter supports the three operations R-ECNJ-R09R
  pins (read, increment, decrement) and nothing else; resetting it
  to zero, querying past values, or recovering its history are out
  of scope. Direct database access is the only way to alter the
  stored value outside the three named operations.
- R-M04F-VG43: rate limiting, quotas, abuse protection.
- R-MOIF-IUXZ: high availability, multi-instance, or clustered
  deployment. One process is the supported topology.
- R-NAGM-EQAH: identity providers other than Google Workspace.
