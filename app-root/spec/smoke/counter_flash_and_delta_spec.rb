# R-NG6O-94I2: directly below the banner the page renders a counter
# card with the chrome the design reference pins (≥640px row /
# <640px column; 56/44px monospaced value; ~42x42 +/- buttons with
# 1px border, 8px radius, 20px glyph; aria-label="Decrement" and
# "Increment"; HTML disabled attribute when no web session; hint
# line beneath). On value change the displayed count flashes in
# the accent color for ~600ms and a small mint-green delta
# indicator (`+1` / `−1`) slides in a few pixels and fades out
# over ~900ms, suppressed under prefers-reduced-motion.
require "rails_helper"

RSpec.describe "R-NG6O-94I2 counter card chrome + flash and delta animation", type: :request do
  CSS_PATH_NG6O = Rails.root.join("app/assets/stylesheets/application.css")
  ERB_PATH_NG6O = Rails.root.join("app/views/home/index.html.erb")

  describe "R_NG6O_94I2 signed-out counter card markup" do
    before { get "/" }

    it "renders a counter-card with label, value, and decrement/increment buttons" do
      expect(response).to have_http_status(:ok)
      expect(response.body).to include('class="counter-card"')
      expect(response.body).to match(/<p class="counter-label">CURRENT COUNT<\/p>/)
      expect(response.body).to match(/<span class="counter-value" id="count">\d+<\/span>/)
    end

    it "renders the counter value inside an aria-live=polite region" do
      expect(response.body).to match(/<span aria-live="polite"><span class="counter-value"/)
    end

    it "renders +/- buttons with aria-labels and the disabled attribute when signed-out" do
      expect(response.body).to match(/<button[^>]*aria-label="Decrement"[^>]*disabled[^>]*>−<\/button>/)
      expect(response.body).to match(/<button[^>]*aria-label="Increment"[^>]*disabled[^>]*>\+<\/button>/)
    end

    it "renders the signed-out hint line directing the visitor to sign in" do
      expect(response.body).to include("Sign in to manipulate the counter from the browser.")
    end
  end

  describe "R_NG6O_94I2 counter card chrome in CSS" do
    let(:css) { File.read(CSS_PATH_NG6O) }

    it "pins the 1px border + 8px radius + 42x42 + 20px glyph button chrome" do
      block = css[/\.counter-card\s+\.counter-button\s*\{[^}]*\}/m]
      expect(block).not_to be_nil, "expected a .counter-card .counter-button rule"
      expect(block).to match(/width\s*:\s*42px/)
      expect(block).to match(/height\s*:\s*42px/)
      expect(block).to match(/border\s*:\s*1px\s+solid/)
      expect(block).to match(/border-radius\s*:\s*8px/)
      expect(block).to match(/font-size\s*:\s*20px/)
    end

    it "pins the disabled button styling (~40% opacity, cursor: not-allowed)" do
      block = css[/\.counter-card\s+\.counter-button\[disabled\]\s*\{[^}]*\}/m]
      expect(block).not_to be_nil
      expect(block).to match(/opacity\s*:\s*0\.4/)
      expect(block).to match(/cursor\s*:\s*not-allowed/)
    end

    it "pins the 56px counter value at >=640px and the 44px override at <640px" do
      large = css[/\.counter-card\s+\.counter-value\s*\{[^}]*\}/m]
      expect(large).not_to be_nil
      expect(large).to match(/font-size\s*:\s*56px/)

      small_blocks = css.scan(/@media\s*\(\s*max-width\s*:\s*639px\s*\)\s*\{(?:[^{}]*\{[^{}]*\})*[^{}]*\}/m)
      expect(small_blocks).not_to be_empty, "expected a max-width: 639px breakpoint"
      expect(small_blocks.join("\n")).to match(/\.counter-card\s+\.counter-value\s*\{[^}]*font-size\s*:\s*44px/m)
    end

    it "lays out the counter-row horizontally at >=640px and collapses to column at <640px" do
      row = css[/\.counter-card\s+\.counter-row\s*\{[^}]*\}/m]
      expect(row).not_to be_nil
      expect(row).to match(/display\s*:\s*flex/)

      small_blocks = css.scan(/@media\s*\(\s*max-width\s*:\s*639px\s*\)\s*\{(?:[^{}]*\{[^{}]*\})*[^{}]*\}/m)
      expect(small_blocks.join("\n")).to match(/\.counter-card\s+\.counter-row\s*\{[^}]*flex-direction\s*:\s*column/m)
    end
  end

  describe "R_NG6O_94I2 flash + delta CSS hooks" do
    let(:css) { File.read(CSS_PATH_NG6O) }

    it "defines a .counter-flash rule that uses the accent color" do
      block = css[/\.counter-value\.counter-flash\s*\{[^}]*\}/m]
      expect(block).not_to be_nil, "expected a .counter-value.counter-flash CSS rule"
      expect(block).to match(/color\s*:\s*var\(--accent\)|color\s*:\s*#d4361e/i)
    end

    it "defines a .counter-delta rule rendered in mint green that animates" do
      block = css[/\.counter-card\s+\.counter-delta\s*\{[^}]*\}/m]
      expect(block).not_to be_nil, "expected a .counter-card .counter-delta CSS rule"
      expect(block).to match(/color\s*:\s*#79d4a9/i)
      expect(block).to match(/animation\s*:/)
    end

    it "defines a keyframes animation for the delta slide-and-fade" do
      expect(css).to match(/@keyframes\s+counter-delta-slide\s*\{/)
    end
  end

  describe "R_NG6O_94I2 flash + delta JS bootstrap in index.html.erb" do
    let(:erb) { File.read(ERB_PATH_NG6O) }

    it "wires a flashAndDelta function gated on prefers-reduced-motion" do
      expect(erb).to match(/function\s+flashAndDelta\s*\(/)
      expect(erb).to match(/matchMedia\(\s*["']\(prefers-reduced-motion:\s*reduce\)["']\s*\)/)
    end

    it "adds the .counter-flash class for ~600ms" do
      expect(erb).to match(/classList\.add\(["']counter-flash["']\)/)
      expect(erb).to match(/classList\.remove\(["']counter-flash["']\).*\}\s*,\s*600\s*\)/m)
    end

    it "renders a .counter-delta element labeled with + or − and removes it after ~900ms" do
      expect(erb).to match(/className\s*=\s*["']counter-delta["']/)
      expect(erb).to match(/\(next\s*>\s*prev\s*\?\s*["']\+["']\s*:\s*["']−["']\)/)
      expect(erb).to match(/removeChild\(delta\).*\}\s*,\s*900\s*\)/m)
    end
  end
end
