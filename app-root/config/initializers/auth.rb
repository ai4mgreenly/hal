# R-LWCN-ZBXO: every numeric and string value that governs the service's
# authentication posture lives here, on `Rails.configuration.x.auth.*`.
# Code that consumes any of these values reads it from this surface and
# does not duplicate the literal anywhere else in the codebase. Changing
# a token lifetime, a session ceiling, the Google scope list, the HSTS
# max-age, etc., is a single-file edit in this initializer.
#
# Secrets (Google client credentials, R-68WP-XVCK) are read from the
# environment here; missing required env vars raise loudly rather than
# being silently defaulted. In the test environment the fake Google
# provider is used and the credentials are not required.

config = Rails.application.config
config.x.auth ||= ActiveSupport::OrderedOptions.new

# R-TNXJ-ZWQ0: access tokens expire one hour after issue.
config.x.auth.access_token_lifetime = 1.hour

# R-8UAA-YKR9: refresh tokens expire thirty days after issue.
config.x.auth.refresh_token_lifetime = 30.days

# R-KJ15-9P17: web sessions are bounded by a 1-hour idle ceiling and a
# 12-hour absolute ceiling; the earlier wins.
config.x.auth.web_session_idle_ttl = 1.hour
config.x.auth.web_session_absolute_ttl = 12.hours

# R-ZPE1-0DV8: authorization codes are short-lived and single-use.
config.x.auth.authorization_code_ttl = 60.seconds

# R-W3K0-QD0E: the OIDC scope list requested at Google.
config.x.auth.google_scopes = %w[openid email profile]

# R-3BKZ-L7R4: the web /login flow forces Google to re-authenticate the
# human via `prompt=login`. R-126C-AM1E: the MCP authorize flow does not
# pass a prompt parameter (rides Google's silent SSO).
config.x.auth.web_login_prompt = "login"
config.x.auth.mcp_login_prompt = nil

# R-5LQM-O89D: the single Workspace domain whose users are allowed.
# Operators set this at deploy time via GOOGLE_WORKSPACE_DOMAIN; tests
# override `Rails.configuration.x.auth.workspace_domain` directly.
config.x.auth.workspace_domain = ENV["GOOGLE_WORKSPACE_DOMAIN"]

# R-3UT3-IKZG: the canonical resource identifier. For a service whose
# protected endpoints sit at the root path of the host, the canonical
# form includes a single trailing `/`.
canonical_default = case Rails.env
when "test" then "http://www.example.com/"
when "development" then "http://localhost:3000/"
else "https://hal.ai.metaspot.org/"
end
config.x.auth.canonical_url = ENV.fetch("HAL_CANONICAL_URL", canonical_default)

# R-ID5L-BSJM: HSTS max-age in seconds (one year).
config.x.auth.hsts_max_age = 31_536_000

# R-68WP-XVCK: Google client credentials. Required outside the test env
# where the fake provider is wired in by the google_identity_provider
# initializer (R-CL63-P202). Missing values raise loudly.
if Rails.env.test?
  config.x.auth.google_client_id = ENV["GOOGLE_CLIENT_ID"]
  config.x.auth.google_client_secret = ENV["GOOGLE_CLIENT_SECRET"]
else
  config.x.auth.google_client_id = ENV.fetch("GOOGLE_CLIENT_ID")
  config.x.auth.google_client_secret = ENV.fetch("GOOGLE_CLIENT_SECRET")
end
