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

Build & run → [`app-root/README.md`](app-root/README.md) · Spec &
requirements layout → [`helper/SPEC.md`](helper/SPEC.md)
