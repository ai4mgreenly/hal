# Request spec for the browser-facing logout flow (reqs/web.md).
require "rails_helper"

RSpec.describe "Web logout (R-AE1P-Z1WC)", type: :request do
  let(:provider) { Rails.configuration.x.google_identity_provider }

  around do |example|
    previous_domain = Rails.configuration.x.auth.workspace_domain
    Rails.configuration.x.auth.workspace_domain = "allowed.example"
    example.run
    Rails.configuration.x.auth.workspace_domain = previous_domain
  end

  def establish_web_session(email:)
    get "https://www.example.com/login"
    upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
    provider.stub_code(
      "code-logout-#{email}",
      sub: "google-#{email}",
      email: email,
      hosted_domain: "allowed.example"
    )
    get "https://www.example.com/oauth/google/callback",
        params: { code: "code-logout-#{email}", state: upstream_state },
        headers: { "X-Forwarded-Proto" => "https" }
    expect(WebSession.find_by_presented_token(session[:web_session_id])&.owner).to eq(email)
  end

  describe "GET /logout" do
    it "R-AE1P-Z1WC clears the web session and redirects to / when signed in" do
      establish_web_session(email: "user@allowed.example")

      get "https://www.example.com/logout"

      expect(response).to redirect_to("/")
      expect(WebSession.find_by_presented_token(session[:web_session_id])).to be_nil
    end

    it "R-AE1P-Z1WC is a no-op redirect to / when not signed in" do
      get "https://www.example.com/logout"

      expect(response).to have_http_status(:found)
      expect(response).to redirect_to("/")
      expect(WebSession.find_by_presented_token(session[:web_session_id])).to be_nil
    end

    it "R-AE1P-Z1WC does not touch the MCP token chain (R-93PJ-FRPY)" do
      establish_web_session(email: "tokenholder@allowed.example")

      access, = OauthToken.issue(
        kind: "access",
        owner: "google-tokenholder@allowed.example",
        lifetime: 1.hour
      )
      refresh, = OauthToken.issue(
        kind: "refresh",
        owner: "google-tokenholder@allowed.example",
        lifetime: 30.days,
        chain_id: access.chain_id
      )

      get "https://www.example.com/logout"

      expect(response).to redirect_to("/")
      expect(OauthToken.exists?(access.id)).to be(true)
      expect(OauthToken.exists?(refresh.id)).to be(true)
    end
  end
end
