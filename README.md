# hal

hal is a small, deliberately readable app that shows how to put **real
authentication** in front of a tiny API. It's a template for the auth
wiring — the API itself is intentionally trivial so what's on display is
the login flows, not a domain model.

**Who can call the API, and how they prove who they are:**

- **People** — log in through OAuth, federated to Google Workspace.
- **Agents** — authenticate over OAuth too, so an MCP client can connect
  on a person's behalf.
- **Automation** *(planned)* — long-lived API keys for scripts and
  services.

A logged-in user can see every agent connected to their account and
**revoke** any of it.

**The API** is one shared counter: read it, increment it, decrement it —
nothing else. It's a stand-in for "some tool that changes state," kept
trivial on purpose.

**Planned**

- API-key authentication for automation.
- Roles / policy (RBAC): a logged-in user grants specific roles to the
  agents and API keys acting for them.

---

**MCP tools.** The tool definitions the server advertises to an agent,
exactly as returned by `tools/list` — verbatim name and description, all
three taking no arguments (`counter_read` is open; `counter_increment`
and `counter_decrement` require an authenticated caller):

```json
[
  {
    "name": "counter_read",
    "description": "Return the current value of the shared counter. Takes no arguments. The value is a non-negative integer that any client can observe; reading does not modify it. Use this when you need to know the counter's state before deciding whether to call counter_increment or counter_decrement."
  },
  {
    "name": "counter_increment",
    "description": "Add one to the shared counter and return the new value. Takes no arguments. The returned value is the counter's state AFTER the increment, a non-negative integer. Use this when the user wants the counter to go up by one; call counter_read first if you need the pre-increment value."
  },
  {
    "name": "counter_decrement",
    "description": "Subtract one from the shared counter and return the new value. Takes no arguments. The returned value is the counter's state AFTER the decrement, a non-negative integer. The counter cannot go below zero: if it is already zero, this tool returns an error and does not modify the counter. Use this when the user wants the counter to go down by one."
  }
]
```

---

Build & run → [`app-root/README.md`](app-root/README.md) · Spec &
requirements layout → [`helper/SPEC.md`](helper/SPEC.md)
