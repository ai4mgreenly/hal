# R-ID5L-BSJM: emit transport-security headers on every response.
# X-Content-Type-Options: nosniff is unconditional. Strict-Transport-Security
# is added only when the request actually arrived over HTTPS, detected via
# the same forwarded-protocol signal R-DA34-WX9P honors from a trusted proxy.
class SecurityHeaders
  HSTS_VALUE = "max-age=31536000; includeSubDomains".freeze

  def initialize(app)
    @app = app
  end

  def call(env)
    status, headers, body = @app.call(env)
    headers["X-Content-Type-Options"] = "nosniff"
    headers["Strict-Transport-Security"] = HSTS_VALUE if ssl_request?(env)
    [ status, headers, body ]
  end

  private

  def ssl_request?(env)
    request = ActionDispatch::Request.new(env)
    request.ssl?
  end
end
