# ouroboros-mcp

A small, deliberately-readable demo of an HTTP-transport **MCP server**
with **real OAuth** in front of writes. Federates logins to Google
Workspace and is deployed at <https://ouroboros.ai.metaspot.org>.

## What this is

A learning project. The point is to be readable end-to-end in one
sitting — every moving part visible, no domain model in the way:

- A Rails 8.1 app on Ruby 4.0, SQLite for persistence.
- A single shared counter as a stand-in for "a tool that mutates state".
- Two MCP tools: read the counter, increment it.
- The service is its own OAuth 2.1 authorization server (DCR, PKCE,
  rotated refresh tokens, RFC 8707 resource binding).
- Google Workspace sits upstream as the actual identity provider;
  access is restricted to a single configured Workspace domain.

If you're trying to build your own MCP server, or wire an agent client
(Claude Desktop, Claude Code, ChatGPT desktop) to a service of your
own, this repo is meant to read like a worked example. It is **not**
production infrastructure: one process, no HA, no rate limiting, no
audit log, one shared counter — see `reqs/OVERVIEW.md` for the
explicit scope and out-of-scope list.

## Where to look

- `reqs/` — the spec, split by topic. Every requirement has an
  `R-XXXX-XXXX` ID; specs name the IDs they verify.
  - `OVERVIEW.md` — scope and stack constraints.
  - `auth.md` — OAuth posture, Google federation, tokens, cookies,
    cross-origin posture, transport security headers.
  - `mcp.md` — MCP transport and tool surface.
  - `api.md` — the JSON HTTP API that mirrors the MCP tools.
  - `web.md`, `counter.md`, `environment.md` — the rest.
- `GOOGLE_SETUP.md` — operator-side checklist for the Google Cloud
  components (OAuth client, consent screen, Workspace-domain
  restriction).
- `app/`, `config/`, `spec/` — the implementation and its tests.

## Setup

With a working Ruby of the version recorded in `.ruby-version`
(currently 4.0.x, see R-OJKW-K8Q6), a fresh checkout is bootstrapped
to a passing test suite with a single command:

```
bin/setup --skip-server
```

That installs gems, prepares the database, and clears logs/tempfiles.
`--skip-server` omits starting the development server so the command
returns when bootstrap is complete. Once it returns, `./test.sh` should
pass without further manual steps.

## Tests

```
./test.sh
```

RSpec, not Minitest. Specs verify spec requirements by ID — `grep`
for any `R-XXXX-XXXX` to find the examples that cover it.

## Lint

```
./lint.sh
```

Rails-omakase RuboCop with one project override
(`Layout/LineLength: 120`).

## Run

```
./launch.sh
```

Starts the service on port 3000 over plain HTTP. TLS is terminated
by the production proxy, not the app.

## Real Google credentials

The current iteration uses an in-memory test double in place of
Google. To wire up real Google Workspace federation, follow
`GOOGLE_SETUP.md` and then export:

```
GOOGLE_CLIENT_ID=…
GOOGLE_CLIENT_SECRET=…
GOOGLE_WORKSPACE_DOMAIN=example.com
```

These are read at process start. They are never committed (R-68WP-XVCK).
