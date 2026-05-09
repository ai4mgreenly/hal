# HTTP API

A small JSON API that mirrors the MCP tools, so the counter can also
be driven from curl, scripts, or any non-MCP HTTP client. The auth
posture matches MCP: read is open, increment requires a bearer token
issued by this service.

- R-2I2S-XB7K: `GET /counter` returns HTTP 200 with a JSON object
  containing the current count as a non-negative integer.
- R-340Z-T6K2: `POST /counter/increment` adds one to the counter and
  returns HTTP 200 with a JSON object containing the post-increment
  value.
- R-3R73-2TN9: `GET /counter` requires no authentication, consistent
  with R-SE5T-HP2J.
- R-4ED6-CGQG: `POST /counter/increment` requires a valid bearer
  access token issued by this service, presented in the standard
  `Authorization: Bearer <token>` header. The accepted token kind is
  the same one MCP gates increment with under R-ZQS0-HWZ8.
- R-53Z2-DNB1: an unauthenticated or invalid-token request to
  `POST /counter/increment` returns HTTP 401 and does not change the
  counter, consistent with R-T2JT-53WF.
