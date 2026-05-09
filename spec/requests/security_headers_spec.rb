# Request specs for transport-security headers (reqs/auth.md).
require "rails_helper"

RSpec.describe "Transport security headers", type: :request do
  describe "R-ID5L-BSJM" do
    it "R-ID5L-BSJM emits X-Content-Type-Options: nosniff on every response" do
      get "/"
      expect(response.headers["X-Content-Type-Options"]).to eq("nosniff")

      get "/counter"
      expect(response.headers["X-Content-Type-Options"]).to eq("nosniff")

      get "/.well-known/oauth-authorization-server"
      expect(response.headers["X-Content-Type-Options"]).to eq("nosniff")
    end

    it "R-ID5L-BSJM emits HSTS with at least 1 year max-age and includeSubDomains when arrived via HTTPS proxy" do
      host! "ouroboros.ai.metaspot.org"
      get "/", headers: { "X-Forwarded-Proto" => "https" }

      hsts = response.headers["Strict-Transport-Security"]
      expect(hsts).to be_a(String)

      max_age = hsts[/max-age=(\d+)/, 1].to_i
      expect(max_age).to be >= 31_536_000
      expect(hsts).to match(/includeSubDomains/)
    end

    it "R-ID5L-BSJM omits HSTS on plain-HTTP local development requests" do
      host! "localhost:3000"
      get "/"

      expect(response.headers["Strict-Transport-Security"]).to be_nil
    end
  end
end
