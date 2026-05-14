# Web UI

The browser-facing surface of the service. A landing page that
demonstrates the MCP-mutable counter primitive and provides copy-paste
configuration for connecting agent clients. The visual chrome follows a
specific design reference (R-G6NK-RP8H); the requirements below pin the
observable properties and call out where the codebase's behavior
deliberately differs from the reference.

## Visual reference

- R-G6NK-RP8H: the index page is rendered at high visual fidelity to
  the design reference checked into `reqs/design.md`. The reference
  defines the color tokens, typography scale, page layout, card
  chrome, motion timings, and visual treatment of each element. The
  build agent recreates that design in the codebase's existing
  environment, reading the reference for visual specifics. The
  load-bearing design tokens —
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
  merged bank in R-G47S-05R3.
- R-8MP8-6B77: the **canonical stylesheet for the index page is the
  file checked in at `reqs/design.css`**, supplied by the designer.
  The build agent **uses this stylesheet directly** — it embeds and
  serves the file directly (e.g. via the binary's embedded-asset
  surface) at whatever path makes the page serve it — rather than
  re-deriving colors, border radii, transitions, hover rules,
  animation timings, or the `:root` custom-property values from
  prose. The CSS custom properties declared at `:root` in
  `reqs/design.css` (`--bg`, `--bg-elev`, `--ink`, `--ink-soft`,
  `--ink-mute`, `--line`, `--line-soft`, `--accent`, `--accent-glow`,
  `--ok`, `--code-bg`, `--code-ink`, `--code-mute`, `--radius`), the
  keyframes (`pulse`), and every class rule defined there are the
  load-bearing definition of the page's visual language; prose
  elsewhere in this spec that names a hex value, a transition timing,
  an animation duration, or a hover treatment is consistent with the
  canonical CSS, and where any prose-vs-CSS conflict arises **the
  canonical CSS wins**.

  Adapting selectors where the rendered DOM differs from the
  canonical reference is permitted — e.g., the build agent may
  wrap the page in framework-specific containers, add ARIA wrappers,
  or rename a single class to match an application idiom — but the
  resulting computed styles for every element the canonical CSS
  styles must match the canonical CSS's output for that element.
  The build agent does **not** rewrite the canonical CSS into a
  preprocessor format (SCSS, PostCSS-with-custom-syntax, CSS-in-JS,
  Tailwind utilities, etc.) that would obscure the side-by-side
  comparison with `reqs/design.css`; the file is plain CSS and the
  application's stylesheet remains plain CSS for the same rules.

  Fonts (`Inter`, `JetBrains Mono`) are referenced by the canonical
  stylesheet but not bundled in `reqs/design.css`; the build agent
  loads them in the codebase's existing fashion (Google Fonts
  `<link>`, embedded font files, or whatever the application
  already uses for web fonts).

  A rendering whose computed styles for `.banner`, `.counter-card`,
  `.counter-value`, `.counter-value.flash`, `.delta`, `.delta.show`,
  `.delta.minus`, `.icon-btn`, `.icon-btn:hover:not(:disabled)`,
  `.icon-btn:disabled`, `.auth-btn`, `.auth-btn:hover`, `.refresh`,
  `.refresh:hover`, `.refresh:active`, `.section`, `.section-head`,
  `.client-tabs`, `.client-tab`, `.client-tab.active`,
  `.client-panel`, `.client-panel.active`, `.scope-block + .scope-block`,
  `.code`, `.copy`, `.copy:hover`, `.copy.copied`, `footer`, or the
  `pulse` keyframes diverge materially from the canonical CSS does
  not satisfy this requirement. (The matching is per-rule, not
  per-character: an extra rule the canonical CSS does not contain
  is permitted as long as it does not change the computed styles
  of the elements the canonical CSS already styles.)
- R-MCHV-YEO4: the index page's rendered HTML **uses the class
  names and DOM hooks `reqs/design.css` targets**, so that
  importing the canonical stylesheet (per R-8MP8-6B77) styles the
  page directly with no parallel CSS authoring. The build agent
  does **not** introduce app-specific class names that shadow or
  parallel the canonical ones; where the canonical CSS expects a
  class, the rendered HTML uses **that** class.

  Concrete mappings the build agent uses verbatim (canonical
  class on the left; if the build agent's current implementation
  uses a different name in parentheses, the build agent updates
  the implementation to the canonical name):

  - Banner card: `<section class="banner">` containing
    `.lens`, `.tag`, `.title`, `.subtitle-row` (which contains
    `.subtitle` and `.refresh`), and `.banner-auth` (which in
    the signed-in state contains a `.who` wrapper with the
    visitor's email and the `.auth-btn` for sign-out; in the
    signed-out state contains just the `.auth-btn` labeled
    `Sign in`). The auth button's class is `.auth-btn` (not
    `.auth-pill` or any project-specific rename).
  - Counter card: `<section class="counter-card">` containing
    `.counter-label`, `.counter-value`, and `.counter-actions`
    holding two `.icon-btn` buttons (**not** `.counter-button`).
    The counter value's flash class is `.flash` applied to the
    `.counter-value` element (**not** `.counter-flash`); the
    `transition: color 0.2s ease` lives on `.counter-value`
    itself so the color transitions in *and* out of the accent
    when `.flash` is toggled. `.counter-value` contains two
    inline children: the digits themselves (in their own span
    or text node) and the `.delta` element. Both are **child
    elements of `.counter-value`**, not siblings of it, so the
    canonical `display: flex; align-items: baseline; gap: 10px`
    on `.counter-value` lays them out side by side on the same
    baseline. The delta is styled by `.delta` (**not**
    `.counter-delta`); making it visible means adding the
    `.show` class (**not** running a one-shot CSS animation),
    so the JS toggles `.flash` on `.counter-value` and
    `.delta.show` on the child `.delta` (with `.delta.minus`
    for a negative delta) and the canonical transitions handle
    the rest. A `.delta` element that lands outside
    `.counter-value` — e.g. appended to `.counter-value`'s
    parent — renders below the digit in block flow and does
    not satisfy this requirement.
  - MCP client instructions area: each client renders inside
    `<article class="section">` with a `.section-head`
    containing the `.num` chip and the title, and a
    `.section-body`. The two clients sit inside `.client-tabs`
    with `.client-tab` triggers and `.client-panel` panels
    (with `.active` toggled to switch which client is visible
    per R-H4LJ-G9HR). The Claude Code panel's two scope
    examples sit in stacked `.scope-block` sub-sections per
    R-G5FO-DXHS (the `.scope-block + .scope-block` selector in
    the canonical CSS provides the separator border).
  - Code blocks: each dark snippet is `<div class="code">` (or
    `<pre class="code">`) so the canonical positioning of the
    `.copy` overlay button anchors correctly inside it. The
    copy button is `<button class="copy">`. On a successful
    copy the build agent adds the `.copied` class for ~1.4s,
    relying on the canonical `.copy.copied` rule for the
    mint-green color flip rather than computing the color in
    JS.
  - Subtitle re-roll: the swap animation uses the canonical
    `.subtitle.swap` class (opacity 0, translateY -3px) toggled
    around the text update, rather than driving opacity via
    inline styles.
  - Footer: `<footer>` with the canonical `.status` element on
    the left (the canonical CSS draws the green dot via
    `footer .status::before`) and the version + flavor on the
    right; the build agent does **not** redo the green-dot
    chrome inline.

  The property: a developer running `diff` between the build
  agent's rendered HTML (class names and structure) and the
  canonical class names in `reqs/design.css` finds **no
  shadowed classes** — no `.counter-button` next to `.icon-btn`,
  no `.counter-flash` next to `.flash`, no `.counter-delta`
  next to `.delta`, no `.auth-pill` next to `.auth-btn`, no
  `.counter-form` wrapping that breaks `.icon-btn:hover` chrome.
  The build agent **renames or removes** any pre-existing
  parallel class in the codebase to bring the rendered HTML in
  line with the canonical CSS; the canonical class names are
  the canonical class names.

  Allowed: adding extra utility classes for behavior the
  canonical CSS does not style (e.g. additional classes the
  implementation needs for its own JS wiring, an `aria-*`
  attribute, an `id` for a JS handle). These additions must
  not displace or rename a canonical class on the same
  element. Adding extra DOM wrappers is allowed
  *only* when they do not break a canonical selector
  (`.counter-card > .counter-row` is fine if `.counter-card`
  still resolves to the styled element; wrapping every
  `.icon-btn` in a `<form>` is fine because `.icon-btn` still
  matches the button itself).

  A rendering that styles the page via a parallel project-
  specific stylesheet that re-declares colors / transitions /
  hover rules for newly-named classes — rather than letting
  `reqs/design.css` style the canonical classes directly —
  does not satisfy this requirement; it also does not satisfy
  R-8MP8-6B77.

- R-9TPL-HQBV: every named block in the index page's layout
  (`reqs/design.md` §1) — banner, counter card, instructions
  head, client tabs, and footer — is a **child of the `.page`
  wrapper**, rendered in that order. The counter hint is
  **not** a `.page` child: per R-EJAP-XUSB it renders inside
  the counter card, not as a sibling beneath it. This is the
  property `reqs/design.css` relies on: `.page` carries
  `max-width: 880px` and `margin: 0 auto`, so any block placed
  outside `.page` spans the full viewport width instead of
  matching the rest of the page. The footer in particular is the
  last child of `.page`, not a sibling of it; a rendering that
  closes `.page` before opening `<footer>` (so the footer ends
  up between `</main>` / `</div class="page">` and `</body>`)
  does not satisfy this requirement — the visual signature is a
  footer whose horizontal extent is wider than the banner above
  it. The build agent does not introduce a second wrapper
  outside `.page` for any of these blocks.

  **Reserved class names.** A small set of class names in
  `reqs/design.css` defines page-scope typography whose
  computed styles would clobber a component if reused inside
  one. The rendered HTML treats those names as reserved —
  applied only to the page-level element the canonical CSS
  targets, never inside a component. The reserved set today
  is `.title` (the 88px page heading; canonical target is the
  `<h1>`-equivalent inside `<section class="banner">`) and
  `.subtitle` (the rotating tagline below it). A `<span
  class="title">` or `<div class="subtitle">` appearing inside
  `.counter-card`, `.client-tabs`, `.client-tab`, `.client-panel`,
  `.section`, `.code`, or any future component renders the page
  incorrectly and is a violation of this requirement. When a
  component needs a label, the build agent uses the component's
  own typography (the component's class rule sets the font) and
  emits the label as a bare text node or, when an element is
  unavoidable, under a component-scoped class that does not
  collide with any rule in `reqs/design.css`. The design
  reference (`reqs/design.md`, §2 "Reserved class names")
  enumerates the reserved set; new additions to that list are
  spec edits, not implementation choices.

- R-NBGD-KUHA: the page's three top-level content sections —
  (1) the **banner card**, (2) the **counter card**, and (3)
  the **MCP client instructions area** (the "Connect an MCP
  client" heading together with the `.client-tabs` panel
  beneath it per R-H4LJ-G9HR) — are separated from one another
  by the **same vertical gap**. The visible space between the
  bottom edge of the banner card and the top of the counter
  card is visually indistinguishable from the visible space
  between the bottom edge of the counter card and the top of
  the **"Connect an MCP client" heading**: a casual visitor
  scanning the page top-to-bottom does not perceive one gap
  as larger or smaller than the other.

  Specific gap magnitude is HOW — exact pixel size, whether
  it derives from a CSS custom property, a Flexbox `gap`, a
  `row-gap` on `.page`, top/bottom margins on the children,
  or any other mechanism — and is governed by
  `reqs/design.css`. The property pinned here is uniformity,
  not a specific magnitude. A rendering in which the gap
  between the counter card and the MCP client instructions
  area is perceptibly smaller or larger than the gap between
  the banner card and the counter card does not satisfy this
  requirement.

  **The MCP client instructions area renders as one visual
  section, not two.** R-9TPL-HQBV makes the instructions head
  (the `<h2>` reading "Connect an MCP client" per
  R-H4LJ-G9HR) and the `.client-tabs` panel separate children
  of `.page`, but the rendered output must read as a **single
  cohesive section**, not as two independently-chromed cards
  with negative space between them.

  Concretely:

  1. **The instructions head (`<h2>`) is NOT wrapped in its
     own card chrome.** The `<h2>` carries no border, no
     rounded-rectangle background, no card-style fill, no
     box-shadow, and no surrounding chrome that would make it
     read as a card distinct from the panel below. The
     heading is a heading, not a card. A rendering in which
     the `<h2>` sits inside a bordered, rounded,
     off-white-filled rectangle visually parallel to the
     counter card's chrome does **not** satisfy this
     requirement — that is precisely the failure mode this
     clause forbids.
  2. **The visible vertical gap between the `<h2>` and the
     top of the `.client-tabs` panel beneath it is small
     enough that the two elements read as one cohesive
     section.** Treat the inter-element gap as *internal
     spacing of the MCP section*, not as another inter-
     section gap. A rendering in which the gap between the
     `<h2>` and the tabs panel is perceptibly similar in
     magnitude to the gap between the counter card and the
     `<h2>` does not satisfy this requirement.
  3. **The inter-section gap is above the heading, not below
     it.** The full inter-section gap defined in the first
     paragraph above is measured from the bottom of the
     counter card to the top of the `<h2>`. The space
     between the `<h2>` and the tabs panel is *internal* to
     the MCP section.

  Acceptance, stated as a visitor scan: looking at the page
  top-to-bottom a visitor sees three coherent sections
  (banner, counter, MCP) floating in equal negative space.
  The visitor does **not** see four sections (banner,
  counter, lone-heading, tabs) bunched as `gap, card, gap,
  card, gap, card, gap, card` — that arrangement has the gap
  in the wrong place between the heading and the tabs.

  The footer (R-K3PV-GHB3) and the agents block (R-2IRD-A7HD)
  are out of scope for this requirement. The footer has its
  own chrome (top border, muted ink) per R-K3PV-GHB3 and is
  visually separated by that border rather than by the
  inter-section gap. The agents block lives inside the banner
  card per R-2IRD-A7HD and shifts the banner card's bottom
  edge downward when present; the inter-section gap is still
  measured from the (new, lower) bottom edge of the banner
  card to the top of the counter card, so the equal-spacing
  property holds in both the with-agents and without-agents
  states.

- R-UBYN-1LY0: each `.client-tab` button contains exactly two
  visible elements: the `.num` chip ("01" / "02") and the
  client's name as a bare text node — `Claude Code`,
  `Claude Desktop`, and any future client added by the
  procedure `reqs/design.md` §4.3 names. The label text is not
  wrapped in any inner element with a class of its own; the
  button's own `.client-tab` typography (set in
  `reqs/design.css`) governs the label. No subtitle, no hint,
  no secondary line lives inside the tab trigger. Content
  describing what the panel will show ("Run the following
  command.", "Add the following JSON to your
  claude_desktop_config.json", etc.) lives inside the matching
  `.client-panel` body, never inside the `.client-tab`. This
  extends R-MCHV-YEO4 for the specific case of the client tabs
  and pins the inner markup the canonical CSS in §4 of
  `reqs/design.md` expects.

## Baseline

- R-QY5R-PYDH: visiting the site's root URL renders the current count
  as a number, in plain server-rendered HTML. No authentication is
  required to view it.
## Banner card

- R-8031-9QQ9: the page presents the project name as a banner card
  near the top with the chrome the design reference pins. The card
  contains, in fixed positions:
  - **Lens dot** (top-left, absolutely positioned, ~14×14): a radial-
    gradient red disk evoking HAL 9000's eye, surrounded by a soft
    outer glow that pulses on a 2.6-second ease-in-out infinite
    loop. This is the only piece of non-hover motion on the page
    and is load-bearing; it is decorative (`aria-hidden="true"`)
    and degrades to a static dot under `prefers-reduced-motion`
    per R-G0K2-UUJ0.
  - **Tag** (top-right, absolutely positioned): mono 11px uppercase
    muted ink with content `MCP Demo`.
  - **Title**: the literal text `HAL 9000`, rendered in the
    display weight at 88px on viewports ≥640px and 64px on
    viewports <640px.
  - **Subtitle row** directly beneath the title: the randomly-
    selected entry from the bank R-G47S-05R3 carries, rendered in
    italic 18px in soft-ink color, followed inline by the re-roll
    control.
  - **Re-roll control**: a small circular button (~28×28, 1px
    border, transparent fill) with a refresh icon glyph,
    rendered as a `<button>` element (or another non-navigating
    control) — **not as an `<a>` whose `href` would navigate the
    browser away from the current page**. Activating it produces
    a freshly-picked subtitle per R-G47S-05R3, swapped into the
    existing subtitle element in place with a ~220ms cross-fade.

    The activation MUST be observable as a perceivable change to
    the visible subtitle text without a page reload: the URL bar
    does not change, the document does not navigate, no `GET /`
    is issued to the server, and the surrounding page state
    (scroll position, focus, open SSE connection, etc.) is
    preserved across the activation. A new subtitle MAY happen to
    match the prior one because the bank R-G47S-05R3 is sampled
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

    **Hover treatment**: on pointer hover, the re-roll button
    **rotates** to a visibly-different angular position and its
    glyph and border color shift to the design reference's dark
    ink (`--ink` ≈ `#14130f`, effectively black) — so the visitor
    sees both a rotation and a color darkening as a single hover
    response. The canonical reference pins this as
    `transform: rotate(90deg)` plus `color: var(--ink)` and
    `border-color: var(--ink)` on `.refresh:hover`; the property
    re-pinned here is **noticeable rotation** (large enough that
    a casual visitor perceives a deliberate angular change, not a
    subtle nudge) combined with the color/border darkening to the
    near-black ink color. Exact angle is HOW (the original 90deg
    is a reasonable default; smaller angles in the 45°–90° range
    are also acceptable as long as the rotation reads as
    deliberate). Under prefers-reduced-motion the rotation
    transition collapses to its end state without animation per
    the reduced-motion req; the steady-state hovered appearance
    (rotated and darkened) is still rendered. A rendering in
    which the re-roll button is visually inert under hover — no
    rotation and no color change — does not satisfy this
    requirement.
  - **Auth area**: the banner card also contains the page's
    Sign in / Sign out affordance, rendered as the bottom row of
    the banner card in the absence of an agents block, or as the
    row immediately above the agents block when one is present
    (R-2IRD-A7HD). Its structure, state-dependent content, and
    load-bearing placement properties are pinned by R-0WB7-RV1W.

    The auth area is visually separated from the subtitle row
    above it by **perceptible vertical space** — the subtitle row
    and the auth area read as two distinct horizontal bands of
    the banner with deliberate negative space between them, not as
    elements crowded against each other on adjacent baselines.
    Exact spacing is HOW (banner bottom padding, the auth area's
    offset from the bottom edge, etc. are all the build agent's
    choice), but a rendering in which the auth pill abuts the
    subtitle's baseline — with no visible breathing room between
    them — does not satisfy this requirement.
- R-1ZS0-XSZ7: the rendered HTML document's `<title>` element
  carries the literal text `HAL` — the short form, not the
  `HAL 9000` form R-8031-9QQ9 pins for the on-page banner title.
  Browser-tab strips, bookmark labels, and window-switcher
  entries are space-constrained surfaces where the shorter name
  reads better; the on-page display title is the canonical
  full name. A `<title>` whose content is `HAL 9000`,
  `HAL · MCP Demo`, the empty string, or any other variant
  does not satisfy this requirement.

- R-G47S-05R3: the subtitle is one entry chosen uniformly at random,
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
  reachable. Re-roll (R-8031-9QQ9) yields a freshly-picked entry
  on each activation.

## Counter card

- R-EJAP-XUSB: directly below the banner the page renders a counter
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

    **Hover treatment** (when not disabled): on pointer hover each
    button **inverts** — the button fills with the design
    reference's dark ink (`--ink` ≈ `#14130f`, effectively
    black), and the glyph and border render in the page
    background color (`--bg` ≈ `#f6f5f1`, the warm off-white the
    page sits on). The resulting hover appearance is a near-black
    filled square with a light glyph. **This hover treatment is
    identical to the auth `Sign in` / `Sign out` button's hover
    treatment pinned in R-0WB7-RV1W** — the same dark-fill
    inversion is shared by both the auth pill and the counter
    `+/−` buttons; a visitor who hovers either control sees the
    same visual response. A rendering in which the `+` and `−`
    buttons remain visually inert under pointer hover when
    enabled — no fill change, no border change, no glyph color
    change — does not satisfy this requirement.

    **The disabled state does not respond to hover.** When the
    buttons are rendered visibly disabled (because no web session
    is active, per the next paragraph), pointer hover produces
    no visible change at all: the disabled-state appearance is
    the only visible state. A rendering in which a disabled
    `+` / `−` button inverts under hover does not satisfy this
    requirement.
  - **Hint line** rendered **inside the counter card**,
    positioned below the counter value and **left-aligned**
    within the card's content area — not beneath the card as a
    separate sibling element, and not right-aligned or centered.
    The hint is in 12px muted ink (same weight and style as the
    rest of the page's muted hint copy) and is rendered
    identically regardless of whether the visitor has an active
    web session: *"Authenticated agents using MCP can read &
    mutate this counter on your behalf."* The hint text does not
    change with the visitor's auth state — an MCP agent's
    ability to read or mutate the counter is governed by the MCP
    auth boundary, which is orthogonal to whether the visitor is
    signed in in their browser. A rendering that places the hint
    outside the counter card (as a sibling beneath it), that
    swaps the hint text between signed-in and signed-out states,
    or that omits the hint entirely in either state, does not
    satisfy this requirement.

  When the visitor has no active web session, the `+` and `−`
  buttons are rendered visibly disabled (~40% opacity,
  `cursor: not-allowed`, the HTML `disabled` attribute on the
  underlying control) and a click does nothing. The buttons are
  present on the page in this state — they are not hidden — so
  the visitor sees what becomes available after sign-in.

  The behavior of the `+` and `−` buttons when the visitor *does*
  have an active web session — what they POST and how the page
  reflects the change — is pinned by R-FY4A-3B1M.
- R-FY4A-3B1M: when a visitor with an active web session
  (R-0WB7-RV1W) activates the index page's `+` or `−` counter
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
  arrives via the live-update channel R-FZC6-H2SB, accompanied by
  the visual transition pinned below. Modulo reduced-motion
  suppression per R-G0K2-UUJ0, every observed change to the
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
  by R-EJAP-XUSB.)
- R-FZC6-H2SB: while a visitor's browser has the index page open
  and JavaScript is running, any change to the counter — regardless
  of which caller produced it (web `+`/`−` button, MCP increment or
  decrement tool, HTTP API endpoint, or any future operation) — is
  reflected on the page without the visitor having to reload. The
  acceptance criteria are:

  - The live-update channel uses Server-Sent Events (SSE) as its
    transport. This is a deliberate decision; the spec does not leave
    the transport open to polling, WebSockets, or any other choice.
  - A change to the counter is reflected on every connected browser
    within 1000 milliseconds (strictly less than one second). A
    measurement of 999 ms passes; a measurement of 1000 ms fails.
  - On every new connection, the server's first event on the stream
    is a snapshot of the current counter value, so a browser that
    just connected (including one that auto-reconnected after a
    network blip) displays the server's current value without
    waiting for the next mutation.
  - The channel requires no authentication; a signed-out visitor
    sees the same live updates a signed-in visitor sees.
  - After a transient network disruption, within 5 seconds of the
    network's return, the live channel is re-established and the
    displayed value reflects the server's current counter. The
    browser-side client auto-reconnects (this is the EventSource
    default), and the snapshot-on-connect property above carries
    the current value on every new connection. A page left
    displaying a stale value after a network blip is a defect.

  The chosen transport must remain compatible with the shutdown
  deadline R-K7DK-LSJ6 pins — a transport whose handler blocks
  process exit on an in-flight request is disqualified. The channel
  carries only the counter value (already public per R-SE5T-HP2J /
  R-3R73-2TN9 / R-0CQ7-DSBQ) and conveys no per-user, auth-protected,
  or session-specific data. On update receipt, the page updates the
  rendered count in place and runs the visual transition R-EJAP-XUSB
  describes, modulo reduced-motion suppression per R-G0K2-UUJ0.
- R-T4FH-IAQQ: the service remains responsive to unrelated requests
  indefinitely while live-update connections (R-FZC6-H2SB) are open.
  Concretely: with one or more browsers holding open live-update
  connections to this service — including connections that are
  currently idle because the counter is not changing — the service
  continues to accept and complete other HTTP requests (a fresh
  `GET /`, `GET /login`, `POST /counter/increment`, etc.) at
  ordinary latency. A transport whose per-connection handler ties
  up a finite concurrent-request-capacity resource (handler, worker,
  file descriptor, …) for the lifetime of the connection in a way
  that causes the service to refuse, queue indefinitely, or
  otherwise fail to promptly serve unrelated requests once many
  live-update connections are open does not satisfy this
  requirement.

  Concrete acceptance: opening N+1 long-lived live-update
  connections (where N is the service's configured concurrent
  request capacity — or simply "many" if that capacity is not a
  fixed number) on the same origin and abandoning them must not
  prevent an unrelated `GET /login` from completing at ordinary
  latency within a short window. This was a real bug in the prior
  implementation; the new implementation must not regress to it. A
  visitor who opens the index page in a few tabs (or who reloads it
  a few times in one tab) must still be able to load `/login` and
  navigate the site without restarting the server.
- R-T5ND-W2HF: a live-update connection (R-FZC6-H2SB) whose browser
  has gone away — tab closed, page navigated away, network dropped,
  process killed — must be detected and released by the service
  within **5 seconds** of the client's departure. Specifically, when
  a client vanishes without the TCP-level connection-close machinery
  firing (no FIN, no RST — e.g. network drop, machine kill, cable
  yanked), the server detects the disappearance and releases
  per-connection resources (handler, broadcast subscription, file
  descriptor) within 5 seconds. Cleanup on a clean disconnect
  (FIN/RST) is implied by R-T4FH-IAQQ and does not need a separate
  budget here. A budget measured in tens of seconds or longer does
  not satisfy this requirement; the service must not require the
  client's eventual OS-level TCP timeout to reclaim the slot.

  Concrete acceptance scenario (the property is observable, not
  internal): with the service running at its default concurrent-
  request-capacity configuration, a client opens N+1 live-update
  connections in succession on the same origin and abandons each
  one without an explicit unsubscribe — where N is the number of
  concurrent request handlers the service is configured for. Within
  5 seconds of the last abandonment, a fresh unrelated request
  (e.g. `GET /login`) issued against the service completes at
  ordinary latency. A configuration in which `GET /login` instead
  waits for one of the abandoned connections to time out — observed
  in practice as a request that completes only after 30+ seconds,
  or not at all until an abandoned connection is released — does
  not satisfy this requirement.

  This requirement names the property, not the mechanism. Whether
  the handler keeps an abandoned connection alive long enough to
  detect it via the next write, watches the underlying socket for
  readability/closure, swaps to a different transport whose
  framework already manages cleanup, or anything else is HOW; what
  matters is the observable budget above.

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
- R-H4LJ-G9HR: the MCP client instructions area sits below the
  counter card and is structured as a **functional two-tab
  interface** — Claude Code (`01`) and Claude Desktop (`02`) — with
  exactly one client's instructions visible at a time. Both clients'
  instructions are present in the rendered HTML on initial page
  load; activating a trigger toggles which panel is visible without
  a page reload.

  **Area header** (above the tab interface): an `h2` reading
  "Connect an MCP client". No secondary subhead, kicker, or
  right-aligned "Point a client at …" line accompanies the
  heading. The base URL the visitor needs appears inside the code
  blocks within each client's panel (derived from the request per
  R-CO4Y-11X7 / R-DA34-WX9P); the heading does not duplicate it.
  A rendering that re-introduces the right-aligned base-URL subhead
  does not satisfy this requirement.

  **Tab triggers**: a row of two side-by-side controls beneath the
  area header, sharing a bottom border in the design reference's
  `--line-soft` color. Each trigger is rendered as a `<button>`
  element (or another non-navigating control — never an `<a>` whose
  `href` would navigate the browser). Trigger 1 represents Claude
  Code; trigger 2 represents Claude Desktop. Each trigger contains,
  all three present together:
  1. A **numeric badge** — small mono `01` / `02` inside a rounded
     chip.
  2. The **literal client title** `Claude Code` or `Claude Desktop`
     rendered in 15px display weight.
  3. A **right-aligned 13px muted-ink instruction** directing the
     visitor what to do with the snippet inside the active panel.
     Per-client wording:
     - Claude Code: `Run the following command.`
     - Claude Desktop: `Add the following JSON to your claude_desktop_config.json`

     Whether this instruction sits inline within the trigger
     itself or as the first element of the panel body is HOW;
     the property is that the visitor sees, alongside the active
     client's snippet, a short imperative sentence telling them
     what to do with it — not a metadata-style client-kind label.
     A rendering that displays a client-kind description
     (`CLI · adds a server entry to a scope` for Claude Code,
     `JSON · add to your claude_desktop_config.json` for Claude
     Desktop) does not satisfy this requirement; the element is
     an instruction, not a description.

  A rendering that omits the client name from a trigger — showing
  only the numeric badge — does not satisfy this requirement; the
  visitor must be able to tell which trigger reveals Claude Code
  and which reveals Claude Desktop without first activating either.

  The active trigger is visually distinguished from the inactive
  trigger using the design reference's tab chrome (typically: the
  inactive trigger has a transparent bottom border and the active
  trigger has an accent-red bottom border `#d4361e`, with the
  inactive label in muted ink and the active label at full ink
  strength). The exact treatment is HOW; the property is that the
  visitor can identify the active trigger at a glance.

  **Default active tab on first render**: Claude Code (`01`).

  **Panels**: two panel elements, one per trigger, both present in
  the rendered HTML at page load. The Claude Code panel and the
  Claude Desktop panel are mutually exclusive — exactly one is
  visible at a time; the inactive panel is hidden (not merely
  pushed out of the viewport in a way that still reserves space).
  The active panel sits directly beneath the tab-trigger row and
  inherits the design reference's panel chrome — a visibly
  bordered, rounded surface with its own off-white fill, clearly
  separated from the area header above.

  **Behavior on activation**: clicking the inactive trigger changes
  the visible state of the tab interface in place. The previously-
  active trigger goes muted and loses its active treatment; the
  just-clicked trigger receives the active treatment; the visible
  panel swaps to the new trigger's corresponding panel. No page
  reload, no URL change, no `GET /` issued to the server; scroll
  position, focus state, and the open SSE connection
  (R-FZC6-H2SB) are preserved across the swap. The swap is
  observable as a near-instant content change — both panels are
  present in the rendered HTML at page load and the JS just
  toggles which one is shown.

  **ARIA semantics**: the tab pattern is wired per the WAI-ARIA
  APG. `role="tablist"` on the trigger container, `role="tab"` on
  each trigger, `role="tabpanel"` on each panel, with
  `aria-selected`, `aria-controls`, and `aria-labelledby` set
  correctly. Cross-referenced from R-G0K2-UUJ0.

  **Body of each panel** contains one or more dark-background code
  blocks rendered in JetBrains Mono on the design reference's
  `--code-bg` fill (`#14130f`). **Every code block in this area
  renders a visible `copy` button** — an overlay positioned inside
  the block, labeled `copy` with a small clipboard glyph — that
  copies the block's plaintext to the visitor's clipboard via the
  standard `navigator.clipboard.writeText` API (with the textarea
  + `execCommand` fallback). On successful copy, the affordance
  flips to a `copied` label in the design reference's mint-green
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

  **Token coloring** in code blocks (CLI flags in warm tan
  `#e0a96d`, command name in cool blue `#a8c8ff`, URLs and
  strings in mint green `#79d4a9`, prompts and punctuation in
  muted gray `#8c8a82`) follows the design reference. The exact
  palette is HOW; the property is that code is highlighted with
  semantic token coloring rather than rendered as undifferentiated
  monospace.

  **Panel contents**:
  - The **Claude Code panel** lays out its two scope examples as
    stacked scope blocks per R-G5FO-DXHS. The two scope blocks are
    stacked **inside this panel**; introducing a second row of tabs
    nested inside the Claude Code panel does not satisfy
    R-G5FO-DXHS.
  - The **Claude Desktop panel** contains a single code block with
    the JSON snippet that names this service in
    `claude_desktop_config.json`, with the URL derived from the
    request per R-CO4Y-11X7.

  **Failure modes named explicitly:**
  - A rendering in which the two clients are presented as **two
    stacked section cards both visible simultaneously** — rather
    than as two tab triggers with one panel visible at a time —
    does not satisfy this requirement.
  - A rendering in which the tab triggers are `<a href=…>`
    elements that navigate (full reload) instead of toggling in
    place does not satisfy this requirement.
  - A rendering in which the triggers are present visually but
    clicking them does not change which panel is visible — "the
    tabs don't do anything" — does not satisfy this requirement.
  - A rendering in which switching tabs causes a server round-
    trip does not satisfy this requirement; the swap is a
    client-side toggle of already-rendered content.
- R-UBPK-DLTT: every dark code-block snippet inside an MCP client
  panel is rendered as a single element carrying the canonical
  `code` class — `<div class="code">` or `<pre class="code">` —
  so the canonical `.code` rule in `reqs/design.css` (dark
  `--code-bg` fill, `--code-ink` foreground, 8px radius,
  JetBrains Mono 13px/1.55, `position: relative` for the `.copy`
  overlay) applies directly. A rendering that wraps the code in
  a `<div class="code-wrap">`, `<div class="code-block">`,
  `<div class="snippet">`, or any other class-name parallel to
  `.code` — even when an inline `style="position:relative"` is
  added to make `.copy`'s absolute positioning work — does not
  satisfy this requirement: the dark background, monospace
  type, and code-token coloring rules R-H4LJ-G9HR pins arrive
  through `.code`, not through invented shadow classes. The
  copy button inside each block is `<button class="copy">` and
  contains an `<svg>` element — the small clipboard glyph
  R-H4LJ-G9HR names — as the canonical `.copy svg` rule
  (`width: 12px; height: 12px`) in `reqs/design.css`
  expects. A `.copy` button whose body is the literal text
  `copy` with no `<svg>` child does not satisfy this
  requirement; the glyph is part of the affordance, not optional
  chrome. The button still carries the accessible label
  R-H4LJ-G9HR / R-G0K2-UUJ0 pin (an `aria-label` of "Copy to
  clipboard" or equivalent visible text); the glyph is in
  addition to that label, not a replacement for it.

- R-772N-VHQE: on first page load — before any visitor interaction
  with the tab triggers — the Claude Code panel is the visible one
  and the Claude Desktop panel is hidden. Concretely: the Claude
  Code trigger and the Claude Code panel both carry the
  canonical `.active` class on first render (the canonical CSS in
  `reqs/design.css` resolves `.client-panel { display: none }` to
  visible only when `.active` is also present); the Claude Desktop
  trigger and panel do not carry `.active`. A rendering in which
  the Claude Code trigger is marked active but its panel is not —
  so the trigger highlights as selected while neither panel is
  visible — does not satisfy this requirement. A rendering in
  which Claude Desktop is the initially-visible panel also does
  not satisfy this requirement. This pins the
  "**Default active tab on first render**: Claude Code (`01`)"
  property R-H4LJ-G9HR states, in terms of the canonical
  `.active`-class mechanism R-MCHV-YEO4 names.

- R-G5FO-DXHS: the Claude Code section card renders its two scope
  examples as **two stacked scope blocks** inside the section card's
  single body — not as a sub-tab interface. Both blocks are visible
  simultaneously on page load; there is no per-scope toggle, no
  client-side state that hides one snippet behind the other, and no
  navigation needed to reach either snippet.

  **Structure**: two sub-sections rendered in fixed order top-to-
  bottom inside the Claude Code section card's body, each consisting
  of a small mono pill label followed by a dark-background code
  block:
  1. **Project scope** (first): pill bearing the literal text
     `project`, followed by a code block containing
     `claude mcp add --transport http --scope project hal <base-url>/mcp`.
  2. **User scope** (second): pill bearing the literal text `user`,
     followed by a code block containing
     `claude mcp add --transport http --scope user hal <base-url>/mcp`.

  `<base-url>` is the request-derived value per R-CO4Y-11X7 /
  R-DA34-WX9P.

  The two scope blocks are visually distinguished from one another:
  the second block is separated from the first by a thin top border
  in the design reference's `--line-soft` color (the design reference
  uses `.scope-block + .scope-block { border-top: 1px solid var(--line-soft); }`).
  The property is that the reader sees two clearly delineated scope
  examples one above the other — labelled, separated, and equally
  reachable — not a single undifferentiated wall of monospace text.

  **Copy affordances**: each scope block's code block exposes its
  own `copy` affordance per R-H4LJ-G9HR. The project block's button
  writes the project-scope command to the clipboard; the user
  block's button writes the user-scope command. What lands on the
  clipboard is the executable form (no leading `$ ` prompt prefix),
  per the clipboard property in R-H4LJ-G9HR.

  **Failure modes named explicitly:**
  - A rendering that shows only one scope's snippet (project OR
    user, but not both) on initial page load does not satisfy this
    requirement.
  - A rendering in which the two scopes are placed behind a sub-tab
    interface — `Project scope` / `User scope` triggers with a
    single panel that swaps between them — does not satisfy this
    requirement. The design reference explicitly forbids nesting a
    second row of tabs inside the outer Claude Code / Claude Desktop
    client-tabs panel; inside-panel variations are stacked scope
    blocks, not sub-tabs.
  - A rendering that fuses the two scope commands into a single
    code block (separated by a blank line, a comment, or any other
    in-block divider) does not satisfy this requirement; each scope
    has its own labelled block with its own copy button.
  - A rendering that omits the `project` / `user` pill labels —
    leaving the reader to guess which scope is which — does not
    satisfy this requirement.
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
    how it's sourced — a constant, a build-time or runtime-sourced
    version string, etc.); the "open my pod bay doors" allusion is
    deliberate flavor and is preserved verbatim.

## Motion + accessibility

- R-G0K2-UUJ0: the index page honors visitor preferences for
  reduced motion and presents an accessible structure for the
  interactive controls it exposes.

  **Reduced motion.** When the visitor's browser reports
  `prefers-reduced-motion: reduce`, the page suppresses:
  - The lens-dot pulse animation (R-8031-9QQ9): the dot is
    rendered without the pulsing outer glow.
  - The subtitle fade-swap (R-8031-9QQ9): the re-roll updates the
    subtitle without an opacity transition.
  - The counter flash and delta animation (R-EJAP-XUSB): the new
    value is shown without the color flash and the delta indicator
    is not animated.
  - Hover-driven transforms (re-roll rotate, copy-button feedback):
    the visual end-states are still rendered, but the transition
    itself is instant.

  **ARIA semantics.**
  - The outer client-tabs interface (Claude Code / Claude Desktop,
    per R-H4LJ-G9HR) is wired per the WAI-ARIA APG tab pattern —
    `role="tablist"` on the trigger container, `role="tab"` on
    each trigger, `role="tabpanel"` on each client's panel, with
    `aria-selected`, `aria-controls`, and `aria-labelledby` set
    correctly. (The Claude Code panel itself contains stacked
    scope blocks per R-G5FO-DXHS, not a nested sub-tab interface,
    so there is no inner tablist.)
  - The counter `+` / `−` buttons (R-EJAP-XUSB) carry
    `aria-label="Increment"` and `aria-label="Decrement"`, the
    HTML `disabled` attribute when no web session is active, and
    sit adjacent to an `aria-live="polite"` region around the
    counter value so live-channel updates (R-FZC6-H2SB) are
    announced to assistive tech.
  - The re-roll button (R-8031-9QQ9) carries
    `aria-label="New subtitle"`.
  - The decorative lens dot (R-8031-9QQ9) carries
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
  `/logout` does not touch any MCP token chain (R-0XJ4-5MSL).
- R-0WB7-RV1W: the index page reflects web-session state. When the
  visitor has an active web session, the page identifies them by the
  Google email recorded for the session — the email appears verbatim
  on the page — and the page exposes a separate, explicitly labeled
  affordance whose activation reaches `/logout`. When the visitor
  has no active web session, the page exposes an affordance whose
  activation reaches `/login`; the page does not render any
  anonymous-visitor placeholder identity (no literal "guest", no
  fake username).

  **Placement is load-bearing**: the auth area lives **inside the
  banner card**, **right-aligned within its row**, rendered in
  the lower portion of the banner card in both the signed-out and
  signed-in states. When no agents block is present (signed-out
  visitor, or signed-in visitor with zero live MCP token chains
  per R-2IRD-A7HD), the auth area is the bottom row of the banner
  card; when an agents block is present, the agents block sits
  immediately below the auth row and is the bottom element of the
  banner card. The design reference (`reqs/design.md` §3) shows
  this row as a `.banner-auth` block positioned in the lower-
  right of the banner's bordered chrome; that visual signature is
  preserved in the no-agents state and shifts upward by one row
  in the with-agents state. The property pinned here is that the
  auth area visually sits *inside the banner's card chrome,
  right-aligned in its row, in the lower portion of the card* —
  not above the banner in a separate top-bar row, not below the
  banner card, not inline in body content, and not anywhere else
  on the page. A rendering that places the auth area outside the
  banner card does not satisfy this requirement.

  **Signed-out state**: the bottom-right of the banner card
  contains a single pill-style button bearing the literal label
  `Sign in` whose activation reaches `/login`.

  **Signed-in state**: the bottom-right of the banner card
  contains two distinct elements rendered side by side, identity
  on the left and the sign-out affordance immediately to its
  right:
  1. The visitor's email rendered as **inert, non-interactive
     text**. Clicking the email does *not* sign the visitor out
     and does not navigate anywhere — it is a label, not a
     control. **No avatar element accompanies the email**: the
     identity display is the bare email address, with no
     preceding circular initials chip, no monogram badge, and no
     other glyphic identity decoration. This is a named
     deviation from the design reference (`reqs/design.md` §3),
     which shows a `.avatar` element bearing the visitor's
     initials (e.g. `DV` for `dave@discovery.one`) to the left of
     the email; the project does not render that element.
  2. To the right of the email, a **separate, explicitly
     labeled sign-out affordance** — rendered as a pill-style
     button or equivalent visibly-actionable control, bearing the
     literal text `Sign out` (or, if a glyph is used, accompanied
     by an `aria-label="Sign out"`) — whose activation reaches
     `/logout`.

  In the design reference's implementation a single button
  element flips its visible label between `Sign in` and `Sign
  out` depending on session state; whether the build agent
  realizes this as one element whose label swaps or as two
  separately-rendered elements is HOW. The property is that a
  signed-in visitor sees an unambiguous, explicitly-labeled
  control whose activation logs them out — distinct from the
  identity display, with discoverable pill chrome — and a
  signed-out visitor sees an unambiguous, explicitly-labeled
  control whose activation begins sign-in.

  A rendering in which clicking the email itself triggers logout
  does not satisfy this requirement: the user-visible logout
  affordance must be a distinct, separately-labelled element
  from the identity display, because "click your name to sign
  out" is not discoverable. A rendering that drops the pill
  chrome on either control and surfaces only a bare text link
  likewise does not satisfy this requirement. A rendering that
  reintroduces the avatar / initials chip to the left of the
  email — even if otherwise correct — does not satisfy this
  requirement.

  **Hover treatment**: on pointer hover, the `Sign in` / `Sign
  out` pill **inverts** — the button fills with the design
  reference's dark ink (`--ink` ≈ `#14130f`, effectively black)
  and the label and border render in the page background color
  (`--bg` ≈ `#f6f5f1`, the warm off-white the page sits on),
  producing a near-black filled pill with light text. The
  hover treatment is the same in the signed-in `Sign out` state
  as in the signed-out `Sign in` state — the visitor hovering
  either label sees the same dark-filled inversion. A rendering
  in which the pointer hover **lightens** the button toward the
  page background (a nearly-white pill with darker text on
  hover) does not satisfy this requirement; the design intent
  is a dark-filled hover, not a light one. A rendering in which
  the button is visually inert under hover — no fill change, no
  border change, no label color change — likewise does not
  satisfy this requirement. Any transition into and out of the
  hover state is permitted (and is suppressed under
  prefers-reduced-motion per the reduced-motion req), but the
  steady-state on-hover and off-hover appearances are pinned
  regardless of transition.

  The property: exactly one of the two states is reflected on
  every page load; in the signed-out state the auth row exposes
  a reachable route to `/login`; in the signed-in state it shows
  the visitor's bare email as a label (no avatar) *and* exposes
  a separate, explicitly labeled sign-out control that reaches
  `/logout`; and the auth area is anchored inside the banner
  card, right-aligned within its row, in the lower portion of
  the card in both states (with the agents block from
  R-2IRD-A7HD sitting beneath it when present).

## Authenticated MCP agents

- R-2IRD-A7HD: when the signed-in visitor has one or more live
  MCP token chains issued to their email, the index page renders
  an **agents block** inside the banner card, immediately below
  the auth row (R-0WB7-RV1W). The block lists one row per live
  token chain whose owner (R-WRDD-TR27) is the signed-in
  visitor's Google email — the email the web session was
  established under per R-8GJG-64MR. "Live" means a chain with
  at least one un-revoked, un-expired refresh token; chains
  whose refresh ceiling has been hit (R-8UAA-YKR9), whose chain
  has been revoked by reuse detection (R-9HGE-87UG /
  R-A26O-QBG9), or that the visitor has revoked from this page
  (R-0SNI-MJTT) are not listed.

  The block is gated on web-session state: a signed-out visitor
  sees nothing here, and is not told whose agents would be
  listed. When the signed-in visitor has **zero** live chains,
  the block does not render at all — the banner card collapses
  to its auth row as its bottom. The agents block is the only
  element that may render inside the banner card below the auth
  row; no other content lives there.

  The block scopes strictly to the requesting visitor's own
  email: a row corresponding to a chain owned by any other
  email never appears in this visitor's view, regardless of how
  the underlying records are stored or retrieved. This is a
  per-request authorization property, not a UI convention — a
  rendering that surfaces chains belonging to a different email
  does not satisfy this requirement.

  **The agents block's visual signature mirrors the signed-in
  auth row above it** (R-0WB7-RV1W): the block stacks one or
  more agent rows in a single column, each row right-aligned
  to the same right edge as the auth row, occupying the lower-
  right portion of the banner card just as the auth row does.
  The per-row visual shape is pinned by R-2JZ9-NZ82 — a row
  reads as a sibling of the `Sign out` row, not as a separate
  bordered card, not as a centered band beneath the subtitle,
  and not as a wide horizontal strip filling the banner's
  width. A rendering in which the agents block is centered, is
  drawn as its own bordered card, sits above the auth row, or
  uses a visual chrome unrelated to the auth row's chrome does
  not satisfy this requirement.

- R-2JZ9-NZ82: each agent row's visual signature mirrors the
  signed-in auth row's signature pinned by R-0WB7-RV1W: an
  **inert identity label on the left** sitting immediately
  beside a **pill-style action control on the right**,
  right-aligned within the row. The pair reads as a sibling of
  the `(email) [Sign out]` row above it, with the same pill
  chrome and the same hover treatment as the `Sign out` pill;
  a visitor hovering either the `Sign out` pill on the auth row
  or the `Revoke` pill on an agent row sees the identical
  dark-fill inversion R-0WB7-RV1W pins. The row's two visible
  elements are:

  1. The **identity label**, rendered as inert, non-interactive
     text (clicking it does nothing, just as clicking the
     visitor's email on the auth row does nothing). The label
     combines, in this order, the chain's DCR-registered
     `client_name` followed by the chain's `client_id`
     truncated to its first 8 characters as a bare prefix
     enclosed in parentheses — e.g. `Claude Code (670ac9fa)`.
     When the registering DCR client supplied no `client_name`
     (an optional field per RFC 7591), the row displays the
     literal text `undefined` in its place — not an empty
     string, not a dash, not the `client_id` substituted in.
     The pathological case where `client_name` is present but
     happens to be the literal string `undefined` is
     indistinguishable on screen from the unset case, which is
     acceptable. The first-8 prefix renders with no ellipsis,
     no `…` suffix, no separator other than the surrounding
     parentheses and the whitespace between `client_name` and
     the prefix; a row showing the full-length `client_id`, or
     showing the first-8 prefix followed by `…` or `...`, does
     not satisfy this requirement.
  2. A **Revoke pill** immediately to the right of the identity
     label, rendered as an actionable pill-style button bearing
     the literal text `Revoke` (or, if a glyph is used,
     accompanied by `aria-label="Revoke"`), with the same pill
     chrome (border, radius, padding, label weight) and the
     same dark-fill hover inversion as the auth row's `Sign
     out` pill (R-0WB7-RV1W). Activating it reaches the
     chain-revoke action R-0SNI-MJTT defines, scoped to *this
     row's* token chain.

  No other per-row content renders: no issued-at timestamp,
  no last-refresh timestamp, no expires-at countdown, no
  chain-internal identifier, no token plaintext (R-SAK8-WB9W
  still forbids that anywhere outside the issuing response),
  no registered redirect URI, no avatar / initials chip
  preceding the identity label (the auth row drops the avatar
  per R-0WB7-RV1W and the agent row does the same). Adding
  more fields is a spec edit, not an implementation choice.

  Renderings that do not satisfy this requirement include: a
  row centered under the subtitle, a row spanning the full
  banner width, a `Revoke` control rendered as a bare text
  link instead of a pill, a `Revoke` pill whose chrome differs
  visibly from the `Sign out` pill above it, a `Revoke` pill
  whose hover lightens toward the page background instead of
  inverting to dark fill, and an identity label rendered as a
  link or other interactive control.

- R-0RFM-8S34: rows in the agents block are ordered by chain
  initial issuance, **most recent first**. The chain whose
  initial token-chain issuance (the authorization-code
  redemption that first created the chain) is most recent
  renders at the top of the block. Subsequent refresh-token
  rotations within a chain (R-89K0-GH5G) update the chain's
  live refresh token but do not change the chain's place in
  the order. A rendering in which a freshly-refreshed chain
  bubbles to the top of the list does not satisfy this
  requirement; the property is a stable order anchored to the
  chain's original authorization time.

- R-0SNI-MJTT: the page exposes a user-initiated chain-revoke
  action reachable from each agent row's Revoke control
  (R-2JZ9-NZ82). The action is authorized **exclusively** by
  the visitor's web-session cookie (R-SLGL-B5B4) — it does
  not accept bearer tokens, and it never operates on a chain
  whose owner email differs from the web session's owner
  email. A web-session request asking to revoke a chain
  owned by a different email is rejected without revoking
  anything; the service does not disclose whether such a
  chain exists. A request from an unauthenticated user-agent
  is rejected per R-T2JT-53WF / R-53Z2-DNB1 and does not
  reach the revocation path.

  On successful authorization, the action applies the same
  chain-wide revocation R-9HGE-87UG / R-A26O-QBG9 define for
  reuse detection: the chain's live refresh token, every
  outstanding access token issued in that chain, and every
  other chain-membership record (R-WRDD-TR27) are marked
  revoked. From the moment revocation completes, any
  presentation of any token from that chain at the MCP
  transport (R-UK7D-Z0IZ) or at the HTTP API mutation
  endpoints (R-340Z-T6K2 / R-H3FE-QFC0 / R-4ED6-CGQG) is
  rejected with `error="invalid_token"` per R-EV2D-QTR1,
  using an `error_description` that discriminates this
  cause from the other revocation paths.

  The action is **instant** — no confirmation prompt, no
  soft-delete grace period, no undo. The visitor's own web
  session is unaffected by the revoke; R-0XJ4-5MSL's
  lifetime-independence property still holds in the other
  direction (a chain revocation does not end the web
  session).

  Activation is observable as a near-instant page change: the
  affected row disappears from the agents block within the
  live-update budget R-0TVF-0BKI names. A rejection (wrong
  owner, missing chain, no web session) leaves the page
  state unchanged and surfaces a brief failure indication
  without disclosing what kind of chain the missed
  identifier would have matched. The action's HTTP shape
  (request path, body, response) is HOW.

- R-0TVF-0BKI: while a signed-in visitor's browser has the
  index page open and JavaScript is running, every change
  to the set of live token chains owned by that visitor's
  email — a new chain coming into existence via successful
  authorization-code redemption (R-ZPE1-0DV8), a chain
  being revoked through this visitor's Revoke control
  (R-0SNI-MJTT), a chain being revoked by reuse detection
  (R-9HGE-87UG / R-A26O-QBG9), or a chain crossing its
  refresh ceiling (R-8UAA-YKR9) — is reflected on the page
  without the visitor having to reload. Acceptance
  criteria:

  - The live-update transport is Server-Sent Events (SSE),
    distinct from the public counter SSE channel
    R-FZC6-H2SB defines. A signed-out user-agent does not
    connect to this stream; the server rejects connection
    attempts that present no valid web-session cookie per
    R-T2JT-53WF / R-53Z2-DNB1.
  - The stream carries only events scoped to the requesting
    web session's owner email — a connected visitor sees
    their own chain events and nothing else. Per-email
    scoping is enforced server-side per connection, not by
    client-side filtering.
  - A change to the set of chains for an email is reflected
    on every connected browser for that visitor within 1000
    milliseconds (strictly less than one second). A
    measurement of 999 ms passes; a measurement of 1000 ms
    fails.
  - On every new connection, the server's first event on
    the stream is a snapshot of the visitor's current live
    chains, so a browser that just connected (including one
    that auto-reconnected after a network blip) displays
    the visitor's current agent list without waiting for
    the next change.
  - After a transient network disruption, within 5 seconds
    of the network's return, the channel is re-established
    and the rendered list reflects the server's current
    view of the visitor's chains. The page does not linger
    displaying a stale row whose chain was revoked while
    the connection was down.
  - The stream is auth-gated for its entire lifetime: when
    the visitor's web session expires (R-KJ15-9P17) or is
    revoked, the server closes the stream; the page does
    not continue receiving events through an authenticated
    channel after the visitor is no longer signed in.

  The chosen transport must remain compatible with the
  shutdown deadline R-K7DK-LSJ6 pins — a transport whose
  handler blocks process exit on an in-flight stream is
  disqualified, by the same rule R-FZC6-H2SB applies to
  the counter channel.

- R-T6VA-9U84: the SSE channel R-0TVF-0BKI defines honors
  the same resource-budget properties pinned for the
  counter SSE channel: the service remains responsive to
  unrelated requests indefinitely while these auth-gated
  streams are open (R-T4FH-IAQQ), and a stream whose
  browser has gone away — tab closed, page navigated away,
  network dropped, process killed — without the TCP-level
  connection-close machinery firing is detected and
  released by the service within 5 seconds (R-T5ND-W2HF).
  Concretely: a visitor opening N+1 long-lived
  agent-stream connections in succession on the same
  origin and abandoning each one (where N is the service's
  configured concurrent-request-capacity) must not prevent
  an unrelated `GET /login` from completing at ordinary
  latency within 5 seconds of the last abandonment. This
  requirement names the property; the mechanism
  (poll-on-write, socket-readability watcher, transport
  that manages cleanup itself, etc.) is HOW.
