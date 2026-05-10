# R-IPU6-RP6Q: persistence is SQLite, accessed through Active Record.
require "rails_helper"

RSpec.describe "Persistence" do
  describe "R-IPU6-RP6Q SQLite via Active Record" do
    it "uses the SQLite Active Record adapter" do
      expect(ActiveRecord::Base.connection.adapter_name).to match(/SQLite/i)
    end
  end
end
