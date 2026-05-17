# MCP server

The service exposes an MCP server so agent-style clients (Claude
Desktop, Claude Code, GPT desktop apps, and other conformant clients)
can read and increment the counter as MCP tools.

## Transport

- R-UK7D-Z0IZ: the MCP server speaks the Streamable HTTP transport
  defined in the current Model Context Protocol specification.
- R-V65K-UVVH: the legacy HTTP+SSE transport is not provided. Older
  clients that cannot speak Streamable HTTP are out of scope.
- R-7A9U-HJFF: the MCP transport endpoint R-UK7D-Z0IZ pins is served
  at the path `/mcp` on the service's origin. This is the path
  component of the canonical resource identifier R-75E8-YGGN
  publishes and R-791Y-3ROQ validates `HAL_RESOURCE_IDENTIFIER`
  against. The path is fixed: the service does not derive it from
  the resource identifier at runtime, and the operator cannot
  configure a different path through environment or flags. A client
  configured with the service's origin (`http://localhost:3000` in
  dev, `https://hal.ai.metaspot.org` in production) reaches the
  MCP endpoint by appending `/mcp` — that is the URL the
  protected-resource metadata document advertises in its `resource`
  field and the URL a conformant MCP client uses as the MCP
  server's base URL.

## Client configuration

- R-VVRG-W2G2: an MCP client only needs the service's base URL
  (https://hal.ai.metaspot.org) to configure access. The client
  must not need to know about Google, about specific OAuth client
  credentials, or about any service-internal paths.
- R-WHPN-RXSK: the same base URL works from every targeted client
  without per-client configuration variants.

## Tools

- R-FUB4-KWWB: the server advertises exactly three tools, one per
  counter operation (R-ECNJ-R09R): a read tool (R-XS1U-B7YY), an
  increment tool (R-YHNQ-CEJJ), and a decrement tool
  (R-GG9B-GS8T). No other tools are exposed.
- R-XS1U-B7YY: the read tool accepts no arguments and returns the
  current counter value as a non-negative integer.
- R-YHNQ-CEJJ: the increment tool accepts no arguments. On success
  it adds one to the counter and returns the post-increment value.
- R-GG9B-GS8T: the decrement tool accepts no arguments. When the
  current counter value is greater than zero, on success it
  subtracts one from the counter and returns the post-decrement
  value, consistent with R-F5X4-XI2F. When the current counter
  value is exactly zero, the tool returns the standard MCP
  tool-error signal — the counter is not modified, and the error
  message names the cause (the counter cannot go below zero). A
  request that invokes the decrement tool requires a valid bearer
  access token issued by this service, by the same rule
  R-ZQS0-HWZ8 establishes for increment. When an MCP client
  presents no credentials and decrement needs them, the server
  responds with the standard signal that prompts the conformant
  client to start the OAuth flow per R-0YOE-9NO8.
- R-Z3LX-89W1: tool names and descriptions are written for a model
  audience: a model reading them should be able to choose the right
  tool without further context.

## Auth at the transport boundary

- R-ZQS0-HWZ8: a request that invokes the increment tool requires a
  valid bearer access token issued by this service.
- R-0CQ7-DSBQ: a request that invokes the read tool may be made
  unauthenticated. (The web index page reveals the same information.)
- R-0YOE-9NO8: when an MCP client presents no credentials and the
  requested tool needs them, the server responds with the standard
  signal that prompts a conformant client to start the OAuth flow
  defined in the MCP authorization specification.
- R-7BHQ-VB64: when the MCP transport endpoint (R-UK7D-Z0IZ /
  R-7A9U-HJFF) rejects a request because the caller presented no
  bearer token or an invalid one — the unauthenticated-tool signal
  R-0YOE-9NO8 names — the HTTP response carries a `WWW-Authenticate`
  header whose value names the `Bearer` scheme and includes a
  `resource_metadata` parameter pointing at the protected-resource
  metadata document URL R-75E8-YGGN pins (path
  `/.well-known/oauth-protected-resource/mcp`). This header is
  required by RFC 9728 §5.1 and the MCP authorization specification:
  without it, a conformant MCP client that hits the endpoint with
  no credentials cannot discover where the metadata document lives
  and therefore cannot begin the OAuth flow R-0YOE-9NO8 names. The
  observable failure this requirement fences: a client configured
  with the MCP endpoint URL reaches the endpoint, receives an
  unauthenticated rejection, fails to find a `resource_metadata`
  pointer in the response, and reports an MCP-server-misconfigured
  error instead of starting the OAuth flow.
- R-51PZ-MEQR: the MCP transport endpoint rejects presented-but-invalid
  bearer credentials at the HTTP authorization boundary, before any
  MCP tool handler runs. When a request to `/mcp` presents an
  Authorization header whose bearer token is malformed, unknown,
  expired, revoked, or not bound to this service's canonical resource
  identifier (R-75E8-YGGN / R-76M5-C87C), the HTTP response is `401
  Unauthorized` and carries the `WWW-Authenticate: Bearer ...`
  challenge R-7BHQ-VB64 defines, including an `error="invalid_token"`
  signal and a distinct `error_description` for the failure cause per
  R-EV2D-QTR1. The request is not reported to the MCP client as a
  successful JSON-RPC response, and it is not surfaced as a
  tool-level error with HTTP 200. In particular, a failed
  `counter_increment` or `counter_decrement` call with an invalid
  bearer token must not produce a `tools/call` result whose content is
  only text such as "bearer token resource binding does not match";
  the counter is not read for mutation, validated for mutation, or
  modified after bearer rejection.
- R-QGEN-MURV: a request to the MCP transport endpoint (R-UK7D-Z0IZ
  / R-7A9U-HJFF) that carries a valid bearer access token issued by
  this service is accepted and dispatched to the requested tool when
  it reaches the service through the production TLS-terminating proxy
  (R-PVA6-Q6OB) under the service's public host — the same
  `https://hal.ai.metaspot.org/mcp` URL a client configured per
  R-VVRG-W2G2 uses. The transport must not refuse such a request on
  the basis of how it reached the process. In production the
  application speaks plain HTTP on a loopback address while the
  externally-presented host is the public FQDN (R-PVA6-Q6OB); the
  transport must treat that proxied shape as the normal production
  topology, not as an attack to be blocked. The trust boundary the
  spec relies on at the transport is the bearer-token and
  resource-binding check R-51PZ-MEQR and R-75E8-YGGN already pin —
  not the listener's bind address, nor the relationship between that
  address and the request's `Host` header. The observable failure
  this requirement fences: a client configured with only the
  production base URL (R-VVRG-W2G2) completes the OAuth flow, the
  token endpoint issues a token, and the client's immediate
  authenticated reconnect to `/mcp` is rejected by the transport
  before any tool handler runs — solely because the request arrived
  under the public host through the proxy rather than directly on
  the loopback listener — so the client reports that the server
  rejected freshly-issued credentials, while the identical client
  and flow succeed against `http://localhost:3000/mcp`. Re-running
  the OAuth flow does not change the outcome, because the rejection
  does not depend on the credentials presented.
