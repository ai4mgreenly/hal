# MCP server endpoint (reqs/mcp.md). Speaks the Streamable HTTP
# transport: a single endpoint that accepts JSON-RPC 2.0 messages over
# POST and returns JSON-RPC responses.
#
# R-UK7D-Z0IZ: Streamable HTTP transport.
# R-Z3LX-89W1: tool names and descriptions are written for a model
# audience.
class McpController < ActionController::API
  PROTOCOL_VERSION = "2025-06-18"

  # R-XS1U-B7YY / R-YHNQ-CEJJ / R-GG9B-GS8T: each takes no arguments.
  # R-FUB4-KWWB: exactly these three tools — no more, no fewer.
  # R-ECNJ-R09R: the three tools correspond one-to-one with the three
  # counter operations (read / increment / decrement); no other
  # counter operations exist on any transport.
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
    },
    {
      name: "counter_decrement",
      description:
        "Decrement the service's singleton counter by one and return " \
        "the post-decrement value. Use this when you want to subtract " \
        "one from the counter. The counter is non-negative; calling " \
        "this when the value is zero returns a tool error and leaves " \
        "the counter unchanged. Takes no arguments. Requires an " \
        "authorized session.",
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
  # R-GG9B-GS8T: counter_decrement takes no arguments, subtracts one
  # when value > 0, returns an MCP tool error when value == 0, and
  # requires a valid bearer access token (same rule as increment).
  def handle_tools_call(id, params)
    case params["name"]
    when "counter_read"
      value = Counter.current.value
      reply_tool(id, value)
    when "counter_increment"
      failure = token_auth_failure
      return challenge_unauthorized(id, failure) if failure

      value = Counter.current.increment!
      reply_tool(id, value)
    when "counter_decrement"
      failure = token_auth_failure
      return challenge_unauthorized(id, failure) if failure

      begin
        value = Counter.current.decrement!
        reply_tool(id, value)
      rescue Counter::DecrementBelowZero
        reply_tool_error(id, "counter is at zero; counter cannot go below zero")
      end
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

  # R-GG9B-GS8T: standard MCP tool-error signal — successful JSON-RPC
  # envelope carrying a result with isError=true and a human-readable
  # message naming the cause.
  def reply_tool_error(id, message)
    reply(id, {
      content: [ { type: "text", text: message } ],
      isError: true
    })
  end

  # R-EV2D-QTR1 / R-IS0W-S2H3 / R-A26O-QBG9 / R-27SO-F63X: returns
  # [error_code, error_description] for each of the six distinct failure
  # causes, or nil when the token is valid.
  def token_auth_failure
    presented = bearer_token_from_header
    return [ "invalid_request", "No bearer token presented" ] unless presented
    return [ "invalid_token", "Token is malformed" ] unless presented.match?(/\A[A-Za-z0-9_-]{43}\z/)

    token = OauthToken.find_by_presented_token(presented)
    return [ "invalid_token", "Token not found" ] unless token&.kind == "access"
    return [ "invalid_token", "Token has been revoked" ] if token.revoked_at.present?
    return [ "invalid_token", "Token has expired" ] if token.expires_at <= Time.current

    canonical = Rails.configuration.x.auth.canonical_url
    return [ "invalid_token", "Token resource binding does not match" ] unless
      token.resource.present? && token.resource == canonical

    nil
  end

  def bearer_token_from_header
    header = request.headers["Authorization"].to_s
    return nil unless header.start_with?("Bearer ")
    value = header.sub(/\ABearer\s+/, "").strip
    value.empty? ? nil : value
  end

  # R-0YOE-9NO8 / R-EV2D-QTR1: 401 with a WWW-Authenticate Bearer
  # challenge and a JSON-RPC error body carrying the OAuth error_code and
  # a distinct error_description for the failure cause.
  def challenge_unauthorized(id, auth_error = nil)
    error_code, error_description = auth_error || [ "invalid_token", "Token validation failed" ]
    base = "#{request.protocol}#{request.host_with_port}"
    metadata_url = "#{base}/.well-known/oauth-protected-resource"
    response.headers["WWW-Authenticate"] =
      %(Bearer resource_metadata="#{metadata_url}")
    render status: :unauthorized,
      json: { jsonrpc: "2.0", id: id,
              error: { code: -32001, message: error_code,
                       data: { error_description: error_description } } }
  end

  def reply(id, result)
    render json: { jsonrpc: "2.0", id: id, result: result }
  end

  def reply_error(id, code, message)
    render json: { jsonrpc: "2.0", id: id, error: { code: code, message: message } }
  end
end
