# R-7GT3-PM1K: access tokens have a finite lifetime; the service issues
# refresh tokens so well-behaved clients can stay logged in without
# re-prompting on every expiry.
require "rails_helper"
require "base64"

RSpec.describe "OAuth Token Endpoint", type: :request do
  def s256(verifier)
    Base64.urlsafe_encode64(Digest::SHA256.digest(verifier), padding: false)
  end

  describe "POST /oauth/token (authorization_code grant)" do
    it "R-7GT3-PM1K issues a finite-lifetime access token alongside a refresh token" do
      verifier = SecureRandom.urlsafe_base64(64)
      challenge = s256(verifier)
      client, _ = OauthClient.register(
        client_name: "Token Test Client",
        redirect_uris: [ "https://client.example.com/cb" ],
        token_endpoint_auth_method: "none"
      )
      _, code_plain = OauthAuthorizationCode.issue(
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_challenge: challenge,
        code_challenge_method: "S256",
        owner: "google-sub-token"
      )

      post "/oauth/token", params: {
        grant_type: "authorization_code",
        code: code_plain,
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_verifier: verifier
      }

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      expect(body["token_type"]).to eq("Bearer")
      expect(body["access_token"]).to be_present
      expect(body["refresh_token"]).to be_present
      expect(body["expires_in"]).to be_a(Integer).and(be > 0)

      access = OauthToken.find_by_presented_token(body["access_token"])
      refresh = OauthToken.find_by_presented_token(body["refresh_token"])
      expect(access.kind).to eq("access")
      expect(refresh.kind).to eq("refresh")
      expect(access.chain_id).to eq(refresh.chain_id)
      expect(access.owner).to eq("google-sub-token")
      expect(access.expires_at).to be > access.issued_at
      expect(refresh.expires_at - refresh.issued_at).to be > (access.expires_at - access.issued_at)
    end

    it "R-TNXJ-ZWQ0 issues an access token that expires one hour after issue" do
      verifier = SecureRandom.urlsafe_base64(64)
      client, _ = OauthClient.register(
        client_name: "Token Lifetime Access Client",
        redirect_uris: [ "https://client.example.com/cb" ],
        token_endpoint_auth_method: "none"
      )
      _, code_plain = OauthAuthorizationCode.issue(
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_challenge: s256(verifier),
        code_challenge_method: "S256",
        owner: "google-sub-access-life"
      )

      post "/oauth/token", params: {
        grant_type: "authorization_code",
        code: code_plain,
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_verifier: verifier
      }

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      expect(body["expires_in"]).to eq(3600)
      access = OauthToken.find_by_presented_token(body["access_token"])
      expect(access.expires_at - access.issued_at).to be_within(1.second).of(1.hour)
    end

    it "R-8UAA-YKR9 issues a refresh token that expires thirty days after its own issue" do
      verifier = SecureRandom.urlsafe_base64(64)
      client, _ = OauthClient.register(
        client_name: "Token Lifetime Refresh Client",
        redirect_uris: [ "https://client.example.com/cb" ],
        token_endpoint_auth_method: "none"
      )
      _, code_plain = OauthAuthorizationCode.issue(
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_challenge: s256(verifier),
        code_challenge_method: "S256",
        owner: "google-sub-refresh-life"
      )

      post "/oauth/token", params: {
        grant_type: "authorization_code",
        code: code_plain,
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_verifier: verifier
      }

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      refresh = OauthToken.find_by_presented_token(body["refresh_token"])
      expect(refresh.expires_at - refresh.issued_at).to be_within(1.second).of(30.days)
    end

    it "R-7GT3-PM1K rejects a code whose PKCE verifier does not hash to the bound challenge" do
      verifier = SecureRandom.urlsafe_base64(64)
      client, _ = OauthClient.register(
        client_name: "Token Test Client B-PKCE-FAIL",
        redirect_uris: [ "https://client.example.com/cb" ],
        token_endpoint_auth_method: "none"
      )
      _, code_plain = OauthAuthorizationCode.issue(
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_challenge: s256(verifier),
        code_challenge_method: "S256",
        owner: "google-sub-bad-pkce-pre"
      )

      post "/oauth/token", params: {
        grant_type: "authorization_code",
        code: code_plain,
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_verifier: "wrong-verifier"
      }

      expect(response).to have_http_status(:bad_request)
      expect(JSON.parse(response.body)["error"]).to eq("invalid_grant")
    end
  end

  describe "POST /oauth/token (refresh_token grant)" do
    def issue_chain(owner)
      verifier = SecureRandom.urlsafe_base64(64)
      client, _ = OauthClient.register(
        client_name: "Refresh Test Client #{owner}",
        redirect_uris: [ "https://client.example.com/cb" ],
        token_endpoint_auth_method: "none"
      )
      _, code_plain = OauthAuthorizationCode.issue(
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_challenge: s256(verifier),
        code_challenge_method: "S256",
        owner: owner
      )
      post "/oauth/token", params: {
        grant_type: "authorization_code",
        code: code_plain,
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_verifier: verifier
      }
      expect(response).to have_http_status(:ok)
      JSON.parse(response.body)
    end

    it "R-89K0-GH5G rotates the refresh token: issues a new pair and invalidates the presented refresh" do
      first = issue_chain("google-sub-rotate")
      old_refresh_row = OauthToken.find_by_presented_token(first["refresh_token"])

      post "/oauth/token", params: {
        grant_type: "refresh_token",
        refresh_token: first["refresh_token"]
      }

      expect(response).to have_http_status(:ok)
      second = JSON.parse(response.body)
      expect(second["access_token"]).to be_present
      expect(second["refresh_token"]).to be_present
      expect(second["refresh_token"]).not_to eq(first["refresh_token"])
      expect(second["access_token"]).not_to eq(first["access_token"])

      old_refresh_row.reload
      expect(old_refresh_row.used_at).to be_present

      new_access = OauthToken.find_by_presented_token(second["access_token"])
      new_refresh = OauthToken.find_by_presented_token(second["refresh_token"])
      expect(new_access.kind).to eq("access")
      expect(new_refresh.kind).to eq("refresh")
      expect(new_access.chain_id).to eq(old_refresh_row.chain_id)
      expect(new_refresh.chain_id).to eq(old_refresh_row.chain_id)
    end

    it "R-9HGE-87UG rejects a replayed refresh token and revokes the entire chain" do
      first = issue_chain("google-sub-replay")
      original_refresh = first["refresh_token"]
      original_access = first["access_token"]

      post "/oauth/token", params: { grant_type: "refresh_token", refresh_token: original_refresh }
      expect(response).to have_http_status(:ok)
      rotated = JSON.parse(response.body)

      post "/oauth/token", params: { grant_type: "refresh_token", refresh_token: original_refresh }
      expect(response).to have_http_status(:bad_request)
      expect(JSON.parse(response.body)["error"]).to eq("invalid_grant")

      chain_id = OauthToken.find_by_presented_token(rotated["access_token"]).chain_id
      OauthToken.where(chain_id: chain_id).find_each do |t|
        expect(t.revoked_at).to be_present
      end
      OauthAuthorizationCode.where(chain_id: chain_id).find_each do |c|
        expect(c.revoked_at).to be_present
      end

      post "/oauth/token", params: { grant_type: "refresh_token", refresh_token: rotated["refresh_token"] }
      expect(response).to have_http_status(:bad_request)
      expect(JSON.parse(response.body)["error"]).to eq("invalid_grant")

      _ = original_access
    end
  end
end
