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

  # R-A26O-QBG9 / R-27SO-F63X / R-IS0W-S2H3 / R-EV2D-QTR1: rejects
  # invalid tokens with a distinct error_description for each of the six
  # named failure causes: no token, malformed, not found, expired, revoked,
  # wrong resource.
  def require_access_token
    presented = bearer_token_from_header
    unless presented
      render json: { error: "invalid_request",
                     error_description: "No bearer token presented" },
             status: :unauthorized
      return
    end

    unless presented.match?(/\A[A-Za-z0-9_-]{43}\z/)
      render json: { error: "invalid_token",
                     error_description: "Token is malformed" },
             status: :unauthorized
      return
    end

    token = OauthToken.find_by_presented_token(presented)
    unless token&.kind == "access"
      render json: { error: "invalid_token",
                     error_description: "Token not found" },
             status: :unauthorized
      return
    end

    if token.revoked_at.present?
      render json: { error: "invalid_token",
                     error_description: "Token has been revoked" },
             status: :unauthorized
      return
    end

    if token.expires_at <= Time.current
      render json: { error: "invalid_token",
                     error_description: "Token has expired" },
             status: :unauthorized
      return
    end

    canonical = Rails.configuration.x.canonical_url
    return if token.resource.present? && token.resource == canonical

    render json: { error: "invalid_token",
                   error_description: "Token resource binding does not match" },
           status: :unauthorized
  end

  def bearer_token_from_header
    header = request.headers["Authorization"].to_s
    return nil unless header.start_with?("Bearer ")
    value = header.sub(/\ABearer\s+/, "").strip
    value.empty? ? nil : value
  end
end
