# Web UI

The browser-facing surface of the service. Deliberately minimal: the
demo's interesting moving parts live in the MCP and auth layers, not
the page.

- R-QY5R-PYDH: visiting the site's root URL renders the current count
  as a number, in plain server-rendered HTML. No authentication is
  required to view it.
- R-TFIQ-6805: the index page presents the project name as "H.A.L."
  (with a period after each letter, including a trailing period) as
  a banner heading, and directly beneath it a one-line subtitle that
  is one entry chosen from the following fixed list of acronym
  expansions for the project name:

      - Holistic Access Layer
      - Human Augmentation Layer
      - Heuristic Agent Liaison
      - Home, APIs, Library
      - Heuristically programmed ALgorithm
      - Helpful Autonomous Liaison
      - Hyperlocal Agent Layer
      - Host Agent Liaison
      - Has Always Listened
      - House Always Loses

  The list is deliberately a mix of plausible readings and obvious
  jokes; no entry is canonical, and the project does not document
  which expansions are "real." The choice is made per request —
  successive page loads can show different expansions — and the
  page is consistent with R-TK21-6AGY (a visitor with JavaScript
  disabled sees a fully-rendered subtitle on the first request).
  Selection is uniform at random over the list (not round-robin),
  but every entry is reachable. The page also provides a control by
  which the visitor can re-roll the subtitle without leaving the
  page; activating the control yields a freshly-picked expansion.
  This control is also consistent with R-TK21-6AGY (it works with
  JavaScript disabled). The glyph or label of the re-roll control,
  the markup used for the banner and subtitle, and any page styling
  are HOW. The properties under test: the page shows "H.A.L." and
  exactly one of the listed expansions on each request; the page
  exposes a re-roll control whose activation produces a freshly-
  picked expansion from the list.
- R-SY3U-AF4G: the index page does not offer any in-page control to
  mutate the count. Mutation is the API/MCP path's job, and that path
  requires authentication.
- R-TK21-6AGY: the index page is usable with JavaScript disabled.
- R-BZQY-DN3B: the index page also displays MCP client configuration
  showing how to connect an agent client to this service. Two
  clients are covered with their own copy-pasteable instructions:
  Claude Code and Claude Desktop. The instructions include the base
  URL the client should be pointed at and any other configuration
  each client needs to wire up the connection — but no Google
  details, no client credentials, and no service-internal paths
  beyond the base URL, consistent with R-VVRG-W2G2.
- R-5GQZ-KWCD: each client's instructions are in the format that
  client itself documents for adding an HTTP-transport MCP server —
  for Claude Code, the `claude mcp add --transport http …` form (or
  the equivalent `.mcp.json` / `~/.claude.json` block); for Claude
  Desktop, the `claude_desktop_config.json` `mcpServers` block. A
  user can paste the displayed instructions directly into their
  client without translation.
- R-CO4Y-11X7: the URLs shown in that configuration are derived
  from the request the visitor used to reach the page. Visiting
  `http://localhost:3000/` shows a `http://localhost:3000` base URL;
  visiting `https://hal.ai.metaspot.org/` shows that origin.
  The page does not hard-code any specific host or scheme.
- R-DA34-WX9P: when a visitor reaches the page through a TLS-
  terminating proxy (the production posture per R-PVA6-Q6OB), the
  configuration shown displays `https://` URLs even though the
  application process itself spoke plain HTTP to the proxy. The
  application honors the standard forwarded-protocol signal supplied
  by the proxy.
