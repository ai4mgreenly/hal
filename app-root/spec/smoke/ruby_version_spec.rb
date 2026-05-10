# R-QC7K-U30Z: running Ruby version matches .ruby-version.
require "rails_helper"

RSpec.describe "Ruby runtime version" do
  describe "R-QC7K-U30Z ruby version matches .ruby-version" do
    it "RUBY_VERSION equals the version recorded in .ruby-version" do
      raw = File.read(Rails.root.join(".ruby-version")).strip
      expected = raw.sub(/\Aruby-/, "")
      expect(RUBY_VERSION).to eq(expected)
    end
  end
end
