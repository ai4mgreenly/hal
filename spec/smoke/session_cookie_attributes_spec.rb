# Posture spec for R-QGB5-EMOO. The session cookie carries Secure,
# HttpOnly, and SameSite=Lax. We hit the authorize endpoint, which
# writes session[:pending_authorizations], then inspect the Set-Cookie
# header on the response. The request is sent with
# X-Forwarded-Proto: https so the trusted-proxy path treats it as
# HTTPS — `secure: true` in the session-store config emits the
# attribute regardless, but exercising the realistic path keeps the
# spec honest.
require "rails_helper"

RSpec.describe "session cookie attributes (R-QGB5-EMOO)", type: :request do
  it "R-QGB5-EMOO sets Secure, HttpOnly, and SameSite=Lax on the session cookie" do
    record, _ = OauthClient.register(
      client_name: "Session Cookie Test",
      redirect_uris: [ "https://client.example.com/cb" ],
      token_endpoint_auth_method: "none"
    )

    get "/oauth/authorize",
        params: {
          client_id: record.client_id,
          redirect_uri: "https://client.example.com/cb",
          response_type: "code",
          scope: "openid",
          state: "client-state",
          code_challenge: "abc",
          code_challenge_method: "S256"
        },
        headers: { "X-Forwarded-Proto" => "https" }

    set_cookie = response.headers["Set-Cookie"]
    expect(set_cookie).to be_present, "expected a Set-Cookie header on a session-writing response"

    cookies = Array(set_cookie).flat_map { |h| h.split("\n") }
    session_cookie = cookies.find { |c| c.start_with?("_ouroboros_mcp_session=") }
    expect(session_cookie).to be_present,
                              "expected a _ouroboros_mcp_session cookie; got: #{cookies.inspect}"

    attrs = session_cookie.split(/;\s*/).map(&:downcase)
    expect(attrs).to include("secure")
    expect(attrs).to include("httponly")
    expect(attrs).to include("samesite=lax")
  end

  it "R-QGB5-EMOO Rails session_store is configured with secure, httponly, and same_site=lax" do
    options = Rails.application.config.session_options
    expect(Rails.application.config.session_store).to eq(ActionDispatch::Session::CookieStore)
    expect(options[:secure]).to eq(true)
    expect(options[:httponly]).to eq(true)
    expect(options[:same_site]).to eq(:lax)
  end
end
