# R-W3K0-QD0E: real Google OAuth/OIDC client. #authorization_url builds
# a URL on Google's documented authorization endpoint with the
# configured client id, redirect uri, OIDC scopes, state, and the `hd`
# parameter pinned to the configured Workspace domain (R-5LQM-O89D,
# R-68WP-XVCK). #exchange_code POSTs the authorization code to
# Google's documented token endpoint, decodes the returned ID token,
# and returns an Identity carrying sub/email/hosted_domain/email_verified.
require "base64"
require "json"
require "net/http"
require "uri"

class GoogleIdentityProvider
  Identity = Struct.new(:sub, :email, :hosted_domain, :email_verified, keyword_init: true)

  AUTHORIZATION_ENDPOINT = "https://accounts.google.com/o/oauth2/v2/auth".freeze
  TOKEN_ENDPOINT = "https://oauth2.googleapis.com/token".freeze

  def authorization_url(state:, redirect_uri:, scope: nil, prompt: nil)
    auth = Rails.configuration.x.auth
    scope ||= auth.google_scopes.join(" ")
    params = {
      client_id: auth.google_client_id,
      redirect_uri: redirect_uri,
      response_type: "code",
      scope: scope,
      state: state,
      hd: auth.workspace_domain
    }
    params[:prompt] = prompt if prompt
    "#{AUTHORIZATION_ENDPOINT}?#{URI.encode_www_form(params)}"
  end

  def exchange_code(code:, redirect_uri:)
    auth = Rails.configuration.x.auth
    response = Net::HTTP.post_form(URI.parse(TOKEN_ENDPOINT), {
      "grant_type"    => "authorization_code",
      "code"          => code,
      "redirect_uri"  => redirect_uri,
      "client_id"     => auth.google_client_id,
      "client_secret" => auth.google_client_secret
    })
    body = JSON.parse(response.body)
    id_token = body.fetch("id_token")
    segment = id_token.split(".")[1].to_s
    padded = segment + ("=" * ((4 - segment.length % 4) % 4))
    claims = JSON.parse(Base64.urlsafe_decode64(padded))
    Identity.new(
      sub: claims["sub"],
      email: claims["email"],
      hosted_domain: claims["hd"],
      email_verified: claims["email_verified"]
    )
  end
end
