# Web UI

The browser-facing surface of the service. A landing page that
demonstrates the MCP-mutable counter primitive and provides copy-paste
configuration for connecting agent clients. The visual chrome follows a
specific design reference (R-LRSQ-5VDG); the requirements below pin the
observable properties and call out where the codebase's behavior
deliberately differs from the reference.

## Visual reference

- R-LRSQ-5VDG: the index page is rendered at high visual fidelity to
  the design reference checked into `reqs/design/HAL.html`, with the
  annotated walkthrough in `reqs/design/README.md`. The reference
  defines the color tokens, typography scale, page layout, card
  chrome, motion timings, and visual treatment of each element. The
  build agent recreates that design in the codebase's existing
  environment (Rails ERB + the project's CSS approach), reading the
  reference for visual specifics. The load-bearing design tokens —
  warm off-white background (`#f6f5f1`), dark ink (`#14130f`), red
  HAL-lens accent (`#d4361e`), JetBrains Mono for code, Inter for UI,
  and the type scale running from the 88px HAL title down to the 11px
  mono badge — are the property and are pinned here. Exact padding,
  gap, breakpoint, and radius values are HOW and live in the
  reference. Deliberate deviations from the reference are enumerated
  by the surrounding requirements: the reference's localStorage-based
  auth is replaced by real Google federation (R-CXJ2-R3BN /
  R-3BKZ-L7R4); the reference's localStorage counter is replaced by
  the server-side counter (R-UC3P-Z0IX / R-VNNS-W2G0); the
  reference's static MCP base URL is replaced by the request-derived
  URL (R-CO4Y-11X7 / R-DA34-WX9P); the reference's footer status
  text is rephrased to avoid disclosing the listening port
  (R-QKYG-HAO2); the reference's subtitle list is replaced by the
  merged bank in R-MG6P-TA7C.

## Baseline

- R-QY5R-PYDH: visiting the site's root URL renders the current count
  as a number, in plain server-rendered HTML. No authentication is
  required to view it.
- R-TK21-6AGY: the index page is usable with JavaScript disabled. All
  the page's primary affordances — the re-roll control (R-N3CT-2XAJ),
  the `+` / `−` counter buttons when signed in (R-NRQS-QC4F), the
  Claude Code scope tabs (R-OZN6-I2TF), and the sign-in / sign-out
  affordance (R-AZZW-UX8U) — function via standard HTML form
  submission plus same-page reload when JS is unavailable. JS-driven
  enhancements (live counter updates, in-place animation, tab
  switching without reload) are additive and degrade gracefully.

## Banner card

- R-N3CT-2XAJ: the page presents the project name as a banner card
  near the top with the chrome the design reference pins. The card
  contains, in fixed positions:
  - **Lens dot** (top-left, absolutely positioned, ~14×14): a radial-
    gradient red disk evoking HAL 9000's eye, surrounded by a soft
    outer glow that pulses on a 2.6-second ease-in-out infinite
    loop. This is the only piece of non-hover motion on the page
    and is load-bearing; it is decorative (`aria-hidden="true"`)
    and degrades to a static dot under `prefers-reduced-motion`
    per R-RLJF-YEWW.
  - **Tag** (top-right, absolutely positioned): mono 11px uppercase
    muted ink with content `MCP · demo`.
  - **Title**: the literal text `HAL`, rendered in the display
    weight at 88px on viewports ≥640px and 64px on viewports
    <640px. (The earlier `H.A.L.` spelling — with periods between
    every letter — is replaced by the bare `HAL` the design reference
    uses; the band-name typography is the visual identity, not the
    punctuation.)
  - **Subtitle row** directly beneath the title: the randomly-
    selected entry from the bank R-MG6P-TA7C carries, rendered in
    italic 18px in soft-ink color, followed inline by the re-roll
    control.
  - **Re-roll control**: a small circular button (~28×28, 1px
    border, transparent fill) with a refresh icon glyph.
    Activating it produces a freshly-picked subtitle per
    R-MG6P-TA7C. With JS enabled, the swap fades the old subtitle
    out and the new one in over ~220ms. With JS disabled
    (R-TK21-6AGY), activating the control issues a same-page
    request that re-renders the page with a different subtitle.
    The button carries `aria-label="New subtitle"`.
- R-MG6P-TA7C: the subtitle is one entry chosen uniformly at random,
  per page render, from the following fixed list of acronym
  expansions for the project name:

      - Holistic Access Layer
      - Human Augmentation Layer
      - Heuristic Agent Liaison
      - Home, APIs, Library
      - Heuristically programmed ALgorithm
      - Heuristically programmed ALgorithmic computer
      - Helpful Autonomous Liaison
      - Hyperlocal Agent Layer
      - Host Agent Liaison
      - Has Always Listened
      - House Always Loses
      - Hardware Abstraction Layer
      - Hyperdimensional Access Layer
      - Holistic Application Logic
      - Highly Adaptive Listener
      - Headless Agent Loop
      - Hosted Action Library
      - Hermetic Authorization Layer
      - Hypertext Application Language
      - High-Availability Lambda
      - Heretical Automation Layer
      - Hyper-tuned Agent Logic
      - Handy Autoresponse Layer
      - Hallucination Avoidance Layer
      - Honest Assistant, Lately
      - Halfway Awake Loop
      - Homemade Agent Lab
      - Heuristic Argument Linker

  The list is a deliberate mix of plausible readings, tech-stack
  in-jokes, and 2001-allusion jokes; no entry is canonical and the
  project does not document which expansions are "real." Selection
  is uniform at random (not round-robin), but every entry is
  reachable. Re-roll (R-N3CT-2XAJ) yields a freshly-picked entry
  on each activation. This requirement supersedes the earlier
  10-entry bank — the design reference contributed additional
  entries and they are merged here.

## Counter card

- R-NRQS-QC4F: directly below the banner the page renders a counter
  card with the chrome the design reference pins. The card contains:
  - **Label**: small uppercase mono `CURRENT COUNT` in muted ink.
  - **Value**: the current counter value (R-2I2S-XB7K) rendered in
    a large monospaced display weight — 56px on viewports ≥640px,
    44px on viewports <640px.
  - **Buttons**: two icon buttons `−` and `+`, ~42×42 each, with
    the chrome the design reference pins (1px border, 8px radius,
    20px glyph). They carry `aria-label="Decrement"` and
    `aria-label="Increment"` respectively.
  - **Hint line** beneath the card, in 12px muted ink. When the
    visitor has an active web session: *"Signed in. The MCP server
    can read & mutate this counter on your behalf."* When not:
    *"Sign in to manipulate the counter from the browser. The MCP
    server can read & mutate it for any signed-in user."*

  When the visitor has an active web session (R-AZZW-UX8U), the
  `+` and `−` buttons are functional: `+` POSTs to
  `/counter/increment` (R-340Z-T6K2), `−` POSTs to
  `/counter/decrement` (R-H3FE-QFC0), with the web-session cookie
  carrying authentication per R-OCH3-8FQ8. A successful click
  changes the stored counter and the page reflects the new value
  via the live-update channel (R-DRX9-8WNY). A `−` click against
  a counter currently at zero is rejected by the endpoint
  (R-F5X4-XI2F / R-H3FE-QFC0); the page conveys the rejection
  briefly and does not change the displayed value.

  When the visitor has no active web session, the `+` and `−`
  buttons are rendered visibly disabled (~40% opacity,
  `cursor: not-allowed`, the HTML `disabled` attribute on the
  underlying control) and a click does nothing. The buttons are
  present on the page in this state — they are not hidden — so
  the visitor sees what becomes available after sign-in.

  With JS enabled, button clicks update the page in place via the
  live channel (R-DRX9-8WNY) and trigger the visual transition
  the design reference pins: the value flashes in the accent
  color for ~600ms and a small green delta indicator (`+1` or
  `−1`) slides in from a few pixels and fades out. With JS
  disabled (R-TK21-6AGY), the buttons function via standard form
  submission (POST + 303 redirect to `/`); the page reloads with
  the new value and no in-place animation fires.

  This requirement supersedes the earlier posture that the index
  page offered no in-page mutation control — the project's posture
  changed when the counter card adopted the design reference's
  +/- chrome and the mutation endpoints were broadened to accept
  web sessions (R-OCH3-8FQ8).
- R-DRX9-8WNY: while a visitor's browser has the index page open
  and JavaScript is running, any change to the counter — regardless
  of which caller produced it (web `+`/`−` button, MCP increment or
  decrement tool, HTTP API endpoint, or any future operation) — is
  reflected on the page within a small bound (target sub-second,
  upper bound ~a few seconds) without the visitor having to reload.
  The page maintains a live connection to the server for this
  purpose; SSE on a dedicated endpoint is the natural fit and the
  spec's intended posture, but the choice of transport is HOW. The
  connection carries only the counter value (already public per
  R-SE5T-HP2J / R-3R73-2TN9 / R-0CQ7-DSBQ) and conveys no per-user,
  auth-protected, or session-specific data. The connection requires
  no authentication; an unauthenticated visitor sees counter updates
  the same way an authenticated visitor does. On update receipt, the
  page updates the rendered count in place and runs the visual
  transition R-NRQS-QC4F describes, modulo reduced-motion
  suppression per R-RLJF-YEWW. With JS disabled (R-TK21-6AGY), the
  page does not maintain a live connection; the displayed count is
  whatever the initial server render returned and subsequent
  changes are not reflected until the page is reloaded — live
  update is a JS-only enhancement, not a baseline.

## MCP client instructions

- R-BZQY-DN3B: the index page displays MCP client configuration
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
- R-PLLD-DY5X: the instructions area sits below the counter card and
  has the structure the design reference pins:
  - **Area header**: an `h2` reading "Connect an MCP client" on the
    left; a 13px muted-ink subhead on the right reading
    `Point a client at <base-url>/mcp` with `<base-url>` derived
    from the request per R-CO4Y-11X7 / R-DA34-WX9P.
  - **Two section cards stacked vertically**, one per supported
    client. Each section card has the chrome the design reference
    pins (1px border, 10px radius, off-white fill), with a header
    containing: a numeric badge (`01` for Claude Code, `02` for
    Claude Desktop) rendered in mono inside a small rounded chip,
    the client title in 15px display weight, and a right-aligned
    13px muted-ink description of the client kind (e.g.
    `CLI · adds a server entry to a scope`).
  - **Body of each section card** contains one or more dark-
    background code blocks rendered in JetBrains Mono on the
    design-reference's `--code-bg` fill (`#14130f`). Each code
    block exposes a copy affordance — an overlay button labeled
    `copy` with a small clipboard glyph — that copies the block's
    plaintext to the visitor's clipboard via the standard
    `navigator.clipboard.writeText` API (with the textarea +
    `execCommand` fallback). On successful copy, the affordance
    flips to a `copied` label in the design-reference's mint-green
    color for ~1.4s and then reverts.
  - Token coloring in code blocks (CLI flags in warm tan
    `#e0a96d`, command name in cool blue `#a8c8ff`, URLs and
    strings in mint green `#79d4a9`, prompts and punctuation in
    muted gray `#8c8a82`) follows the design reference. The exact
    palette is HOW; the property is that code is highlighted with
    semantic token coloring rather than rendered as undifferentiated
    monospace.
  - The Claude Desktop section card contains a single code block
    with the JSON snippet that names this service in
    `claude_desktop_config.json`, with the URL derived from the
    request per R-CO4Y-11X7.
  - The Claude Code section card uses the tab layout R-OZN6-I2TF
    pins for its two scope examples.
- R-OZN6-I2TF: the Claude Code section card renders the two
  scope examples in a tab layout with two tab triggers — `Project
  scope` (with small mono `.mcp.json`) and `User scope` (with
  small mono `~/.claude.json`) — with the Project tab active by
  default. Switching tabs swaps the visible code block and the
  short scope-meta description above it:
  - Project: *"Commits the server to `.mcp.json` at the repo root —
    everyone on the project gets it."* Code:
    `$ claude mcp add --transport http --scope project hal <base-url>/mcp`
  - User: *"Records the server in `~/.claude.json` — available in
    every project you open."* Code:
    `$ claude mcp add --transport http --scope user hal <base-url>/mcp`

  `<base-url>` is the request-derived value per R-CO4Y-11X7 /
  R-DA34-WX9P.

  With JS enabled, the tabs are interactive — the selected tab
  gets the design reference's accent-red bottom border and
  full-strength ink color; the unselected tab is muted. With JS
  disabled (R-TK21-6AGY), the tabs degrade to both panels being
  visible — the visitor sees both scope examples without needing
  to click. Activating a tab does not require a network request.

  Tab semantics use the WAI-ARIA `tablist` / `tab` / `tabpanel`
  pattern with `aria-selected`, `aria-controls`, and
  `aria-labelledby` wired correctly (R-RLJF-YEWW). Each code
  block independently exposes a copy affordance per R-PLLD-DY5X.

  This requirement supersedes the earlier two-example-with-
  unspecified-layout posture; the layout is now specifically
  tabbed with the no-JS fallback to both-visible.
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

## Footer

- R-QKYG-HAO2: the page renders a footer below the instructions
  area, separated by a top border, with the chrome the design
  reference pins (12px mono, muted ink, flex row). The footer
  contains:
  - **Left**: a small green status indicator dot (~7×7, with a
    soft glow) followed by the text "MCP server live". The
    indicator is decorative (the textual phrase carries the
    meaning). The text deliberately omits the listening port — a
    deployment-internal detail the index page does not disclose.
    This is the named deviation from the design reference, which
    showed "server listening on :3000".
  - **Right**: the literal text "v0.1.0 · open my pod bay doors".
    The version string identifier is HOW (the build agent picks
    how it's sourced — a constant, a Rake-emitted value, etc.);
    the "open my pod bay doors" allusion is deliberate flavor and
    is preserved verbatim.

## Motion + accessibility

- R-RLJF-YEWW: the index page honors visitor preferences for
  reduced motion and presents an accessible structure for the
  interactive controls it exposes.

  **Reduced motion.** When the visitor's browser reports
  `prefers-reduced-motion: reduce`, the page suppresses:
  - The lens-dot pulse animation (R-N3CT-2XAJ): the dot is
    rendered without the pulsing outer glow.
  - The subtitle fade-swap (R-N3CT-2XAJ): the re-roll updates the
    subtitle without an opacity transition.
  - The counter flash and delta animation (R-NRQS-QC4F): the new
    value is shown without the color flash and the delta indicator
    is not animated.
  - Hover-driven transforms (re-roll rotate, copy-button feedback):
    the visual end-states are still rendered, but the transition
    itself is instant.

  **ARIA semantics.**
  - The tab pattern for Claude Code scopes (R-OZN6-I2TF) uses
    `role="tablist"` / `role="tab"` / `role="tabpanel"` with
    `aria-selected`, `aria-controls`, and `aria-labelledby` wired
    per the WAI-ARIA APG.
  - The counter `+` / `−` buttons (R-NRQS-QC4F) carry
    `aria-label="Increment"` and `aria-label="Decrement"`, the
    HTML `disabled` attribute when no web session is active, and
    sit adjacent to an `aria-live="polite"` region around the
    counter value so live-channel updates (R-DRX9-8WNY) are
    announced to assistive tech.
  - The re-roll button (R-N3CT-2XAJ) carries
    `aria-label="New subtitle"`.
  - The decorative lens dot (R-N3CT-2XAJ) carries
    `aria-hidden="true"`. The footer status dot
    (R-QKYG-HAO2) likewise.

## Auth routes

- R-9PNQ-BN2G: a request to `GET /login` from a user-agent without an
  active web session immediately initiates the federation flow
  R-8GJG-64MR defines — the response is a redirect to Google, with no
  service-rendered "Sign in with Google" interstitial in between. The
  visitor's first observable click takes them to Google's login
  screen. A request to `/login` from a user-agent that already carries
  an active web session redirects to `/` instead of starting a fresh
  federation round-trip.
- R-AE1P-Z1WC: `/logout` ends the current web session and returns the
  user-agent to `/` via redirect; the resulting `/` response shows the
  visitor as not signed in. `/logout` from a user-agent that has no
  active web session is a no-op redirect to `/`, not an error.
  `/logout` does not touch any MCP token chain (R-93PJ-FRPY).
- R-AZZW-UX8U: the index page reflects web-session state. When the
  visitor has an active web session, the page identifies them by the
  Google email recorded for the session — the email appears verbatim
  on the page — and the page exposes an affordance whose activation
  reaches `/logout`. When the visitor has no active web session, the
  page exposes an affordance whose activation reaches `/login`; the
  page does not render any anonymous-visitor placeholder identity
  (no literal "guest", no fake username). Placement and styling of
  these affordances follow the design reference R-LRSQ-5VDG cites
  (top-bar pill buttons, with the signed-in state optionally
  preceded by an avatar element). The property: exactly one of the
  two states is reflected on every page load and the corresponding
  route is reachable from the page itself.
