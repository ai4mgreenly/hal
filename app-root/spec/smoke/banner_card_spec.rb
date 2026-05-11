# R-K7QI-9YW4: the index page presents the project name as a banner
# card with fixed chrome: a decorative pulsing lens dot (top-left,
# aria-hidden), a mono uppercase "MCP · demo" tag (top-right), the
# literal title "HAL" in display weight, an italic subtitle row drawn
# from the R-MG6P-TA7C bank, and a small circular re-roll control. The
# re-roll is rendered as a <button> (or other non-navigating element),
# carries aria-label="New subtitle", and on activation samples a fresh
# entry from the embedded bank with a ~220ms cross-fade — without
# issuing GET /, without navigating, and without losing scroll, focus,
# or the open SSE connection.
require "rails_helper"

RSpec.describe "R-K7QI-9YW4 banner card", type: :request do
  def banner_section
    response.body[%r{<section[^>]*class=["'][^"']*\bbanner\b[^"']*["'][^>]*>.*?</section>}m]
  end

  describe "R_K7QI_9YW4 chrome and structure" do
    before { get "/" }

    it "R_K7QI_9YW4 renders a banner section near the top of the page" do
      expect(response).to have_http_status(:ok)
      expect(banner_section).not_to be_nil
    end

    it "R_K7QI_9YW4 places a decorative lens dot inside the banner with aria-hidden=true" do
      section = banner_section
      expect(section).to match(%r{<span[^>]*class=["'][^"']*\blens\b[^"']*["'][^>]*aria-hidden=["']true["']}i)
    end

    it "R_K7QI_9YW4 places the 'MCP · demo' tag inside the banner" do
      section = banner_section
      tag = section[%r{<span[^>]*class=["'][^"']*\btag\b[^"']*["'][^>]*>(.*?)</span>}m, 1]
      expect(tag).not_to be_nil
      expect(CGI.unescapeHTML(tag).strip).to eq("MCP · demo")
    end

    it "R_K7QI_9YW4 renders the literal title 'HAL' (no periods)" do
      section = banner_section
      expect(section).to match(%r{<h1[^>]*class=["'][^"']*\btitle\b[^"']*["'][^>]*>\s*HAL\s*</h1>}i)
      title = section[%r{<h1[^>]*>\s*(.*?)\s*</h1>}m, 1]
      expect(title).to eq("HAL")
    end

    it "R_K7QI_9YW4 renders a subtitle row with #subtitle drawn from the R-MG6P-TA7C bank" do
      allow_any_instance_of(HomeController).to receive(:pick_subtitle).and_return("Headless Agent Loop")
      get "/"
      section = banner_section
      row = section[%r{<div[^>]*class=["'][^"']*\bsubtitle-row\b[^"']*["'][^>]*>.*?</div>}m]
      expect(row).not_to be_nil
      expect(row).to match(%r{<span[^>]*id=["']subtitle["'][^>]*>\s*Headless Agent Loop\s*</span>}i)
      expect(row).to match(%r{id=["']reroll["']})
    end
  end

  describe "R_K7QI_9YW4 re-roll control is a <button>, not a navigating <a>" do
    before { get "/" }

    it "R_K7QI_9YW4 renders the re-roll as a <button> element" do
      expect(response.body).to match(/<button[^>]*id=["']reroll["']/i)
    end

    it "R_K7QI_9YW4 does NOT render the re-roll as an <a> (forbidden by the spec)" do
      expect(response.body).not_to match(/<a[^>]*id=["']reroll["']/i)
    end

    it "R_K7QI_9YW4 labels the re-roll with aria-label=\"New subtitle\"" do
      reroll = response.body[/<button[^>]*id=["']reroll["'][^>]*>/i]
      expect(reroll).not_to be_nil
      expect(reroll).to match(/aria-label=["']New subtitle["']/i)
    end
  end

  describe "R_K7QI_9YW4 client-side sampling without a server round-trip" do
    before { get "/" }
    let(:body) { response.body }

    it "R_K7QI_9YW4 embeds the full R-MG6P-TA7C subtitle bank for client-side sampling" do
      bank_block = body[%r{<script[^>]*id=["']subtitle-bank["'][^>]*>(.*?)</script>}m, 1]
      expect(bank_block).not_to be_nil
      parsed = JSON.parse(bank_block)
      expect(parsed).to be_an(Array)
      # The bank carries the R-MG6P-TA7C 28-entry list.
      expect(parsed.size).to eq(HomeController::EXPANSIONS.size)
      expect(parsed).to match_array(HomeController::EXPANSIONS)
    end

    it "R_K7QI_9YW4 wires a click handler on #reroll that updates #subtitle in place" do
      # The bootstrap script binds to #reroll and rewrites the
      # #subtitle element's textContent — no fetch/XHR to /, no
      # navigation, no GET / is issued.
      expect(body).to match(/getElementById\(["']reroll["']\)/)
      expect(body).to match(/getElementById\(["']subtitle["']\)/)
      expect(body).to match(/getElementById\(["']subtitle-bank["']\)/)
      # No re-roll handler should fetch the root page.
      reroll_script = body[%r{<script[^>]*>(?:(?!</script>).)*?getElementById\(["']reroll["']\).*?</script>}m]
      expect(reroll_script).not_to be_nil
      expect(reroll_script).not_to match(%r{fetch\(["']/["']})
      expect(reroll_script).not_to match(/location\s*\.\s*(?:href|assign|replace)/)
    end
  end

  describe "R_K7QI_9YW4 subtitle cross-fade in CSS" do
    let(:css) { File.read(Rails.root.join("app/assets/stylesheets/application.css")) }

    it "R_K7QI_9YW4 declares a ~220ms opacity transition on .subtitle for the cross-fade" do
      sub_rule = css[/\.banner\s+\.subtitle\s*\{[^}]*\}/m]
      expect(sub_rule).not_to be_nil
      expect(sub_rule).to match(/transition\s*:\s*opacity\s+220ms/i)
    end
  end
end
