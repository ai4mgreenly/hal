# R-7GT3-PM1K: POST /oauth/token issues access tokens with a finite
# lifetime and refresh tokens that let well-behaved clients stay logged
# in without re-prompting on every expiry. The authorization_code grant
# redeems a service-issued authorization code (R-ZPE1-0DV8) and mints a
# fresh chain consisting of one access token (R-TNXJ-ZWQ0: 1h) and one
# refresh token (R-8UAA-YKR9: 30d) bound to the code's owner.
require "base64"

class OauthTokenController < ActionController::API
  def create
    # R-4GRA-EGBY: issue-time canonical-resource check. If the request carries
    # a `resource` parameter, it must be byte-equal to the configured
    # canonical resource identifier (R-3UT3-IKZG). Reject before any grant
    # handler runs and before any token is minted. Use RFC 8707 §3.2's
    # `invalid_target`. The auth-code-bound match (R-IS0W-S2H3) below still
    # applies once we've cleared this gate.
    if params[:resource].present? &&
       params[:resource].to_s != Rails.configuration.x.auth.canonical_url
      render_error("invalid_target")
      return
    end

    case params[:grant_type].to_s
    when "authorization_code"
      handle_authorization_code
    when "refresh_token"
      handle_refresh_token
    else
      render_error("unsupported_grant_type")
    end
  end

  private

  def handle_authorization_code
    code = OauthAuthorizationCode.find_by_presented_code(params[:code].to_s)
    return render_error("invalid_grant") if code.nil?
    return render_error("invalid_grant") unless code.client_id == params[:client_id].to_s
    return render_error("invalid_grant") unless code.redirect_uri == params[:redirect_uri].to_s
    return render_error("invalid_grant") unless pkce_ok?(code, params[:code_verifier].to_s)
    # R-IS0W-S2H3: if the token request also presents a `resource`, it
    # must match the resource the auth code was issued against. The
    # auth-code resource is what flows onto the minted tokens.
    return render_error("invalid_grant") if params[:resource].present? && params[:resource].to_s != code.resource.to_s
    return render_error("invalid_grant") unless code.redeem!

    issue_chain_tokens(owner: code.owner, chain_id: code.chain_id, resource: code.resource)
  end

  # R-89K0-GH5G: each successful refresh-token use issues a new refresh
  # token alongside the new access token, and the presented refresh
  # token is invalidated (used_at stamped) the moment its successor is
  # issued.
  # R-9HGE-87UG: a refresh token presented after it has already been
  # used (or was otherwise invalidated) is treated as evidence of
  # compromise; the entire chain is revoked and the request rejected.
  def handle_refresh_token
    presented = params[:refresh_token].to_s
    return render_error("invalid_grant") if presented.blank?

    token = OauthToken.find_by_presented_token(presented)
    return render_error("invalid_grant") if token.nil?
    return render_error("invalid_grant") unless token.kind == "refresh"

    if token.used_at.present? || token.revoked_at.present?
      revoke_chain!(token.chain_id)
      return render_error("invalid_grant")
    end

    return render_error("invalid_grant") if Time.current >= token.expires_at

    token.update!(used_at: Time.current)
    # R-IS0W-S2H3: rotated tokens carry forward the resource binding
    # established when the chain was first authorized.
    issue_chain_tokens(owner: token.owner, chain_id: token.chain_id, resource: token.resource)
  end

  def issue_chain_tokens(owner:, chain_id:, resource:)
    access_lifetime = Rails.configuration.x.auth.access_token_lifetime
    refresh_lifetime = Rails.configuration.x.auth.refresh_token_lifetime
    _, access_plain = OauthToken.issue(
      kind: "access", owner: owner, chain_id: chain_id, lifetime: access_lifetime, resource: resource
    )
    _, refresh_plain = OauthToken.issue(
      kind: "refresh", owner: owner, chain_id: chain_id, lifetime: refresh_lifetime, resource: resource
    )

    render json: {
      access_token: access_plain,
      token_type: "Bearer",
      expires_in: access_lifetime.to_i,
      refresh_token: refresh_plain
    }
  end

  def revoke_chain!(chain_id)
    now = Time.current
    OauthToken.where(chain_id: chain_id, revoked_at: nil).update_all(revoked_at: now)
    OauthAuthorizationCode.where(chain_id: chain_id, revoked_at: nil).update_all(revoked_at: now)
  end

  def pkce_ok?(code, verifier)
    return false if verifier.blank?
    case code.code_challenge_method
    when "S256"
      Base64.urlsafe_encode64(Digest::SHA256.digest(verifier), padding: false) == code.code_challenge
    else
      false
    end
  end

  def render_error(error)
    render json: { error: error }, status: :bad_request
  end
end
