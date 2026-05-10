# R-5LQM-O89D: receives Google's redirect after upstream login,
# exchanges the code, and rejects any user whose Google identity is
# outside the configured Workspace domain. No token is issued in the
# rejection path.
#
# R-ETP6-60VA: the callback is accepted only when the returned
# `state` is recognized (lives in *this* browser's session-keyed
# pending-authorizations map), unexpired (the recorded
# `expires_at` is still in the future), and has not been consumed
# before. The pending entry is `delete`d on consumption, making
# `state` single-use; a different browser session, an unknown
# value, an expired value, or a replayed value all hit the
# rejection path and no token chain is issued.
class OauthGoogleCallbackController < ApplicationController
  def show
    # R-8GJG-64MR: web-login flows stash their upstream `state` in a
    # separate session bucket. If the presented `state` lives there,
    # this is a browser federation round-trip — handle it and stop.
    web_pending = (session[:pending_web_logins] || {}).delete(params[:state])
    return handle_web_login(web_pending) if web_pending

    pending = (session[:pending_authorizations] || {}).delete(params[:state])
    if pending.nil?
      render plain: "Unknown or expired authorization state.", status: :bad_request
      return
    end

    expires_at = pending["expires_at"]
    if expires_at.present? && Time.now.utc >= Time.iso8601(expires_at)
      render plain: "Unknown or expired authorization state.", status: :bad_request
      return
    end

    callback_uri = "#{request.base_url}/oauth/google/callback"
    provider = Rails.configuration.x.google_identity_provider
    # R-T0B2-A4E5: the seam returns an Identity carrying the four claims
    # the callback consumes; callers do not branch on which implementation
    # is wired and do not look for extras either side may incidentally expose.
    identity = provider.exchange_code(code: params[:code], redirect_uri: callback_uri)

    allowed_domain = Rails.configuration.x.auth.workspace_domain

    if allowed_domain.blank? || identity.hosted_domain != allowed_domain
      @allowed_domain = allowed_domain
      @presented_domain = identity.hosted_domain
      @presented_email = identity.email
      render "oauth_google_callback/domain_rejected", status: :forbidden
      return
    end

    # R-AYLJ-8SYX: rotate the session identifier on successful federated
    # login so any session ID an attacker may have planted in the victim's
    # browser before the flow began is no longer valid afterwards.
    reset_session

    # R-ZPE1-0DV8: mint a service-issued authorization code bound to
    # the originating authorize request's client_id, redirect_uri, and
    # PKCE code_challenge (with method). The code is short-lived and
    # single-use; the plaintext is returned to the user-agent exactly
    # once via the redirect query string and never persisted server-
    # side beyond its SHA-256 digest.
    # R-IS0W-S2H3: forward the resource binding captured at authorize time
    # onto the issued auth code. Absent a client-supplied value we fall
    # through to the model default (the configured canonical URL).
    issue_kwargs = {
      client_id: pending["client_id"],
      redirect_uri: pending["redirect_uri"],
      code_challenge: pending["code_challenge"].to_s,
      code_challenge_method: pending["code_challenge_method"].to_s,
      owner: identity.sub.to_s
    }
    issue_kwargs[:resource] = pending["resource"] if pending["resource"].present?
    _, code_plaintext = OauthAuthorizationCode.issue(**issue_kwargs)

    target = URI.parse(pending["redirect_uri"])
    query = URI.decode_www_form(target.query.to_s)
    query << [ "code", code_plaintext ]
    query << [ "state", pending["state"] ] if pending["state"].present?
    target.query = URI.encode_www_form(query)
    redirect_to target.to_s, allow_other_host: true
  end

  private

  # R-8GJG-64MR: handle the web-login leg of the Google callback.
  # The federation, expiry check, and workspace-domain gate are the
  # same as the MCP flow's; on success we record a web session keyed
  # by the visitor's Google email (the identity the rest of the app
  # sees for the signed-in visitor) and redirect to "/".
  def handle_web_login(pending)
    expires_at = pending["expires_at"]
    if expires_at.present? && Time.now.utc >= Time.iso8601(expires_at)
      render plain: "Unknown or expired authorization state.", status: :bad_request
      return
    end

    callback_uri = "#{request.base_url}/oauth/google/callback"
    provider = Rails.configuration.x.google_identity_provider
    identity = provider.exchange_code(code: params[:code], redirect_uri: callback_uri)

    allowed_domain = Rails.configuration.x.auth.workspace_domain
    if allowed_domain.blank? || identity.hosted_domain != allowed_domain
      @allowed_domain = allowed_domain
      @presented_domain = identity.hosted_domain
      @presented_email = identity.email
      render "oauth_google_callback/domain_rejected", status: :forbidden
      return
    end

    # R-SLGL-B5B4: persist the web session as a row keyed by a hashed
    # opaque identifier; the cookie carries only the plaintext id.
    establish_web_session(email: identity.email.to_s)
    redirect_to "/"
  end
end
