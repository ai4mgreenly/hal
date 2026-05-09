# R-7NWT-PODV: .rubocop.yml enables Layout/LineLength with Max 120,
# overriding the omakase default that disables it.
require "rails_helper"
require "yaml"

RSpec.describe ".rubocop.yml" do
  describe "R-7NWT-PODV Layout/LineLength override" do
    it "enables Layout/LineLength with Max set to 120" do
      config = YAML.load_file(Rails.root.join(".rubocop.yml"))
      cop = config["Layout/LineLength"]

      expect(cop).to be_a(Hash),
        "expected .rubocop.yml to declare Layout/LineLength"
      expect(cop["Enabled"]).to eq(true),
        "expected Layout/LineLength to be Enabled: true"
      expect(cop["Max"]).to eq(120),
        "expected Layout/LineLength Max: 120"
    end
  end
end
