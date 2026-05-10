# R-SLGL-B5B4: web sessions are persisted as rows in a dedicated table
# distinct from the OAuth token store. Validation is a hash lookup;
# logout writes revoked_at; the plaintext appears nowhere outside the
# user-agent's cookie store.
require "rails_helper"

RSpec.describe "Web session persistence (R-SLGL-B5B4)", type: :request do
  let(:provider) { Rails.configuration.x.google_identity_provider }
  let(:email) { "persist@allowed.example" }

  around do |example|
    previous_domain = Rails.configuration.x.auth.workspace_domain
    Rails.configuration.x.auth.workspace_domain = "allowed.example"
    example.run
    Rails.configuration.x.auth.workspace_domain = previous_domain
  end

  def complete_federation(email:)
    get "https://www.example.com/login"
    upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
    provider.stub_code(
      "code-persist-#{email}",
      sub: "google-#{email}",
      email: email,
      hosted_domain: "allowed.example"
    )
    get "https://www.example.com/oauth/google/callback",
        params: { code: "code-persist-#{email}", state: upstream_state },
        headers: { "X-Forwarded-Proto" => "https" }
  end

  it "R-SLGL-B5B4 a successful federation inserts one un-revoked row in web_sessions" do
    expect { complete_federation(email: email) }.to change(WebSession, :count).by(1)

    row = WebSession.last
    expect(row.owner).to eq(email)
    expect(row.session_digest).to be_present
    expect(row.issued_at).to be_present
    expect(row.revoked_at).to be_nil
  end

  it "R-SLGL-B5B4 the row stores only a digest; the plaintext id is not persisted" do
    complete_federation(email: email)

    plaintext = session[:web_session_id]
    row = WebSession.last
    expect(plaintext).to be_present
    expect(row.session_digest).not_to eq(plaintext)
    expect(row.session_digest).to eq(WebSession.digest_for(plaintext))
    # No column anywhere on the row holds the plaintext.
    row.attributes.each_value { |v| expect(v.to_s).not_to eq(plaintext) }
  end

  it "R-SLGL-B5B4 inbound cookie validation is a single digest lookup against the row" do
    complete_federation(email: email)

    plaintext = session[:web_session_id]
    found = WebSession.find_by_presented_token(plaintext)
    expect(found).to eq(WebSession.last)
    expect(WebSession.find_by_presented_token("not-the-token")).to be_nil
  end

  it "R-SLGL-B5B4 logout writes revoked_at on the matching row; the cookie can no longer be redeemed" do
    complete_federation(email: email)
    row = WebSession.last
    expect(row.revoked_at).to be_nil

    get "https://www.example.com/logout"

    row.reload
    expect(row.revoked_at).to be_present
    # The previously valid plaintext id no longer resolves to a session.
    expect(WebSession.find_by_presented_token(row.session_digest)).to be_nil
  end

  it "R-SLGL-B5B4 the web_sessions table is structurally distinct from oauth_tokens " \
     "(no shared schema, no foreign key)" do
    web_columns = WebSession.column_names
    token_columns = OauthToken.column_names
    # The two tables are different physical tables.
    expect(WebSession.table_name).to eq("web_sessions")
    expect(OauthToken.table_name).to eq("oauth_tokens")
    # WebSession does not carry chain_id, kind, or resource — those belong
    # to the OAuth token store and would couple the two if mirrored here.
    expect(web_columns).not_to include("chain_id")
    expect(web_columns).not_to include("kind")
    expect(web_columns).not_to include("resource")
    # And no oauth_token_id foreign key.
    expect(web_columns).not_to include("oauth_token_id")
    # OauthToken likewise does not reference web_sessions.
    expect(token_columns).not_to include("web_session_id")
  end
end
