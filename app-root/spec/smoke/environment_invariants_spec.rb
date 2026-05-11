require "rails_helper"

RSpec.describe "Environment invariants" do
  ROOT = Rails.root

  describe "R-EN0Y-1NKZ launch.sh starts the service" do
    it "R_EN0Y_1NKZ launch.sh exists at the repo root and is executable" do
      path = ROOT.join("launch.sh")
      expect(File.exist?(path)).to be(true), "launch.sh not found at repo root"
      expect(File.executable?(path)).to be(true), "launch.sh is not executable"
    end
  end

  describe "R-FA71-BAO6 launch.sh listens on TCP 3000" do
    it "R_FA71_BAO6 launch.sh runs bin/rails server with no port override" do
      body = File.read(ROOT.join("launch.sh"))
      expect(body).to include("bin/rails server"),
                      "launch.sh must boot the service via bin/rails server"
      expect(body).not_to match(/bin\/rails server[^\n]*\s-(?:p|-port)\b/),
                          "launch.sh must not override the Rails default port 3000"
    end
  end

  describe "R-FUXB-TE9Z test.sh runs the full test suite" do
    it "R_FUXB_TE9Z test.sh exists, is executable, and invokes rspec" do
      path = ROOT.join("test.sh")
      expect(File.exist?(path)).to be(true)
      expect(File.executable?(path)).to be(true)
      expect(File.read(path)).to match(/\bbundle exec rspec\b/)
    end
  end

  describe "R-GGVI-P9MH lint.sh runs Rubocop" do
    it "R_GGVI_P9MH lint.sh exists, is executable, and invokes rubocop" do
      path = ROOT.join("lint.sh")
      expect(File.exist?(path)).to be(true)
      expect(File.executable?(path)).to be(true)
      expect(File.read(path)).to match(/\bbundle exec rubocop\b/)
    end
  end

  describe "R-H6HE-QG72 Ruby dependencies managed with Bundler" do
    it "R_H6HE_QG72 Gemfile and Gemfile.lock both exist at the repo root" do
      expect(File.exist?(ROOT.join("Gemfile"))).to be(true)
      expect(File.exist?(ROOT.join("Gemfile.lock"))).to be(true)
    end
  end

  describe "R-HSFL-MBJK gems pinned to exact versions" do
    # R-DE3F-TY7F documents the rubocop-* family as the explicit exception.
    LINT_EXCEPTIONS = /\A(rubocop|rubocop-[a-z0-9_-]+)\z/

    it "R_HSFL_MBJK every Gemfile gem carries an exact version, except documented lint gems" do
      body = File.read(ROOT.join("Gemfile"))
      offenders = []
      body.each_line do |line|
        # Strip comments.
        code = line.sub(/#.*\z/, "").strip
        next unless code.start_with?("gem ")
        m = code.match(/\Agem\s+["']([^"']+)["'](.*)\z/)
        next unless m
        name = m[1]
        rest = m[2]
        # An exact pin looks like: , "1.2.3"  (a quoted version with no operator chars).
        has_exact_pin = rest =~ /,\s*["']\d[\w.\-+]*["']/
        next if has_exact_pin
        next if name.match?(LINT_EXCEPTIONS)
        offenders << name
      end
      expect(offenders).to be_empty,
                           "Gemfile entries missing an exact version pin: #{offenders.join(', ')}"
    end
  end

  describe "R-NWET-ALMZ .ruby-version pins one specific Ruby" do
    it "R_NWET_ALMZ .ruby-version exists and names a single specific patch version" do
      path = ROOT.join(".ruby-version")
      expect(File.exist?(path)).to be(true)
      raw = File.read(path).strip
      # Accept either "ruby-X.Y.Z" or "X.Y.Z" — single specific patch version.
      expect(raw).to match(/\A(?:ruby-)?\d+\.\d+\.\d+\z/),
                     ".ruby-version must name one specific patch version, got: #{raw.inspect}"
    end
  end

  describe "R-OJKW-K8Q6 pinned Ruby is in the 4.0.x line" do
    it "R_OJKW_K8Q6 .ruby-version is in the 4.0.x line" do
      raw = File.read(ROOT.join(".ruby-version")).strip
      version = raw.sub(/\Aruby-/, "")
      expect(version).to match(/\A4\.0\./),
                         "expected Ruby 4.0.x in .ruby-version, got: #{version}"
    end
  end

  describe "R-P4B7-2CBZ Rails pinned to 8.1.x via the Gemfile" do
    it "R_P4B7_2CBZ Gemfile pins rails to an 8.1.x exact version" do
      body = File.read(ROOT.join("Gemfile"))
      m = body.match(/^\s*gem\s+["']rails["']\s*,\s*["']([^"']+)["']/)
      expect(m).not_to be_nil, "Gemfile does not pin the rails gem"
      expect(m[1]).to match(/\A8\.1\.\d+\z/),
                      "Gemfile rails pin must be 8.1.x, got: #{m[1]}"
    end
  end

  describe "R-CQXC-KB48 rubocop-rails-omakase is the lint toolchain" do
    it "R_CQXC_KB48 Gemfile includes rubocop-rails-omakase" do
      body = File.read(ROOT.join("Gemfile"))
      expect(body).to match(/^\s*gem\s+["']rubocop-rails-omakase["']/),
                      "Gemfile must include rubocop-rails-omakase"
    end

    it "R_CQXC_KB48 .rubocop.yml inherits from rubocop-rails-omakase" do
      path = ROOT.join(".rubocop.yml")
      expect(File.exist?(path)).to be(true), ".rubocop.yml not found at repo root"
      expect(File.read(path)).to include("rubocop-rails-omakase"),
                                 ".rubocop.yml must reference rubocop-rails-omakase"
    end
  end

  describe "R-0MVV-UEZW tests are RSpec, not Minitest" do
    it "R_0MVV_UEZW Gemfile includes rspec-rails" do
      body = File.read(ROOT.join("Gemfile"))
      expect(body).to match(/^\s*gem\s+["']rspec-rails["']/),
                      "Gemfile must include rspec-rails"
    end

    it "R_0MVV_UEZW spec/ tree exists at the repo root" do
      expect(Dir.exist?(ROOT.join("spec"))).to be(true)
    end

    it "R_0MVV_UEZW test.sh invokes rspec, not minitest" do
      body = File.read(ROOT.join("test.sh"))
      expect(body).to match(/\brspec\b/)
      expect(body).not_to match(/\b(?:rake\s+test|minitest)\b/)
    end
  end
end
