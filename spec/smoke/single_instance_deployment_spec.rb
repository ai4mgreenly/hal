# R-JBSD-NKJ8: deployment target is a single instance.
require "rails_helper"

RSpec.describe "Deployment posture" do
  describe "R-JBSD-NKJ8 single-instance Puma" do
    it "does not configure clustered Puma workers" do
      puma_config = Rails.root.join("config/puma.rb").read
      expect(puma_config).not_to match(/^\s*workers\s+/)
    end
  end
end
