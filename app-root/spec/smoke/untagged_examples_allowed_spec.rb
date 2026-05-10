# R-H74C-7WFF: not every example must carry a requirement ID. The
# traceability requirement is one-way — every claimed ID is locatable,
# but examples without an ID are tolerated by the suite.
require "rails_helper"

ID_RE_H74C = /R-[A-Z0-9]{4}-[A-Z0-9]{4}/

# An intentionally untagged describe — used as evidence by the
# verifying example below that the suite tolerates examples whose
# full descriptions carry no requirement ID at all.
RSpec.describe "helper smoke (intentionally untagged)" do
  it "is a no-op helper example" do
    expect(true).to be(true)
  end
end

RSpec.describe "Untagged examples allowed" do
  describe "R-H74C-7WFF helper and untagged examples are tolerated" do
    def all_full_descriptions
      out = []
      walk = lambda do |group, parent|
        desc = [ parent, group.description.to_s ].reject(&:empty?).join(" ").strip
        group.examples.each { |ex| out << "#{desc} #{ex.description}".strip }
        group.children.each { |child| walk.call(child, desc) }
      end
      RSpec.world.example_groups.each { |g| walk.call(g, "") }
      out
    end

    it "the suite contains at least one example whose full description carries no requirement ID" do
      untagged = all_full_descriptions.reject { |d| d.match?(ID_RE_H74C) }
      expect(untagged).not_to be_empty,
                              "expected at least one untagged example to demonstrate the one-way trace"
    end
  end
end
