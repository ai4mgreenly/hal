# Request spec for the browser-facing login flow (reqs/web.md +
# reqs/auth.md "Web sessions").
require "rails_helper"

RSpec.describe "Web login (R-8GJG-64MR)", type: :request do
  let(:provider) { Rails.configuration.x.google_identity_provider }

  around do |example|
    previous_domain = Rails.configuration.x.auth.workspace_domain
    Rails.configuration.x.auth.workspace_domain = "allowed.example"
    example.run
    Rails.configuration.x.auth.workspace_domain = previous_domain
  end

  def initiate_web_login
    # Use https so the Secure session cookie set by R-AYLJ-8SYX
    # comes back on the callback request.
    get "https://www.example.com/login"
    URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
  end

  describe "GET /login" do
    it "R-8GJG-64MR initiates Google federation distinct from the MCP authorize flow" do
      get "https://www.example.com/login"
      expect(response).to have_http_status(:found)
      uri = URI.parse(response.location)
      expect(uri.host).to eq("accounts.google.com")
      query = URI.decode_www_form(uri.query.to_s).to_h
      expect(query["state"]).to be_present
      # The web flow does not put its state into the MCP authorize bucket.
      expect(session[:pending_authorizations]).to be_blank
      expect(session[:pending_web_logins]).to have_key(query["state"])
    end
  end

  describe "GET /login forces re-authentication at Google (R-3BKZ-L7R4)" do
    it "R-3BKZ-L7R4 redirect carries prompt=login so Google re-authenticates the human" do
      get "https://www.example.com/login"
      uri = URI.parse(response.location)
      query = URI.decode_www_form(uri.query.to_s).to_h
      expect(query["prompt"]).to eq("login")
    end
  end

  describe "GET /login when already signed in (R-9PNQ-BN2G)" do
    it "R-9PNQ-BN2G redirects to / instead of starting a fresh federation round-trip" do
      # Establish an active web session via the federation round-trip.
      upstream_state = initiate_web_login
      provider.stub_code(
        "code-web-already",
        sub: "google-web-3",
        email: "already@allowed.example",
        hosted_domain: "allowed.example"
      )
      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-web-already", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }
      expect(WebSession.find_by_presented_token(session[:web_session_id])&.owner).to eq("already@allowed.example")

      # A second hit to /login must not bounce out to Google.
      get "https://www.example.com/login"

      expect(response).to redirect_to("/")
      uri = URI.parse(response.location)
      expect(uri.host).not_to eq("accounts.google.com")
    end
  end

  describe "GET /oauth/google/callback (web-login leg)" do
    it "R-8GJG-64MR rejects identities outside the configured Workspace domain " \
       "with a clear error and no web session" do
      upstream_state = initiate_web_login
      provider.stub_code(
        "code-web-outside",
        sub: "google-web-1",
        email: "intruder@other.example",
        hosted_domain: "other.example"
      )

      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-web-outside", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }

      expect(response).to have_http_status(:forbidden)
      expect(response.body).to include("Sign-in not allowed")
      expect(response.body).to include("allowed.example")
      expect(WebSession.find_by_presented_token(session[:web_session_id])).to be_nil
    end

    it "R-8GJG-64MR records a web session identified by the visitor's Google email " \
       "on successful federation" do
      upstream_state = initiate_web_login
      provider.stub_code(
        "code-web-allowed",
        sub: "google-web-2",
        email: "user@allowed.example",
        hosted_domain: "allowed.example"
      )

      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-web-allowed", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }

      expect(response).to redirect_to("/")
      expect(WebSession.find_by_presented_token(session[:web_session_id])&.owner).to eq("user@allowed.example")
      # Web flow does not mint an MCP authorization code.
      expect(OauthAuthorizationCode.where(owner: "google-web-2").count).to eq(0)
    end
  end
end
