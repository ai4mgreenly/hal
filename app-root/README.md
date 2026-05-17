# hal — build & run

The `hal` service: how to build, test, and run it locally.

## Requirements

- Go 1.26.2 (pinned in [`go.mod`](go.mod))

## Make targets

| Command | What it does |
|---|---|
| `make build` | Build a static `hal` binary (`CGO_ENABLED=0`, `linux/amd64`) into this directory. |
| `make test` | Run the full test suite (`go test ./...`). |
| `make install` | Build, then install the binary to `~/.local/bin/hal`. |

## Running

The binary has three subcommands:

- `hal serve` — start the HTTP server. Defaults: binds `127.0.0.1:3000`,
  SQLite database at `./hal.DB`, plain HTTP.
  - `--port` — TCP port to listen on (default `3000`)
  - `--ip` — local interface to bind (default `127.0.0.1`)
  - `--db` — path to the SQLite database file (default `./hal.DB`)
- `hal reset` — reset the database to a clean, empty state.
- `hal version` — print the version and exit.

Run with no subcommand, or an unknown one, prints usage and exits
non-zero.

## Google credentials

`hal serve` reads these at startup:

- `GOOGLE_CLIENT_ID`
- `GOOGLE_CLIENT_SECRET`
- `GOOGLE_WORKSPACE_DOMAIN`

With none of them set, an in-memory Google double runs in their place —
so a fresh checkout serves, and the full test suite passes, with no
credentials and no further setup.
