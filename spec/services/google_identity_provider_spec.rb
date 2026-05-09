require "rails_helper"

RSpec.describe GoogleIdentityProvider do
  describe "R-VF61-2Y6I the test-env provider is the double and exercises Google-shaped payloads" do
    let(:provider) { Rails.configuration.x.google_identity_provider }

    it "R-VF61-2Y6I wires Rails.configuration.x.google_identity_provider to the in-memory fake in the test env" do
      expect(Rails.env).to eq("test")
      expect(provider).to be_a(GoogleIdentityProvider::Fake)
      expect(provider).to be_a(GoogleIdentityProvider)
    end

    it "R-VF61-2Y6I the double builds an authorization URL on the documented Google authorization endpoint" do
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

    it "R-VF61-2Y6I the double exchanges a stubbed code for an Identity drawn from Google-shaped OIDC claims" do
      provider.stub_code(
        "code-xyz",
        sub: "google-sub-1",
        email: "alice@workspace.example",
        hosted_domain: "workspace.example"
      )
      identity = provider.exchange_code(
        code: "code-xyz",
        redirect_uri: "https://example.test/oauth/callback"
      )
      expect(identity).to be_a(GoogleIdentityProvider::Identity)
      expect(identity.sub).to eq("google-sub-1")
      expect(identity.email).to eq("alice@workspace.example")
      expect(identity.hosted_domain).to eq("workspace.example")
      expect(identity.email_verified).to eq(true)
    end
  end

  describe "R-W3K0-QD0E real GoogleIdentityProvider builds OAuth URL and exchanges code without sentinel" do
    let(:provider) { GoogleIdentityProvider.new }
    let(:redirect_uri) { "https://service.example.com/oauth/google/callback" }

    around do |example|
      original_client_id = ENV["GOOGLE_CLIENT_ID"]
      original_client_secret = ENV["GOOGLE_CLIENT_SECRET"]
      original_domain = Rails.configuration.x.google_workspace_domain
      ENV["GOOGLE_CLIENT_ID"] = "real-client-id.apps.googleusercontent.com"
      ENV["GOOGLE_CLIENT_SECRET"] = "real-client-secret"
      Rails.configuration.x.google_workspace_domain = "workspace.example"
      example.run
    ensure
      ENV["GOOGLE_CLIENT_ID"] = original_client_id
      ENV["GOOGLE_CLIENT_SECRET"] = original_client_secret
      Rails.configuration.x.google_workspace_domain = original_domain
    end

    it "R-W3K0-QD0E #authorization_url builds a URL on Google's documented authorization endpoint" do
      url = provider.authorization_url(state: "state-xyz", redirect_uri: redirect_uri)
      uri = URI.parse(url)
      expect("#{uri.scheme}://#{uri.host}#{uri.path}").to eq("https://accounts.google.com/o/oauth2/v2/auth")
      params = URI.decode_www_form(uri.query).to_h
      expect(params).to include(
        "client_id" => "real-client-id.apps.googleusercontent.com",
        "redirect_uri" => redirect_uri,
        "response_type" => "code",
        "scope" => "openid email profile",
        "state" => "state-xyz",
        "hd" => "workspace.example"
      )
    end

    it "R-W3K0-QD0E #authorization_url does not raise NotImplementedError" do
      expect {
        provider.authorization_url(state: "s", redirect_uri: redirect_uri)
      }.not_to raise_error
    end

    it "R-W3K0-QD0E #exchange_code POSTs to the token endpoint and returns an Identity from the ID-token claims" do
      claims = {
        "sub" => "115525393412345678901",
        "email" => "bob@workspace.example",
        "email_verified" => true,
        "hd" => "workspace.example"
      }
      header_seg = Base64.urlsafe_encode64({ alg: "RS256", typ: "JWT" }.to_json, padding: false)
      payload_seg = Base64.urlsafe_encode64(claims.to_json, padding: false)
      id_token = "#{header_seg}.#{payload_seg}.signature"
      token_response_body = {
        "access_token" => "ya29.fake",
        "expires_in" => 3599,
        "scope" => "openid email profile",
        "token_type" => "Bearer",
        "id_token" => id_token
      }.to_json

      received = {}
      fake_response = instance_double(Net::HTTPResponse, body: token_response_body)
      allow(Net::HTTP).to receive(:post_form) do |uri, form|
        received[:uri] = uri
        received[:form] = form
        fake_response
      end

      identity = provider.exchange_code(code: "auth-code-abc", redirect_uri: redirect_uri)

      expect(received[:uri].to_s).to eq("https://oauth2.googleapis.com/token")
      expect(received[:form]).to include(
        "grant_type" => "authorization_code",
        "code" => "auth-code-abc",
        "redirect_uri" => redirect_uri,
        "client_id" => "real-client-id.apps.googleusercontent.com",
        "client_secret" => "real-client-secret"
      )
      expect(identity).to be_a(GoogleIdentityProvider::Identity)
      expect(identity.sub).to eq("115525393412345678901")
      expect(identity.email).to eq("bob@workspace.example")
      expect(identity.hosted_domain).to eq("workspace.example")
      expect(identity.email_verified).to eq(true)
    end
  end
end
