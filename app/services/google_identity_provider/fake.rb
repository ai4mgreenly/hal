# R-CL63-P202: in-memory stand-in for Google's OAuth/OIDC endpoints.
# Returns payloads whose shape matches Google's documented responses
# so the service code exercises the same code paths it will use
# against the real Google. R-DBZW-40BC: subclasses GoogleIdentityProvider
# so the swap is a single configuration point.
require "base64"
require "json"
require "securerandom"
require "uri"

class GoogleIdentityProvider
  class Fake < GoogleIdentityProvider
    CLIENT_ID = "fake-google-client-id".freeze

    def initialize
      @identities = {}
    end

    # Pre-register an identity that will be returned when `code` is
    # exchanged. Lets request specs script the upstream login outcome.
    def stub_code(code, sub:, email:, hosted_domain:, email_verified: true)
      @identities[code] = Identity.new(
        sub: sub,
        email: email,
        hosted_domain: hosted_domain,
        email_verified: email_verified
      )
    end

    def authorization_url(state:, redirect_uri:, scope: DEFAULT_SCOPE)
      params = {
        client_id: CLIENT_ID,
        redirect_uri: redirect_uri,
        response_type: "code",
        scope: scope,
        state: state,
        access_type: "offline",
        prompt: "consent"
      }
      "#{AUTHORIZATION_ENDPOINT}?#{URI.encode_www_form(params)}"
    end

    def exchange_code(code:, redirect_uri:)
      identity = @identities.fetch(code) do
        raise ArgumentError, "fake Google: unknown authorization code #{code.inspect}"
      end
      access_token = "fake-google-access-#{SecureRandom.hex(8)}"
      refresh_token = "fake-google-refresh-#{SecureRandom.hex(8)}"
      claims = {
        "iss" => "https://accounts.google.com",
        "aud" => CLIENT_ID,
        "sub" => identity.sub,
        "email" => identity.email,
        "email_verified" => identity.email_verified,
        "hd" => identity.hosted_domain,
        "iat" => Time.now.to_i,
        "exp" => Time.now.to_i + 3600
      }
      {
        "access_token" => access_token,
        "expires_in" => 3599,
        "refresh_token" => refresh_token,
        "scope" => DEFAULT_SCOPE,
        "token_type" => "Bearer",
        "id_token" => encode_id_token(claims),
        "id_token_claims" => claims
      }
    end

    private

    def encode_id_token(claims)
      header = Base64.urlsafe_encode64({ alg: "none", typ: "JWT" }.to_json, padding: false)
      payload = Base64.urlsafe_encode64(claims.to_json, padding: false)
      "#{header}.#{payload}."
    end
  end
end
