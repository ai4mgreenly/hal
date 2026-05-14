# HAL — index page design reference

This document is the structural walkthrough of the index page's visual
contract. It names the components, the class structure, the design
tokens, and the conventions the canonical stylesheet
(`reqs/design.css`) expects. The build agent reads this alongside
`design.css` to render an index page whose computed styles match the
canonical CSS per R-8MP8-6B77.

The design reference was originally drawn as a single self-contained
HTML file. In this project the index page is rendered by the running
service — its DOM structure, class names, and styling must still match
the contract this document describes. Fonts (`Inter`, `JetBrains
Mono`) are loaded from Google Fonts at runtime.

This document is a HOWTO, not a list of separately-tagged
requirements. The load-bearing requirements that govern the index
page's visual contract live in `web.md` (notably R-G6NK-RP8H,
R-8MP8-6B77, R-MCHV-YEO4) and cite the conventions here.

---

## 1. Layout

The body is a single `.page` wrapper (max-width 880px, auto margin).
Inside it, in order:

| Block | Element | Notes |
|---|---|---|
| Hero / banner | `<section class="banner">` | HAL title, rotating subtitle, sign-in control |
| Counter card | `<section class="counter-card">` | Live counter the MCP server can read/mutate |
| Counter hint | `<div id="counterHint">` | Inline string that toggles based on auth state |
| Instructions head | `<div class="instructions-head">` | "Connect an MCP client" + endpoint |
| Client tabs | `<article class="section">` | Tabbed connect instructions (see §4) |
| Footer | `<footer>` | Status line |

Stick to this order. If you add a new section, give it
`border: 1px solid var(--line)` + `background: var(--bg-elev)` +
`border-radius: var(--radius)` so it matches the visual rhythm of
`.banner` / `.section` / `.counter-card`.

---

## 2. Design tokens

All colors, spacing, and radii are CSS variables on `:root`. Read
from them; do not hard-code colors.

```
--bg          page background           #f6f5f1   (warm off-white)
--bg-elev     elevated surface          #fbfaf6
--ink         primary text              #14130f
--ink-soft    secondary text            #4a4842
--ink-mute    tertiary / muted text     #8a877e
--line        primary border            #e3e0d6
--line-soft   subtle divider            #ecebe3
--accent      HAL-lens red              #d4361e
--accent-glow rgb tuple for shadows     212, 54, 30
--code-bg     code-block background     #14130f
--code-ink    code-block foreground     #f3f1ea
--code-mute   code-block muted          #8c8a82
--radius      card corner radius        10px
```

Typography:
- Body text: `'Inter', system-ui, …`, 15px / 1.55.
- Monospace (any `.mono` class, code blocks, pills, the small `01`
  chip on tabs): `'JetBrains Mono', ui-monospace, …`.

There are exactly **two** typefaces. Don't add a third.

### Reserved class names

Some class names in `design.css` carry page-scope styling whose
typography would clobber a component if reused inside one. Treat
these as reserved — never apply them inside a component:

- `.title` — the 88px page heading ("hal"). Component labels never
  use this class.
- `.subtitle` — the rotating subtitle below the page heading.
  Component subtitles, if any, get their own component-scoped
  class.

When tempted to call something `class="title"` inside a component
because "it's the title of that component", reach for a more
specific name (or no class — bare text inheriting from the
component's own typography rule). R-9TPL-HQBV in `web.md` pins
this discipline.

---

## 3. The banner

```html
<section class="banner">
  <span class="lens" aria-hidden="true"></span>   <!-- pulsing red dot, top-left -->
  <span class="tag">MCP · demo</span>             <!-- monospace label, top-right -->
  <h1 class="title">HAL 9000</h1>
  <div class="subtitle-row">
    <span class="subtitle" id="subtitle">…</span>
    <button class="refresh" id="refresh">…svg…</button>
  </div>
  <div class="banner-auth">
    <span class="who" id="who" hidden>
      <span>dave@discovery.one</span>
    </span>
    <button class="auth-btn" id="authBtn">Sign in</button>
  </div>
</section>
```

- `.lens` is a CSS-only pulsing red dot — driven by the `pulse`
  keyframes. Don't replace with an image.
- `.banner-auth` is absolutely positioned bottom-right. Sign-in lives
  **inside the banner**, not above it.
- `#subtitle` is populated at runtime from the subtitle bank defined
  by R-G47S-05R3 (in `web.md`). Click `#refresh` swaps to a new random
  one with a quick fade.

### Adding a new subtitle

The subtitle bank is pinned in the spec (R-G47S-05R3). Adding,
removing, or replacing entries is a spec edit, not a code edit.

---

## 4. The client-tabs component

This is the only non-trivial interactive element. Two tabs, two
panels.

```
.client-tabs                         flex row, bottom border = --line-soft
  .client-tab[aria-controls=…]       button; gets .active when selected
    .num                             "01" / "02" mono chip
    (label text)                     plain text node: "Claude Code", etc.
.client-panel[id=…]                  hidden by default; .active makes it visible
```

The label text — `Claude Code`, `Claude Desktop`, and any future
client added per §4.3 — sits inside the button as a bare text node,
not wrapped in a child element with a class of its own. The button's
own `.client-tab` typography governs the label; wrapping the label
in a class (`.title`, `.label`, `.name`, anything) invites
global-class collisions and is forbidden. The tab contains exactly
two visible elements: the `.num` chip and the label text. No
subtitle, no hint, no secondary line — content that explains what
the panel will show ("Run the following command.", "Add the
following JSON to…") belongs inside the panel body, not inside the
tab trigger. R-UBYN-1LY0 in `web.md` pins this contract.

Wiring (sketch — the actual implementation is the build agent's
choice, but the observable behavior is the same):

```js
clientTabs.forEach(t => t.addEventListener('click', () => {
  clientTabs.forEach(x => x.classList.remove('active'));
  t.classList.add('active');
  const target = t.getAttribute('aria-controls');
  clientPanels.forEach(p => p.classList.toggle('active', p.id === target));
}));
```

### Tab 01 — Claude Code

Contains **two stacked scope blocks** (`.scope-block`, separated by a
top border via `.scope-block + .scope-block`), per R-G5FO-DXHS:

1. **project** — pill `project`, command uses `--scope project`.
2. **user** — pill `user`, command uses `--scope user`.

Each block has its own `<div class="code">` with a
`<button class="copy" data-copy="cli-project">` /
`data-copy="cli-user"` and the command marked up token-by-token in
spans (`.arg .flag .url`) so colors come through.

### Tab 02 — Claude Desktop

A single JSON code block. The copy button uses `data-copy="json"` and
pulls from `#jsonCmd`.

### Adding a third client

1. Add a `<button class="client-tab" role="tab" aria-controls="panel-newthing">`
   with a `.num` chip.
2. Add a matching `<div class="client-panel" id="panel-newthing">`
   with `.section-body` inside.
3. Add a unique `data-copy="…"` to its copy button and extend the
   copy handler.

Do **not** nest a second row of tabs inside a panel. Inside-panel
variations are stacked `.scope-block`s, not sub-tabs.

---

## 5. The counter

```html
<section class="counter-card">
  <div>
    <div class="counter-label">Current count</div>
    <div class="counter-value" id="counterValue">
      <span id="count">0</span>
      <span class="delta" id="delta">+1</span>
    </div>
  </div>
  <div class="counter-actions">
    <button class="icon-btn" id="decBtn" disabled>−</button>
    <button class="icon-btn" id="incBtn" disabled>+</button>
  </div>
</section>
```

The counter value is **server-owned**, not browser-local. The
canonical visual contract above (DOM structure, class names, flash
and delta animation) is what this document pins; the actual value
flows from the service per `counter.md` and is delivered via the
live-update channel per R-FZC6-H2SB. The `+` / `−` buttons drive the
HTTP API mutation endpoints per R-FY4A-3B1M, and remain disabled when
no web session is active (R-EJAP-XUSB).

The visual transition contract — value flash, delta-badge slide,
disabled state — lives in `design.css` and the rules R-EJAP-XUSB and
R-FY4A-3B1M cite. Reduced-motion suppression follows R-G0K2-UUJ0.

---

## 6. Auth state

The sign-in / sign-out UI in the banner reflects whether the visitor
currently has an active web session. Web sessions are real: they are
established by the Google federation flow per `auth.md` (R-8GJG-64MR,
R-CXJ2-R3BN, R-3BKZ-L7R4) and persisted server-side per R-SLGL-B5B4.
The page does not "remember" auth state in `localStorage` or any
other client-side store — the server is the source of truth.

On render, the page reflects the session-cookie state:

- Signed-out: the auth area shows a single **Sign in** button. The
  counter `+` / `−` buttons are disabled. The counter hint copy
  invites the visitor to sign in.
- Signed-in: the auth area shows the visitor's email and a **Sign
  out** button (per R-0WB7-RV1W, no avatar / initials chip). The
  counter `+` / `−` buttons are enabled. The counter hint copy
  acknowledges the active session.

The auth-area DOM nodes match the canonical class names
(`.banner-auth`, `.who`, `.auth-btn`) so the canonical stylesheet
styles them directly.

---

## 7. Code blocks

```html
<div class="code">
  <button class="copy" data-copy="UNIQUE_KEY">…clipboard svg… <span>copy</span></button>
  <span class="prompt">$ </span>           <!-- optional shell prompt -->
  <span id="someId">
    <span class="arg">claude</span>
    mcp add
    <span class="flag">--transport</span> http
    <span class="url">http://localhost:3000/mcp</span>
  </span>
</div>
```

Token classes available: `.arg` (command), `.flag` (CLI flag),
`.url`, `.prompt`, `.key`, `.str`, `.punct` (for JSON). All defined
in the style block; they read from `--code-ink` / `--accent` etc.

URLs in code blocks are derived from the request the page was served
on, per R-CO4Y-11X7 and R-DA34-WX9P — they are not hard-coded.

To wire a new copy button, add a branch in the click handler that
matches the new `data-copy` key and copies the corresponding text
node.

---

## 8. Responsive

There is one breakpoint at `max-width: 640px`:

- `.page` padding tightens.
- `.banner` padding tightens; `.title` drops from 88px to 64px.
- `.counter-card` becomes a column.
- The lens dot and `MCP · demo` tag move closer to the corners.

If you add new sections, check them at ≤ 640px before merging.

---

## 9. Do / don't

**Do**
- Use CSS variables for any new color or border.
- Reuse `.section` / `.banner` / `.counter-card` chrome for new
  cards.
- Match the existing tone: short, dry, slightly knowing. No emoji.

**Don't**
- Don't introduce a CSS framework or a CSS build/preprocess step
  (per R-8MP8-6B77; the served stylesheet is the canonical CSS,
  plain).
- Don't add a third typeface.
- Don't nest tabs inside tabs — stack `.scope-block`s instead.
- Don't move the sign-in button out of `.banner-auth`.
- Don't hard-code hex colors when a token exists.
- Don't fall back to client-side persistence (e.g. `localStorage`)
  for the counter value or for auth state. The server is the source
  of truth for both.
