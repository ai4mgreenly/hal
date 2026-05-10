# R-ELEJ-EV2V: the standard scripts self-activate the project's Ruby
# environment so a fresh login shell at the repo root can invoke them
# directly with no prior `rvm use`, `bundle exec`, or `source .envrc`.
require "rails_helper"

RSpec.describe "standard scripts" do
  describe "R-ELEJ-EV2V self-activate environment" do
    %w[launch.sh test.sh lint.sh].each do |script|
      context script do
        let(:body) { File.read(Rails.root.join(script)) }

        it "uses set -euo pipefail" do
          expect(body).to match(/^\s*set -euo pipefail\b/)
        end

        it "cd's to its own directory" do
          expect(body).to match(/cd "\$\(dirname "\$0"\)"/)
        end

        it "sources rvm and selects the pinned Ruby version" do
          expect(body).to match(/source "?\$HOME\/\.rvm\/scripts\/rvm"?/)
          expect(body).to match(/rvm use "?\$\(cat \.ruby-version\)"?/)
        end

        it "is executable" do
          expect(File.executable?(Rails.root.join(script))).to be true
        end
      end
    end
  end
end
