# R-68WP-XVCK: Google client credentials must be sourced from environment
# configuration, never committed to the repository.
require "rails_helper"

RSpec.describe "Google client credentials posture" do
  describe "R-68WP-XVCK Google client credentials are env-only and uncommitted" do
    it "no tracked file contains a Google OAuth client-secret value" do
      offenders = []
      Dir.glob(Rails.root.join("**", "*"), File::FNM_DOTMATCH).each do |path|
        next unless File.file?(path)
        rel = path.to_s.sub("#{Rails.root}/", "")
        next if rel.start_with?(".git/", "log/", "tmp/", "storage/")
        next if rel.start_with?("node_modules/", "vendor/bundle/", ".bundle/")
        contents = begin
          File.read(path, encoding: "BINARY")
        rescue StandardError
          nil
        end
        next unless contents
        offenders << rel if contents.match?(/GOCSPX-[A-Za-z0-9_-]{20,}/)
      end
      expect(offenders).to eq([])
    end

    it "any reference to google_client_id/secret in app/lib/config sources it from ENV" do
      paths = Dir.glob(Rails.root.join("{app,lib,config}", "**", "*.{rb,erb,yml}"))
      offenders = []
      paths.each do |path|
        File.read(path).each_line.with_index do |line, idx|
          next unless line =~ /google_client_(id|secret)/i
          next if line.lstrip.start_with?("#")
          offenders << "#{path}:#{idx + 1}" unless line.include?("ENV[")
        end
      end
      expect(offenders).to eq([])
    end
  end
end
