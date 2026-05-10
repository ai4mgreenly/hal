# OAuth 2.0 Protected Resource Metadata document (RFC 9728), referenced
# by the MCP server's WWW-Authenticate challenge so a conformant client
# can discover the authorization server from the base URL alone
# (reqs/mcp.md).
class OauthProtectedResourceMetadataController < ActionController::API
  # R-0YOE-9NO8: published at /.well-known/oauth-protected-resource so
  # that an MCP client receiving a 401 + WWW-Authenticate challenge from
  # /mcp can resolve the authorization server and begin the OAuth flow
  # defined in the MCP authorization specification.
  # R-3UT3-IKZG: publish the configured canonical resource identifier
  # verbatim (byte-for-byte) — the same string tokens are bound to and
  # the validation check compares against. Not derived from the
  # request, because the identifier is a single configured value.
  def show
    canonical = Rails.configuration.x.auth.canonical_url
    render json: {
      resource: canonical,
      authorization_servers: [ canonical ],
      bearer_methods_supported: [ "header" ]
    }
  end
end
