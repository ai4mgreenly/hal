require "rails_helper"

RSpec.describe GoogleIdentityProvider do
  describe "R-CL63-P202 R-DBZW-40BC the test double stands in for Google's OAuth endpoints" do
    let(:provider) { Rails.configuration.x.google_identity_provider }

    it "R-CL63-P202 wires Rails.configuration.x.google_identity_provider to the in-memory fake under test" do
      expect(provider).to be_a(GoogleIdentityProvider::Fake)
      expect(provider).to be_a(GoogleIdentityProvider)
    end

    it "R-CL63-P202 builds an authorization URL on the documented Google authorization endpoint" do
      url = provider.authorization_url(
        state: "abc123",
        redirect_uri: "https://example.test/oauth/callback"
      )
      uri = URI.parse(url)
      expect("#{uri.scheme}://#{uri.host}#{uri.path}").to eq("https://accounts.google.com/o/oauth2/v2/auth")
      params = URI.decode_www_form(uri.query).to_h
      expect(params).to include(
        "response_type" => "code",
        "redirect_uri" => "https://example.test/oauth/callback",
        "state" => "abc123"
      )
      expect(params["scope"]).to include("openid")
      expect(params["client_id"]).to be_present
    end

    it "R-CL63-P202 exchanges a stubbed code for a Google-shaped token response with hd-bearing id_token claims" do
      provider.stub_code(
        "code-xyz",
        sub: "google-sub-1",
        email: "alice@workspace.example",
        hosted_domain: "workspace.example"
      )
      response = provider.exchange_code(
        code: "code-xyz",
        redirect_uri: "https://example.test/oauth/callback"
      )
      expect(response).to include(
        "token_type" => "Bearer",
        "scope" => a_string_including("openid")
      )
      expect(response["access_token"]).to be_present
      expect(response["refresh_token"]).to be_present
      expect(response["expires_in"]).to be > 0
      expect(response["id_token"]).to match(/\A[^.]+\.[^.]+\.[^.]*\z/)
      expect(response["id_token_claims"]).to include(
        "iss" => "https://accounts.google.com",
        "sub" => "google-sub-1",
        "email" => "alice@workspace.example",
        "hd" => "workspace.example",
        "email_verified" => true
      )
    end

    it "R-DBZW-40BC defines a seam class with #authorization_url / #exchange_code matching real Google" do
      expect(GoogleIdentityProvider.instance_method(:authorization_url).parameters).to match_array([
        [ :keyreq, :state ], [ :keyreq, :redirect_uri ], [ :key, :scope ]
      ])
      expect(GoogleIdentityProvider.instance_method(:exchange_code).parameters).to match_array([
        [ :keyreq, :code ], [ :keyreq, :redirect_uri ]
      ])
      expect { GoogleIdentityProvider.new.authorization_url(state: "s", redirect_uri: "u") }
        .to raise_error(NotImplementedError, /R-DBZW-40BC/)
      expect { GoogleIdentityProvider.new.exchange_code(code: "c", redirect_uri: "u") }
        .to raise_error(NotImplementedError, /R-DBZW-40BC/)
    end
  end
end
