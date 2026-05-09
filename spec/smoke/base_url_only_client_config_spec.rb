# Posture specs: an MCP client only needs the service's base URL to
# configure access (R-VVRG-W2G2), and the same base URL works from
# every targeted client without per-client configuration variants
# (R-WHPN-RXSK).
require "rails_helper"

RSpec.describe "Base-URL-only client configuration posture", type: :request do
  let(:metadata_path) { "/.well-known/oauth-authorization-server" }

  describe "R-VVRG-W2G2 base URL is sufficient to onboard a client" do
    it "publishes discovery + DCR with no Google details, no client credentials, and no off-origin paths" do
      get metadata_path

      expect(response).to have_http_status(:ok)
      body = JSON.parse(response.body)

      base = "#{request.protocol}#{request.host_with_port}"
      expect(body["issuer"]).to eq(base)

      # Self-onboarding: the metadata advertises a registration endpoint
      # so clients with the base URL alone can register via DCR — no
      # out-of-band credentials needed.
      expect(body["registration_endpoint"]).to be_a(String)
      expect(body["registration_endpoint"]).to start_with(base)

      # Every advertised endpoint sits on the issuer's origin: nothing
      # the client needs lives off-base.
      body.each_value do |v|
        next unless v.is_a?(String) && v.match?(%r{\Ahttps?://}i)
        expect(v).to start_with(base),
          "expected #{v.inspect} to share origin with issuer #{base.inspect}"
      end

      # The metadata document must not leak Google specifics — clients
      # are not required to know the upstream IDP.
      raw = response.body.downcase
      %w[google googleapis accounts.google gmail workspace].each do |needle|
        expect(raw).not_to include(needle),
          "metadata document leaks upstream IDP detail #{needle.inspect}"
      end

      # And it must not advertise a service-issued client credential —
      # those are handed out by DCR, not embedded in discovery.
      %w[client_id client_secret].each do |k|
        expect(body).not_to have_key(k),
          "metadata document must not embed #{k.inspect}"
      end
    end
  end

  describe "R-WHPN-RXSK same base URL works from every client" do
    it "returns byte-identical metadata regardless of User-Agent" do
      get metadata_path, headers: { "User-Agent" => "ClaudeCode/1.0" }
      first_status = response.status
      first_body   = response.body

      get metadata_path, headers: { "User-Agent" => "ClaudeDesktop/2.5" }
      second_status = response.status
      second_body   = response.body

      get metadata_path, headers: { "User-Agent" => "curl/8.4.0" }
      third_status = response.status
      third_body   = response.body

      expect([ first_status, second_status, third_status ]).to all(eq(200))
      expect(second_body).to eq(first_body)
      expect(third_body).to eq(first_body)
    end

    it "keeps the registration endpoint client-agnostic (no per-client branching in routes)" do
      # The set of routes the app exposes does not vary by client. We
      # cannot easily assert "no User-Agent branching" globally, but we
      # can pin that the public OAuth surface is a fixed, finite set —
      # any per-client variant would require additional routes.
      Rails.application.reload_routes_unless_loaded
      paths = Rails.application.routes.routes.map { |r| r.path.spec.to_s }

      oauth_paths = paths.grep(%r{\A/(oauth|\.well-known/oauth)})
      # Strip Rails' format suffix marker for stable comparison.
      normalized = oauth_paths.map { |p| p.sub(/\(\.:format\)\z/, "") }.sort.uniq

      expect(normalized).to contain_exactly(
        "/.well-known/oauth-authorization-server",
        "/.well-known/oauth-protected-resource",
        "/oauth/register",
        "/oauth/authorize",
        "/oauth/google/callback",
        "/oauth/token"
      )
    end
  end
end
