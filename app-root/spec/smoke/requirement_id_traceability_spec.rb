# R-GJY8-Y9C8: a requirement ID referenced in an RSpec spec file must
# appear as a literal substring of some example's full description.
require "rails_helper"

RSpec.describe "Requirement ID traceability" do
  describe "R-GJY8-Y9C8 verified requirement IDs appear in example full descriptions" do
    ID_RE = /R-[A-Z0-9]{4}-[A-Z0-9]{4}/
    SELF_BASENAME = "requirement_id_traceability_spec.rb"

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

    it "every requirement ID referenced in a spec file appears in some example's full description" do
      descriptions = all_full_descriptions
      missing = []
      Dir[Rails.root.join("spec/**/*_spec.rb").to_s].each do |path|
        next if File.basename(path) == SELF_BASENAME
        ids = File.read(path).scan(ID_RE).uniq
        ids.each do |id|
          missing << "#{id} (in #{File.basename(path)})" unless descriptions.any? { |d| d.include?(id) }
        end
      end
      expect(missing).to be_empty,
                         "Requirement IDs missing from example descriptions:\n  #{missing.join("\n  ")}"
    end
  end
end
