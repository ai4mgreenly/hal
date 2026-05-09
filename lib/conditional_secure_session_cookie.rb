# R-AYLJ-8SYX: the session cookie carries `Secure` only when the
# response is being returned over HTTPS, detected via the same
# forwarded-protocol signal R-ID5L-BSJM uses to gate HSTS (i.e.
# `request.ssl?`, which honors the trusted proxy under R-DA34-WX9P).
# Local development per R-PVA6-Q6OB speaks plain HTTP; without this
# dispensation modern browsers refuse to store a `Secure` cookie set
# over `http://`, the session evaporates between the authorize redirect
# and the Google callback, and the state-binding check (R-ETP6-60VA)
# rejects every callback in dev.
class ConditionalSecureSessionCookie
  def initialize(app, key:)
    @app = app
    @key = key
  end

  def call(env)
    status, headers, body = @app.call(env)
    request = ActionDispatch::Request.new(env)
    rewrite!(headers, secure: request.ssl?)
    [ status, headers, body ]
  end

  private

  HEADER_KEYS = %w[Set-Cookie set-cookie].freeze

  def rewrite!(headers, secure:)
    HEADER_KEYS.each do |key|
      next unless headers.key?(key)
      original = headers[key]
      cookies = flatten_cookies(original)
      rewritten = cookies.map { |c| rewrite_cookie(c, secure: secure) }
      headers[key] = original.is_a?(Array) ? rewritten : rewritten.join("\n")
    end
  end

  def flatten_cookies(value)
    Array(value).flat_map { |c| c.is_a?(String) ? c.split("\n") : [ c ] }
  end

  def rewrite_cookie(cookie, secure:)
    return cookie unless session_cookie?(cookie)
    stripped = cookie.sub(/;\s*secure(?=;|\z)/i, "")
    secure ? "#{stripped}; secure" : stripped
  end

  def session_cookie?(cookie)
    cookie.start_with?("#{@key}=")
  end
end
