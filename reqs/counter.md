# Counter

The single shared integer that is the entire point of the service.

## Value

- R-UC3P-Z0IX: there is exactly one counter, shared by all callers
  across all transports — web, HTTP API, MCP.
- R-UZ9T-8NM4: the counter is a non-negative integer.
- R-VNNS-W2G0: the counter persists across process restarts. After a
  crash and restart, reads return the last successfully incremented
  value.
- R-WD9O-X90L: on a fresh database the counter is zero.

## Operations

- R-WZ7V-T4D3: there are exactly two operations on the counter: read
  (returns the current value) and increment.
- R-XMDZ-2RGA: increment takes no arguments. Each successful call adds
  exactly one to the stored value.
- R-RQZQ-81ZC: increment returns the value as it stands after the
  increment is applied.
- R-SE5T-HP2J: read does not require authentication.
- R-T2JT-53WF: increment requires an authenticated caller. An
  unauthenticated request to increment is rejected and the stored
  value does not change.

## Concurrency

- R-TOI0-0Z8X: concurrent increments do not lose updates. If N
  successful increment calls return, the post-state value has gone up
  by exactly N relative to the pre-state value.
