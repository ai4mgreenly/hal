# R-901W-IT88: launch.sh applies pending migrations before the service
# begins accepting requests, so the first inbound request is served
# against an up-to-date schema regardless of whether the checkout is
# fresh, freshly-pulled, or already current.
require "rails_helper"

RSpec.describe "launch.sh schema-currency guarantee" do
  let(:script) do
    File.readlines(Rails.root.join("launch.sh"))
        .reject { |line| line.lstrip.start_with?("#") }
        .join
  end

  describe "R-901W-IT88 pending migrations applied before server start" do
    it "invokes a Rails migration step in the launch sequence" do
      expect(script).to match(/bin\/rails\s+db:(prepare|migrate)\b/)
    end

    it "runs the migration step before bin/rails server" do
      migrate_idx = script =~ /bin\/rails\s+db:(prepare|migrate)\b/
      server_idx = script =~ /bin\/rails\s+server\b/
      expect(migrate_idx).not_to be_nil
      expect(server_idx).not_to be_nil
      expect(migrate_idx).to be < server_idx
    end

    it "chains the migration step with && so a migration failure aborts the launch" do
      expect(script).to match(/bin\/rails\s+db:(prepare|migrate)\b[^\n]*&&[^\n]*bin\/rails\s+server\b/)
    end
  end
end
