# R-9VH6-C2KA: the index page honors visitor preferences for reduced
# motion and presents an accessible structure for the interactive
# controls it exposes. When prefers-reduced-motion: reduce is set, the
# page suppresses the lens-dot pulse, the subtitle fade-swap, the
# counter flash/delta animation, and hover-driven transforms. ARIA
# semantics: the +/- buttons carry aria-label="Increment"/"Decrement"
# and the HTML disabled attribute when no web session is active, and
# sit adjacent to an aria-live="polite" region around the counter
# value; the re-roll button carries aria-label="New subtitle"; the
# decorative lens dot and the footer status dot carry
# aria-hidden="true".
require "rails_helper"

RSpec.describe "R-9VH6-C2KA reduced motion and ARIA semantics", type: :request do
  CSS_PATH_9VH6 = Rails.root.join("app/assets/stylesheets/application.css")

  describe "R_9VH6_C2KA reduced-motion suppression in CSS" do
    let(:css) { File.read(CSS_PATH_9VH6) }

    it "declares a prefers-reduced-motion: reduce media block" do
      expect(css).to match(/@media\s*\(\s*prefers-reduced-motion\s*:\s*reduce\s*\)/)
    end

    it "suppresses the lens-dot pulse under reduced motion" do
      # The lens animation is set to none inside the reduced-motion block.
      block = css[/@media\s*\(\s*prefers-reduced-motion\s*:\s*reduce\s*\)\s*\{(?:[^{}]*\{[^{}]*\})*[^{}]*\}/m]
      expect(block).not_to be_nil, "expected a prefers-reduced-motion: reduce block in CSS"
      expect(block).to match(/\.banner\s+\.lens\b[^{]*\{[^}]*animation\s*:\s*none/m)
    end

    it "suppresses the subtitle fade transition under reduced motion" do
      block = css[/@media\s*\(\s*prefers-reduced-motion\s*:\s*reduce\s*\)\s*\{(?:[^{}]*\{[^{}]*\})*[^{}]*\}/m]
      expect(block).to match(/\.subtitle\b[^{]*\{[^}]*transition\s*:\s*none/m)
    end

    it "collapses all animation and transition durations under reduced motion" do
      # A universal rule zeroing animation-duration / transition-duration
      # is what suppresses the counter-flash, counter-delta, and any
      # hover-driven transform animations across the page.
      block = css[/@media\s*\(\s*prefers-reduced-motion\s*:\s*reduce\s*\)\s*\{(?:[^{}]*\{[^{}]*\})*[^{}]*\}/m]
      expect(block).to match(/\*,\s*\*::before,\s*\*::after\s*\{[^}]*animation-duration\s*:\s*0\.001ms/m)
      expect(block).to match(/\*,\s*\*::before,\s*\*::after\s*\{[^}]*transition-duration\s*:\s*0\.001ms/m)
    end
  end

  describe "R_9VH6_C2KA counter-flash JS gates on prefers-reduced-motion" do
    it "checks the prefers-reduced-motion media query before running the flash/delta" do
      get "/"
      body = response.body
      # The bootstrap script should consult matchMedia for the
      # prefers-reduced-motion: reduce preference and short-circuit
      # the visual transition when it matches.
      expect(body).to match(/matchMedia\(["']\(prefers-reduced-motion:\s*reduce\)["']\)/)
    end
  end

  describe "R_9VH6_C2KA ARIA on the counter buttons (signed-out)" do
    before { get "/" }
    let(:card) { response.body[%r{<section[^>]*id=["']counter-card["'][^>]*>.*?</section>}m] }

    it "labels the increment button" do
      expect(card).to match(/<button[^>]*aria-label=["']Increment["']/i)
    end

    it "labels the decrement button" do
      expect(card).to match(/<button[^>]*aria-label=["']Decrement["']/i)
    end

    it "marks both buttons disabled when no web session is active" do
      expect(card).to match(/<button[^>]*aria-label=["']Increment["'][^>]*disabled/i)
      expect(card).to match(/<button[^>]*aria-label=["']Decrement["'][^>]*disabled/i)
    end

    it "wraps the counter value in an aria-live=\"polite\" region" do
      expect(card).to match(%r{<span[^>]*aria-live=["']polite["'][^>]*>\s*<span[^>]*class=["'][^"']*\bcounter-value\b[^"']*["']}m)
    end
  end

  describe "R_9VH6_C2KA ARIA on re-roll and decorative elements" do
    before { get "/" }

    it "labels the re-roll control with aria-label=\"New subtitle\"" do
      expect(response.body).to match(/<button[^>]*id=["']reroll["'][^>]*aria-label=["']New subtitle["']/i)
    end

    it "marks the decorative lens dot aria-hidden=\"true\"" do
      expect(response.body).to match(/<span[^>]*class=["'][^"']*\blens\b[^"']*["'][^>]*aria-hidden=["']true["']/i)
    end

    it "marks the footer status dot aria-hidden=\"true\"" do
      expect(response.body).to match(/<span[^>]*class=["'][^"']*\bstatus-dot\b[^"']*["'][^>]*aria-hidden=["']true["']/i)
    end
  end
end
