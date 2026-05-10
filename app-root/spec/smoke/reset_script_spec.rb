# R-1D56-BHLP: ./reset.sh brings the local-development database back to
# the state of a fresh, never-launched checkout — every application
# table gone, no schema present, no migrations applied. Test database
# is untouched.
require "rails_helper"

RSpec.describe "reset.sh" do
  let(:script_path) { Rails.root.join("reset.sh") }
  let(:body) { File.read(script_path) }

  describe "R-1D56-BHLP local-development database reset" do
    it "exists at the repo root" do
      expect(File.file?(script_path)).to be true
    end

    it "is executable" do
      expect(File.executable?(script_path)).to be true
    end

    it "removes the development sqlite database file" do
      dev_db = Rails.configuration.database_configuration["development"]["database"]
      expect(body).to include(dev_db)
    end

    it "does not touch the test database" do
      test_db = Rails.configuration.database_configuration["test"]["database"]
      effective = body.lines.reject { |l| l.lstrip.start_with?("#") }.join
      expect(effective).not_to include(test_db)
    end

    it "uses set -euo pipefail so a failure aborts the reset" do
      expect(body).to match(/^\s*set -euo pipefail\b/)
    end

    it "cd's to its own directory so it works from any cwd" do
      expect(body).to match(/cd "\$\(dirname "\$0"\)"/)
    end
  end
end
