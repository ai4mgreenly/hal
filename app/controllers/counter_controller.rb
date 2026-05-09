# JSON HTTP API for the singleton counter (reqs/api.md).
class CounterController < ActionController::API
  # R-4ED6-CGQG: POST /counter/increment requires a valid bearer access
  # token issued by this service, presented as `Authorization: Bearer
  # <token>`. R-53Z2-DNB1: an unauthenticated or invalid-token request
  # returns 401 and does not change the counter.
  before_action :require_access_token, only: :increment

  # R-2I2S-XB7K: returns the current count as a non-negative integer
  # under the "value" key. R-3R73-2TN9: no authentication required.
  def show
    render json: { value: Counter.current.value }
  end

  # R-340Z-T6K2: increment the counter by one and return the
  # post-increment value as JSON `{ "value": <int> }`.
  # R-6UUW-TQP2: any issued access token grants permission to call
  # this tool; there are no finer-grained scopes.
  def increment
    render json: { value: Counter.current.increment! }
  end

  private

  # R-A26O-QBG9: a token whose `revoked_at` is set is rejected, so a
  # chain revoked by reuse-detection cascade takes effect for newly
  # arriving requests against this protected endpoint.
  # R-27SO-F63X: bearer tokens are looked up exclusively in the
  # service's own `oauth_tokens` table via the digest scheme — Google
  # access tokens (or any externally-minted credential) cannot match,
  # because only tokens this service minted and signed (digested) are
  # storable here.
  # R-IS0W-S2H3: the presented access token's recorded resource binding
  # must equal this service's configured canonical URL. A token bound to
  # any other resource — or carrying no recorded binding at all — is
  # rejected, even though only one resource currently exists.
  def require_access_token
    presented = bearer_token_from_header
    token = presented && OauthToken.find_by_presented_token(presented)
    canonical = Rails.configuration.x.canonical_url
    return if token && token.kind == "access" &&
              token.revoked_at.nil? && token.expires_at > Time.current &&
              token.resource.present? && token.resource == canonical

    render json: { error: "invalid_token" }, status: :unauthorized
  end

  def bearer_token_from_header
    header = request.headers["Authorization"].to_s
    return nil unless header.start_with?("Bearer ")
    value = header.sub(/\ABearer\s+/, "").strip
    value.empty? ? nil : value
  end
end
