# R-RNRN-R4Y2: bundled Rails version must be in the 8.1.x line.
require "rails_helper"

RSpec.describe "Rails version" do
  describe "R-RNRN-R4Y2 rails version is 8.1.x" do
    it "Rails.version starts with 8.1." do
      expect(Rails.version).to match(/\A8\.1\./)
    end
  end
end
