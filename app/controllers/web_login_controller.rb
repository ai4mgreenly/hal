# R-8GJG-64MR: browser-facing login flow distinct from the MCP
# authorization flow. The stable web entry point (/login) initiates
# the same Google Workspace federation that the MCP authorize flow
# uses; the upstream `state` value is stashed in this browser's
# session so the callback can recognize the request as a web-login
# round-trip and, on success, record a web session keyed by the
# visitor's Google email.
class WebLoginController < ApplicationController
  STATE_TTL_SECONDS = 600

  def show
    # R-9PNQ-BN2G: a visitor who already has an active web session
    # short-circuits to `/` instead of starting a fresh federation
    # round-trip. Visitors without a web session fall through to the
    # immediate Google redirect (no service-rendered interstitial).
    if current_web_email.present?
      redirect_to "/"
      return
    end

    upstream_state = SecureRandom.urlsafe_base64(24)
    session[:pending_web_logins] ||= {}
    session[:pending_web_logins][upstream_state] = {
      "expires_at" => (Time.now.utc + STATE_TTL_SECONDS).iso8601
    }
    callback_uri = "#{request.base_url}/oauth/google/callback"
    provider = Rails.configuration.x.google_identity_provider
    # R-3BKZ-L7R4: every web /login redirect must force Google to
    # re-authenticate the human rather than satisfying the request via
    # silent SSO from an existing Google session cookie.
    redirect_to provider.authorization_url(
                  state: upstream_state,
                  redirect_uri: callback_uri,
                  prompt: Rails.configuration.x.auth.web_login_prompt
                ),
                allow_other_host: true
  end
end
