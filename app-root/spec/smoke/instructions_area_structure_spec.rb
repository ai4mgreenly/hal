# R-9WP2-PUAZ: the instructions area sits below the counter card and has
# the structure the design reference pins — an area header with an h2
# reading "Connect an MCP client" on the left and a 13px muted-ink
# subhead on the right reading "Point a client at <base-url>/mcp"
# (base-url derived from the request per R-CO4Y-11X7 / R-DA34-WX9P),
# followed by two section cards stacked vertically, each rendered as a
# visibly bordered, rounded card with its own off-white fill — not flat
# full-width sections divided only by whitespace. Each section card's
# header carries together a numeric badge (01/02) in mono inside a small
# rounded chip, the client title as the literal text "Claude Code" or
# "Claude Desktop" in 15px display weight, and a right-aligned 13px
# muted-ink description of the client kind. The copy button writes the
# executable form of the snippet to the clipboard — a leading "$ " or
# "> " shell-prompt prefix is stripped. Token coloring in code blocks
# uses warm tan #e0a96d for CLI flags, cool blue #a8c8ff for the
# command name, mint green #79d4a9 for URLs/strings, and muted gray
# #8c8a82 for prompts/punctuation.
require "rails_helper"

RSpec.describe "R-9WP2-PUAZ instructions area structure", type: :request do
  CSS_PATH_9WP2 = Rails.root.join("app/assets/stylesheets/application.css")
  VIEW_PATH_9WP2 = Rails.root.join("app/views/home/index.html.erb")

  describe "R-9WP2-PUAZ area header" do
    before { get "/" }

    it "renders the h2 'Connect an MCP client' inside the instructions head" do
      expect(response.body).to match(
        %r{<div[^>]*class="[^"]*\binstructions-head\b[^"]*"[^>]*>\s*<h2[^>]*>\s*Connect an MCP client\s*</h2>}m
      )
    end

    it "renders the muted subhead 'Point a client at <base-url>/mcp'" do
      expect(response.body).to match(
        %r{<p[^>]*class="[^"]*\binstructions-subhead\b[^"]*"[^>]*>\s*Point a client at\s*<span[^>]*>\s*http[s]?://[^<]+/mcp\s*</span>}
      )
    end

    it "styles the subhead at 13px muted-ink" do
      css = File.read(CSS_PATH_9WP2)
      expect(css).to match(
        /\.instructions-head\s+\.instructions-subhead\s*\{[^}]*font-size:\s*13px/m
      )
    end
  end

  describe "R-9WP2-PUAZ two stacked section cards" do
    before { get "/" }

    it "renders the Claude Code section card before the Claude Desktop section card" do
      body = response.body
      cc = body.index('id="section-claude-code"')
      cd = body.index('id="section-claude-desktop"')
      expect(cc).not_to be_nil
      expect(cd).not_to be_nil
      expect(cc).to be < cd
    end

    it "renders each section card with a numeric badge, client title, and kind description" do
      expect(response.body).to match(
        %r{<header[^>]*class="[^"]*\bsection-head\b[^"]*"[^>]*>\s*<span[^>]*class="[^"]*\bnum\b[^"]*"[^>]*>\s*01\s*</span>\s*<h3[^>]*>\s*Claude Code\s*</h3>\s*<span[^>]*class="[^"]*\bdesc\b[^"]*"[^>]*>[^<]*CLI[^<]*</span>}m
      )
      expect(response.body).to match(
        %r{<header[^>]*class="[^"]*\bsection-head\b[^"]*"[^>]*>\s*<span[^>]*class="[^"]*\bnum\b[^"]*"[^>]*>\s*02\s*</span>\s*<h3[^>]*>\s*Claude Desktop\s*</h3>\s*<span[^>]*class="[^"]*\bdesc\b[^"]*"[^>]*>}m
      )
    end

    it "styles each section card as a visibly bordered, rounded, off-white-filled card — not flat" do
      css = File.read(CSS_PATH_9WP2)
      section_rule = css[/#mcp-config\s+\.section\s*\{[^}]*\}/m]
      expect(section_rule).not_to be_nil, "expected a `#mcp-config .section` rule in application.css"
      expect(section_rule).to match(/border:\s*1px\s+solid/i)
      expect(section_rule).to match(/border-radius:\s*\d+px/i)
      expect(section_rule).to match(/background:\s*#[0-9a-f]{3,6}/i)
    end

    it "renders the client titles at 15px display weight" do
      css = File.read(CSS_PATH_9WP2)
      expect(css).to match(/#mcp-config\s+\.section-head\s+h3\s*\{[^}]*font-size:\s*15px/m)
    end

    it "renders the kind description as right-aligned 13px muted ink" do
      css = File.read(CSS_PATH_9WP2)
      expect(css).to match(
        /#mcp-config\s+\.section-head\s+\.desc\s*\{[^}]*margin-left:\s*auto[^}]*font-size:\s*13px/m
      )
    end
  end

  describe "R-9WP2-PUAZ instructions area sits below the counter card" do
    before { get "/" }

    it "the #mcp-config section appears after the #counter-card section in the page" do
      body = response.body
      cc = body.index('id="counter-card"')
      mc = body.index('id="mcp-config"')
      expect(cc).not_to be_nil
      expect(mc).not_to be_nil
      expect(cc).to be < mc
    end
  end

  describe "R-9WP2-PUAZ clipboard payload strips shell-prompt prefix" do
    it "the copy-button JS strips a leading '$ ' or '> ' before writing to the clipboard" do
      view = File.read(VIEW_PATH_9WP2)
      # The JS must apply a strip to the textContent rather than copying it raw.
      expect(view).to match(/textContent\.replace\(\s*\/\^\[\\?\$>\]\\s\+\/\s*,\s*"['"]?\s*"['"]?\s*\)/) \
        .or match(/textContent\.replace\(\s*\/\^\[\\?\$>\]\\s\+\/\s*,\s*['"]\s*['"]\s*\)/)
    end

    it "rendered Claude Code blocks visually carry a '$ ' prompt prefix" do
      get "/"
      expect(response.body).to match(
        %r{<pre[^>]*class="code"[^>]*id="claude-code-config-project"[^>]*>\s*<code>\s*<span[^>]*class="prompt"[^>]*>\s*\$\s*</span>}m
      )
    end
  end

  describe "R-9WP2-PUAZ token coloring in code blocks" do
    let(:css) { File.read(CSS_PATH_9WP2) }

    it "colors CLI flags warm tan #e0a96d" do
      expect(css).to match(/#mcp-config\s+pre\.code\s+\.flag\s*\{[^}]*color:\s*#e0a96d/i)
    end

    it "colors the command name cool blue #a8c8ff" do
      expect(css).to match(/#mcp-config\s+pre\.code\s+\.arg\s*\{[^}]*color:\s*#a8c8ff/i)
    end

    it "colors URLs and strings mint green #79d4a9" do
      expect(css).to match(/#mcp-config\s+pre\.code\s+\.url\s*\{[^}]*color:\s*#79d4a9/i)
      expect(css).to match(/#mcp-config\s+pre\.code\s+\.str\s*\{[^}]*color:\s*#79d4a9/i)
    end

    it "colors prompts and punctuation muted gray #8c8a82" do
      expect(css).to match(/#mcp-config\s+pre\.code\s+\.prompt\s*\{[^}]*color:\s*#8c8a82/i)
      expect(css).to match(/#mcp-config\s+pre\.code\s+\.punct\s*\{[^}]*color:\s*#8c8a82/i)
    end
  end
end
