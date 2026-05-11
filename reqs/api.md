# HTTP API

A small JSON API that mirrors the MCP tools, so the counter can also
be driven from curl, scripts, or any non-MCP HTTP client. The auth
posture matches MCP: read is open, mutations require an authenticated
caller. The mutation endpoints also accept a valid web session cookie
so that the index page's `+` / `−` buttons (R-NG6O-94I2) can drive
the same endpoints without bearer tokens.

- R-2I2S-XB7K: `GET /counter` returns HTTP 200 with a JSON object
  containing the current count as a non-negative integer.
- R-340Z-T6K2: `POST /counter/increment` adds one to the counter and
  returns HTTP 200 with a JSON object containing the post-increment
  value.
- R-H3FE-QFC0: `POST /counter/decrement` subtracts one from the
  counter and returns HTTP 200 with a JSON object containing the
  post-decrement value, consistent with R-F5X4-XI2F. When the
  current counter value is zero, the request returns HTTP 409
  (Conflict) with a JSON error body naming the cause; the stored
  value is unchanged. The endpoint accepts the same authentication
  modes R-OCH3-8FQ8 defines for the mutation endpoints. An
  unauthenticated or invalid-auth request returns HTTP 401 and does
  not change the counter, consistent with R-T2JT-53WF. The endpoint
  is a protected endpoint for the purposes of R-MHYT-TIF7 (rejects
  cross-origin browser requests) and R-DH2I-28CK (resource-binding
  match for bearer-presented tokens).
- R-3R73-2TN9: `GET /counter` requires no authentication, consistent
  with R-SE5T-HP2J.
- R-4ED6-CGQG: `POST /counter/increment` requires a valid bearer
  access token issued by this service, presented in the standard
  `Authorization: Bearer <token>` header. The accepted token kind is
  the same one MCP gates increment with under R-ZQS0-HWZ8. In
  addition, the endpoint accepts the web-session-cookie mode
  R-OCH3-8FQ8 defines; one valid mode is sufficient.
- R-OCH3-8FQ8: the counter-mutation endpoints — `POST
  /counter/increment` (R-340Z-T6K2) and `POST /counter/decrement`
  (R-H3FE-QFC0) — accept either of two authentication modes on a
  per-request basis: (1) a valid bearer access token issued by this
  service, presented in the `Authorization: Bearer <token>` header
  per R-4ED6-CGQG; or (2) a valid web session cookie identifying an
  active session per R-SLGL-B5B4 / R-KJ15-9P17. A request that
  presents neither, or presents an invalid value of either, is
  treated as unauthenticated and rejected per R-53Z2-DNB1 (HTTP
  401, counter unchanged). When a request happens to carry both,
  the service accepts it if either is valid. The two auth modes are
  validated against their own stores; a mutation succeeding via
  session cookie does not touch the token store, and vice versa —
  the lifetime/revocation independence R-93PJ-FRPY pins between web
  sessions and MCP token chains is preserved at the mutation path.
  The intent: a single pair of endpoints serves both the
  MCP/CLI/script surface (bearer-token clients) and the browser's
  in-page +/- buttons on the index page (web-session clients),
  without duplicating endpoint logic.
- R-53Z2-DNB1: an unauthenticated or invalid-auth request to
  `POST /counter/increment` or `POST /counter/decrement` returns
  HTTP 401 and does not change the counter, consistent with
  R-T2JT-53WF.
