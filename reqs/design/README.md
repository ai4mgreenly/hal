# Handoff: HAL — MCP demo landing page

## Overview
A single-page landing site for **HAL**, a demo MCP (Model Context Protocol) server. The page demonstrates an MCP-mutable state primitive — a shared counter — and gives developers copy-paste instructions for connecting Claude Code or Claude Desktop to the local MCP server.

## About the design files
The file `HAL.html` in this bundle is a **design reference** built in HTML/CSS/vanilla JS. It is not production code to ship verbatim — your task is to **recreate this design inside the target codebase's existing environment** (React/Next, Vue, etc.), using whatever component library, styling solution, and auth/state plumbing is already established. If no frontend environment exists yet, pick the framework that best fits the rest of the stack (Next.js + Tailwind is a reasonable default) and implement there.

## Fidelity
**High-fidelity.** Final colors, type, spacing, and interactions. Match it closely.

## Screens / Views

There is one screen. From top to bottom:

### 1. Top bar (auth)
- **Layout**: 36px tall row, right-aligned content, 24px margin below.
- **Signed-out state**: shows a single pill button **"Sign in"**.
- **Signed-in state**: shows `[avatar] dave@discovery.one` then the **"Sign out"** pill.
  - Avatar: 22×22 circle, dark gradient (`linear-gradient(135deg, #2a2823, #14130f)`), white initials "DV" centered, 10px / 600 weight.
  - Email: 13px, `--ink-mute`.
- **Auth button**: 1px border, `--bg-elev` fill, 13px / 500, `8px 14px` padding, fully rounded (`border-radius: 999px`). Hover inverts to dark fill (`--ink` bg, `--bg` text, `--ink` border).

### 2. Banner card
- **Container**: full-width card, 1px `--line` border, `--bg-elev` fill, 10px radius, `56px 40px 48px` padding, centered text.
- **Lens dot** (absolute top-left, 22/24px in): 14×14 circle. Background is a radial gradient `radial-gradient(circle at 35% 35%, #ff7a5a 0%, #d4361e 45%, #6a0e00 100%)`. Outer glow via two stacked box-shadows pulsing on a 2.6s ease-in-out loop (see `@keyframes pulse`). This is the only piece of motion on the page besides hover transitions; do not remove it.
- **Tag** (absolute top-right): mono 11px / 500, uppercase, `--ink-mute`, letter-spacing 0.08em — content `"MCP · demo"`.
- **Title**: `HAL`, 88px / 700, letter-spacing `-0.04em`, line-height 0.95, margin 0. Drops to 64px under 640px.
- **Subtitle row** (inline-flex, 10px gap, 14px above):
  - Italic phrase, 18px, `--ink-soft`. The text is a HAL **backronym** picked at random on every page load. See "Subtitle bank" below for the full list.
  - Refresh icon button: 28×28 circle, 1px `--line` border, transparent fill. On hover: border `--ink`, color `--ink`, `transform: rotate(90deg)`. On click: `rotate(180deg)`, then the subtitle fades out (220ms), swaps, and fades back in.

### 3. Counter card
- **Container**: same card chrome as banner; 28px gap above; `32px 36px` padding; flex row with `space-between` (stacks on <640px).
- **Left block**:
  - Label `CURRENT COUNT` — 12px, `--ink-mute`, letter-spacing 0.12em, uppercase.
  - Value: monospace 56px / 600, baseline-aligned with a green delta indicator (`+1` / `-1`) that fades in/out on each change.
  - On change: number flashes `--accent` red briefly (600ms), delta slides in from -4px and fades out.
- **Right block (counter actions)**: two 42×42 icon buttons `−` and `+`, 8px radius, 1px border, 20px / 500 glyph. **Disabled when signed out** (40% opacity, not-allowed cursor). Hover when enabled inverts to dark fill.
- **Hint line** (below card, 8px gap, 4px x-padding): 12px `--ink-mute`. Copy depends on auth state:
  - signed out: *"Sign in to manipulate the counter from the browser. The MCP server can read & mutate it for any signed-in user."*
  - signed in:  *"Signed in. The MCP server can read & mutate this counter on your behalf."*

### 4. Instructions header
- 48px above, 16px below. Flex row, baseline-aligned, `space-between`.
- Left: `<h2>Connect an MCP client</h2>` — 18px / 600, letter-spacing `-0.01em`.
- Right: `Point a client at <code>http://localhost:3000/mcp</code>` — 13px `--ink-mute`, the URL in JetBrains Mono.

### 5. Section — Claude Code (CLI)
- Card chrome (same), 14px gap between sections.
- **Section head** (`16px 22px`, bottom border `--line-soft`):
  - Numeric badge `01` — mono 11px in a 1px `--line` rounded-4 chip, `--bg` fill.
  - Title `Claude Code`, 15px / 600.
  - Right-aligned desc *"CLI · adds a server entry to a scope"* — 13px `--ink-mute`.
- **Section body** (`18px 22px 22px`):
  - **Tabs** (`role="tablist"`): two tab buttons, no background, 2px bottom border (transparent / accent red when active). Labels:
    - `Project scope` (small mono `.mcp.json`) — **default**
    - `User scope` (small mono `~/.claude.json`)
  - **Scope meta row** (12px gap above code): a `--bg`-fill pill (`shared` or `personal`) followed by a one-sentence description of where the scope writes:
    - project → "Commits the server to `.mcp.json` at the repo root — everyone on the project gets it."
    - user → "Records the server in `~/.claude.json` — available in every project you open."
  - **Code block** (dark): see "Code block" component below. Content swaps with the active tab:
    - project: `$ claude mcp add --transport http --scope project hal http://localhost:3000/mcp`
    - user:    `$ claude mcp add --transport http --scope user hal http://localhost:3000/mcp`

### 6. Section — Claude Desktop (JSON)
- Same section chrome. Number badge `02`. Desc *"JSON · add to your `claude_desktop_config.json`"*.
- Single dark code block with the snippet (4-space indent):
  ```json
  {
    "mcpServers": {
      "hal": {
        "type": "http",
        "url": "http://localhost:3000/mcp"
      }
    }
  }
  ```

### 7. Footer
- Top border `--line`, 56px gap above, 22px padding-top. Flex row, `space-between`, 12px mono `--ink-mute`.
- Left: a status indicator. 7×7 green dot (`--ok`) with a soft glow, then text *"server listening on :3000"*.
- Right: *"v0.1.0 · open my pod bay doors"*.

## Reusable components

### Code block (dark)
- Background `--code-bg` (#14130f), text `--code-ink` (#f3f1ea), 1px `#1f1d18` border, 8px radius, `14px 16px` padding, JetBrains Mono 13px / 500, `white-space: pre`, horizontal scroll allowed.
- Token colors:
  - `--code-mute` (#8c8a82): the `$` prompt, JSON punctuation
  - `#e0a96d` (warm tan): CLI flags (`--transport`, `--scope`), JSON keys
  - `#a8c8ff` (cool blue): command name (`claude`)
  - `#79d4a9` (mint): URLs and JSON string values
- **Copy button** (absolute top-right, 10px in): semi-transparent white pill (`rgba(255,255,255,0.06)` bg, `0.12` border), 11px mono "copy" label + small clipboard SVG, 6/10 padding, 6px radius. On click, copies the block's plain text via `navigator.clipboard.writeText` (with a `<textarea> + execCommand` fallback), then flips to mint-green border + text `"copied"` for 1.4s.

### Auth button / pill button
- See "Top bar" above. Reuse for any tertiary pill action.

### Section card
- 1px `--line` border, `--bg-elev` fill, 10px radius, overflow hidden, 14px vertical gap between stacked instances. Optional header with numeric badge / title / right-aligned description, bottom-bordered with `--line-soft`.

## Interactions & behavior

| Trigger | Behavior |
|---|---|
| Page load | Pick a random subtitle from the bank. Restore `count` and `auth` from localStorage. |
| Click refresh icon | Fade subtitle out (220ms), pick a different random entry (never the same one twice in a row), fade back in. Icon rotates 90° on hover, 180° on active. |
| Click "Sign in" | Set `hal:auth` = `"1"`, button label → "Sign out", reveal user pill, enable +/− buttons, swap hint copy. |
| Click "Sign out" | Reverse the above. |
| Click `+` / `−` | Mutate `count` by ±1, persist to `hal:count`, animate value color flash + delta indicator (600ms). |
| Click tab (project/user) | Swap pill text, description, and CLI command. Mark tab active (red bottom border + `--ink` text). |
| Click copy button | Copy plaintext to clipboard. Show "copied" state for 1.4s. |

All non-essential transitions are 120–250ms, ease.

## State

Two persisted values, plus transient UI state:

| Key | Storage | Type | Notes |
|---|---|---|---|
| `hal:auth` | localStorage | `"1"` or empty | Bool-ish. Empty = signed out. |
| `hal:count` | localStorage | stringified int | The counter. |
| `lastSubtitleIdx` | in-memory | int | Avoid repeating subtitles back-to-back. |
| `activeScope` | in-memory | `"project"` \| `"user"` | Which Claude Code tab is selected. |

**In a real app**, replace localStorage auth with the codebase's existing auth system (NextAuth, Clerk, custom session, etc.) and the counter with whatever backend the MCP server is actually reading from (Redis, Postgres row, in-memory map). The MCP server's tool surface should expose at minimum `get_count`, `increment_count`, `decrement_count` (or a single `set_count`).

## Subtitle bank

Pick at random on each page load and on each refresh-icon click:

```
Human Augmentation Layer
Hardware Abstraction Layer
Heuristically programmed ALgorithmic computer
Hyperdimensional Access Layer
Holistic Application Logic
Helpful Autonomous Liaison
Highly Adaptive Listener
Headless Agent Loop
Hosted Action Library
Hermetic Authorization Layer
Hypertext Application Language
High-Availability Lambda
Heretical Automation Layer
Hyper-tuned Agent Logic
Handy Autoresponse Layer
Hallucination Avoidance Layer
Honest Assistant, Lately
Halfway Awake Loop
Homemade Agent Lab
Heuristic Argument Linker
```

## Design tokens

### Color
| Token | Value | Use |
|---|---|---|
| `--bg` | `#f6f5f1` | Page background (warm off-white) |
| `--bg-elev` | `#fbfaf6` | Card surfaces |
| `--ink` | `#14130f` | Primary text |
| `--ink-soft` | `#4a4842` | Secondary text |
| `--ink-mute` | `#8a877e` | Tertiary / meta |
| `--line` | `#e3e0d6` | Card + control borders |
| `--line-soft` | `#ecebe3` | Inner dividers |
| `--accent` | `#d4361e` | HAL lens, active tab underline, count flash |
| `--ok` | `oklch(60% 0.12 145)` | Status dot, increment delta |
| `--code-bg` | `#14130f` | Dark code block bg |
| `--code-ink` | `#f3f1ea` | Code text |
| `--code-mute` | `#8c8a82` | Prompt + JSON punctuation |
| (token) flag | `#e0a96d` | CLI flags / JSON keys |
| (token) arg | `#a8c8ff` | Command name |
| (token) url/str | `#79d4a9` | URLs / strings |

### Typography
- **Body / UI**: Inter, weights 400/500/600/700.
- **Mono / code**: JetBrains Mono, weights 400/500.
- Use the system font stack as fallback for both.

Type scale used:
| Role | Size | Weight | Notes |
|---|---|---|---|
| Title (HAL) | 88px (64 mobile) | 700 | `letter-spacing: -0.04em`, `line-height: 0.95` |
| Counter value | 56px (44 mobile) | 600 | mono, `letter-spacing: -0.02em` |
| H2 section header | 18px | 600 | `letter-spacing: -0.01em` |
| Subtitle | 18px | 400 italic | |
| Section title | 15px | 600 | |
| Body | 15px | 400 | line-height 1.55 |
| Code | 13px | 500 | mono |
| Meta / desc | 13px | 400 | `--ink-mute` |
| Counter label | 12px | 500 | uppercase, letter-spacing 0.12em |
| Status / footer | 12px | 400 | mono |
| Mono badges | 11px | 500 | uppercase where used |

### Spacing
- Page max-width 880px, padding `28px 32px 96px` (20/18/64 mobile).
- Section card gap: 14px between stacked sections.
- Standard card padding: `32–56px` block, `22–40px` inline.
- Standard control padding: `8px 14px` (pill), `10px 14px` (tab), `6px 10px` (copy button).

### Radius
- `10px` for cards, `8px` for code blocks and icon buttons, `4px` for the section-number badge, `999px` for pills, `50%` for circles (avatar, lens, refresh, status dot).

### Shadows / glow
- Lens dot pulse: `0 0 0 3-4px rgba(212,54,30,0.08–0.12), 0 0 14–24px rgba(212,54,30,0.35–0.65)`, animated 2.6s.
- Status dot: `0 0 6px var(--ok)`.

### Motion
- Most transitions: 120–250ms, default ease.
- Subtitle swap: 220ms opacity + 3px translateY.
- Refresh icon rotate: 150ms hover → 90°, active → 180°.
- Counter delta + flash: 600ms total.
- Lens pulse: 2.6s ease-in-out infinite.

## Assets
No external image assets. Only:
- Inline SVG icons (refresh, clipboard) — feather/lucide style, 2px stroke. Replace with your codebase's existing icon set (`lucide-react`, `heroicons`, etc.).
- Google Fonts `Inter` + `JetBrains Mono`. Replace with your project's font loading (Next `next/font/google`, Vite plugin, etc.).

## Responsive
- ≤640px: tighten page padding, drop title to 64px, drop counter value to 44px, stack counter card vertically. Lens / tag move 6px inward.
- ≥640px: as documented above.

## Accessibility checklist
- Tabs use `role="tablist"` / `role="tab"` / `aria-selected`. Tabpanels are not strictly defined in the reference — add `role="tabpanel"` + `aria-labelledby` when porting.
- Refresh button: `aria-label="New subtitle"`.
- Counter buttons: `aria-label="Increment"` / `"Decrement"`, `disabled` when signed out.
- Decorative lens dot: `aria-hidden="true"`.
- Live-region for the counter value would be a nice addition (`aria-live="polite"`) — not in the reference.
- Honor `prefers-reduced-motion`: kill the lens pulse and the icon rotation/transitions.

## Files in this handoff
- `HAL.html` — the full design reference (self-contained, opens in any browser).
- `README.md` — this document.
