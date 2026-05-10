# R-PP1H-KFXS: running interpreter must be MRI / CRuby.
require "rails_helper"

RSpec.describe "Ruby engine" do
  describe "R-PP1H-KFXS ruby engine is MRI/CRuby" do
    it "RUBY_ENGINE equals 'ruby'" do
      expect(RUBY_ENGINE).to eq("ruby")
    end
  end
end
