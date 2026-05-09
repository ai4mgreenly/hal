# Request specs for the OAuth 2.1 authorization-server metadata document.
require "rails_helper"

RSpec.describe "OAuth authorization-server metadata", type: :request do
  describe "GET /.well-known/oauth-authorization-server" do
    it "R-2XEK-GCOI publishes a JSON metadata document with the discovery fields conformant clients require" do
      get "/.well-known/oauth-authorization-server"

      expect(response).to have_http_status(:ok)
      expect(response.media_type).to eq("application/json")
      body = JSON.parse(response.body)
      expect(body).to be_a(Hash)

      expect(body["issuer"]).to be_a(String)
      expect(body["issuer"]).not_to be_empty

      %w[authorization_endpoint token_endpoint registration_endpoint].each do |key|
        expect(body[key]).to be_a(String), "expected #{key} to be a string URL"
        expect(body[key]).to start_with(body["issuer"]),
          "expected #{key} (#{body[key]}) to share origin with issuer (#{body['issuer']})"
      end

      expect(body["response_types_supported"]).to include("code")
      expect(body["grant_types_supported"]).to include("authorization_code", "refresh_token")
      expect(body["code_challenge_methods_supported"]).to include("S256")
    end

    it "R-1KML-5J0Q advertises endpoints on the same origin as the issuer" do
      get "/.well-known/oauth-authorization-server"

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)

      expected_origin = "#{request.protocol}#{request.host_with_port}"
      expect(body["issuer"]).to eq(expected_origin)

      origin_of = ->(url) {
        u = URI.parse(url)
        "#{u.scheme}://#{u.host}#{u.port ? ":#{u.port}" : ''}"
      }
      issuer_origin = origin_of.call(body["issuer"])

      %w[authorization_endpoint token_endpoint registration_endpoint].each do |key|
        url = body[key]
        expect(url).to be_a(String), "expected #{key} to be a string URL"
        expect(origin_of.call(url)).to eq(issuer_origin),
          "expected #{key} (#{url}) to share origin with issuer (#{body['issuer']})"
      end
    end

    it "R-42V5-GJW4 advertises Authorization Code + PKCE only; no implicit or password grants" do
      get "/.well-known/oauth-authorization-server"

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)

      # Authorization Code flow with PKCE is the supported flow.
      expect(body["response_types_supported"]).to include("code")
      expect(body["code_challenge_methods_supported"]).to include("S256")
      expect(body["grant_types_supported"]).to include("authorization_code")

      # Implicit flow and password grants are explicitly unsupported.
      expect(body["response_types_supported"]).not_to include("token")
      expect(body["response_types_supported"]).not_to include("id_token")
      expect(body["grant_types_supported"]).not_to include("password")
      expect(body["grant_types_supported"]).not_to include("implicit")
    end
  end
end
