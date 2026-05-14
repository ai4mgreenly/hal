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

- R-ECNJ-R09R: there are exactly three operations on the counter:
  read (returns the current value), increment (adds one), and
  decrement (subtracts one). The MCP server advertises three
  corresponding tools per R-FUB4-KWWB; the HTTP API exposes
  endpoints for the read and the two mutations per R-2I2S-XB7K,
  R-340Z-T6K2, R-H3FE-QFC0; the web index page exposes both
  mutations as `+` / `−` buttons per R-EJAP-XUSB. No other counter
  operations exist.
- R-XMDZ-2RGA: increment takes no arguments. Each successful call
  adds exactly one to the stored value.
- R-RQZQ-81ZC: increment returns the value as it stands after the
  increment is applied.
- R-F5X4-XI2F: decrement takes no arguments. When the stored value
  is greater than zero, a successful call subtracts exactly one and
  returns the value as it stands after the decrement is applied.
  When the stored value is exactly zero, decrement is rejected: the
  operation does not change the stored value and signals the
  rejection to its caller in the form natural to that caller's
  transport (an MCP tool error, an HTTP 4xx response — see
  R-GG9B-GS8T and R-H3FE-QFC0 for the per-transport details). A
  zero-counter decrement is **not** silently clamped to zero with a
  success response; it is an explicit refusal. This preserves
  R-UZ9T-8NM4 (the counter is a non-negative integer) while keeping
  callers informed when their action could not be applied.
- R-SE5T-HP2J: read does not require authentication.
- R-T2JT-53WF: increment requires an authenticated caller. An
  unauthenticated request to increment is rejected and the stored
  value does not change. The same rule applies to decrement
  (R-F5X4-XI2F): an unauthenticated decrement request is rejected
  and the stored value does not change. The accepted authentication
  modes for these mutations are defined by R-OCH3-8FQ8.

## Concurrency

- R-TOI0-0Z8X: concurrent increments do not lose updates. If N
  successful increment calls return, the post-state value has gone
  up by exactly N relative to the pre-state value. The same property
  holds for decrement: if M successful decrement calls return, the
  post-state value has gone down by exactly M. Interleaved increments
  and decrements compose: N successful increments and M successful
  decrements (with N ≥ M relative to the pre-state) leave the
  post-state value (N − M) above the pre-state.
