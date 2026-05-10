# R-LWCN-ZBXO: every numeric and string value that governs the service's
# authentication posture is sourced from a single configuration surface,
# `Rails.configuration.x.auth.*`, populated by `config/initializers/auth.rb`.
require "rails_helper"

RSpec.describe "Auth configuration surface" do
  let(:auth) { Rails.configuration.x.auth }

  describe "R-LWCN-ZBXO single source of truth for auth-governing values" do
    it "exposes the token lifetimes (R-TNXJ-ZWQ0, R-8UAA-YKR9)" do
      expect(auth.access_token_lifetime).to eq(1.hour)
      expect(auth.refresh_token_lifetime).to eq(30.days)
    end

    it "exposes the web-session ceilings (R-KJ15-9P17)" do
      expect(auth.web_session_idle_ttl).to eq(1.hour)
      expect(auth.web_session_absolute_ttl).to eq(12.hours)
    end

    it "exposes the authorization-code TTL (R-ZPE1-0DV8)" do
      expect(auth.authorization_code_ttl).to eq(60.seconds)
    end

    it "exposes the Google OIDC scope list (R-W3K0-QD0E)" do
      expect(auth.google_scopes).to eq(%w[openid email profile])
    end

    it "exposes the per-flow forced-auth posture (R-3BKZ-L7R4, R-126C-AM1E)" do
      expect(auth.web_login_prompt).to eq("login")
      expect(auth.mcp_login_prompt).to be_nil
    end

    it "exposes the canonical resource identifier (R-3UT3-IKZG)" do
      expect(auth.canonical_url).to eq("http://www.example.com/")
    end

    it "exposes the workspace domain (R-5LQM-O89D) settable at deploy time" do
      expect(auth).to respond_to(:workspace_domain)
    end

    it "exposes the HSTS max-age (R-ID5L-BSJM)" do
      expect(auth.hsts_max_age).to eq(31_536_000)
    end

    it "exposes the Google client credentials (R-68WP-XVCK) populated from ENV" do
      expect(auth).to respond_to(:google_client_id)
      expect(auth).to respond_to(:google_client_secret)
    end

    it "no source file under app/, lib/, or non-auth initializers redefines these literals" do
      offenders = []
      patterns = {
        "1.hour access lifetime" => /\b1\.hour\b/,
        "30.days refresh lifetime" => /\b30\.days\b/,
        "12.hours absolute ceiling" => /\b12\.hours\b/,
        "60.seconds code TTL" => /\b60\.seconds\b/,
        "openid email profile scope literal" => /"openid email profile"/,
        "HSTS max-age literal" => /\b31_?536_?000\b/
      }
      paths = Dir.glob(Rails.root.join("{app,lib}", "**", "*.rb")) +
              Dir.glob(Rails.root.join("config/initializers", "*.rb")).reject { |p| p.end_with?("/auth.rb") }
      paths.each do |path|
        File.read(path).each_line.with_index do |line, idx|
          next if line.lstrip.start_with?("#")
          patterns.each do |label, re|
            offenders << "#{path}:#{idx + 1} (#{label})" if line =~ re
          end
        end
      end
      expect(offenders).to eq([])
    end
  end
end
