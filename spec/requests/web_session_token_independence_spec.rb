# Request spec for R-93PJ-FRPY: a web session and an MCP token chain are
# independent identity contexts that do not share lifetime or revocation.
require "rails_helper"

RSpec.describe "Web session vs MCP token independence (R-93PJ-FRPY)", type: :request do
  let(:provider) { Rails.configuration.x.google_identity_provider }
  let(:email) { "dual@allowed.example" }
  let(:owner) { "google-#{email}" }

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
      "code-indep-#{email}",
      sub: "google-#{email}",
      email: email,
      hosted_domain: "allowed.example"
    )
    get "https://www.example.com/oauth/google/callback",
        params: { code: "code-indep-#{email}", state: upstream_state },
        headers: { "X-Forwarded-Proto" => "https" }
    expect(WebSession.find_by_presented_token(session[:web_session_id])&.owner).to eq(email)
  end

  it "R-93PJ-FRPY logout leaves MCP tokens issued for the same email intact" do
    establish_web_session(email: email)

    access, = OauthToken.issue(kind: "access", owner: owner, lifetime: 1.hour)
    refresh, = OauthToken.issue(
      kind: "refresh", owner: owner, lifetime: 30.days, chain_id: access.chain_id
    )

    get "https://www.example.com/logout"

    expect(response).to redirect_to("/")
    expect(WebSession.find_by_presented_token(session[:web_session_id])).to be_nil
    expect(OauthToken.exists?(access.id)).to be(true)
    expect(OauthToken.exists?(refresh.id)).to be(true)
    expect(OauthToken.find(access.id).owner).to eq(owner)
  end

  it "R-93PJ-FRPY destroying an MCP token chain leaves the web session intact" do
    establish_web_session(email: email)

    access, = OauthToken.issue(kind: "access", owner: owner, lifetime: 1.hour)
    refresh, = OauthToken.issue(
      kind: "refresh", owner: owner, lifetime: 30.days, chain_id: access.chain_id
    )

    OauthToken.where(chain_id: access.chain_id).destroy_all
    expect(OauthToken.exists?(access.id)).to be(false)
    expect(OauthToken.exists?(refresh.id)).to be(false)

    get "https://www.example.com/"

    expect(response).to have_http_status(:ok)
    expect(WebSession.find_by_presented_token(session[:web_session_id])&.owner).to eq(email)
    expect(response.body).to include(email)
  end

  it "R-93PJ-FRPY expiring an MCP token chain leaves the web session intact" do
    establish_web_session(email: email)

    access, = OauthToken.issue(kind: "access", owner: owner, lifetime: 1.hour)
    OauthToken.issue(
      kind: "refresh", owner: owner, lifetime: 30.days, chain_id: access.chain_id
    )

    OauthToken.where(chain_id: access.chain_id).update_all(expires_at: 1.hour.ago)

    get "https://www.example.com/"

    expect(response).to have_http_status(:ok)
    expect(WebSession.find_by_presented_token(session[:web_session_id])&.owner).to eq(email)
    expect(response.body).to include(email)
  end
end
