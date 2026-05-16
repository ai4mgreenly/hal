# Banner Auth Stack Examples

These fragments illustrate the structural requirement
R-3RL1-IUP6. They are examples, not a mandatory tag-for-tag
implementation. Any markup that satisfies the same observable
properties is acceptable.

## Acceptable shape

The browser-auth row and agent rows participate in one shared
identity/action stack. The labels and controls are descendants of the
same layout container, so they can share one label column and one
action column.

```html
<div class="banner-auth" aria-label="Signed-in identity and MCP agents">
  <span class="auth-email">michaelgreenly@logic-refinery.com</span>
  <form method="post" action="/logout" class="auth-form">
    <button class="auth-btn" type="submit">Sign out</button>
  </form>

  <span class="agent-name">Claude Code (21bad071)</span>
  <form method="post" action="/agents/revoke" class="agent-form">
    <input type="hidden" name="chain_id" value="...">
    <button class="auth-btn" type="submit">Revoke</button>
  </form>

  <span class="agent-name">Codex (f8da5ca5)</span>
  <form method="post" action="/agents/revoke" class="agent-form">
    <input type="hidden" name="chain_id" value="...">
    <button class="auth-btn" type="submit">Revoke</button>
  </form>
</div>
```

An equivalent row-wrapper shape is also acceptable if the row wrappers
still participate in the same stack and do not become separate banner
flow items:

```html
<div class="banner-auth" aria-label="Signed-in identity and MCP agents">
  <div class="identity-row">
    <span class="auth-email">michaelgreenly@logic-refinery.com</span>
    <form method="post" action="/logout" class="auth-form">
      <button class="auth-btn" type="submit">Sign out</button>
    </form>
  </div>

  <div class="agent-row" data-chain-id="...">
    <span class="agent-name">Claude Code (21bad071)</span>
    <form method="post" action="/agents/revoke" class="agent-form">
      <input type="hidden" name="chain_id" value="...">
      <button class="auth-btn" type="submit">Revoke</button>
    </form>
  </div>
</div>
```

## Broken shape

This shape does not satisfy R-3RL1-IUP6 because `.banner-auth` owns
only the web-session row while `.agents-block` is a separate sibling
flow item under `.banner`. Selectors such as `.banner-auth .agent-row`
cannot align rows that are not descendants of `.banner-auth`.

```html
<section class="banner">
  <h1 class="title">HAL 9000</h1>
  <div class="subtitle-row">...</div>

  <div class="banner-auth">
    <span class="auth-email">michaelgreenly@logic-refinery.com</span>
    <form method="post" action="/logout" class="auth-form">
      <button class="auth-btn" type="submit">Sign out</button>
    </form>
  </div>

  <div class="agents-block">
    <div class="agent-row" data-chain-id="...">
      <span class="agent-name">Claude Code (21bad071)</span>
      <form method="post" action="/agents/revoke">
        <input type="hidden" name="chain_id" value="...">
        <button class="auth-btn" type="submit">Revoke</button>
      </form>
    </div>
  </div>
</section>
```
