# R-SDDJ-SBIN: a fresh checkout reaches a passing test suite via a
# single documented setup command.
require "rails_helper"

RSpec.describe "Bootstrap" do
  describe "R-SDDJ-SBIN single documented setup command" do
    let(:root) { Rails.root }

    it "ships an executable bin/setup" do
      setup = root.join("bin/setup")
      expect(setup).to exist
      expect(File.executable?(setup)).to be true
    end

    it "documents the setup command in README.md" do
      readme = File.read(root.parent.join("README.md"))
      expect(readme).to match(/bin\/setup/)
    end

    it "runs bin/setup --skip-server to completion" do
      Dir.chdir(root) do
        out = `bin/setup --skip-server 2>&1`
        expect($?.exitstatus).to eq(0), "bin/setup failed:\n#{out}"
      end
    end
  end
end
