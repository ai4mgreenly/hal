# R-COO1-AMEK: implementation language is Ruby (MRI). The exact
# version is pinned in environment.md.
require "rails_helper"

RSpec.describe "Implementation language" do
  describe "R-COO1-AMEK implementation language is Ruby (MRI)" do
    it ".ruby-version pins an MRI Ruby (no jruby/truffleruby/etc. prefix)" do
      pin = File.read(Rails.root.join(".ruby-version")).strip
      expect(pin).to match(/\A(?:ruby-)?\d+\.\d+\.\d+\z/)
    end
  end
end
