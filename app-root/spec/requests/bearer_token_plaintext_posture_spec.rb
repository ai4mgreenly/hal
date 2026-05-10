# R-SAK8-WB9W posture: bearer tokens are accepted only in the
# `Authorization: Bearer` request header — never via query strings or
# path segments — and any logging that captures `Authorization` values
# or token-named parameters has them redacted by the parameter filter.
require "rails_helper"

RSpec.describe "Bearer token plaintext posture", type: :request do
  describe "R-SAK8-WB9W tokens are not accepted in URL query strings" do
    let(:valid_access_token) do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour
      )
      plaintext
    end

    it "R-SAK8-WB9W rejects a valid token presented as ?access_token=... query parameter" do
      Counter.current.update!(value: 0)

      post "/counter/increment?access_token=#{valid_access_token}"

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(0)
    end

    it "R-SAK8-WB9W rejects a valid token presented as ?bearer=... query parameter" do
      Counter.current.update!(value: 0)

      post "/counter/increment?bearer=#{valid_access_token}"

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(0)
    end

    it "R-SAK8-WB9W rejects a valid token presented in the request body" do
      Counter.current.update!(value: 0)

      post "/counter/increment",
           params: { access_token: valid_access_token }

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(0)
    end
  end

  describe "R-SAK8-WB9W parameter filter redacts token-bearing names" do
    let(:filter) { ActiveSupport::ParameterFilter.new(Rails.application.config.filter_parameters) }

    it "R-SAK8-WB9W filters parameters whose name contains 'token'" do
      filtered = filter.filter("access_token" => "secret-plaintext-1", "refresh_token" => "secret-plaintext-2")
      expect(filtered["access_token"]).to eq("[FILTERED]")
      expect(filtered["refresh_token"]).to eq("[FILTERED]")
    end

    it "R-SAK8-WB9W filters parameters whose name contains 'authorization'" do
      filtered = filter.filter("Authorization" => "Bearer secret-plaintext")
      expect(filtered["Authorization"]).to eq("[FILTERED]")
    end
  end

  describe "R-SAK8-WB9W bearer reader only consults the Authorization header" do
    it "R-SAK8-WB9W returns nil when Authorization header absent, regardless of token-named query params" do
      controller = CounterController.new
      controller.request = ActionDispatch::Request.new(
        "QUERY_STRING" => "access_token=plaintext-secret&bearer=plaintext-secret",
        "rack.input" => StringIO.new("")
      )

      expect(controller.send(:bearer_token_from_header)).to be_nil
    end

    it "R-SAK8-WB9W returns the value only from a `Bearer ` Authorization header" do
      controller = CounterController.new
      controller.request = ActionDispatch::Request.new(
        "HTTP_AUTHORIZATION" => "Bearer header-token-value",
        "rack.input" => StringIO.new("")
      )

      expect(controller.send(:bearer_token_from_header)).to eq("header-token-value")
    end
  end
end
