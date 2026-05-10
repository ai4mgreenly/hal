# R-CL63-P202: in-memory stand-in for Google's OAuth/OIDC endpoints.
# Returns payloads whose shape matches Google's documented responses
# so the service code exercises the same code paths it will use
# against the real Google. R-DBZW-40BC: subclasses GoogleIdentityProvider
# so the swap is a single configuration point.
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

    # R-126C-AM1E: callers that need to force fresh re-authentication
    # (the web /login flow per R-3BKZ-L7R4) pass prompt explicitly; the
    # provider does not bake it in, so the MCP authorize redirect rides
    # Google's default silent-SSO behavior.
    def authorization_url(state:, redirect_uri:, scope: nil, prompt: nil)
      scope ||= Rails.configuration.x.auth.google_scopes.join(" ")
      params = {
        client_id: CLIENT_ID,
        redirect_uri: redirect_uri,
        response_type: "code",
        scope: scope,
        state: state
      }
      params[:prompt] = prompt if prompt
      "#{AUTHORIZATION_ENDPOINT}?#{URI.encode_www_form(params)}"
    end

    # R-T0B2-A4E5: returns the same Identity contract as the real
    # provider so callers do not branch on which implementation is wired.
    def exchange_code(code:, redirect_uri:)
      identity = @identities.fetch(code) do
        raise ArgumentError, "fake Google: unknown authorization code #{code.inspect}"
      end
      Identity.new(
        sub: identity.sub,
        email: identity.email,
        hosted_domain: identity.hosted_domain,
        email_verified: identity.email_verified
      )
    end
  end
end
