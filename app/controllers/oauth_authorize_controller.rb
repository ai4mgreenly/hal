# R-4SH1-HQGP: when a user reaches the service's authorize endpoint,
# the service redirects them to Google so that Google performs the
# actual login. The client's authorization request (client_id,
# redirect_uri, response_type, scope, state, code_challenge,
# code_challenge_method) is stashed in the session keyed by the
# upstream `state` token; the Google callback (R-5LQM-O89D, future
# iteration) reads it back to mint this service's own authorization
# code.
#
# R-ETP6-60VA: the upstream `state` is a fresh unguessable value
# generated per request and recorded in the user's browser session
# (the only place the in-flight authorize request lives). Stashing
# it in `session[:pending_authorizations]` binds the value to both
# the in-flight request and the originating browser session — a
# different browser presenting the same `state` will not have the
# entry. An `expires_at` timestamp gives the callback an
# expiry-rejection mechanism; single-use is enforced at the
# callback by deleting the entry on consumption.
class OauthAuthorizeController < ApplicationController
  STATE_TTL_SECONDS = 600

  def show
    # R-1ERW-YD9G: validate redirect_uri is byte-for-byte registered
    # for the requesting client before doing anything else. A
    # mismatched redirect_uri is refused here — never used as a
    # redirect target — so it cannot serve as an open redirect.
    client = OauthClient.find_by(client_id: params[:client_id].to_s)
    if client.nil? || !client.redirect_uris.include?(params[:redirect_uri].to_s)
      render plain: "invalid_request: redirect_uri is not registered for this client",
             status: :bad_request
      return
    end

    upstream_state = SecureRandom.urlsafe_base64(24)
    session[:pending_authorizations] ||= {}
    session[:pending_authorizations][upstream_state] = {
      "client_id" => params[:client_id],
      "redirect_uri" => params[:redirect_uri],
      "response_type" => params[:response_type],
      "scope" => params[:scope],
      "state" => params[:state],
      "code_challenge" => params[:code_challenge],
      "code_challenge_method" => params[:code_challenge_method],
      # R-IS0W-S2H3: capture the RFC 8707 `resource` parameter when the
      # client sends one so the eventual auth code (and the tokens minted
      # from it) record the resource binding the client requested.
      "resource" => params[:resource],
      "expires_at" => (Time.now.utc + STATE_TTL_SECONDS).iso8601
    }

    callback_uri = "#{request.base_url}/oauth/google/callback"
    provider = Rails.configuration.x.google_identity_provider
    redirect_to provider.authorization_url(state: upstream_state, redirect_uri: callback_uri),
                allow_other_host: true
  end
end
