# Request spec for the Google OAuth callback (reqs/auth.md).
require "rails_helper"

RSpec.describe "OAuth Google Callback", type: :request do
  let(:provider) { Rails.configuration.x.google_identity_provider }

  around do |example|
    previous_domain = Rails.configuration.x.google_workspace_domain
    Rails.configuration.x.google_workspace_domain = "allowed.example"
    example.run
    Rails.configuration.x.google_workspace_domain = previous_domain
  end

  def initiate_authorize
    record, _ = OauthClient.register(
      client_name: "Callback Test Client",
      redirect_uris: [ "https://client.example.com/cb" ],
      token_endpoint_auth_method: "none"
    )
    # Hit the authorize endpoint over https so the Secure session
    # cookie set by R-QGB5-EMOO will travel back on the subsequent
    # callback request (Rack::Test refuses to send Secure cookies on
    # http URLs).
    get "https://www.example.com/oauth/authorize",
        params: {
          client_id: record.client_id,
          redirect_uri: "https://client.example.com/cb",
          response_type: "code",
          scope: "openid",
          state: "client-state",
          code_challenge: "abc",
          code_challenge_method: "S256"
        }
    URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
  end

  describe "GET /oauth/google/callback" do
    it "R-5LQM-O89D rejects users whose Google identity is outside the configured Workspace domain" do
      upstream_state = initiate_authorize
      provider.stub_code(
        "code-outside",
        sub: "google-sub-1",
        email: "intruder@other.example",
        hosted_domain: "other.example"
      )

      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-outside", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }

      expect(response).to have_http_status(:forbidden)
      expect(response.body).to include("Sign-in not allowed")
      expect(response.body).to include("allowed.example")
      expect(response.body).to include("No access token has been issued")
      expect(response.headers["Set-Cookie"].to_s).not_to include("access_token")
    end

    it "R-5LQM-O89D rejects users with no hosted-domain claim (personal Google accounts)" do
      upstream_state = initiate_authorize
      provider.stub_code(
        "code-personal",
        sub: "google-sub-2",
        email: "someone@gmail.com",
        hosted_domain: nil
      )

      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-personal", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }

      expect(response).to have_http_status(:forbidden)
      expect(response.body).to include("No access token has been issued")
    end

    it "R-5LQM-O89D accepts users whose hosted-domain claim matches the configured Workspace domain" do
      upstream_state = initiate_authorize
      provider.stub_code(
        "code-allowed",
        sub: "google-sub-3",
        email: "user@allowed.example",
        hosted_domain: "allowed.example"
      )

      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-allowed", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }

      expect(response).to have_http_status(:found)
      target = URI.parse(response.location)
      expect("#{target.scheme}://#{target.host}#{target.path}")
        .to eq("https://client.example.com/cb")
      query = URI.decode_www_form(target.query.to_s).to_h
      expect(query["state"]).to eq("client-state")
      expect(query["code"]).to be_present
    end

    it "R-ZPE1-0DV8 mints a service authorization code bound to the originating client_id, redirect_uri, " \
       "and PKCE code_challenge" do
      upstream_state = initiate_authorize
      provider.stub_code(
        "code-bound",
        sub: "google-sub-bound",
        email: "user@allowed.example",
        hosted_domain: "allowed.example"
      )

      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-bound", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }

      query = URI.decode_www_form(URI.parse(response.location).query.to_s).to_h
      record = OauthAuthorizationCode.find_by_presented_code(query["code"])
      expect(record).not_to be_nil
      expect(record.redirect_uri).to eq("https://client.example.com/cb")
      expect(record.code_challenge).to eq("abc")
      expect(record.code_challenge_method).to eq("S256")
      expect(record.client_id).to be_present
      expect(record.owner).to eq("google-sub-bound")
    end

    it "R-ZPE1-0DV8 issues authorization codes that are short-lived and single-use" do
      upstream_state = initiate_authorize
      provider.stub_code(
        "code-life",
        sub: "google-sub-life",
        email: "user@allowed.example",
        hosted_domain: "allowed.example"
      )
      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-life", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }
      plaintext = URI.decode_www_form(URI.parse(response.location).query.to_s).to_h["code"]
      record = OauthAuthorizationCode.find_by_presented_code(plaintext)

      expect(record.expires_at - record.issued_at).to be <= 5.minutes
      expect(record.redeem!).to be(true)
      expect(record.reload.used_at).to be_present
      expect(record.redeem!).to be(false)

      fresh, plain2 = OauthAuthorizationCode.issue(
        client_id: record.client_id, redirect_uri: record.redirect_uri,
        code_challenge: record.code_challenge,
        code_challenge_method: record.code_challenge_method,
        owner: record.owner, lifetime: -1.second
      )
      expect(OauthAuthorizationCode.find_by_presented_code(plain2).redeem!).to be(false)
      expect(fresh.reload.used_at).to be_nil
    end
  end
end
