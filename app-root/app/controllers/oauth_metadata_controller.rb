# OAuth 2.1 authorization-server metadata document (reqs/auth.md).
class OauthMetadataController < ActionController::API
  # R-2XEK-GCOI: publishes the authorization-server metadata document
  # (RFC 8414) so MCP clients can discover the authorization endpoints
  # from the base URL alone.
  # R-42V5-GJW4: advertises Authorization Code + PKCE only — no
  # implicit ("token"/"id_token") response types and no "password" or
  # "implicit" grants.
  # R-1KML-5J0Q: every advertised endpoint shares the same origin as
  # the issuer (which is the MCP server's own origin). Clients only
  # need to know that one origin.
  def show
    base = "#{request.protocol}#{request.host_with_port}"
    render json: {
      issuer: base,
      authorization_endpoint: "#{base}/oauth/authorize",
      token_endpoint: "#{base}/oauth/token",
      registration_endpoint: "#{base}/oauth/register",
      response_types_supported: [ "code" ],
      grant_types_supported: [ "authorization_code", "refresh_token" ],
      code_challenge_methods_supported: [ "S256" ],
      token_endpoint_auth_methods_supported: [ "none", "client_secret_basic" ]
    }
  end
end
