# MCP server

The service exposes an MCP server so agent-style clients (Claude
Desktop, Claude Code, GPT desktop apps, and other conformant clients)
can read and increment the counter as MCP tools.

## Transport

- R-UK7D-Z0IZ: the MCP server speaks the Streamable HTTP transport
  defined in the current Model Context Protocol specification.
- R-V65K-UVVH: the legacy HTTP+SSE transport is not provided. Older
  clients that cannot speak Streamable HTTP are out of scope.

## Client configuration

- R-VVRG-W2G2: an MCP client only needs the service's base URL
  (https://ouroboros.ai.metaspot.org) to configure access. The client
  must not need to know about Google, about specific OAuth client
  credentials, or about any service-internal paths.
- R-WHPN-RXSK: the same base URL works from every targeted client
  without per-client configuration variants.

## Tools

- R-X4VR-1KVR: the server advertises exactly two tools, one for each
  counter operation: a read tool and an increment tool.
- R-XS1U-B7YY: the read tool accepts no arguments and returns the
  current counter value as a non-negative integer.
- R-YHNQ-CEJJ: the increment tool accepts no arguments. On success it
  adds one to the counter and returns the post-increment value.
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
