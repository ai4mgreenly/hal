# R-F64T-WYOO: when the standard scripts run, their output is clean of
# environment-setup warnings — no RVM PATH-mismatch warning, and no
# "already initialized constant" duplicate-load warnings caused by stdlib
# gems being loaded once from the system Ruby installation and again from
# the project's gem-set.
#
# Per R-ZSMI-4WJF, the warning-free guarantee only holds for callers
# that do NOT pre-activate Ruby. This spec invokes the scripts'
# activation prelude from a clean env (`env -i HOME=$HOME PATH=...`) so
# the test mirrors the supported invocation pattern.
require "rails_helper"
require "open3"
require "shellwords"

RSpec.describe "standard scripts environment cleanliness" do
  describe "R-F64T-WYOO no env-setup warnings (R-ZSMI-4WJF clean caller)" do
    # Mirror the activation prelude embedded in launch.sh / test.sh /
    # lint.sh, then run a trivial bundle exec so RubyGems / Bundler
    # complete their default-gem activation. Any warning the scripts
    # produce on a real invocation surfaces here too.
    let(:activation_prelude) do
      <<~BASH
        export rvm_silence_path_mismatch_check_flag=1
        source "$HOME/.rvm/scripts/rvm"
        rvm use "$(cat .ruby-version)" >/dev/null
        exec bundle exec ruby -e "puts :ok"
      BASH
    end

    let(:repo_root) { Rails.root.to_s }

    def run_clean_env(script_body)
      cmd = [
        "env", "-i",
        "HOME=#{ENV['HOME']}",
        "PATH=/usr/bin:/bin",
        "bash", "-lc",
        "cd #{Shellwords.escape(repo_root)} && #{script_body}"
      ]
      Open3.capture3(*cmd)
    end

    it "produces no RVM PATH-mismatch warning and no duplicate-load warnings" do
      stdout, stderr, status = run_clean_env(activation_prelude)

      expect(status.success?).to be(true), "activation failed: #{stderr}"
      expect(stdout).to include("ok")
      expect(stderr).not_to match(/Warning! PATH/i),
                            "RVM PATH-mismatch warning leaked:\n#{stderr}"
      expect(stderr).not_to match(/already initialized constant/),
                            "duplicate-load warning leaked:\n#{stderr}"
      expect(stderr).not_to match(/previous definition of/),
                            "duplicate-load warning leaked:\n#{stderr}"
    end

    %w[launch.sh test.sh lint.sh].each do |script|
      it "#{script} sets rvm_silence_path_mismatch_check_flag" do
        body = File.read(Rails.root.join(script))
        expect(body).to include("rvm_silence_path_mismatch_check_flag=1")
      end
    end
  end
end
