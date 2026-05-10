# hal

A small demo of an HTTP-transport MCP server with real OAuth, deployed
at https://hal.ai.metaspot.org. The point is to be readable
end-to-end in one sitting: a Rails app, a single shared counter, three
MCP tools (read, increment, decrement), and OAuth-with-Google sitting
in front of writes. The counter is a placeholder for "a tool that
mutates state" so the moving parts on display are the MCP transport,
the auth flow, and the Rails wiring — not a domain model.

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
- `environment.md` — Ruby and Rails versions, the standard scripts,
  testing conventions, and dependency rules.

Cross-references between requirements use `R-XXXX-XXXX` IDs directly
and span files freely; topics that touch multiple surfaces (auth is
the obvious one) live in the file that owns the concern, with other
files pointing at it by ID rather than restating the rule.

## Stack constraints

- R-COO1-AMEK: implementation language is Ruby (MRI). The exact
  version is pinned in `environment.md`.
- R-I2O3-I23J: web framework is Rails. The exact version is pinned in
  `environment.md`.
- R-IPU6-RP6Q: persistence is SQLite, accessed through Active Record.
  Other databases are out of scope unless this requirement is replaced.
- R-JBSD-NKJ8: the deployment target is a single instance reachable at
  https://hal.ai.metaspot.org. Local development is also
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
  stored value outside the three named operations. (Decrement was
  previously listed here as out-of-scope; it has since been
  brought in-scope and is pinned by R-ECNJ-R09R / R-F5X4-XI2F /
  R-GG9B-GS8T / R-H3FE-QFC0.)
- R-M04F-VG43: rate limiting, quotas, abuse protection.
- R-MOIF-IUXZ: high availability, multi-instance, or clustered
  deployment. One process is the supported topology.
- R-NAGM-EQAH: identity providers other than Google Workspace.
