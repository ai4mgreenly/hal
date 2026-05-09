# OAuth 2.0 Protected Resource Metadata document (RFC 9728), referenced
# by the MCP server's WWW-Authenticate challenge so a conformant client
# can discover the authorization server from the base URL alone
# (reqs/mcp.md).
class OauthProtectedResourceMetadataController < ActionController::API
  # R-0YOE-9NO8: published at /.well-known/oauth-protected-resource so
  # that an MCP client receiving a 401 + WWW-Authenticate challenge from
  # /mcp can resolve the authorization server and begin the OAuth flow
  # defined in the MCP authorization specification.
  def show
    base = "#{request.protocol}#{request.host_with_port}"
    render json: {
      resource: base,
      authorization_servers: [ base ],
      bearer_methods_supported: [ "header" ]
    }
  end
end
