require "rails_helper"

RSpec.describe "MCP server", type: :request do
  def jsonrpc(method, params: {}, id: 1)
    post "/mcp",
      params: { jsonrpc: "2.0", id: id, method: method, params: params }.to_json,
      headers: { "Content-Type" => "application/json" }
  end

  describe "tools/list (R-XS1U-B7YY R-YHNQ-CEJJ R-GG9B-GS8T R-Z3LX-89W1)" do
    it "exposes the read tool with an empty argument schema (R-XS1U-B7YY)" do
      jsonrpc("tools/list")

      tools = JSON.parse(response.body).dig("result", "tools")
      read = tools.find { |t| t["name"] == "counter_read" }
      expect(read).not_to be_nil
      expect(read.dig("inputSchema", "properties")).to eq({})
    end

    it "exposes the increment tool with an empty argument schema (R-YHNQ-CEJJ)" do
      jsonrpc("tools/list")

      tools = JSON.parse(response.body).dig("result", "tools")
      inc = tools.find { |t| t["name"] == "counter_increment" }
      expect(inc).not_to be_nil
      expect(inc.dig("inputSchema", "properties")).to eq({})
    end

    it "exposes the decrement tool with an empty argument schema (R-GG9B-GS8T)" do
      jsonrpc("tools/list")

      tools = JSON.parse(response.body).dig("result", "tools")
      dec = tools.find { |t| t["name"] == "counter_decrement" }
      expect(dec).not_to be_nil
      expect(dec.dig("inputSchema", "properties")).to eq({})
    end

    it "names and describes each tool so a model can choose without further context (R-Z3LX-89W1)" do
      jsonrpc("tools/list")

      tools = JSON.parse(response.body).dig("result", "tools")
      tools.each do |t|
        expect(t["description"].to_s.length).to be >= 40
        expect(t["name"]).to match(/\Acounter_/)
      end
    end

    it "advertises exactly three tools, one per counter operation (R-FUB4-KWWB)" do
      jsonrpc("tools/list")

      tools = JSON.parse(response.body).dig("result", "tools")
      expect(tools.map { |t| t["name"] }).to contain_exactly(
        "counter_read", "counter_increment", "counter_decrement"
      )
    end
  end

  describe "tools/call counter_read (R-XS1U-B7YY)" do
    it "returns the current counter value as a non-negative integer (R-XS1U-B7YY)" do
      Counter.current.update!(value: 7)

      jsonrpc("tools/call", params: { name: "counter_read", arguments: {} })

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      result = body["result"]
      expect(result["isError"]).to eq(false)
      text = result.dig("content", 0, "text")
      expect(text).to eq("7")
      parsed = Integer(text)
      expect(parsed).to be >= 0
      expect(parsed).to eq(7)
    end

    it "accepts a tools/call with no arguments object at all (R-XS1U-B7YY)" do
      Counter.current.update!(value: 0)

      jsonrpc("tools/call", params: { name: "counter_read" })

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      expect(body.dig("result", "content", 0, "text")).to eq("0")
    end
  end

  describe "tools/call counter_increment (R-YHNQ-CEJJ R-ZQS0-HWZ8)" do
    let(:access_token) do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour
      )
      plaintext
    end

    def jsonrpc_with_auth(method, token, params: {}, id: 1)
      post "/mcp",
        params: { jsonrpc: "2.0", id: id, method: method, params: params }.to_json,
        headers: {
          "Content-Type" => "application/json",
          "Authorization" => "Bearer #{token}"
        }
    end

    it "increments the counter and returns the post-increment value (R-YHNQ-CEJJ)" do
      Counter.current.update!(value: 4)

      jsonrpc_with_auth("tools/call", access_token,
                        params: { name: "counter_increment", arguments: {} })

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      result = body["result"]
      expect(result["isError"]).to eq(false)
      expect(result.dig("content", 0, "text")).to eq("5")
      expect(result.dig("structuredContent", "value")).to eq(5)
      expect(Counter.current.value).to eq(5)
    end

    it "rejects counter_increment without a bearer token (R-ZQS0-HWZ8)" do
      Counter.current.update!(value: 9)

      jsonrpc("tools/call", params: { name: "counter_increment" })

      expect(response).to have_http_status(:unauthorized)
      body = JSON.parse(response.body)
      expect(body["error"]).to be_present
      expect(Counter.current.value).to eq(9)
    end

    it "rejects counter_increment with a non-service-minted bearer token (R-ZQS0-HWZ8)" do
      Counter.current.update!(value: 9)

      jsonrpc_with_auth("tools/call", "not-a-real-token",
                        params: { name: "counter_increment" })

      expect(response).to have_http_status(:unauthorized)
      body = JSON.parse(response.body)
      expect(body["error"]).to be_present
      expect(Counter.current.value).to eq(9)
    end

    it "rejects counter_increment with a refresh-kind token (R-ZQS0-HWZ8)" do
      _record, refresh_plaintext = OauthToken.issue(
        kind: "refresh",
        owner: "user-1",
        lifetime: 1.hour
      )
      Counter.current.update!(value: 9)

      jsonrpc_with_auth("tools/call", refresh_plaintext,
                        params: { name: "counter_increment" })

      expect(response).to have_http_status(:unauthorized)
      body = JSON.parse(response.body)
      expect(body["error"]).to be_present
      expect(Counter.current.value).to eq(9)
    end
  end

  describe "tools/call counter_decrement (R-GG9B-GS8T)" do
    let(:access_token) do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour,
        resource: Rails.configuration.x.auth.canonical_url
      )
      plaintext
    end

    def jsonrpc_with_auth(method, token, params: {}, id: 1)
      post "/mcp",
        params: { jsonrpc: "2.0", id: id, method: method, params: params }.to_json,
        headers: {
          "Content-Type" => "application/json",
          "Authorization" => "Bearer #{token}"
        }
    end

    it "decrements the counter and returns the post-decrement value (R-GG9B-GS8T)" do
      Counter.current.update!(value: 4)

      jsonrpc_with_auth("tools/call", access_token,
                        params: { name: "counter_decrement", arguments: {} })

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      result = body["result"]
      expect(result["isError"]).to eq(false)
      expect(result.dig("content", 0, "text")).to eq("3")
      expect(result.dig("structuredContent", "value")).to eq(3)
      expect(Counter.current.value).to eq(3)
    end

    it "returns an MCP tool error when the counter is zero, leaving it unchanged (R-GG9B-GS8T)" do
      Counter.current.update!(value: 0)

      jsonrpc_with_auth("tools/call", access_token,
                        params: { name: "counter_decrement", arguments: {} })

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      result = body["result"]
      expect(result["isError"]).to eq(true)
      expect(result.dig("content", 0, "text")).to match(/zero|below/i)
      expect(Counter.current.value).to eq(0)
    end

    it "rejects counter_decrement without a bearer token (R-GG9B-GS8T)" do
      Counter.current.update!(value: 5)

      jsonrpc("tools/call", params: { name: "counter_decrement" })

      expect(response).to have_http_status(:unauthorized)
      body = JSON.parse(response.body)
      expect(body["error"]).to be_present
      expect(Counter.current.value).to eq(5)
    end

    it "issues a Bearer challenge when an unauthenticated client invokes counter_decrement (R-GG9B-GS8T R-0YOE-9NO8)" do
      jsonrpc("tools/call", params: { name: "counter_decrement" })

      expect(response).to have_http_status(:unauthorized)
      challenge = response.headers["WWW-Authenticate"].to_s
      expect(challenge).to start_with("Bearer ")
      expect(challenge).to include("resource_metadata=")
    end
  end

  describe "tools/call counter_read without auth (R-0CQ7-DSBQ)" do
    it "permits counter_read without any Authorization header (R-0CQ7-DSBQ)" do
      Counter.current.update!(value: 3)

      jsonrpc("tools/call", params: { name: "counter_read" })

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      expect(body.dig("result", "isError")).to eq(false)
      expect(body.dig("result", "content", 0, "text")).to eq("3")
    end
  end

  describe "OAuth challenge on protected tool call (R-0YOE-9NO8)" do
    it "returns 401 with a Bearer challenge pointing at the metadata (R-0YOE-9NO8)" do
      jsonrpc("tools/call", params: { name: "counter_increment" })

      expect(response).to have_http_status(:unauthorized)
      challenge = response.headers["WWW-Authenticate"].to_s
      expect(challenge).to start_with("Bearer ")
      expect(challenge).to include(
        %(resource_metadata="http://www.example.com/.well-known/oauth-protected-resource)
      )
    end

    it "publishes RFC 9728 metadata linking to the authorization server (R-0YOE-9NO8)" do
      get "/.well-known/oauth-protected-resource"

      expect(response).to have_http_status(:ok)
      doc = JSON.parse(response.body)
      expect(doc["resource"]).to eq("http://www.example.com/")
      expect(doc["authorization_servers"]).to eq([ "http://www.example.com/" ])
      expect(doc["bearer_methods_supported"]).to include("header")
    end
  end

  describe "R-3UT3-IKZG single configured canonical resource identifier" do
    it "R-3UT3-IKZG canonical identifier carries the trailing slash for a root-path service" do
      expect(Rails.configuration.x.auth.canonical_url).to eq("http://www.example.com/")
    end

    it "R-3UT3-IKZG metadata document publishes the canonical identifier verbatim, byte-for-byte" do
      get "/.well-known/oauth-protected-resource"

      doc = JSON.parse(response.body)
      expect(doc["resource"]).to eq(Rails.configuration.x.auth.canonical_url)
    end

    it "R-3UT3-IKZG issued tokens record the same canonical string the metadata publishes" do
      get "/.well-known/oauth-protected-resource"
      published = JSON.parse(response.body)["resource"]

      _record, plaintext = OauthToken.issue(kind: "access", owner: "user-1", lifetime: 1.hour)
      token = OauthToken.find_by_presented_token(plaintext)
      expect(token.resource).to eq(published)
    end

    it "R-3UT3-IKZG a token bound to the canonical identifier is accepted at the MCP endpoint" do
      _record, plaintext = OauthToken.issue(
        kind: "access", owner: "user-1", lifetime: 1.hour,
        resource: Rails.configuration.x.auth.canonical_url
      )

      post "/mcp",
        params: { jsonrpc: "2.0", id: 1, method: "tools/call",
                  params: { name: "counter_increment", arguments: {} } }.to_json,
        headers: { "Content-Type" => "application/json",
                   "Authorization" => "Bearer #{plaintext}" }

      expect(response).to have_http_status(:ok)
    end
  end

  describe "R-DH2I-28CK resource binding at the MCP endpoint — byte-for-byte equality against the single configured canonical URL" do
    let(:canonical) { Rails.configuration.x.auth.canonical_url }

    it "R-DH2I-28CK rejects a token whose resource is the canonical URL with a trailing slash" do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour,
        resource: "#{canonical}/"
      )
      Counter.current.update!(value: 9)

      post "/mcp",
        params: { jsonrpc: "2.0", id: 1, method: "tools/call",
                  params: { name: "counter_increment", arguments: {} } }.to_json,
        headers: { "Content-Type" => "application/json",
                   "Authorization" => "Bearer #{plaintext}" }

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(9)
    end

    it "R-DH2I-28CK rejects a token whose resource has the /mcp sub-path appended to the canonical URL" do
      _record, plaintext = OauthToken.issue(
        kind: "access",
        owner: "user-1",
        lifetime: 1.hour,
        resource: "#{canonical}/mcp"
      )
      Counter.current.update!(value: 9)

      post "/mcp",
        params: { jsonrpc: "2.0", id: 1, method: "tools/call",
                  params: { name: "counter_increment", arguments: {} } }.to_json,
        headers: { "Content-Type" => "application/json",
                   "Authorization" => "Bearer #{plaintext}" }

      expect(response).to have_http_status(:unauthorized)
      expect(Counter.current.value).to eq(9)
    end
  end

  describe "R-EV2D-QTR1 error_description discriminates bearer-token rejection causes at MCP counter_increment" do
    def mcp_increment(token: nil)
      headers = { "Content-Type" => "application/json" }
      headers["Authorization"] = "Bearer #{token}" if token
      post "/mcp",
        params: { jsonrpc: "2.0", id: 1, method: "tools/call",
                  params: { name: "counter_increment", arguments: {} } }.to_json,
        headers: headers
    end

    it "R-EV2D-QTR1 no token presented → error=invalid_request with distinct description" do
      mcp_increment

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body.dig("error", "message")).to eq("invalid_request")
      expect(body.dig("error", "data", "error_description")).to be_present
    end

    it "R-EV2D-QTR1 malformed token → error=invalid_token with malformed-cause description" do
      mcp_increment(token: "bad-token!")

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body.dig("error", "message")).to eq("invalid_token")
      expect(body.dig("error", "data", "error_description")).to match(/malform/i)
    end

    it "R-EV2D-QTR1 token not in store → error=invalid_token with not-found-cause description" do
      mcp_increment(token: "a" * 43)

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body.dig("error", "message")).to eq("invalid_token")
      expect(body.dig("error", "data", "error_description")).to match(/not found/i)
    end

    it "R-EV2D-QTR1 expired token → error=invalid_token with expired-cause description (R-TNXJ-ZWQ0)" do
      _record, plaintext = OauthToken.issue(kind: "access", owner: "user-1", lifetime: 1.hour)
      OauthToken.find_by(token_digest: OauthToken.digest_for(plaintext))
                .update!(expires_at: 1.minute.ago)

      mcp_increment(token: plaintext)

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body.dig("error", "message")).to eq("invalid_token")
      expect(body.dig("error", "data", "error_description")).to match(/expir/i)
    end

    it "R-EV2D-QTR1 revoked token → error=invalid_token with revoked-cause description (R-9HGE-87UG / R-A26O-QBG9)" do
      _record, plaintext = OauthToken.issue(kind: "access", owner: "user-1", lifetime: 1.hour)
      OauthToken.find_by(token_digest: OauthToken.digest_for(plaintext))
                .update!(revoked_at: Time.current)

      mcp_increment(token: plaintext)

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body.dig("error", "message")).to eq("invalid_token")
      expect(body.dig("error", "data", "error_description")).to match(/revok/i)
    end

    it "R-EV2D-QTR1 wrong resource → error=invalid_token with resource-mismatch description (R-IS0W-S2H3 / R-DH2I-28CK)" do
      _record, plaintext = OauthToken.issue(
        kind: "access", owner: "user-1", lifetime: 1.hour,
        resource: "https://other.example.com"
      )

      mcp_increment(token: plaintext)

      body = JSON.parse(response.body)
      expect(response).to have_http_status(:unauthorized)
      expect(body.dig("error", "message")).to eq("invalid_token")
      expect(body.dig("error", "data", "error_description")).to match(/resource/i)
    end
  end

  describe "transport (R-UK7D-Z0IZ)" do
    it "responds to JSON-RPC initialize over Streamable HTTP POST (R-UK7D-Z0IZ)" do
      jsonrpc("initialize", params: { protocolVersion: "2025-06-18", capabilities: {} })

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      expect(body["jsonrpc"]).to eq("2.0")
      expect(body.dig("result", "protocolVersion")).to be_present
      expect(body.dig("result", "capabilities", "tools")).not_to be_nil
    end
  end
end
