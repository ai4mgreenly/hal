# R-ID5L-BSJM: emit transport-security headers on every response.
# X-Content-Type-Options: nosniff is unconditional. Strict-Transport-Security
# is added only when the request actually arrived over HTTPS, detected via
# the same forwarded-protocol signal R-DA34-WX9P honors from a trusted proxy.
class SecurityHeaders
  def initialize(app)
    @app = app
  end

  def call(env)
    status, headers, body = @app.call(env)
    headers["X-Content-Type-Options"] = "nosniff"
    if ssl_request?(env)
      max_age = Rails.configuration.x.auth.hsts_max_age
      headers["Strict-Transport-Security"] = "max-age=#{max_age}; includeSubDomains"
    end
    [ status, headers, body ]
  end

  private

  def ssl_request?(env)
    request = ActionDispatch::Request.new(env)
    request.ssl?
  end
end
