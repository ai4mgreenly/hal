# Google Cloud Setup

What an operator has to configure on the Google side so this service can
federate logins to Google Workspace per `reqs/auth.md`. Assumes you
already own a Google Workspace tenant; that part isn't covered here.

The service uses Google purely as an upstream OpenID Connect identity
provider. It does **not** call any Workspace API (Drive, Gmail, Admin
SDK, etc.) and asks for no scopes beyond `openid email profile`. That
matters because it keeps the app squarely inside the
"Internal user type, no app verification" lane.

The end state is three values you'll hand to the Rails app via
environment variables:

| Variable                   | Source                                                |
| -------------------------- | ----------------------------------------------------- |
| `GOOGLE_CLIENT_ID`         | OAuth 2.0 client created in step 3                    |
| `GOOGLE_CLIENT_SECRET`     | Same client (shown once at creation; download then)   |
| `GOOGLE_WORKSPACE_DOMAIN`  | The single Workspace domain you're allowing (e.g. `example.com`) |

`reqs/auth.md` R-68WP-XVCK requires these never be committed to the
repo. Put them in your deployment's secret store and in a local
`.env`/Rails credentials file that is gitignored.

---

## 1. Create (or pick) a Cloud project

Cloud Console → **top bar project picker** → **New project**.

- **Name** — anything; e.g. `hal`.
- **Organization** — pick your Workspace organization (not "No
  organization"). This is what makes the Internal user type available
  in step 2 and what restricts the app to your Workspace.
- **Location** — your Workspace org (or a folder under it).

If you're reusing an existing project, confirm it's owned by the
Workspace organization (Cloud Console → IAM & Admin → Settings →
*Organization*). Personal-Gmail-owned projects cannot be Internal.

## 2. Configure the Google Auth Platform (OAuth consent screen)

Cloud Console → **Menu ▸ Google Auth platform ▸ Branding**. (This is
the area formerly labeled "OAuth consent screen.") If the project has
never been configured, you'll be walked through a single wizard;
otherwise the same fields live under the **Branding**, **Audience**,
and **Data Access** tabs.

Fill in:

- **App name** — `Hal` (or whatever you want users to see on
  the Google consent screen).
- **User support email** — a real address from your Workspace.
- **App logo** — optional. Internal apps don't get verified, so a logo
  isn't required.
- **Application home page** — `https://hal.ai.metaspot.org`.
- **Authorized domains** — `metaspot.org` (the registrable domain of
  the deployment URL). Localhost does not need to be listed.
- **Developer contact email** — your address.

On the **Audience** tab, set **User type = Internal**. This is the
critical setting:

- Only accounts inside your Workspace can sign in. Random `@gmail.com`
  users can't even reach the consent screen.
- The app skips Google's app-verification process.
- Restricted/sensitive scope review does not apply.

The `hd` parameter the service sends on the authorize request and the
post-login `hd`-claim check are still required regardless — see
"Defense in depth" below — but Internal user type gives you a hard
outer fence.

You don't need to declare scopes on the **Data Access** tab. The
service requests only `openid`, `email`, and `profile`; these are the
non-sensitive default OIDC scopes and are not listed on the consent
screen for Internal apps.

Save. The status should show **In production** with **Internal**
audience. No "Publish app" button — Internal apps don't have a testing
mode to graduate from.

## 3. Create the OAuth 2.0 Web Application client

Cloud Console → **Menu ▸ Google Auth platform ▸ Clients** → **Create
client** (or go directly to
<https://console.developers.google.com/auth/clients>).

- **Application type** — Web application.
- **Name** — internal label only, e.g. `hal-prod`. Not shown
  to users.
- **Authorized JavaScript origins** — leave empty. The service is a
  server-side OAuth client; no browser-side Google JS is loaded.
- **Authorized redirect URIs** — add **exactly** these, byte-for-byte:

  - `https://hal.ai.metaspot.org/oauth/google/callback` (production)
  - `http://localhost:3000/oauth/google/callback` (local development;
    omit if the operator never runs the dev server against real
    Google)

  Google does byte-exact matching on this URI; trailing slashes,
  scheme, host, port, and path all matter. The path is fixed by
  `config/routes.rb` (`/oauth/google/callback`) and
  `OauthAuthorizeController` builds it as
  `"#{request.base_url}/oauth/google/callback"` — match that.

  If you ever stand up another environment (staging, preview), add
  its callback URL here too. Each environment uses the same client
  ID/secret; what differentiates them is the redirect URI Google sees.

Click **Create**. A modal shows the **Client ID** and **Client
secret**.

> The secret is shown in full **once**. After you dismiss the modal,
> Cloud Console only displays the last four characters. Download the
> JSON ("Download JSON" button on the client's detail page) or copy
> the secret into your secret manager immediately.

## 4. Wire the credentials into the deployment

The service reads three environment variables. The first two are not
yet read by the current code (R-DBZW-40BC defers real-Google to a
later iteration), but the operator-side names below are the ones
`reqs/auth.md` is written against and the ones the live integration
will consume:

```bash
GOOGLE_CLIENT_ID=1234567890-abcdef....apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-...
GOOGLE_WORKSPACE_DOMAIN=example.com
```

`GOOGLE_WORKSPACE_DOMAIN` is already consumed:
`config/initializers/google_identity_provider.rb` reads it into
`Rails.configuration.x.google_workspace_domain`, and the callback
controller rejects any Google identity whose `hd` claim doesn't match
(R-5LQM-O89D).

Set these in whatever the deployment uses for env injection
(systemd unit's `Environment=`, Kamal secrets, Docker `--env-file`,
your platform's secret manager, etc.). Locally, a gitignored `.env`
is fine — just confirm `.env` is in `.gitignore` before you put a
secret in it.

## 5. Smoke test

1. From a browser signed into a Workspace account in the allowed
   domain, hit `https://hal.ai.metaspot.org/oauth/authorize?...`
   (any spec-conformant authorize request — the easiest path is to
   point an MCP client at the service and let it run DCR + the
   authorize flow).
2. You should be redirected to Google, see the standard Google account
   chooser scoped to your Workspace, and land back at
   `/oauth/google/callback` with a code.
3. From a browser signed into a `@gmail.com` account (or any
   non-Workspace account), repeat. Google should refuse to even show
   the consent screen — the Internal user type fences this off
   upstream of your service.
4. From a browser signed into a Workspace account in a *different*
   Workspace domain (if you have one), repeat. Google may let you
   through; the service's own `hd`-claim check then renders the
   `domain_rejected` page (R-5LQM-O89D).

## Defense in depth

Three independent layers enforce the Workspace-only posture, and you
want all three:

1. **Internal user type** (step 2) — Google won't show the consent
   screen to outside accounts at all.
2. **`hd` parameter on the authorize request** — the live
   `GoogleIdentityProvider#authorization_url` should append
   `hd=<GOOGLE_WORKSPACE_DOMAIN>` so Google's account chooser is
   pre-filtered to your domain. Per Google's own docs this is a UX
   optimization and is **not** a security boundary on its own.
3. **`hd`-claim verification in the ID token** (R-5LQM-O89D) — the
   callback controller already compares the Google identity's
   `hosted_domain` to `Rails.configuration.x.google_workspace_domain`
   and refuses to mint a token chain on mismatch. This is the layer
   that actually protects the service; the other two are
   convenience/UX.

## Things you specifically don't need

- **APIs & Services ▸ Library** — no Google APIs to enable. Pure OIDC
  uses no API quota.
- **Service accounts** — the service authenticates as itself with the
  client ID/secret; there's no service-account flow.
- **Domain-wide delegation** — only relevant for impersonating
  Workspace users to call Workspace APIs, which this service doesn't.
- **App verification / brand verification** — not required for
  Internal user type apps.
- **OAuth scopes beyond `openid email profile`** — the service needs
  the subject identifier, the verified email, and the hosted-domain
  claim. Nothing else.

## Rotating the client secret

Cloud Console → **Google Auth platform ▸ Clients ▸** *(your client)*
→ **Add secret**. Google supports two active secrets concurrently so
you can roll without downtime: add the new one, deploy it, then
delete the old one. After deletion, the old secret stops working
immediately.

## When you replace the test double with real Google

`reqs/auth.md` calls out R-CL63-P202 / R-DBZW-40BC: the current code
ships a `GoogleIdentityProvider::Fake` and the real
`GoogleIdentityProvider` raises `NotImplementedError`. The seam is
deliberately narrow (`#authorization_url`, `#exchange_code`). When
real-Google work lands, retire those two requirements per the note
in `reqs/auth.md` and mint fresh IDs for whatever replaces them.

## References

- [Manage OAuth Clients — Google Cloud help](https://support.google.com/cloud/answer/15549257?hl=en)
- [Manage OAuth App Branding — Google Cloud help](https://support.google.com/cloud/answer/15549049?hl=en)
- [Configure the OAuth consent screen and choose scopes](https://developers.google.com/workspace/guides/configure-oauth-consent)
- [Manage App Audience (Internal vs External)](https://support.google.com/cloud/answer/15549945?hl=en)
- [OpenID Connect on Google — `hd` parameter and claims](https://developers.google.com/identity/openid-connect/openid-connect)
- [Using OAuth 2.0 to Access Google APIs](https://developers.google.com/identity/protocols/oauth2)
