# R-V65K-UVVH: the legacy HTTP+SSE transport is not provided.
require "rails_helper"

RSpec.describe "Legacy HTTP+SSE transport absence" do
  describe "R-V65K-UVVH no SSE-flavored gems are bundled" do
    it "Gemfile.lock contains no SSE/eventsource gems" do
      lockfile = Rails.root.join("Gemfile.lock").read
      gem_names = lockfile.scan(/^    ([a-z0-9][a-z0-9_-]*) \(/i).flatten.map(&:downcase).uniq
      forbidden_substrings = %w[sse eventsource event-stream event_source]
      offenders = gem_names.select do |name|
        forbidden_substrings.any? { |s| name.include?(s) }
      end
      expect(offenders).to eq([])
    end
  end

  describe "R-V65K-UVVH no SSE-shaped routes are defined" do
    it "no route path matches /sse, /events, or /stream segments" do
      # R-K65O-80SH's /counter/stream is the live counter channel —
      # an unrelated SSE feature, not the legacy MCP transport that
      # R-V65K-UVVH forbids.
      sse_paths = Rails.application.routes.routes.map { |r| r.path.spec.to_s }.select do |path|
        path.match?(%r{/(sse|events|event-stream|stream)(\(|/|$)})
      end
      sse_paths -= [ "/counter/stream(.:format)" ]
      expect(sse_paths).to eq([])
    end
  end

  describe "R-V65K-UVVH no controller streams responses" do
    it "no controller class includes ActionController::Live" do
      Rails.application.eager_load!
      live_controllers = ApplicationController.descendants.select do |klass|
        klass.included_modules.include?(ActionController::Live)
      end
      expect(live_controllers).to eq([])
    end
  end

  describe "R-V65K-UVVH no source file emits or negotiates text/event-stream" do
    it "tracked controller/view/lib files contain no text/event-stream literal" do
      roots = %w[app/controllers app/views lib]
      # R-K65O-80SH's CounterStreamController serves SSE for the live
      # counter channel — not the legacy MCP transport R-V65K-UVVH
      # forbids — so it is exempt from this scan.
      allowed = [ Rails.root.join("app/controllers/counter_stream_controller.rb").to_s ]
      offenders = roots.flat_map do |root|
        Dir.glob(Rails.root.join(root, "**", "*")).select { |p| File.file?(p) }
      end.select do |path|
        File.read(path).include?("text/event-stream")
      end
      offenders -= allowed
      expect(offenders).to eq([])
    end
  end
end
