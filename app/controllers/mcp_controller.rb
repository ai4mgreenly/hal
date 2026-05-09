# MCP server endpoint (reqs/mcp.md). Speaks the Streamable HTTP
# transport: a single endpoint that accepts JSON-RPC 2.0 messages over
# POST and returns JSON-RPC responses.
#
# R-UK7D-Z0IZ: Streamable HTTP transport.
# R-X4VR-1KVR: advertises exactly two tools (read + increment).
# R-Z3LX-89W1: tool names and descriptions are written for a model
# audience.
class McpController < ActionController::API
  PROTOCOL_VERSION = "2025-06-18"

  # R-X4VR-1KVR: exactly these two tool definitions are advertised.
  # R-XS1U-B7YY / R-YHNQ-CEJJ: each takes no arguments.
  # R-Z3LX-89W1: names + descriptions written so a model can pick the
  # right tool with no other context.
  TOOLS = [
    {
      name: "counter_read",
      description:
        "Read the service's singleton counter. Returns the current " \
        "non-negative integer value. Use this when you need to know " \
        "the counter's current value without changing it. Takes no " \
        "arguments.",
      inputSchema: { type: "object", properties: {}, additionalProperties: false }
    },
    {
      name: "counter_increment",
      description:
        "Increment the service's singleton counter by one and return " \
        "the post-increment value. Use this when you want to add one " \
        "to the counter. Takes no arguments. Requires an authorized " \
        "session.",
      inputSchema: { type: "object", properties: {}, additionalProperties: false }
    }
  ].freeze

  def handle
    body = request.body.read
    message = body.present? ? JSON.parse(body) : {}
    id = message["id"]
    method = message["method"]

    case method
    when "initialize"
      reply(id, {
        protocolVersion: PROTOCOL_VERSION,
        capabilities: { tools: {} },
        serverInfo: { name: "hal", version: "0.1.0" }
      })
    when "tools/list"
      reply(id, { tools: TOOLS })
    when "tools/call"
      handle_tools_call(id, message["params"] || {})
    when "notifications/initialized", nil
      head :accepted
    else
      reply_error(id, -32601, "Method not found")
    end
  rescue JSON::ParserError
    reply_error(nil, -32700, "Parse error")
  end

  private

  # R-XS1U-B7YY: counter_read takes no arguments and returns the
  # current counter value as a non-negative integer.
  # R-YHNQ-CEJJ: counter_increment takes no arguments, adds one to the
  # counter, and returns the post-increment value.
  # R-ZQS0-HWZ8: counter_increment requires a valid bearer access token.
  def handle_tools_call(id, params)
    case params["name"]
    when "counter_read"
      value = Counter.current.value
      reply_tool(id, value)
    when "counter_increment"
      return challenge_unauthorized(id) unless valid_access_token?

      value = Counter.current.increment!
      reply_tool(id, value)
    else
      reply_error(id, -32602, "Unknown tool")
    end
  end

  def reply_tool(id, value)
    reply(id, {
      content: [ { type: "text", text: value.to_s } ],
      structuredContent: { value: value },
      isError: false
    })
  end

  # Mirrors CounterController#require_access_token (R-IS0W-S2H3,
  # R-A26O-QBG9, R-27SO-F63X): only this service's own access tokens,
  # not revoked, not expired, bound to the canonical URL.
  def valid_access_token?
    presented = bearer_token_from_header
    token = presented && OauthToken.find_by_presented_token(presented)
    canonical = Rails.configuration.x.canonical_url
    token && token.kind == "access" &&
      token.revoked_at.nil? && token.expires_at > Time.current &&
      token.resource.present? && token.resource == canonical
  end

  def bearer_token_from_header
    header = request.headers["Authorization"].to_s
    return nil unless header.start_with?("Bearer ")
    value = header.sub(/\ABearer\s+/, "").strip
    value.empty? ? nil : value
  end

  # R-0YOE-9NO8: when an MCP client invokes a tool that needs auth
  # without valid credentials, respond with HTTP 401 and a
  # WWW-Authenticate challenge that points at this server's
  # OAuth 2.0 Protected Resource Metadata document. A conformant MCP
  # client treats that pair as the signal to start the OAuth flow.
  def challenge_unauthorized(id)
    base = "#{request.protocol}#{request.host_with_port}"
    metadata_url = "#{base}/.well-known/oauth-protected-resource"
    response.headers["WWW-Authenticate"] =
      %(Bearer resource_metadata="#{metadata_url}")
    render status: :unauthorized,
      json: { jsonrpc: "2.0", id: id,
              error: { code: -32001, message: "invalid_token" } }
  end

  def reply(id, result)
    render json: { jsonrpc: "2.0", id: id, result: result }
  end

  def reply_error(id, code, message)
    render json: { jsonrpc: "2.0", id: id, error: { code: code, message: message } }
  end
end
