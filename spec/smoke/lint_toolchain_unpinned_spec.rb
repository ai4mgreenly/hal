# R-DE3F-TY7F: lint-toolchain gems carry no version constraint.
require "rails_helper"

RSpec.describe "Gemfile lint toolchain" do
  describe "R-DE3F-TY7F lint toolchain unpinned" do
    it "lists every rubocop* gem without a version constraint" do
      gemfile = File.read(Rails.root.join("Gemfile"))
      active = gemfile.lines.reject { |l| l =~ /^\s*#/ }
      rubocop_lines = active.select { |l| l =~ /^\s*gem\s+["']rubocop[\w-]*["']/ }

      expect(rubocop_lines).not_to be_empty,
        "expected at least one rubocop* gem entry in Gemfile"

      rubocop_lines.each do |line|
        # After the gem name, the only allowed args are options like
        # `require: false`. A version literal would appear as a second
        # positional string argument: `, "1.2.3"` or `, '~> 1.2'`.
        without_name = line.sub(/^\s*gem\s+["']rubocop[\w-]*["']/, "")
        expect(without_name).not_to match(/,\s*["'][^"']+["']/),
          "rubocop* gem must be unpinned: #{line.strip}"
      end
    end
  end
end
