# Request specs for Dynamic Client Registration (RFC 7591).
require "rails_helper"

RSpec.describe "OAuth Dynamic Client Registration", type: :request do
  describe "POST /oauth/register" do
    it "R-3JCR-C810 self-registers a public client and returns a client_id without a secret" do
      post "/oauth/register",
        params: { redirect_uris: [ "https://client.example.com/cb" ], client_name: "MCP CLI" }.to_json,
        headers: { "Content-Type" => "application/json" }

      expect(response).to have_http_status(:created)
      expect(response.media_type).to eq("application/json")
      body = JSON.parse(response.body)

      expect(body["client_id"]).to be_a(String)
      expect(body["client_id"]).not_to be_empty
      expect(body["redirect_uris"]).to eq([ "https://client.example.com/cb" ])
      expect(body["client_name"]).to eq("MCP CLI")
      expect(body["token_endpoint_auth_method"]).to eq("none")
      expect(body).not_to have_key("client_secret")
      expect(body["grant_types"]).to include("authorization_code", "refresh_token")
      expect(body["response_types"]).to include("code")

      record = OauthClient.find_by(client_id: body["client_id"])
      expect(record).to be_present
      expect(record.client_secret_digest).to be_nil
    end

    it "R-3JCR-C810 issues a client_secret for confidential clients and stores only its digest" do
      post "/oauth/register",
        params: {
          redirect_uris: [ "https://confidential.example.com/cb" ],
          token_endpoint_auth_method: "client_secret_basic"
        }.to_json,
        headers: { "Content-Type" => "application/json" }

      expect(response).to have_http_status(:created)
      body = JSON.parse(response.body)
      secret = body["client_secret"]
      expect(secret).to be_a(String)
      expect(secret.length).to be >= 32

      record = OauthClient.find_by(client_id: body["client_id"])
      expect(record.client_secret_digest).to eq(Digest::SHA256.hexdigest(secret))
      expect(record.attributes.values.map(&:to_s)).not_to include(secret)
    end

    it "R-3JCR-C810 rejects a registration with no redirect_uris" do
      post "/oauth/register",
        params: { client_name: "no uris" }.to_json,
        headers: { "Content-Type" => "application/json" }

      expect(response).to have_http_status(:bad_request)
      expect(JSON.parse(response.body)["error"]).to eq("invalid_client_metadata")
    end

    it "R-3JCR-C810 mints a fresh client_id per registration" do
      post "/oauth/register",
        params: { redirect_uris: [ "https://a.example.com/cb" ] }.to_json,
        headers: { "Content-Type" => "application/json" }
      first_id = JSON.parse(response.body)["client_id"]

      post "/oauth/register",
        params: { redirect_uris: [ "https://b.example.com/cb" ] }.to_json,
        headers: { "Content-Type" => "application/json" }
      second_id = JSON.parse(response.body)["client_id"]

      expect(first_id).not_to eq(second_id)
    end
  end
end
