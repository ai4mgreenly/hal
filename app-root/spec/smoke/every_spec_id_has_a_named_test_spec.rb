# R-K9TD-DC0K: every requirement in ../reqs/ (IDs are two four-char
# upper-alphanumeric chunks joined by dashes, prefixed with R-) must be
# satisfied by at least one automated test that fails when violated,
# identified per R-GJY8-Y9C8 (ID appears as a literal substring of an
# example's full description). This spec asserts the coverage half of
# that requirement: for each ID found in the spec under ../reqs/, the
# suite must contain at least one example whose full description carries
# that ID. The "fails-on-violation" half is upheld by the surrounding
# test bodies the individual iterations wrote; this audit makes the
# coverage gap visible the moment a new requirement lands without a
# corresponding test.
require "rails_helper"

RSpec.describe "R-K9TD-DC0K every spec ID has a named test" do
  ID_RE_K9TD = /R-[A-Z0-9]{4}-[A-Z0-9]{4}/
  # Reserved placeholder per AGENTS.md — built from parts so this very
  # spec file does not itself contain the literal placeholder string
  # (which would otherwise trip the R-GJY8-Y9C8 traceability audit).
  PLACEHOLDER_K9TD = "R-#{"X" * 4}-#{"X" * 4}"
  REQS_DIR_K9TD = Rails.root.join("..", "reqs").expand_path

  def k9td_all_full_descriptions
    out = []
    walk = lambda do |group, parent|
      desc = [ parent, group.description.to_s ].reject(&:empty?).join(" ").strip
      group.examples.each { |ex| out << "#{desc} #{ex.description}".strip }
      group.children.each { |child| walk.call(child, desc) }
    end
    RSpec.world.example_groups.each { |g| walk.call(g, "") }
    out
  end

  # An ID occurrence is "retired-prose" if the word "prior" or "earlier"
  # appears between the start of its sentence and the ID. Such occurrences
  # are historical references in supersession prose, not requirements
  # needing test coverage. An ID counts as a real requirement only if it
  # has at least one non-retired occurrence somewhere in the spec.
  RETIRED_PROSE_RE_K9TD = /\b(?:prior|earlier)\b[^.\n]*\z/m

  def k9td_spec_ids
    has_active_occurrence = Hash.new(false)
    Dir[REQS_DIR_K9TD.join("*.md").to_s].sort.each do |path|
      text = File.read(path)
      text.scan(ID_RE_K9TD) do
        m = Regexp.last_match
        id = m[0]
        next if id == PLACEHOLDER_K9TD
        preceding = text[[ 0, m.begin(0) - 80 ].max...m.begin(0)]
        retired = !!(preceding =~ RETIRED_PROSE_RE_K9TD)
        has_active_occurrence[id] ||= !retired
      end
    end
    has_active_occurrence.select { |_, v| v }.keys.sort
  end

  it "R_K9TD_DC0K reqs directory is present and contains at least one spec ID" do
    expect(Dir.exist?(REQS_DIR_K9TD)).to be(true),
                                         "../reqs/ not found relative to Rails.root"
    expect(k9td_spec_ids).not_to be_empty
  end

  it "R_K9TD_DC0K every requirement ID in ../reqs/ appears in some example's full description" do
    descriptions = k9td_all_full_descriptions
    missing = k9td_spec_ids.reject { |id| descriptions.any? { |d| d.include?(id) } }
    expect(missing).to be_empty,
                       "Requirement IDs from ../reqs/ with no covering example:\n  #{missing.join("\n  ")}"
  end
end
