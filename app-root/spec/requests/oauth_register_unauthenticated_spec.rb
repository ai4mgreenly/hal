# Posture spec for R-25DN-9PUR — DCR endpoint accepts registration
# requests from anyone, unauthenticated, by design. No initial access
# token, admin allowlist, or any other gating. The credentials it
# returns confer no access to the counter on their own.
require "rails_helper"

RSpec.describe "DCR is unauthenticated", type: :request do
  describe "R-25DN-9PUR POST /oauth/register" do
    it "R-25DN-9PUR succeeds with no Authorization header" do
      post "/oauth/register",
        params: { redirect_uris: [ "https://client.example.com/cb" ] }.to_json,
        headers: { "Content-Type" => "application/json" }

      expect(response).to have_http_status(:created)
      expect(request.headers["Authorization"]).to be_nil
    end

    it "R-25DN-9PUR ignores any Authorization header presented (no gating)" do
      post "/oauth/register",
        params: { redirect_uris: [ "https://client.example.com/cb" ] }.to_json,
        headers: {
          "Content-Type" => "application/json",
          "Authorization" => "Bearer obviously-not-a-real-token"
        }

      expect(response).to have_http_status(:created)
    end

    it "R-25DN-9PUR controller has no auth before_action" do
      callbacks = OauthRegisterController._process_action_callbacks.select { |cb| cb.kind == :before }
      auth_callbacks = callbacks.select do |cb|
        name = cb.filter.to_s
        name.match?(/auth|token|authenticate|require_/i)
      end
      expect(auth_callbacks).to be_empty
    end

    it "R-25DN-9PUR the returned client_id confers no access to the counter" do
      post "/oauth/register",
        params: { redirect_uris: [ "https://client.example.com/cb" ] }.to_json,
        headers: { "Content-Type" => "application/json" }
      client_id = JSON.parse(response.body)["client_id"]

      post "/counter/increment", headers: { "Authorization" => "Bearer #{client_id}" }
      expect(response).to have_http_status(:unauthorized)
    end

    it "R-25DN-9PUR the returned client_secret confers no access to the counter" do
      post "/oauth/register",
        params: {
          redirect_uris: [ "https://confidential.example.com/cb" ],
          token_endpoint_auth_method: "client_secret_basic"
        }.to_json,
        headers: { "Content-Type" => "application/json" }
      secret = JSON.parse(response.body)["client_secret"]
      expect(secret).to be_present

      post "/counter/increment", headers: { "Authorization" => "Bearer #{secret}" }
      expect(response).to have_http_status(:unauthorized)
    end
  end
end
