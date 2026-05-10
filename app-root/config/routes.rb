Rails.application.routes.draw do
  # Define your application routes per the DSL in https://guides.rubyonrails.org/routing.html

  # Reveal health status on /up that returns 200 if the app boots with no exceptions, otherwise 500.
  # Can be used by load balancers and uptime monitors to verify that the app is live.
  get "up" => "rails/health#show", as: :rails_health_check

  # Render dynamic PWA files from app/views/pwa/* (remember to link manifest in application.html.erb)
  # get "manifest" => "rails/pwa#manifest", as: :pwa_manifest
  # get "service-worker" => "rails/pwa#service_worker", as: :pwa_service_worker

  # R-QY5R-PYDH: root URL renders the current count in plain HTML, no
  # authentication required.
  root "home#index"

  # OAuth 2.1 authorization-server metadata document (reqs/auth.md).
  # R-2XEK-GCOI: published at the well-known path so MCP clients can
  # discover endpoints from the base URL alone.
  get "/.well-known/oauth-authorization-server" => "oauth_metadata#show"

  # R-0YOE-9NO8: OAuth 2.0 Protected Resource Metadata (RFC 9728) for
  # the MCP server. Referenced from the WWW-Authenticate challenge a
  # 401 from /mcp returns, so a conformant client can discover the
  # authorization server and start the OAuth flow.
  get "/.well-known/oauth-protected-resource" => "oauth_protected_resource_metadata#show"

  # R-3JCR-C810: Dynamic Client Registration endpoint (RFC 7591). MCP
  # clients POST a registration document and receive a client_id (and
  # client_secret if confidential) without manual operator setup.
  post "/oauth/register" => "oauth_register#create"

  # R-4SH1-HQGP: GET /oauth/authorize redirects the user to Google so
  # that Google performs the actual login.
  get "/oauth/authorize" => "oauth_authorize#show"

  # R-8GJG-64MR: browser-facing login flow distinct from the MCP
  # authorization flow. /login is the stable web entry point; it
  # runs the same Google Workspace federation (R-5LQM-O89D) and on
  # success records a web session keyed by the visitor's email.
  get "/login" => "web_login#show"

  # R-AE1P-Z1WC: /logout ends the current web session and redirects
  # to /. Unauthenticated visits are a no-op redirect, not an error.
  # Does not touch any MCP token chain (R-93PJ-FRPY).
  get "/logout" => "web_logout#show"

  # R-5LQM-O89D: Google redirects back here with an authorization
  # code. The service exchanges the code, enforces the configured
  # Workspace domain, and rejects users from any other domain.
  get "/oauth/google/callback" => "oauth_google_callback#show"

  # R-7GT3-PM1K: token endpoint. Redeems a service authorization code
  # and mints a finite-lifetime access token plus a refresh token.
  post "/oauth/token" => "oauth_token#create"

  # R-UK7D-Z0IZ: MCP server endpoint (Streamable HTTP transport). MCP
  # clients connect to the service base URL + "/mcp" and exchange
  # JSON-RPC 2.0 messages over POST.
  post "/mcp" => "mcp#handle"

  # R-DRX9-8WNY: SSE live-update channel. No authentication required;
  # streams the current counter value on subscribe and on every change.
  get "counter/stream" => "counter_stream#show"

  # JSON HTTP API (reqs/api.md).
  # R-2I2S-XB7K: GET /counter returns the current value.
  get "counter" => "counter#show"
  # R-340Z-T6K2: POST /counter/increment adds one and returns the
  # post-increment value.
  post "counter/increment" => "counter#increment"
  # R-H3FE-QFC0: POST /counter/decrement subtracts one and returns the
  # post-decrement value; 409 when the counter is at zero.
  post "counter/decrement" => "counter#decrement"
end
