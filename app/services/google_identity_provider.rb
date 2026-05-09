# R-CL63-P202: the seam between this service and Google's OAuth/OIDC
# endpoints. The real implementation talks to Google; in tests the
# Fake subclass below stands in. R-DBZW-40BC: a single class with a
# narrow interface (#authorization_url, #exchange_code) is the swap
# point so live-Google integration can land later without rewriting
# the specs that consume it.
class GoogleIdentityProvider
  Identity = Struct.new(:sub, :email, :hosted_domain, :email_verified, keyword_init: true)

  AUTHORIZATION_ENDPOINT = "https://accounts.google.com/o/oauth2/v2/auth".freeze
  TOKEN_ENDPOINT = "https://oauth2.googleapis.com/token".freeze
  DEFAULT_SCOPE = "openid email profile".freeze

  def authorization_url(state:, redirect_uri:, scope: DEFAULT_SCOPE)
    raise NotImplementedError, "real Google integration deferred (R-DBZW-40BC)"
  end

  def exchange_code(code:, redirect_uri:)
    raise NotImplementedError, "real Google integration deferred (R-DBZW-40BC)"
  end
end
