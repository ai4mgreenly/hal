# R-CXJ2-R3BN: the only code path that establishes a web session is the
# successful completion of the Google federation round-trip. No other
# request — anonymous browsing, /login itself, MCP token issuance,
# /logout — may insert a WebSession row.
require "rails_helper"
require "base64"

RSpec.describe "Web session is only established via Google federation (R-CXJ2-R3BN)", type: :request do
  let(:provider) { Rails.configuration.x.google_identity_provider }

  around do |example|
    previous_domain = Rails.configuration.x.auth.workspace_domain
    Rails.configuration.x.auth.workspace_domain = "allowed.example"
    example.run
    Rails.configuration.x.auth.workspace_domain = previous_domain
  end

  def s256(verifier)
    Base64.urlsafe_encode64(Digest::SHA256.digest(verifier), padding: false)
  end

  it "R-CXJ2-R3BN GET / while logged out does not insert a WebSession row" do
    expect {
      get "https://www.example.com/"
    }.not_to change(WebSession, :count)
  end

  it "R-CXJ2-R3BN GET /login redirects to Google and inserts no WebSession row" do
    expect {
      get "https://www.example.com/login"
    }.not_to change(WebSession, :count)

    expect(response).to have_http_status(:found)
    expect(URI.parse(response.location).host).to eq("accounts.google.com")
    # The pre-federation handshake state lives in the encrypted cookie session,
    # not in the WebSession table.
    expect(session[:pending_web_logins]).to be_present
  end

  it "R-CXJ2-R3BN a callback whose hosted-domain fails the workspace check inserts no row" do
    get "https://www.example.com/login"
    upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
    provider.stub_code(
      "code-cxj2-bad-domain",
      sub: "google-cxj2-1",
      email: "intruder@other.example",
      hosted_domain: "other.example"
    )

    expect {
      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-cxj2-bad-domain", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }
    }.not_to change(WebSession, :count)

    expect(response).to have_http_status(:forbidden)
  end

  it "R-CXJ2-R3BN successful Google callback is the one path that inserts a row" do
    get "https://www.example.com/login"
    upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
    provider.stub_code(
      "code-cxj2-ok",
      sub: "google-cxj2-2",
      email: "user@allowed.example",
      hosted_domain: "allowed.example"
    )

    expect {
      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-cxj2-ok", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }
    }.to change(WebSession, :count).by(1)

    expect(WebSession.find_by_presented_token(session[:web_session_id])&.owner).to eq("user@allowed.example")
  end

  it "R-CXJ2-R3BN /oauth/token issuance does not establish a web session" do
    verifier = SecureRandom.urlsafe_base64(64)
    challenge = s256(verifier)
    client, _ = OauthClient.register(
      client_name: "CXJ2 Token Client",
      redirect_uris: [ "https://client.example.com/cb" ],
      token_endpoint_auth_method: "none"
    )
    _, code_plain = OauthAuthorizationCode.issue(
      client_id: client.client_id,
      redirect_uri: "https://client.example.com/cb",
      code_challenge: challenge,
      code_challenge_method: "S256",
      owner: "user@allowed.example"
    )

    expect {
      post "/oauth/token", params: {
        grant_type: "authorization_code",
        code: code_plain,
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_verifier: verifier
      }
    }.not_to change(WebSession, :count)

    expect(response).to have_http_status(:ok)
    expect(JSON.parse(response.body)["access_token"]).to be_present
  end

  it "R-CXJ2-R3BN MCP requests presenting an access token do not establish a web session" do
    record, plaintext = OauthToken.issue(kind: "access", owner: "user@allowed.example", lifetime: 1.hour)
    expect(record).to be_present

    expect {
      post "/mcp",
           params: { jsonrpc: "2.0", id: 1, method: "initialize", params: {} }.to_json,
           headers: {
             "Authorization" => "Bearer #{plaintext}",
             "Content-Type" => "application/json",
             "Accept" => "application/json, text/event-stream"
           }
    }.not_to change(WebSession, :count)
  end

  it "R-CXJ2-R3BN GET /logout does not insert a row (it only revokes)" do
    # Establish a session so logout has something to revoke.
    get "https://www.example.com/login"
    upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
    provider.stub_code(
      "code-cxj2-logout",
      sub: "google-cxj2-3",
      email: "logout@allowed.example",
      hosted_domain: "allowed.example"
    )
    get "https://www.example.com/oauth/google/callback",
        params: { code: "code-cxj2-logout", state: upstream_state },
        headers: { "X-Forwarded-Proto" => "https" }
    expect(WebSession.count).to eq(1)

    expect {
      delete "https://www.example.com/logout"
    }.not_to change(WebSession, :count)
  end
end
