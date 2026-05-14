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
