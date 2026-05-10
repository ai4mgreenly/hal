# R-I2O3-I23J: web framework is Rails.
require "rails_helper"

RSpec.describe "Web framework" do
  describe "R-I2O3-I23J web framework is Rails" do
    it "boots a Rails::Application" do
      expect(defined?(Rails::Application)).to eq("constant")
      expect(Rails.application).to be_a(Rails::Application)
    end
  end
end
