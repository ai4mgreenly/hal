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
      expect(JSON.parse(response.body)).to include("error" => "invalid_token")
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
      canonical = Rails.configuration.x.auth.canonical_url

      expect(access.resource).to eq(canonical)
      expect(refresh.resource).to eq(canonical)
    end

    describe "R-DH2I-28CK resource equality is byte-for-byte — canonical URL bound token accepted at both endpoints; near-misses rejected" do
      let(:canonical) { Rails.configuration.x.auth.canonical_url }

      it "R-DH2I-28CK accepts a token bound to the exact configured canonical URL at the counter endpoint" do
        _record, plaintext = OauthToken.issue(
          kind: "access",
          owner: "user-1",
          lifetime: 1.hour,
          resource: canonical
        )
        Counter.current.update!(value: 4)

        post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }

        expect(response).to have_http_status(:ok)
        expect(JSON.parse(response.body)).to eq("value" => 5)
      end

      it "R-DH2I-28CK rejects a token whose resource is the canonical URL with a trailing slash" do
        _record, plaintext = OauthToken.issue(
          kind: "access",
          owner: "user-1",
          lifetime: 1.hour,
          resource: "#{canonical}/"
        )
        Counter.current.update!(value: 9)

        post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }

        expect(response).to have_http_status(:unauthorized)
        expect(Counter.current.value).to eq(9)
      end

      it "R-DH2I-28CK rejects a token whose resource has a sub-path appended to the canonical URL" do
        _record, plaintext = OauthToken.issue(
          kind: "access",
          owner: "user-1",
          lifetime: 1.hour,
          resource: "#{canonical}/counter"
        )
        Counter.current.update!(value: 9)

        post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }

        expect(response).to have_http_status(:unauthorized)
        expect(Counter.current.value).to eq(9)
      end

      it "R-DH2I-28CK a token bound to the canonical URL is also accepted at the MCP endpoint — both share the same single resource identifier" do
        _record, plaintext = OauthToken.issue(
          kind: "access",
          owner: "user-1",
          lifetime: 1.hour,
          resource: canonical
        )
        Counter.current.update!(value: 0)

        post "/mcp",
          params: { jsonrpc: "2.0", id: 1, method: "tools/call",
                    params: { name: "counter_increment", arguments: {} } }.to_json,
          headers: { "Content-Type" => "application/json",
                     "Authorization" => "Bearer #{plaintext}" }

        expect(response).to have_http_status(:ok)
        expect(Counter.current.value).to eq(1)
      end
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
      expect(JSON.parse(response.body)).to include("error" => "invalid_token")
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

  describe "POST /counter/decrement" do
    let(:access_token) do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour
      )
      plaintext
    end
    let(:auth_headers) { { "Authorization" => "Bearer #{access_token}" } }

    it "R-H3FE-QFC0 returns 200 with a JSON object containing the post-decrement value" do
      Counter.current.update!(value: 5)

      post "/counter/decrement", headers: auth_headers

      expect(response).to have_http_status(:ok)
      expect(response.media_type).to eq("application/json")
      body = JSON.parse(response.body)
      expect(body).to be_a(Hash)
      expect(body).to have_key("value")
      expect(body["value"]).to eq(4)
      expect(Counter.current.value).to eq(4)
    end

    it "R-H3FE-QFC0 returns 409 with a JSON error body when the counter is zero; stored value unchanged" do
      Counter.current.update!(value: 0)

      post "/counter/decrement", headers: auth_headers

      expect(response).to have_http_status(:conflict)
      expect(response.media_type).to eq("application/json")
      body = JSON.parse(response.body)
      expect(body["error"]).to be_present
      expect(body["error_description"]).to be_present
      expect(Counter.current.value).to eq(0)
    end

    it "R-H3FE-QFC0 R-53Z2-DNB1 returns 401 and does not change the counter without auth" do
      Counter.current.update!(value: 9)

      post "/counter/decrement"

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(9)
    end

    it "R-H3FE-QFC0 R-OCH3-8FQ8 a valid web session cookie alone authenticates decrement" do
      provider = Rails.configuration.x.google_identity_provider
      previous_domain = Rails.configuration.x.auth.workspace_domain
      Rails.configuration.x.auth.workspace_domain = "allowed.example"
      begin
        get "https://www.example.com/login"
        upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
        provider.stub_code(
          "code-h3fe-decrement",
          sub: "google-h3fe",
          email: "h3fe@allowed.example",
          hosted_domain: "allowed.example"
        )
        get "https://www.example.com/oauth/google/callback",
            params: { code: "code-h3fe-decrement", state: upstream_state },
            headers: { "X-Forwarded-Proto" => "https" }

        Counter.current.update!(value: 3)

        post "https://www.example.com/counter/decrement"

        expect(response).to have_http_status(:ok)
        expect(JSON.parse(response.body)).to eq("value" => 2)
        expect(Counter.current.value).to eq(2)
      ensure
        Rails.configuration.x.auth.workspace_domain = previous_domain
      end
    end

    it "R-H3FE-QFC0 R-MHYT-TIF7 the protected decrement endpoint emits no CORS headers" do
      Counter.current.update!(value: 1)

      post "/counter/decrement", headers: auth_headers.merge("Origin" => "https://evil.example")

      expect(response).to have_http_status(:ok)
      expect(response.headers).not_to have_key("Access-Control-Allow-Origin")
      expect(response.headers["Access-Control-Allow-Credentials"]).not_to eq("true")
    end

    it "R-H3FE-QFC0 R-DH2I-28CK rejects a bearer token bound to a different resource" do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour,
        resource: "https://other.example.com"
      )
      Counter.current.update!(value: 5)

      post "/counter/decrement", headers: { "Authorization" => "Bearer #{plaintext}" }

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(5)
    end
  end

  describe "R-OCH3-8FQ8 the mutation endpoints accept either bearer or web-session-cookie auth" do
    let(:provider) { Rails.configuration.x.google_identity_provider }
    let(:email) { "och3@allowed.example" }

    around do |example|
      previous_domain = Rails.configuration.x.auth.workspace_domain
      Rails.configuration.x.auth.workspace_domain = "allowed.example"
      example.run
      Rails.configuration.x.auth.workspace_domain = previous_domain
    end

    def establish_web_session!(email:)
      get "https://www.example.com/login"
      upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
      provider.stub_code(
        "code-och3-#{email}",
        sub: "google-#{email}",
        email: email,
        hosted_domain: "allowed.example"
      )
      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-och3-#{email}", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }
    end

    it "R-OCH3-8FQ8 a valid web session cookie alone authenticates POST /counter/increment" do
      establish_web_session!(email: email)
      Counter.current.update!(value: 2)

      post "https://www.example.com/counter/increment"

      expect(response).to have_http_status(:ok)
      expect(JSON.parse(response.body)).to eq("value" => 3)
      expect(Counter.current.value).to eq(3)
    end

    it "R-OCH3-8FQ8 a valid session + invalid bearer still succeeds (either mode sufficient)" do
      establish_web_session!(email: email)
      Counter.current.update!(value: 0)

      post "https://www.example.com/counter/increment",
           headers: { "Authorization" => "Bearer not-a-real-token" }

      expect(response).to have_http_status(:ok)
      expect(JSON.parse(response.body)).to eq("value" => 1)
    end

    it "R-OCH3-8FQ8 no auth at all returns 401 and does not change the counter (R-53Z2-DNB1)" do
      Counter.current.update!(value: 5)

      post "https://www.example.com/counter/increment"

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(5)
    end

    it "R-OCH3-8FQ8 an invalid (revoked) session cookie alone returns 401" do
      establish_web_session!(email: email)
      WebSession.last.revoke!
      Counter.current.update!(value: 5)

      post "https://www.example.com/counter/increment"

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(5)
    end

    it "R-OCH3-8FQ8 a session-cookie mutation does not consult the OAuth token store" do
      establish_web_session!(email: email)
      Counter.current.update!(value: 0)

      # No find/find_by on OauthToken is invoked during the mutation.
      allow(OauthToken).to receive(:find_by_presented_token).and_call_original
      expect(OauthToken).not_to receive(:find_by_presented_token)

      post "https://www.example.com/counter/increment"

      expect(response).to have_http_status(:ok)
      expect(Counter.current.value).to eq(1)
    end

    it "R-OCH3-8FQ8 a session-cookie mutation does not revoke or touch any OauthToken row " \
       "(R-93PJ-FRPY lifetime independence at the mutation path)" do
      _record, plaintext = OauthToken.issue(kind: "access", owner: "user-1", lifetime: 1.hour)
      access_row = OauthToken.find_by(token_digest: OauthToken.digest_for(plaintext))
      pre_updated = access_row.updated_at

      establish_web_session!(email: email)
      Counter.current.update!(value: 0)

      post "https://www.example.com/counter/increment"

      expect(response).to have_http_status(:ok)
      access_row.reload
      expect(access_row.revoked_at).to be_nil
      expect(access_row.updated_at).to eq(pre_updated)
    end

    it "R-OCH3-8FQ8 a bearer-token mutation (no web session) does not insert or touch a WebSession row " \
       "(R-93PJ-FRPY lifetime independence at the mutation path)" do
      _record, plaintext = OauthToken.issue(
        kind: "access", owner: "user-1", lifetime: 1.hour
      )

      expect {
        post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }
      }.not_to change(WebSession, :count)
      expect(response).to have_http_status(:ok)
    end
  end

  describe "R-EV2D-QTR1 error_description discriminates bearer-token rejection causes at POST /counter/increment" do
    it "R-EV2D-QTR1 no token presented → error=invalid_request with distinct description" do
      post "/counter/increment"

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body["error"]).to eq("invalid_request")
      expect(body["error_description"]).to be_present
    end

    it "R-EV2D-QTR1 malformed token → error=invalid_token with malformed-cause description" do
      post "/counter/increment", headers: { "Authorization" => "Bearer bad-token!" }

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body["error"]).to eq("invalid_token")
      expect(body["error_description"]).to match(/malform/i)
    end

    it "R-EV2D-QTR1 token not in store → error=invalid_token with not-found-cause description" do
      post "/counter/increment", headers: { "Authorization" => "Bearer #{"a" * 43}" }

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body["error"]).to eq("invalid_token")
      expect(body["error_description"]).to match(/not found/i)
    end

    it "R-EV2D-QTR1 expired token → error=invalid_token with expired-cause description (R-TNXJ-ZWQ0)" do
      _record, plaintext = OauthToken.issue(kind: "access", owner: "user-1", lifetime: 1.hour)
      OauthToken.find_by(token_digest: OauthToken.digest_for(plaintext))
                .update!(expires_at: 1.minute.ago)

      post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body["error"]).to eq("invalid_token")
      expect(body["error_description"]).to match(/expir/i)
    end

    it "R-EV2D-QTR1 revoked token → error=invalid_token with revoked-cause description (R-9HGE-87UG / R-A26O-QBG9)" do
      _record, plaintext = OauthToken.issue(kind: "access", owner: "user-1", lifetime: 1.hour)
      OauthToken.find_by(token_digest: OauthToken.digest_for(plaintext))
                .update!(revoked_at: Time.current)

      post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body["error"]).to eq("invalid_token")
      expect(body["error_description"]).to match(/revok/i)
    end

    it "R-EV2D-QTR1 wrong resource → error=invalid_token with resource-mismatch description (R-IS0W-S2H3 / R-DH2I-28CK)" do
      _record, plaintext = OauthToken.issue(
        kind: "access", owner: "user-1", lifetime: 1.hour,
        resource: "https://other.example.com"
      )

      post "/counter/increment", headers: { "Authorization" => "Bearer #{plaintext}" }

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body["error"]).to eq("invalid_token")
      expect(body["error_description"]).to match(/resource/i)
    end

    it "R-EV2D-QTR1 the six error_description strings are all distinct" do
      descriptions = [
        "No bearer token presented",
        "Token is malformed",
        "Token not found",
        "Token has expired",
        "Token has been revoked",
        "Token resource binding does not match"
      ]
      expect(descriptions.uniq.length).to eq(6)
    end
  end
end
