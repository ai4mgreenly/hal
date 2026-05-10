# Request spec for the OAuth authorize endpoint (reqs/auth.md).
require "rails_helper"

RSpec.describe "OAuth Authorize", type: :request do
  describe "GET /oauth/authorize" do
    let(:registered_redirect) { "https://client.example.com/cb" }
    let!(:client_record) do
      record, _ = OauthClient.register(
        client_name: "Test Client",
        redirect_uris: [ registered_redirect ],
        token_endpoint_auth_method: "none"
      )
      record
    end

    it "R-4SH1-HQGP redirects the user to Google's authorization endpoint" do
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        redirect_uri: registered_redirect,
        response_type: "code",
        scope: "openid",
        state: "client-state",
        code_challenge: "abc",
        code_challenge_method: "S256"
      }

      expect(response).to have_http_status(:found)
      location = URI.parse(response.location)
      expect("#{location.scheme}://#{location.host}#{location.path}")
        .to eq("https://accounts.google.com/o/oauth2/v2/auth")

      params = URI.decode_www_form(location.query).to_h
      expect(params["response_type"]).to eq("code")
      expect(params["redirect_uri"]).to eq("http://www.example.com/oauth/google/callback")
      expect(params["state"]).to be_present
      expect(params["state"]).not_to eq("client-state")
      expect(params["scope"]).to include("openid")
      expect(params["client_id"]).to be_present
    end

    it "R-126C-AM1E omits forced-reauthentication parameters on the MCP redirect to Google" do
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        redirect_uri: registered_redirect,
        response_type: "code",
        scope: "openid",
        state: "client-state",
        code_challenge: "abc",
        code_challenge_method: "S256"
      }

      expect(response).to have_http_status(:found)
      params = URI.decode_www_form(URI.parse(response.location).query).to_h
      expect(params).not_to have_key("prompt")
      expect(params).not_to have_key("max_age")
      expect(params).not_to have_key("login_hint")
      expect(params).not_to have_key("approval_prompt")
    end

    it "R-4SH1-HQGP issues a fresh upstream state per authorize request" do
      get "/oauth/authorize", params: {
        client_id: client_record.client_id, redirect_uri: registered_redirect, state: "s1"
      }
      first = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]

      get "/oauth/authorize", params: {
        client_id: client_record.client_id, redirect_uri: registered_redirect, state: "s2"
      }
      second = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]

      expect(first).to be_present
      expect(second).to be_present
      expect(first).not_to eq(second)
    end

    it "R-1ERW-YD9G rejects an unknown client_id without redirecting" do
      get "/oauth/authorize", params: {
        client_id: "no-such-client",
        redirect_uri: registered_redirect,
        response_type: "code",
        state: "s"
      }

      expect(response).to have_http_status(:bad_request)
      expect(response.location).to be_nil
    end

    it "R-1ERW-YD9G rejects a redirect_uri that is not registered for the client" do
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        redirect_uri: "https://attacker.example.com/cb",
        response_type: "code",
        state: "s"
      }

      expect(response).to have_http_status(:bad_request)
      expect(response.location).to be_nil
    end

    it "R-1ERW-YD9G does not redirect the user-agent to the supplied (mismatched) redirect_uri" do
      mismatched = "https://attacker.example.com/cb"
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        redirect_uri: mismatched,
        response_type: "code",
        state: "s"
      }

      expect(response).to have_http_status(:bad_request)
      expect(response.body).not_to include(mismatched)
      expect(response.headers["Location"]).to be_nil
    end

    it "R-1ERW-YD9G enforces byte-for-byte match — trailing slash is not tolerated" do
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        redirect_uri: "#{registered_redirect}/",
        response_type: "code",
        state: "s"
      }

      expect(response).to have_http_status(:bad_request)
    end

    it "R-1ERW-YD9G enforces byte-for-byte match — case differences are not normalized" do
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        redirect_uri: registered_redirect.upcase,
        response_type: "code",
        state: "s"
      }

      expect(response).to have_http_status(:bad_request)
    end

    it "R-4GRA-EGBY rejects a non-canonical resource without redirecting and without recording state" do
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        redirect_uri: registered_redirect,
        response_type: "code",
        state: "s",
        code_challenge: "abc",
        code_challenge_method: "S256",
        resource: "https://wrong.example.org/"
      }

      expect(response).to have_http_status(:bad_request)
      expect(response.headers["Location"]).to be_nil
      expect(response.body).to include("invalid_target")
      # The offending value must not be echoed into any redirect target,
      # and no pending-authorization entry should have been written.
      expect(session[:pending_authorizations].to_h).to be_empty
    end

    it "R-4GRA-EGBY rejects a canonical-but-trailing-slash-mismatched resource" do
      mismatched = Rails.configuration.x.auth.canonical_url.chomp("/")
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        redirect_uri: registered_redirect,
        response_type: "code",
        state: "s",
        resource: mismatched
      }

      expect(response).to have_http_status(:bad_request)
      expect(response.headers["Location"]).to be_nil
      expect(response.body).to include("invalid_target")
    end

    it "R-4GRA-EGBY accepts an exact canonical resource and still redirects to Google" do
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        redirect_uri: registered_redirect,
        response_type: "code",
        state: "s",
        code_challenge: "abc",
        code_challenge_method: "S256",
        resource: Rails.configuration.x.auth.canonical_url
      }

      expect(response).to have_http_status(:found)
      expect(URI.parse(response.location).host).to eq("accounts.google.com")
    end

    it "R-4GRA-EGBY redirects to Google when no resource parameter is supplied (the parameter is optional)" do
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        redirect_uri: registered_redirect,
        response_type: "code",
        state: "s",
        code_challenge: "abc",
        code_challenge_method: "S256"
      }

      expect(response).to have_http_status(:found)
      expect(URI.parse(response.location).host).to eq("accounts.google.com")
    end

    it "R-1ERW-YD9G rejects a request with no redirect_uri at all" do
      get "/oauth/authorize", params: {
        client_id: client_record.client_id,
        response_type: "code",
        state: "s"
      }

      expect(response).to have_http_status(:bad_request)
    end
  end
end
