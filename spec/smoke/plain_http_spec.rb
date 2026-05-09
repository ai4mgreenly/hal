# R-PVA6-Q6OB: locally-launched service speaks plain HTTP, not HTTPS.
require "rails_helper"

RSpec.describe "launch transport" do
  describe "R-PVA6-Q6OB plain HTTP, no TLS termination" do
    it "does not enable force_ssl in production config" do
      prod = File.read(Rails.root.join("config/environments/production.rb"))
      active = prod.lines.reject { |l| l =~ /^\s*#/ }.join
      expect(active).not_to match(/^\s*config\.force_ssl\s*=\s*true/)
      expect(active).not_to match(/^\s*config\.assume_ssl\s*=\s*true/)
    end

    it "launch.sh does not bind a TLS listener" do
      launch = File.read(Rails.root.join("launch.sh"))
      active = launch.lines.reject { |l| l =~ /^\s*#/ }.join
      expect(active).not_to match(/--ssl|\btls\b|https:\/\//i)
    end
  end
end
