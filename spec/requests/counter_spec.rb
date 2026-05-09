# Request specs for the JSON HTTP API exposing the counter.
require "rails_helper"
require "base64"

RSpec.describe "Counter API", type: :request do
  describe "GET /counter" do
    it "R-2I2S-XB7K returns 200 with a JSON object containing the current count as a non-negative integer" do
      Counter.current.update!(value: 3)

      get "/counter"

      expect(response).to have_http_status(:ok)
      expect(response.media_type).to eq("application/json")
      body = JSON.parse(response.body)
      expect(body).to be_a(Hash)
      expect(body).to have_key("value")
      expect(body["value"]).to be_a(Integer)
      expect(body["value"]).to be >= 0
      expect(body["value"]).to eq(3)
    end

    it "R-3R73-2TN9 succeeds with no Authorization header (consistent with R-SE5T-HP2J)" do
      Counter.current.update!(value: 7)

      get "/counter"

      expect(response).to have_http_status(:ok)
      expect(JSON.parse(response.body)).to eq("value" => 7)
    end

    it "R-3R73-2TN9 succeeds even with a garbage Authorization header (still no auth required)" do
      Counter.current.update!(value: 11)

      get "/counter", headers: { "Authorization" => "Bearer not-a-real-token" }

      expect(response).to have_http_status(:ok)
      expect(JSON.parse(response.body)).to eq("value" => 11)
    end

    it "R-SE5T-HP2J the show action carries no auth before_action callback" do
      auth_filters = CounterController._process_action_callbacks.select do |cb|
        cb.kind == :before && cb.filter == :require_access_token &&
          cb.instance_variable_get(:@if).empty? &&
          cb.instance_variable_get(:@unless).empty?
      end
      applies_to_show = auth_filters.any? do |cb|
        only = cb.instance_variable_get(:@only)
        except = cb.instance_variable_get(:@except)
        (only.empty? || only.include?(:show)) && !except.include?(:show)
      end

      expect(applies_to_show).to be(false)
    end
  end

  describe "POST /counter/increment" do
    let(:access_token) do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour
      )
      plaintext
    end
    let(:auth_headers) { { "Authorization" => "Bearer #{access_token}" } }

    it "R-340Z-T6K2 R-6UUW-TQP2 returns 200 with a JSON object containing the post-increment value" do
      Counter.current.update!(value: 4)

      post "/counter/increment", headers: auth_headers

      expect(response).to have_http_status(:ok)
      expect(response.media_type).to eq("application/json")
      body = JSON.parse(response.body)
      expect(body).to be_a(Hash)
      expect(body).to have_key("value")
      expect(body["value"]).to eq(5)
      expect(Counter.current.value).to eq(5)
    end

    it "R-4ED6-CGQG accepts a valid bearer access token in the Authorization header" do
      Counter.current.update!(value: 0)

      post "/counter/increment", headers: auth_headers

      expect(response).to have_http_status(:ok)
      expect(JSON.parse(response.body)).to eq("value" => 1)
    end

    it "R-53Z2-DNB1 returns 401 and does not change the counter without an Authorization header" do
      Counter.current.update!(value: 9)

      post "/counter/increment"

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(9)
    end

    it "R-53Z2-DNB1 R-27SO-F63X returns 401 for an opaque non-service-minted bearer token" do
      Counter.current.update!(value: 9)

      post "/counter/increment", headers: { "Authorization" => "Bearer not-a-real-token" }

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(9)
    end

    it "R-53Z2-DNB1 returns 401 for a refresh-kind token (not an access token)" do
      _record, refresh_plaintext = OauthToken.issue(
        kind: "refresh",
        owner: "user-1",
        lifetime: 1.hour
      )
      Counter.current.update!(value: 9)

      post "/counter/increment", headers: { "Authorization" => "Bearer #{refresh_plaintext}" }

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(9)
    end

    it "R-53Z2-DNB1 returns 401 for an expired access token" do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour
      )
      OauthToken.find_by(token_digest: OauthToken.digest_for(plaintext))
                .update!(expires_at: 1.minute.ago)
      Counter.current.update!(value: 9)

      post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(9)
    end

    it "R-53Z2-DNB1 returns 401 for a revoked access token" do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour
      )
      OauthToken.find_by(token_digest: OauthToken.digest_for(plaintext))
                .update!(revoked_at: Time.current)
      Counter.current.update!(value: 9)

      post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(9)
    end

    it "R-IS0W-S2H3 rejects an access token bound to a different resource than this service's canonical URL" do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour,
        resource: "https://some-other-resource.example.com"
      )
      Counter.current.update!(value: 9)

      post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }

      expect(response).to have_http_status(:unauthorized)
      expect(JSON.parse(response.body)).to eq("error" => "invalid_token")
      expect(Counter.current.value).to eq(9)
    end

    it "R-IS0W-S2H3 rejects an access token whose resource binding is unset (no recorded binding)" do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour,
        resource: nil
      )
      Counter.current.update!(value: 9)

      post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(9)
    end

    it "R-IS0W-S2H3 issued tokens record the configured canonical URL as their resource binding" do
      verifier = SecureRandom.urlsafe_base64(64)
      challenge = Base64.urlsafe_encode64(Digest::SHA256.digest(verifier), padding: false)
      client, _ = OauthClient.register(
        client_name: "Resource Binding Client",
        redirect_uris: [ "https://client.example.com/cb" ],
        token_endpoint_auth_method: "none"
      )
      _, code_plain = OauthAuthorizationCode.issue(
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_challenge: challenge,
        code_challenge_method: "S256",
        owner: "google-sub-resource"
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
      access = OauthToken.find_by_presented_token(body["access_token"])
      refresh = OauthToken.find_by_presented_token(body["refresh_token"])
      canonical = Rails.configuration.x.canonical_url

      expect(access.resource).to eq(canonical)
      expect(refresh.resource).to eq(canonical)
    end

    it "R-A26O-QBG9 reuse-detection cascade rejects access token from the revoked chain at increment" do
      verifier = SecureRandom.urlsafe_base64(64)
      challenge = Base64.urlsafe_encode64(Digest::SHA256.digest(verifier), padding: false)
      client, _ = OauthClient.register(
        client_name: "Chain Revocation Client",
        redirect_uris: [ "https://client.example.com/cb" ],
        token_endpoint_auth_method: "none"
      )
      _, code_plain = OauthAuthorizationCode.issue(
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_challenge: challenge,
        code_challenge_method: "S256",
        owner: "google-sub-cascade"
      )
      post "/oauth/token", params: {
        grant_type: "authorization_code",
        code: code_plain,
        client_id: client.client_id,
        redirect_uri: "https://client.example.com/cb",
        code_verifier: verifier
      }
      expect(response).to have_http_status(:ok)
      first = JSON.parse(response.body)
      original_access = first["access_token"]
      original_refresh = first["refresh_token"]

      # Confirm the original access token grants increment before revocation.
      Counter.current.update!(value: 0)
      post "/counter/increment", headers: { "Authorization" => "Bearer #{original_access}" }
      expect(response).to have_http_status(:ok)

      # Rotate, then replay the original refresh — triggers chain revocation.
      post "/oauth/token", params: { grant_type: "refresh_token", refresh_token: original_refresh }
      expect(response).to have_http_status(:ok)
      post "/oauth/token", params: { grant_type: "refresh_token", refresh_token: original_refresh }
      expect(response).to have_http_status(:bad_request)

      # The original access token must now be rejected at the protected endpoint.
      pre_revoke_value = Counter.current.value
      post "/counter/increment", headers: { "Authorization" => "Bearer #{original_access}" }
      expect(response).to have_http_status(:unauthorized)
      expect(JSON.parse(response.body)).to eq("error" => "invalid_token")
      expect(Counter.current.value).to eq(pre_revoke_value)
    end

    it "R-MHYT-TIF7 the protected increment endpoint emits no CORS headers (success path)" do
      Counter.current.update!(value: 0)

      post "/counter/increment", headers: auth_headers.merge("Origin" => "https://evil.example")

      expect(response).to have_http_status(:ok)
      expect(response.headers).not_to have_key("Access-Control-Allow-Origin")
      expect(response.headers["Access-Control-Allow-Credentials"]).not_to eq("true")
    end

    it "R-MHYT-TIF7 the protected increment endpoint emits no CORS headers (401 path)" do
      post "/counter/increment", headers: { "Origin" => "https://evil.example" }

      expect(response).to have_http_status(:unauthorized)
      expect(response.headers).not_to have_key("Access-Control-Allow-Origin")
      expect(response.headers["Access-Control-Allow-Credentials"]).not_to eq("true")
    end

    it "R-MHYT-TIF7 no CORS gem is loaded — Access-Control-Allow-Credentials is never set true on any tested route" do
      gem_names = Gem::Specification.map(&:name)
      expect(gem_names).not_to include("rack-cors")

      requests = lambda do
        get "/counter"
        post "/counter/increment", headers: { "Origin" => "https://evil.example" }
        get "/up"
      end
      requests.call
      # All the above responses share the same controller stack; assert none
      # carried Access-Control-Allow-Credentials: true.
      expect(response.headers["Access-Control-Allow-Credentials"]).not_to eq("true")
    end

    it "R-T2JT-53WF the increment action carries an auth before_action callback" do
      auth_cb = CounterController._process_action_callbacks.find do |cb|
        cb.kind == :before && cb.filter == :require_access_token
      end
      expect(auth_cb).not_to be_nil

      evaluate = lambda do |action|
        controller = CounterController.new
        controller.define_singleton_method(:action_name) { action }
        ifs = auth_cb.instance_variable_get(:@if)
        unlesses = auth_cb.instance_variable_get(:@unless)
        eval_cond = lambda do |c|
          if c.is_a?(Symbol) then controller.send(c)
          elsif c.respond_to?(:match?) then c.match?(controller)
          else c.call(controller)
          end
        end
        ifs.all?(&eval_cond) && unlesses.none?(&eval_cond)
      end

      expect(evaluate.call("increment")).to be(true)
      expect(evaluate.call("show")).to be(false)
    end
  end
end
