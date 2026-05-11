# R-9U99-YATL: the index page is rendered at high visual fidelity to
# the design reference at reqs/design/HAL.html. The load-bearing design
# tokens — warm off-white background (#f6f5f1), dark ink (#14130f), red
# HAL-lens accent (#d4361e), JetBrains Mono for code, Inter for UI, the
# 88px (≥640px) / 64px (<640px) HAL title type scale, and the literal
# `HAL` band-name title rendered in the index page — are property of the
# requirement and are pinned here. Verification means confirming the
# tokens are actually realized: declared in application.css and (for the
# title) rendered into the index page output.
require "rails_helper"

RSpec.describe "R-9U99-YATL design reference fidelity", type: :request do
  CSS_PATH_9U99 = Rails.root.join("app/assets/stylesheets/application.css")

  describe "R-9U99-YATL load-bearing design tokens are declared in application.css" do
    let(:css) { File.read(CSS_PATH_9U99) }

    it "pins the warm off-white background token #f6f5f1" do
      expect(css).to match(/#f6f5f1/i)
    end

    it "pins the dark ink token #14130f" do
      expect(css).to match(/#14130f/i)
    end

    it "pins the HAL-lens red accent token #d4361e" do
      expect(css).to match(/#d4361e/i)
    end

    it "declares Inter as the UI font family" do
      expect(css).to match(/"Inter"/)
    end

    it "declares JetBrains Mono as the code font family" do
      expect(css).to match(/"JetBrains Mono"/)
    end

    it "anchors the HAL title at 88px on viewports ≥640px and 64px on viewports <640px" do
      expect(css).to match(/\.banner\s+\.title\s*\{[^}]*font-size:\s*88px/m)
      expect(css).to match(/font-size:\s*64px/)
    end
  end

  describe "R-9U99-YATL the index page renders the literal `HAL` title" do
    it "renders <h1 class=\"title\">HAL</h1> as the banner title" do
      get "/"
      expect(response).to have_http_status(:ok)
      expect(response.body).to match(/<h1[^>]*class="[^"]*\btitle\b[^"]*"[^>]*>\s*HAL\s*<\/h1>/)
    end
  end
end
