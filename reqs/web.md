# Web UI

The browser-facing surface of the service. Deliberately minimal: the
demo's interesting moving parts live in the MCP and auth layers, not
the page.

- R-QY5R-PYDH: visiting the site's root URL renders the current count
  as a number, in plain server-rendered HTML. No authentication is
  required to view it.
- R-TFIQ-6805: the index page renders "H.A.L." (with a period after
  each letter, including a trailing period) as a banner heading, and
  directly beneath it a single-line subtitle drawn from a fixed
  server-side list of acronym expansions for the project name. The
  pick is made server-side on each request, so a visitor with
  JavaScript disabled still sees a rendered subtitle (consistent with
  R-TK21-6AGY). The list is deliberately a mix of plausible
  expansions and obvious jokes; no entry is the canonical meaning,
  and the project does not document which expansions are "real."
  Successive requests may show the same expansion (selection is
  uniform at random, not round-robin), but every entry in the list is
  reachable. The subtitle line ends with a clickable reroll
  affordance: a plain anchor whose `href` is the index page itself,
  so following it re-issues the request and yields a freshly-picked
  expansion. The affordance does not require JavaScript; it is a
  static `<a>` element. The exact contents of the list, the glyph
  used for the affordance, and the page styling are HOW. The
  properties under test are that the page shows "H.A.L." plus exactly
  one expansion taken from the configured set on each request, and
  that the subtitle line contains a `<a href="/">` reroll link.
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
