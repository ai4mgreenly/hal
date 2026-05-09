require "rails_helper"

RSpec.describe "MCP server", type: :request do
  def jsonrpc(method, params: {}, id: 1)
    post "/mcp",
      params: { jsonrpc: "2.0", id: id, method: method, params: params }.to_json,
      headers: { "Content-Type" => "application/json" }
  end

  describe "tools/list (R-X4VR-1KVR R-XS1U-B7YY R-YHNQ-CEJJ R-Z3LX-89W1)" do
    it "advertises exactly two tools, one for read and one for increment (R-X4VR-1KVR)" do
      jsonrpc("tools/list")

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)
      tools = body.dig("result", "tools")
      expect(tools).to be_an(Array)
      expect(tools.length).to eq(2)
    end

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

    it "names and describes each tool so a model can choose without further context (R-Z3LX-89W1)" do
      jsonrpc("tools/list")

      tools = JSON.parse(response.body).dig("result", "tools")
      tools.each do |t|
        expect(t["description"].to_s.length).to be >= 40
        expect(t["name"]).to match(/\Acounter_/)
      end
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
      expect(doc["resource"]).to eq("http://www.example.com")
      expect(doc["authorization_servers"]).to eq([ "http://www.example.com" ])
      expect(doc["bearer_methods_supported"]).to include("header")
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
