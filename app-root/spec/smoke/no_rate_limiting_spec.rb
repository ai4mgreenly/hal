require "rails_helper"

RSpec.describe "Rate-limiting posture" do
  describe "R-M04F-VG43 no rate limiting, quotas, or abuse protection" do
    it "loads no rate-limit gems in Gemfile.lock" do
      lock = Rails.root.join("Gemfile.lock").read
      forbidden = lock.scan(/^\s+(rack-attack|rack-throttle|rack-defense|rack-ratelimit|prosopite)\b/).flatten.uniq
      expect(forbidden).to eq([])
    end

    it "mounts no rate-limit middleware" do
      names = Rails.application.middleware.map { |m| m.klass.name.to_s }
      forbidden = names.grep(/(Attack|Throttle|RateLimit|Ratelimit)/)
      expect(forbidden).to eq([])
    end
  end
end
