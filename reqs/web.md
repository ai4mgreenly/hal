# Web UI

The browser-facing surface of the service. A landing page that
demonstrates the MCP-mutable counter primitive and provides copy-paste
configuration for connecting agent clients. The visual chrome follows a
specific design reference (R-9U99-YATL); the requirements below pin the
observable properties and call out where the codebase's behavior
deliberately differs from the reference.

## Visual reference

- R-9U99-YATL: the index page is rendered at high visual fidelity to
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
  mono badge — are the property and are pinned here. Equally load-
  bearing, and equally part of this requirement: the page content is
  **centered with a max content width on the order of 880px** (not
  rendered full-bleed across the viewport), and the index is
  **visibly card-grouped** — the banner, the counter, and each MCP
  client section render inside distinct bordered, rounded cards
  separated by negative space, not as flat full-width sections of the
  page. Exact padding, gap, breakpoint, and radius values are HOW and
  live in the reference. Verifying this requirement means comparing
  the rendered page to the reference at a layout level — confirming
  the centered container, the card grouping, and the type/color
  tokens are actually realized in the rendered output — not just
  asserting that the tokens are declared somewhere in the
  stylesheet. Deliberate deviations from the reference are enumerated
  by the surrounding requirements: the reference's localStorage-based
  auth is replaced by real Google federation (R-CXJ2-R3BN /
  R-3BKZ-L7R4); the reference's localStorage counter is replaced by
  the server-side counter (R-UC3P-Z0IX / R-VNNS-W2G0); the
  reference's static MCP base URL is replaced by the request-derived
  URL (R-CO4Y-11X7 / R-DA34-WX9P); the reference's footer status
  text is rephrased to avoid disclosing the listening port
  (R-K3PV-GHB3); the reference's subtitle list is replaced by the
  merged bank in R-MG6P-TA7C.

## Baseline

- R-QY5R-PYDH: visiting the site's root URL renders the current count
  as a number, in plain server-rendered HTML. No authentication is
  required to view it.
## Banner card

- R-K7QI-9YW4: the page presents the project name as a banner card
  near the top with the chrome the design reference pins. The card
  contains, in fixed positions:
  - **Lens dot** (top-left, absolutely positioned, ~14×14): a radial-
    gradient red disk evoking HAL 9000's eye, surrounded by a soft
    outer glow that pulses on a 2.6-second ease-in-out infinite
    loop. This is the only piece of non-hover motion on the page
    and is load-bearing; it is decorative (`aria-hidden="true"`)
    and degrades to a static dot under `prefers-reduced-motion`
    per R-9VH6-C2KA.
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
    border, transparent fill) with a refresh icon glyph,
    rendered as a `<button>` element (or another non-navigating
    control) — **not as an `<a>` whose `href` would navigate the
    browser away from the current page**. Activating it produces
    a freshly-picked subtitle per R-MG6P-TA7C, swapped into the
    existing subtitle element in place with a ~220ms cross-fade.

    The activation MUST be observable as a perceivable change to
    the visible subtitle text without a page reload: the URL bar
    does not change, the document does not navigate, no `GET /`
    is issued to the server, and the surrounding page state
    (scroll position, focus, open SSE connection, etc.) is
    preserved across the activation. A new subtitle MAY happen to
    match the prior one because the bank R-MG6P-TA7C is sampled
    uniformly at random with 28 entries — that is acceptable and
    rare — but it must remain rare; an implementation that
    deterministically returns the same subtitle on consecutive
    activations does not satisfy this requirement. An
    implementation in which the control is a plain `<a href="/">`
    that triggers a full page reload (and therefore depends on
    the server picking a different subtitle on the next render,
    while losing all client state) does not satisfy this
    requirement.

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
  reachable. Re-roll (R-K7QI-9YW4) yields a freshly-picked entry
  on each activation. This requirement supersedes the earlier
  10-entry bank — the design reference contributed additional
  entries and they are merged here.

## Counter card

- R-NG6O-94I2: directly below the banner the page renders a counter
  card with the chrome the design reference pins. **Layout**: at
  viewports ≥640px the card lays out as a single horizontal row —
  the label and value sit on the left edge of the card; the `−` and
  `+` buttons sit on the right edge of the same row, side by side.
  The buttons are *not* stacked above and below the value, and they
  do not occupy the full width of the card. At viewports <640px the
  card collapses to a vertical column (label/value above, buttons
  below) per the design reference's small-viewport breakpoint. The
  card contains:
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

  When the visitor has no active web session, the `+` and `−`
  buttons are rendered visibly disabled (~40% opacity,
  `cursor: not-allowed`, the HTML `disabled` attribute on the
  underlying control) and a click does nothing. The buttons are
  present on the page in this state — they are not hidden — so
  the visitor sees what becomes available after sign-in.

  The behavior of the `+` and `−` buttons when the visitor *does*
  have an active web session — what they POST and how the page
  reflects the change — is pinned by R-1LLM-Y4XF.

  This requirement supersedes the earlier posture that the index
  page offered no in-page mutation control — the project's posture
  changed when the counter card adopted the design reference's
  +/- chrome and the mutation endpoints were broadened to accept
  web sessions (R-OCH3-8FQ8).
- R-1LLM-Y4XF: when a visitor with an active web session
  (R-NJUD-EFQ5) activates the index page's `+` or `−` counter
  button, the click drives an actual mutation against the canonical
  counter and the displayed value reflects the result. Concretely:
  the `+` button POSTs to `/counter/increment` (R-340Z-T6K2) and
  the `−` button POSTs to `/counter/decrement` (R-H3FE-QFC0), with
  the web-session cookie carrying authentication per R-OCH3-8FQ8
  (no bearer token is required from the browser). On a successful
  mutation, the displayed count updates to the new value. A `−`
  click against a counter currently at zero is rejected by the
  endpoint per R-F5X4-XI2F / R-H3FE-QFC0; the page conveys the
  rejection briefly and the displayed value does not change.

  With JavaScript enabled, the click is handled in-page (no full
  reload): the request is issued asynchronously and the new value
  arrives via the live-update channel R-K65O-80SH, accompanied by
  the visual transition pinned below. Modulo reduced-motion
  suppression per R-9VH6-C2KA, every observed change to the
  displayed counter value — whether produced by this visitor's
  own `+`/`−` click or by another caller (MCP, HTTP API, another
  browser tab) — fires the same transition. Concretely:

  - **Value flash**: at the moment the displayed value changes, the
    digit is rendered in the design reference's red accent
    (`#d4361e`) and remains visibly in that color for **at least
    600ms** before returning to the default ink color. The
    transition into and out of the accent is permitted but must
    not consume the 600-millisecond perceivable window — i.e. the
    digit is unambiguously red for the full 600ms, not "fading
    through red." The change is not a subtle tint; the new
    numeral reads as red, not as a slightly-warmer-than-usual ink,
    against the page background.
  - **Delta indicator**: at the moment the displayed value changes,
    a small element bearing the literal text `+N` or `−N` (where
    N is the absolute difference between the previous displayed
    value and the new one — usually 1) is inserted into the page
    adjacent to the counter value. The element is rendered in the
    design reference's mint-green color (`#79d4a9`), at a weight
    and size that read clearly against the page background (the
    design reference pins ~18px, ~600 weight). It is **visible for
    at least 600ms** from insertion before fading or removing, and
    it must not visually collide with the counter value itself
    (the value remains legible the entire time). Whether it slides
    a few pixels vertically and fades — the design reference's
    treatment — or animates in some other way is HOW; the property
    is that the visitor unambiguously sees a `+1` or `−1` cue
    appear and disappear in the same gesture as the value change.

  An implementation in which the value's color briefly tints
  toward the accent (because, e.g., a CSS color transition is
  applied without a sustain-at-target keyframe), or in which the
  delta element exists in the DOM but its visible duration is
  measured in milliseconds rather than hundreds of milliseconds,
  does not satisfy this requirement: the perceivable property is
  "a visitor sees a red flash plus a `+1`/`−1` cue every time the
  number changes," and that property must hold without the
  visitor having to look for it.

  This requirement is the standalone observable: a visitor who is
  signed in and clicks `+` or `−` sees the count change. A
  rendering that paints the buttons but leaves them inert — no
  network request issued, no value change, no error surfaced —
  does not satisfy this requirement. The click MUST cause an HTTP
  POST to `/counter/increment` or `/counter/decrement` to actually
  reach the server; a client-side handler that consumes the click
  and then fails to issue the request (silent fetch error,
  swallowed exception, etc.) does not satisfy this requirement.
  (Disabled-state rendering for signed-out visitors remains pinned
  by R-NG6O-94I2.)
- R-K65O-80SH: while a visitor's browser has the index page open
  and JavaScript is running, any change to the counter — regardless
  of which caller produced it (web `+`/`−` button, MCP increment or
  decrement tool, HTTP API endpoint, or any future operation) — is
  reflected on the page within 2 seconds without the visitor having
  to reload, with sub-second latency as the design target. The
  choice of transport is HOW; the build agent picks among the
  candidates Rails offers (ActionCable, Turbo Streams, SSE,
  polling, …) under the canonical-Rails preference R-K8LG-ZK9V
  pins, and any chosen transport must remain compatible with the
  shutdown deadline R-K7DK-LSJ6 pins — a transport whose handler
  blocks process exit on an in-flight request is disqualified.
  The channel carries only the counter value (already public per
  R-SE5T-HP2J / R-3R73-2TN9 / R-0CQ7-DSBQ) and conveys no per-user,
  auth-protected, or session-specific data. The channel requires
  no authentication; an unauthenticated visitor sees counter updates
  the same way an authenticated visitor does. On update receipt, the
  page updates the rendered count in place and runs the visual
  transition R-NG6O-94I2 describes, modulo reduced-motion
  suppression per R-9VH6-C2KA.

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
- R-9WP2-PUAZ: the instructions area sits below the counter card and
  has the structure the design reference pins:
  - **Area header**: an `h2` reading "Connect an MCP client" on the
    left; a 13px muted-ink subhead on the right reading
    `Point a client at <base-url>/mcp` with `<base-url>` derived
    from the request per R-CO4Y-11X7 / R-DA34-WX9P.
  - **Two section cards stacked vertically**, one per supported
    client, each rendered as a **visibly bordered, rounded card with
    its own off-white fill and clear separation from its neighbor and
    from the area header** — not as flat full-width sections divided
    only by whitespace. (Exact border, radius, and fill values are
    HOW and live in the design reference; the property is that each
    client's instructions are visually contained as a card.) Each
    section card's header carries, all three present together: a
    numeric badge (`01` for Claude Code, `02` for Claude Desktop)
    rendered in mono inside a small rounded chip; the client title
    rendered as the literal text `Claude Code` or `Claude Desktop`
    in 15px display weight; and a right-aligned 13px muted-ink
    description of the client kind (e.g. `CLI · adds a server entry
    to a scope`). A rendering that omits the client name from the
    header — showing only the numeric badge — does not satisfy
    this requirement.
  - **Body of each section card** contains one or more dark-
    background code blocks rendered in JetBrains Mono on the
    design-reference's `--code-bg` fill (`#14130f`). **Every code
    block in this area renders a visible `copy` button** — an
    overlay positioned inside the block, labeled `copy` with a
    small clipboard glyph — that copies the block's plaintext to
    the visitor's clipboard via the standard
    `navigator.clipboard.writeText` API (with the textarea +
    `execCommand` fallback). On successful copy, the affordance
    flips to a `copied` label in the design-reference's mint-green
    color for ~1.4s and then reverts. A rendering that ships the
    code blocks without copy buttons does not satisfy this
    requirement.

    **What lands on the clipboard is the executable form of the
    snippet — not the visual framing around it.** A leading shell-
    prompt prefix (a `$ ` for a sh-style prompt or a `> ` for a
    PowerShell-style prompt) that appears in the rendered block to
    cue the reader "this is a shell command" is *not* included in
    the text written to the clipboard. So a Claude Code block that
    displays as `$ claude mcp add --transport http --scope project
    hal http://localhost:3000/mcp` writes only `claude mcp add
    --transport http --scope project hal http://localhost:3000/mcp`
    to the clipboard — pasted verbatim into the visitor's terminal,
    it executes; it does not error with `$: command not found`.
    Trailing whitespace and the trailing newline policy are HOW;
    the property is that the clipboard payload is what the visitor
    needs to paste, with framing characters stripped. JSON snippets
    (e.g. the Claude Desktop block) carry no shell prompt and are
    copied verbatim.
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
  - The Claude Code section card lays out its two scope examples
    as a functional tabbed interface per R-9T1D-KJ2W.
- R-9T1D-KJ2W: the Claude Code section card renders its two scope
  examples inside a small **functional** tab interface: two tab
  triggers above one code-block panel, where activating a trigger
  swaps which scope's snippet is visible in the panel below.

  **Triggers**: two side-by-side controls, each rendered as a
  `<button>` element (or another non-navigating control — never
  an `<a>` whose `href` would navigate the browser), styled with
  the design reference's tab chrome (no fill, a 2px bottom border
  that is transparent for the inactive trigger and the accent red
  `#d4361e` for the active one; muted-ink label for inactive,
  full-strength ink for active). Trigger labels:
  - `Project scope` followed by small mono `.mcp.json`
  - `User scope` followed by small mono `~/.claude.json`

  The `Project scope` trigger is the **default active tab** on
  first render.

  **Panel**: a single code block visible at a time, holding the
  active scope's snippet:
  - Project active → `claude mcp add --transport http --scope project hal <base-url>/mcp`
  - User active → `claude mcp add --transport http --scope user hal <base-url>/mcp`

  `<base-url>` is the request-derived value per R-CO4Y-11X7 /
  R-DA34-WX9P. The panel exposes a `copy` affordance per
  R-9WP2-PUAZ, and what the copy writes to the clipboard is the
  active panel's executable form (no `$ ` prompt prefix, per the
  clipboard property in R-9WP2-PUAZ).

  **Behavior on activation**: clicking the inactive trigger
  changes the visible state of the tab interface in place. The
  active trigger's accent-red bottom border moves to the just-
  clicked trigger; the previously active trigger goes muted; the
  panel's text content swaps to the other scope's snippet. No
  page reload, no URL change, no `GET /` issued to the server,
  scroll position and surrounding state preserved. The swap is
  observable to the visitor as a near-instant content change —
  no perceptible delay round-trip while the interface "fetches"
  anything; both snippets are present in the rendered HTML at
  page load and the JS just toggles which one is shown.

  **ARIA semantics**: the tab pattern is wired per the WAI-ARIA
  APG — `role="tablist"` on the trigger container,
  `role="tab"` on each trigger, `role="tabpanel"` on the panel,
  with `aria-selected`, `aria-controls`, and `aria-labelledby`
  set correctly. Cross-referenced from R-9VH6-C2KA.

  **Failure modes named explicitly:**
  - A rendering in which the tab triggers are `<a href=…>`
    elements that navigate (full reload) instead of toggling in
    place does not satisfy this requirement.
  - A rendering in which the triggers are present visually but
    clicking them does not change which snippet is visible —
    "the links don't do anything" — does not satisfy this
    requirement.
  - A rendering that shows both snippets stacked at once with no
    tab interface, or shows only one snippet with no way to reach
    the other, does not satisfy this requirement.
  - A rendering in which switching tabs causes a server round-
    trip does not satisfy this requirement; the swap is a
    client-side toggle of already-rendered content.

  This requirement supersedes the earlier "both stacked, no
  chrome" posture (the prior R-HNUA-WLNX ID) — the design
  reference's tabbed treatment turns out to be the intended look,
  and the previous removal traded an unimplemented tabbed
  interface for a stacked layout that lost the design intent.
  The tabbed layout is restored; the property is that it
  functions as a real tab interface, not as decoration.
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

- R-K3PV-GHB3: the page renders a footer below the instructions
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

- R-9VH6-C2KA: the index page honors visitor preferences for
  reduced motion and presents an accessible structure for the
  interactive controls it exposes.

  **Reduced motion.** When the visitor's browser reports
  `prefers-reduced-motion: reduce`, the page suppresses:
  - The lens-dot pulse animation (R-K7QI-9YW4): the dot is
    rendered without the pulsing outer glow.
  - The subtitle fade-swap (R-K7QI-9YW4): the re-roll updates the
    subtitle without an opacity transition.
  - The counter flash and delta animation (R-NG6O-94I2): the new
    value is shown without the color flash and the delta indicator
    is not animated.
  - Hover-driven transforms (re-roll rotate, copy-button feedback):
    the visual end-states are still rendered, but the transition
    itself is instant.

  **ARIA semantics.**
  - The Claude Code scope tab interface (R-9T1D-KJ2W) uses
    `role="tablist"` on the trigger container, `role="tab"` on
    each trigger, `role="tabpanel"` on the panel, with
    `aria-selected`, `aria-controls`, and `aria-labelledby` wired
    per the WAI-ARIA APG.
  - The counter `+` / `−` buttons (R-NG6O-94I2) carry
    `aria-label="Increment"` and `aria-label="Decrement"`, the
    HTML `disabled` attribute when no web session is active, and
    sit adjacent to an `aria-live="polite"` region around the
    counter value so live-channel updates (R-K65O-80SH) are
    announced to assistive tech.
  - The re-roll button (R-K7QI-9YW4) carries
    `aria-label="New subtitle"`.
  - The decorative lens dot (R-K7QI-9YW4) carries
    `aria-hidden="true"`. The footer status dot
    (R-K3PV-GHB3) likewise.

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
- R-NJUD-EFQ5: the index page reflects web-session state. When the
  visitor has an active web session, the page identifies them by the
  Google email recorded for the session — the email appears verbatim
  on the page — and the page exposes a separate, explicitly labeled
  affordance whose activation reaches `/logout`. When the visitor
  has no active web session, the page exposes an affordance whose
  activation reaches `/login`; the page does not render any
  anonymous-visitor placeholder identity (no literal "guest", no
  fake username).

  **Placement is load-bearing**: the auth area is anchored to the
  **top-right of the page**, in a top-bar row that sits above and
  to the right of the banner card, in both the signed-out and
  signed-in states. A rendering that places the auth area
  elsewhere — top-left, inline with body content, below the
  banner, etc. — does not satisfy this requirement.

  **Signed-out state**: the top-right contains a single pill-style
  button bearing the literal label `Sign in` whose activation
  reaches `/login`.

  **Signed-in state**: the top-right contains two distinct
  elements rendered side by side, identity on the left and the
  sign-out affordance immediately to its right:
  1. The visitor's email rendered as **inert, non-interactive
     text** (optionally preceded by an avatar element). Clicking
     the email does *not* sign the visitor out and does not
     navigate anywhere — it is a label, not a control.
  2. To the right of the email, a **separate, explicitly labeled
     `Sign out` affordance** — rendered as a pill-style button or
     equivalent visibly-actionable control, bearing the literal
     text `Sign out` (or, if a glyph is used, accompanied by an
     `aria-label="Sign out"`) — whose activation reaches `/logout`.

  A rendering in which clicking the email itself triggers logout
  does not satisfy this requirement: the user-visible logout
  affordance must be a distinct element from the identity display,
  with its own actionable label, because "click your name to sign
  out" is not discoverable. A rendering that drops the pill chrome
  on either control and surfaces only a bare text link likewise
  does not satisfy this requirement.

  The property: exactly one of the two states is reflected on every
  page load; in the signed-out state the top-right exposes a
  reachable route to `/login`; in the signed-in state the top-right
  shows the visitor's email as a label *and* exposes a separate,
  explicitly labeled sign-out control that reaches `/logout`; and
  the auth area is in the top-right position in both states.
