# R-T0B2-A4E5: the two implementations of the Google identity-provider
# seam return values of identical shape; callers depend only on
# `#exchange_code` returning an Identity carrying sub/email/hosted_domain/
# email_verified, and they do not branch on which implementation is wired.
require "rails_helper"

RSpec.describe "Google identity provider seam return shape" do
  describe "R-T0B2-A4E5 #exchange_code returns an Identity from both implementations" do
    let(:redirect_uri) { "https://service.example.com/oauth/google/callback" }

    it "R-T0B2-A4E5 the test double returns an Identity with the four required claim fields" do
      fake = GoogleIdentityProvider::Fake.new
      fake.stub_code(
        "seam-code",
        sub: "double-sub",
        email: "user@workspace.example",
        hosted_domain: "workspace.example",
        email_verified: true
      )

      identity = fake.exchange_code(code: "seam-code", redirect_uri: redirect_uri)

      expect(identity).to be_a(GoogleIdentityProvider::Identity)
      expect(identity.sub).to eq("double-sub")
      expect(identity.email).to eq("user@workspace.example")
      expect(identity.hosted_domain).to eq("workspace.example")
      expect(identity.email_verified).to eq(true)
    end

    it "R-T0B2-A4E5 the real provider returns an Identity with the four required claim fields" do
      claims = {
        "sub" => "real-sub",
        "email" => "user@workspace.example",
        "email_verified" => true,
        "hd" => "workspace.example"
      }
      header_seg = Base64.urlsafe_encode64({ alg: "RS256", typ: "JWT" }.to_json, padding: false)
      payload_seg = Base64.urlsafe_encode64(claims.to_json, padding: false)
      id_token = "#{header_seg}.#{payload_seg}.signature"
      body = { "id_token" => id_token, "token_type" => "Bearer" }.to_json
      fake_response = instance_double(Net::HTTPResponse, body: body)
      allow(Net::HTTP).to receive(:post_form).and_return(fake_response)

      original_id = ENV["GOOGLE_CLIENT_ID"]
      original_secret = ENV["GOOGLE_CLIENT_SECRET"]
      ENV["GOOGLE_CLIENT_ID"] = "real.apps.googleusercontent.com"
      ENV["GOOGLE_CLIENT_SECRET"] = "real-secret"
      begin
        identity = GoogleIdentityProvider.new.exchange_code(code: "auth-code", redirect_uri: redirect_uri)
        expect(identity).to be_a(GoogleIdentityProvider::Identity)
        expect(identity.sub).to eq("real-sub")
        expect(identity.email).to eq("user@workspace.example")
        expect(identity.hosted_domain).to eq("workspace.example")
        expect(identity.email_verified).to eq(true)
      ensure
        ENV["GOOGLE_CLIENT_ID"] = original_id
        ENV["GOOGLE_CLIENT_SECRET"] = original_secret
      end
    end

    it "R-T0B2-A4E5 the Identity exposes only the four seam fields (callers cannot depend on extras)" do
      expect(GoogleIdentityProvider::Identity.members).to contain_exactly(
        :sub, :email, :hosted_domain, :email_verified
      )
    end
  end
end
