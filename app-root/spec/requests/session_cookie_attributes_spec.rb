# R-AYLJ-8SYX: any cookie the service uses to identify a browser
# session is set with HttpOnly and SameSite=Lax. The Secure attribute
# is added only when the response was served over HTTPS, detected via
# the same forwarded-protocol signal R-ID5L-BSJM uses for HSTS. Local
# dev (plain HTTP, R-PVA6-Q6OB) omits Secure so the session cookie can
# survive the OAuth round-trip. The session identifier is rotated on
# successful federated Google login so a planted session ID is no
# longer valid afterwards.
require "rails_helper"

RSpec.describe "Session cookie attributes (R-AYLJ-8SYX)", type: :request do
  def session_set_cookie_lines
    Array(response.headers["Set-Cookie"])
      .flat_map { |h| h.is_a?(String) ? h.split("\n") : [ h ] }
      .select { |c| c.start_with?("_hal_session=") }
  end

  def register_client
    record, _ = OauthClient.register(
      client_name: "Cookie Attr Test #{SecureRandom.hex(4)}",
      redirect_uris: [ "https://client.example/cb" ],
      token_endpoint_auth_method: "none"
    )
    record
  end

  def hit_authorize(scheme:, host:, headers: {})
    record = register_client
    get "#{scheme}://#{host}/oauth/authorize",
        params: {
          client_id: record.client_id, redirect_uri: "https://client.example/cb",
          response_type: "code", scope: "openid", state: "cs",
          code_challenge: "abc", code_challenge_method: "S256"
        },
        headers: headers
  end

  describe "R-AYLJ-8SYX HttpOnly and SameSite=Lax on every session cookie" do
    it "R-AYLJ-8SYX session cookie set over HTTPS carries HttpOnly, SameSite=Lax, and Secure" do
      hit_authorize(scheme: "https", host: "hal.ai.metaspot.org",
                    headers: { "X-Forwarded-Proto" => "https" })
      cookie = session_set_cookie_lines.first
      expect(cookie).to be_present, "expected the service to set a _hal_session cookie"
      expect(cookie).to match(/;\s*HttpOnly/i)
      expect(cookie).to match(/;\s*SameSite=Lax/i)
      expect(cookie).to match(/;\s*secure/i)
    end

    it "R-AYLJ-8SYX session cookie set over plain HTTP omits Secure (dev dispensation per R-PVA6-Q6OB)" do
      hit_authorize(scheme: "http", host: "localhost")
      cookie = session_set_cookie_lines.first
      expect(cookie).to be_present
      expect(cookie).to match(/;\s*HttpOnly/i)
      expect(cookie).to match(/;\s*SameSite=Lax/i)
      expect(cookie).not_to match(/;\s*secure/i)
    end
  end

  describe "R-AYLJ-8SYX session identifier rotates on successful federated login" do
    around do |example|
      previous_domain = Rails.configuration.x.auth.workspace_domain
      Rails.configuration.x.auth.workspace_domain = "allowed.example"
      example.run
      Rails.configuration.x.auth.workspace_domain = previous_domain
    end

    it "R-AYLJ-8SYX the session cookie value differs after a successful Google callback" do
      provider = Rails.configuration.x.google_identity_provider
      record, _ = OauthClient.register(
        client_name: "Rotation Test",
        redirect_uris: [ "https://client.example/cb" ],
        token_endpoint_auth_method: "none"
      )

      get "https://www.example.com/oauth/authorize",
          params: {
            client_id: record.client_id, redirect_uri: "https://client.example/cb",
            response_type: "code", scope: "openid", state: "cs",
            code_challenge: "abc", code_challenge_method: "S256"
          },
          headers: { "X-Forwarded-Proto" => "https" }
      pre_cookie = session_set_cookie_lines.first
      pre_value = pre_cookie[/_hal_session=([^;]+)/, 1]
      upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]

      provider.stub_code(
        "code-rotate", sub: "google-sub-rotate",
        email: "user@allowed.example", hosted_domain: "allowed.example"
      )

      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-rotate", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }

      post_cookie = session_set_cookie_lines.first
      expect(post_cookie).to be_present, "expected callback to set a fresh session cookie"
      post_value = post_cookie[/_hal_session=([^;]+)/, 1]
      expect(post_value).not_to eq(pre_value), "session identifier must rotate on successful federated login"
    end
  end
end
