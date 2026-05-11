# Request specs for the browser-facing index page (reqs/web.md).
require "rails_helper"

RSpec.describe "Home page", type: :request do
  describe "GET /" do
    it "R-QY5R-PYDH renders the current count as a number in plain HTML with no auth required" do
      Counter.current.update!(value: 42)

      get "/"

      expect(response).to have_http_status(:ok)
      expect(response.media_type).to eq("text/html")
      expect(response.body).to include("42")
    end

    it "R-QY5R-PYDH reflects updates to the counter value" do
      Counter.current.update!(value: 0)
      get "/"
      expect(response.body).to include(">0<")

      Counter.current.update!(value: 9)
      get "/"
      expect(response.body).to include(">9<")
    end

    it "R-TK21-6AGY is usable with JavaScript disabled" do
      Counter.current.update!(value: 7)

      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      # The count is present in the server-rendered body without needing JS.
      expect(body).to match(/>\s*7\s*</)

      # No JS module imports or importmap references.
      expect(body).not_to match(/type=["']module["']/i)
      expect(body).not_to match(/importmap/i)
      # No data-turbo-driven mutation hooks tied to the count.
      expect(body).not_to match(/data-turbo-stream/i)

      # The R-K65O-80SH live-update bootstrap is the only permitted
      # script and is a progressive enhancement: the count above is
      # already rendered by the server, the +/- forms function via
      # POST + 303 redirect (R-YJ05-EQRH), and the script merely
      # subscribes to the SSE channel for in-place updates.
    end

    it "R-BZQY-DN3B displays MCP client config for Claude Code and Desktop with copy-pasteable instructions" do
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      # Both clients are covered.
      expect(body).to match(/Claude Code/i)
      expect(body).to match(/Claude Desktop/i)

      # Each block is rendered in a <pre> for copy-paste.
      expect(body).to match(%r{<pre[^>]*id=["']claude-code-config-(?:project|user)["'][^>]*>.*</pre>}m)
      expect(body).to match(%r{<pre[^>]*id=["']claude-desktop-config["'][^>]*>.*</pre>}m)

      # The base URL appears in both blocks.
      expect(body).to include("http://www.example.com/mcp")
      # Claude Desktop block contains the mcpServers JSON key.
      expect(body).to match(/mcpServers/)

      # No Google details, no client credentials, no internal paths beyond base URL.
      expect(body).not_to match(/google/i)
      expect(body).not_to match(/client_secret/i)
      expect(body).not_to match(/client_id/i)
      expect(body).not_to match(%r{/oauth/})
      expect(body).not_to match(%r{\.well-known})
    end

    it "R-5GQZ-KWCD shows each client's instructions in that client's documented format" do
      host! "hal.ai.metaspot.org"
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      # Claude Code: a `claude mcp add --transport http …` line in each scope-flagged block.
      # The block carries semantic token-coloring spans (R-YHS9-0Z0S); strip inner
      # tags before comparing the plaintext, and tolerate a leading `$ ` prompt.
      code_text = lambda do |id|
        m = body.match(%r{<pre[^>]*id=["']#{Regexp.escape(id)}["'][^>]*>\s*<code[^>]*>(?<inner>.*?)</code>\s*</pre>}m)
        next nil if m.nil?
        m[:inner].gsub(/<[^>]+>/, "").sub(/\A\s*\$\s+/, "").strip
      end

      expect(code_text.call("claude-code-config-project")).to eq(
        "claude mcp add --transport http --scope project hal http://hal.ai.metaspot.org/mcp"
      )
      expect(code_text.call("claude-code-config-user")).to eq(
        "claude mcp add --transport http --scope user hal http://hal.ai.metaspot.org/mcp"
      )

      # Claude Desktop: a `mcpServers` JSON block with type "http" and the URL.
      desktop_match = body.match(
        %r{<pre[^>]*id=["']claude-desktop-config["'][^>]*>\s*<code[^>]*>(?<json>.*?)</code>\s*</pre>}m
      )
      expect(desktop_match).not_to be_nil
      desktop_json_text = desktop_match[:json].gsub(/<[^>]+>/, "")
      json = JSON.parse(desktop_json_text)
      expect(json).to have_key("mcpServers")
      expect(json["mcpServers"]).to have_key("hal")
      expect(json["mcpServers"]["hal"]).to eq(
        "type" => "http",
        "url" => "http://hal.ai.metaspot.org/mcp"
      )
    end

    it "R-9T1D-KJ2W ships both Claude Code scope snippets in the rendered HTML, each copy-pasteable" do
      host! "hal.ai.metaspot.org"
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      project_match = body.match(
        %r{<pre[^>]*id=["']claude-code-config-project["'][^>]*>\s*<code[^>]*>(?<cmd>.*?)</code>\s*</pre>}m
      )
      user_match = body.match(
        %r{<pre[^>]*id=["']claude-code-config-user["'][^>]*>\s*<code[^>]*>(?<cmd>.*?)</code>\s*</pre>}m
      )
      expect(project_match).not_to be_nil
      expect(user_match).not_to be_nil

      strip_cmd = ->(raw) { raw.gsub(/<[^>]+>/, "").sub(/\A\s*\$\s+/, "").strip }
      project_cmd = strip_cmd.call(project_match[:cmd])
      user_cmd = strip_cmd.call(user_match[:cmd])

      expect(project_cmd).to start_with("claude mcp add")
      expect(project_cmd).to include("--scope project")
      expect(project_cmd).to include("http://hal.ai.metaspot.org/mcp")

      expect(user_cmd).to start_with("claude mcp add")
      expect(user_cmd).to include("--scope user")
      expect(user_cmd).to include("http://hal.ai.metaspot.org/mcp")

      # The Claude Desktop block remains unchanged in shape.
      expect(body).to match(%r{<pre[^>]*id=["']claude-desktop-config["']})
    end

    it "R-CO4Y-11X7 derives the MCP base URL from the request host" do
      host! "localhost:3000"
      get "/"
      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to include("http://localhost:3000/mcp")
      expect(body).not_to include("www.example.com")

      host! "hal.ai.metaspot.org"
      get "/"
      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to include("http://hal.ai.metaspot.org/mcp")
      expect(body).not_to include("localhost:3000")
      expect(body).not_to include("www.example.com")
    end

    it "R-DA34-WX9P honors X-Forwarded-Proto from a TLS-terminating proxy" do
      host! "hal.ai.metaspot.org"
      get "/", headers: { "X-Forwarded-Proto" => "https" }

      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to include("https://hal.ai.metaspot.org/mcp")
      expect(body).not_to include("http://hal.ai.metaspot.org/mcp")
    end

    context "R-JXMD-JMLM banner and subtitle" do
      let(:expansions) { HomeController::EXPANSIONS }

      it "R-JXMD-JMLM shows the project name as the banner heading" do
        # R-N3CT-2XAJ replaces the earlier H.A.L. spelling with bare HAL.
        get "/"

        expect(response).to have_http_status(:ok)
        expect(response.body).to match(%r{<h1[^>]*class=["'][^"']*\btitle\b[^"']*["'][^>]*>\s*HAL\s*</h1>}i)
      end

      it "R-JXMD-JMLM shows exactly one expansion from the fixed list as the subtitle" do
        expansion = expansions.first
        allow_any_instance_of(HomeController).to receive(:pick_subtitle).and_return(expansion)

        get "/"

        # The visible subtitle element renders the one chosen expansion;
        # the others do not appear in the rendered subtitle. (Per
        # R-K7QI-9YW4 the full bank IS embedded as JSON elsewhere on the
        # page for client-side re-roll sampling — that data island is
        # not part of the rendered subtitle text.)
        subtitle = response.body[%r{<span[^>]*id=["']subtitle["'][^>]*>(.*?)</span>}m, 1]
        expect(subtitle).to eq(expansion)
        (expansions - [ expansion ]).each do |other|
          expect(subtitle).not_to include(other)
        end
      end

      it "R-JXMD-JMLM every expansion in the list can be shown as the subtitle" do
        expansions.each do |expansion|
          allow_any_instance_of(HomeController).to receive(:pick_subtitle).and_return(expansion)
          get "/"
          expect(response.body).to include(expansion), "expected #{expansion.inspect} to appear when sampled"
        end
      end

      it "R-JXMD-JMLM successive requests can produce different subtitles" do
        call_count = 0
        allow_any_instance_of(HomeController).to receive(:pick_subtitle) do
          call_count += 1
          call_count == 1 ? expansions[0] : expansions[1]
        end

        get "/"
        first_body = response.body
        get "/"
        second_body = response.body

        expect(first_body).to include(expansions[0])
        expect(second_body).to include(expansions[1])
      end

      it "R-K7QI-9YW4 provides a re-roll control rendered as a non-navigating button" do
        get "/"

        expect(response).to have_http_status(:ok)
        # R-K7QI-9YW4 explicitly forbids an <a> whose href would navigate
        # the browser away from the current page; the control is a
        # <button> (or another non-navigating element).
        expect(response.body).to match(/<button[^>]*id=["']reroll["']/)
        expect(response.body).not_to match(/<a[^>]*id=["']reroll["']/)
      end
    end

    context "R-N3CT-2XAJ banner card" do
      def banner_section
        body = response.body
        body[%r{<section[^>]*class=["'][^"']*\bbanner\b[^"']*["'][^>]*>.*?</section>}m]
      end

      it "R_N3CT_2XAJ renders the banner card as a section.banner near the top of the page" do
        get "/"

        expect(response).to have_http_status(:ok)
        body = response.body
        banner_idx = body.index(%r{<section[^>]*class=["'][^"']*\bbanner\b})
        mcp_idx = body.index("mcp-config")
        expect(banner_idx).not_to be_nil
        expect(mcp_idx).not_to be_nil
        expect(banner_idx).to be < mcp_idx
      end

      it "R_N3CT_2XAJ places a decorative lens dot inside the banner with aria-hidden=true" do
        get "/"

        section = banner_section
        expect(section).not_to be_nil
        expect(section).to match(%r{<span[^>]*class=["'][^"']*\blens\b[^"']*["'][^>]*aria-hidden=["']true["'][^>]*>}i)
      end

      it "R_N3CT_2XAJ places the MCP &middot; demo tag inside the banner" do
        get "/"

        section = banner_section
        expect(section).not_to be_nil
        tag = section[%r{<span[^>]*class=["'][^"']*\btag\b[^"']*["'][^>]*>(?<text>.*?)</span>}m]
        expect(tag).not_to be_nil
        # The visible text reads "MCP · demo" (the middle character is a middle dot).
        rendered_text = tag[%r{<span[^>]*>(?<t>.*?)</span>}, 1] || ""
        decoded = CGI.unescapeHTML(rendered_text).strip
        expect(decoded).to eq("MCP · demo")
      end

      it "R_N3CT_2XAJ renders HAL as the title (bare, no periods)" do
        get "/"

        section = banner_section
        expect(section).not_to be_nil
        expect(section).to match(%r{<h1[^>]*class=["'][^"']*\btitle\b[^"']*["'][^>]*>\s*HAL\s*</h1>}i)
        # The earlier H.A.L. spelling is no longer used in the title.
        title = section[%r{<h1[^>]*>\s*(?<text>[^<]*?)\s*</h1>}, 1]
        expect(title).to eq("HAL")
      end

      it "R_N3CT_2XAJ renders a subtitle row with the picked expansion and a re-roll control" do
        allow_any_instance_of(HomeController).to receive(:pick_subtitle).and_return("Headless Agent Loop")

        get "/"

        section = banner_section
        expect(section).not_to be_nil
        row = section[%r{<div[^>]*class=["'][^"']*\bsubtitle-row\b[^"']*["'][^>]*>.*?</div>}m]
        expect(row).not_to be_nil
        expect(row).to match(%r{<span[^>]*class=["'][^"']*\bsubtitle\b[^"']*["'][^>]*id=["']subtitle["'][^>]*>\s*Headless Agent Loop\s*</span>}i)
        # The re-roll control sits inside the subtitle row.
        expect(row).to match(%r{id=["']reroll["']})
      end

      it "R_N3CT_2XAJ the re-roll control carries aria-label=\"New subtitle\"" do
        get "/"

        section = banner_section
        expect(section).not_to be_nil
        reroll = section[%r{<button[^>]*id=["']reroll["'][^>]*>.*?</button>}m]
        expect(reroll).not_to be_nil
        expect(reroll).to match(/aria-label=["']New subtitle["']/i)
        # R-K7QI-9YW4: rendered as <button>, not an <a href="/"> that would
        # navigate; the re-roll mutates the subtitle in place via JS.
      end
    end

    context "R-MG6P-TA7C merged subtitle bank" do
      # The merged bank pins every entry that the spec lists verbatim,
      # in any order. The constant on the controller is the bank.
      let(:expected_entries) do
        [
          "Holistic Access Layer",
          "Human Augmentation Layer",
          "Heuristic Agent Liaison",
          "Home, APIs, Library",
          "Heuristically programmed ALgorithm",
          "Heuristically programmed ALgorithmic computer",
          "Helpful Autonomous Liaison",
          "Hyperlocal Agent Layer",
          "Host Agent Liaison",
          "Has Always Listened",
          "House Always Loses",
          "Hardware Abstraction Layer",
          "Hyperdimensional Access Layer",
          "Holistic Application Logic",
          "Highly Adaptive Listener",
          "Headless Agent Loop",
          "Hosted Action Library",
          "Hermetic Authorization Layer",
          "Hypertext Application Language",
          "High-Availability Lambda",
          "Heretical Automation Layer",
          "Hyper-tuned Agent Logic",
          "Handy Autoresponse Layer",
          "Hallucination Avoidance Layer",
          "Honest Assistant, Lately",
          "Halfway Awake Loop",
          "Homemade Agent Lab",
          "Heuristic Argument Linker"
        ]
      end

      it "R_MG6P_TA7C the bank contains every entry the spec pins" do
        expect(HomeController::EXPANSIONS).to match_array(expected_entries)
      end

      it "R_MG6P_TA7C every bank entry is reachable as a subtitle" do
        HomeController::EXPANSIONS.each do |entry|
          allow_any_instance_of(HomeController).to receive(:pick_subtitle).and_return(entry)
          get "/"
          expect(response.body).to include(entry), "expected #{entry.inspect} to render when sampled"
        end
      end

      it "R_MG6P_TA7C selection is uniform random across the bank" do
        # Draw many subtitles and confirm the empirical distribution
        # covers most of the bank — uniform random across N=28 over
        # 4000 draws is overwhelmingly likely to hit every entry.
        controller = HomeController.new
        draws = Array.new(4000) { controller.send(:pick_subtitle) }
        expect(draws.uniq.sort).to eq(HomeController::EXPANSIONS.sort)
      end

      it "R_MG6P_TA7C re-roll yields a freshly-picked entry on each activation" do
        calls = 0
        allow_any_instance_of(HomeController).to receive(:pick_subtitle) do
          calls += 1
          HomeController::EXPANSIONS[calls % HomeController::EXPANSIONS.length]
        end
        get "/"
        first_body = response.body
        get "/"
        second_body = response.body
        # Two successive renders pick different entries from the bank.
        expect(first_body).not_to eq(second_body)
      end
    end

  end

  describe "R-YLFY-6A8V index page reflects web-session state" do
    let(:provider) { Rails.configuration.x.google_identity_provider }

    around do |example|
      previous_domain = Rails.configuration.x.auth.workspace_domain
      Rails.configuration.x.auth.workspace_domain = "allowed.example"
      example.run
      Rails.configuration.x.auth.workspace_domain = previous_domain
    end

    def establish_web_session(email:)
      get "https://www.example.com/login"
      upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
      provider.stub_code(
        "code-home-#{email}",
        sub: "google-#{email}",
        email: email,
        hosted_domain: "allowed.example"
      )
      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-home-#{email}", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }
      expect(WebSession.find_by_presented_token(session[:web_session_id])&.owner).to eq(email)
    end

    it "R-YLFY-6A8V shows the visitor's email and a /logout affordance when signed in" do
      email = "signed-in@allowed.example"
      establish_web_session(email: email)

      get "https://www.example.com/"

      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to include(email)
      expect(body).to match(%r{<a[^>]*href=["']/logout["']}i)
      expect(body).not_to match(%r{<a[^>]*href=["']/login["']}i)

      # Placement is load-bearing: the affordance lives in the top-bar
      # row above the banner card, and renders as a pill (not a bare link).
      top_bar_idx = body.index(%r{<div[^>]*id=["']top-bar["']}i)
      banner_idx = body.index(%r{<section[^>]*class=["'][^"']*\bbanner\b}i)
      expect(top_bar_idx).not_to be_nil, "expected a #top-bar element"
      expect(banner_idx).not_to be_nil, "expected the banner section"
      expect(top_bar_idx).to be < banner_idx
      expect(body).to match(%r{<a\b(?=[^>]*\bhref=["']/logout["'])(?=[^>]*\bclass=["'][^"']*\bauth-pill\b)[^>]*>}i)
    end

    it "R-YLFY-6A8V shows a /login affordance and no placeholder identity when not signed in" do
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to match(%r{<a[^>]*href=["']/login["']}i)
      expect(body).not_to match(%r{<a[^>]*href=["']/logout["']}i)
      expect(body).not_to match(/guest/i)
      expect(body).not_to match(/anonymous/i)

      # Placement is load-bearing: the Sign in affordance lives in the
      # top-bar row above the banner card, and renders as a pill labeled
      # "Sign in" (not a bare text link).
      top_bar_idx = body.index(%r{<div[^>]*id=["']top-bar["']}i)
      banner_idx = body.index(%r{<section[^>]*class=["'][^"']*\bbanner\b}i)
      expect(top_bar_idx).not_to be_nil, "expected a #top-bar element"
      expect(banner_idx).not_to be_nil, "expected the banner section"
      expect(top_bar_idx).to be < banner_idx
      expect(body).to match(%r{<a\b(?=[^>]*\bhref=["']/login["'])(?=[^>]*\bclass=["'][^"']*\bauth-pill\b)[^>]*>\s*Sign in\s*</a>}i)
    end
  end

  describe "R-NJUD-EFQ5 index page reflects web-session state" do
    let(:provider) { Rails.configuration.x.google_identity_provider }

    around do |example|
      previous_domain = Rails.configuration.x.auth.workspace_domain
      Rails.configuration.x.auth.workspace_domain = "allowed.example"
      example.run
      Rails.configuration.x.auth.workspace_domain = previous_domain
    end

    def establish_web_session(email:)
      get "https://www.example.com/login"
      upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
      provider.stub_code(
        "code-njud-#{email}",
        sub: "google-#{email}",
        email: email,
        hosted_domain: "allowed.example"
      )
      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-njud-#{email}", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }
      expect(WebSession.find_by_presented_token(session[:web_session_id])&.owner).to eq(email)
    end

    it "R-NJUD-EFQ5 signed-in: email rendered as inert non-anchor text, with a separate Sign out pill linking to /logout" do
      email = "visitor@allowed.example"
      establish_web_session(email: email)

      get "https://www.example.com/"

      expect(response).to have_http_status(:ok)
      body = response.body

      # Email appears verbatim, and is NOT wrapped by any <a> tag — it is
      # a label, not a control.
      expect(body).to include(email)
      expect(body).not_to match(%r{<a\b[^>]*>[^<]*#{Regexp.escape(email)}[^<]*</a>}i)

      # A separate, explicitly labeled `Sign out` pill links to /logout.
      expect(body).to match(
        %r{<a\b(?=[^>]*\bhref=["']/logout["'])(?=[^>]*\bclass=["'][^"']*\bauth-pill\b)[^>]*>\s*Sign out\s*</a>}i
      )
      expect(body).not_to match(%r{<a[^>]*href=["']/login["']}i)

      # Top-right anchoring: auth area sits in #top-bar above the banner.
      top_bar_idx = body.index(%r{<div[^>]*id=["']top-bar["']}i)
      banner_idx = body.index(%r{<section[^>]*class=["'][^"']*\bbanner\b}i)
      expect(top_bar_idx).not_to be_nil
      expect(banner_idx).not_to be_nil
      expect(top_bar_idx).to be < banner_idx
    end

    it "R-NJUD-EFQ5 signed-out: single Sign in pill linking to /login, no placeholder identity" do
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      expect(body).to match(
        %r{<a\b(?=[^>]*\bhref=["']/login["'])(?=[^>]*\bclass=["'][^"']*\bauth-pill\b)[^>]*>\s*Sign in\s*</a>}i
      )
      expect(body).not_to match(%r{<a[^>]*href=["']/logout["']}i)
      expect(body).not_to match(/guest/i)
      expect(body).not_to match(/anonymous/i)

      top_bar_idx = body.index(%r{<div[^>]*id=["']top-bar["']}i)
      banner_idx = body.index(%r{<section[^>]*class=["'][^"']*\bbanner\b}i)
      expect(top_bar_idx).not_to be_nil
      expect(banner_idx).not_to be_nil
      expect(top_bar_idx).to be < banner_idx
    end
  end

  describe "R-YJ05-EQRH counter card with +/- buttons" do
    let(:provider) { Rails.configuration.x.google_identity_provider }

    around do |example|
      previous_domain = Rails.configuration.x.auth.workspace_domain
      Rails.configuration.x.auth.workspace_domain = "allowed.example"
      example.run
      Rails.configuration.x.auth.workspace_domain = previous_domain
    end

    def establish_web_session!(email:)
      get "https://www.example.com/login"
      upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
      provider.stub_code(
        "code-nrqs-#{email}",
        sub: "google-#{email}",
        email: email,
        hosted_domain: "allowed.example"
      )
      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-nrqs-#{email}", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }
    end

    it "R-YJ05-EQRH renders a counter card with CURRENT COUNT label, the value, and decrement/increment buttons" do
      Counter.current.update!(value: 42)

      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      card = body[%r{<section[^>]*id=["']counter-card["'][^>]*>.*?</section>}m]
      expect(card).not_to be_nil, "expected a counter card section"
      expect(card).to match(/CURRENT COUNT/)
      expect(card).to match(/>\s*42\s*</)
      expect(card).to match(/aria-label=["']Decrement["']/i)
      expect(card).to match(/aria-label=["']Increment["']/i)

      # Geometry: card carries the .counter-card class (border/radius chrome
      # in CSS keys off this), and the buttons are grouped under a single
      # .counter-controls wrapper so they sit side-by-side on the right of
      # the row rather than interleaved with the value.
      expect(card).to match(/class=["'][^"']*\bcounter-card\b[^"']*["']/)
      expect(card).to match(%r{<div[^>]*class=["'][^"']*\bcounter-main\b[^"']*["']}m)
      expect(card).to match(%r{<div[^>]*class=["'][^"']*\bcounter-controls\b[^"']*["']}m)
      # The label/value wrapper precedes the controls wrapper in DOM order —
      # left edge first, right edge second.
      main_at = card.index(/<div[^>]*\bcounter-main\b/)
      controls_at = card.index(/<div[^>]*\bcounter-controls\b/)
      expect(main_at).not_to be_nil
      expect(controls_at).not_to be_nil
      expect(main_at).to be < controls_at
      # Both buttons live inside the controls wrapper (grouped on the right).
      controls_block = card[%r{<div[^>]*\bcounter-controls\b[^>]*>.*?</div>\s*</div>}m]
      expect(controls_block).not_to be_nil
      expect(controls_block).to match(/aria-label=["']Decrement["']/i)
      expect(controls_block).to match(/aria-label=["']Increment["']/i)

      # CSS pins the 1px border / 8px radius on .counter-button and the
      # 56px / 44px display-weight on .counter-value per the design ref.
      css = Rails.root.join("app/assets/stylesheets/application.css").read
      expect(css).to match(/\.counter-card\b[^{]*\{[^}]*border\s*:\s*1px\s+solid/m)
      expect(css).to match(/\.counter-button\b[^{]*\{[^}]*border\s*:\s*1px\s+solid/m)
      expect(css).to match(/\.counter-button\b[^{]*\{[^}]*border-radius\s*:\s*8px/m)
      expect(css).to match(/\.counter-value\b[^{]*\{[^}]*font-size\s*:\s*56px/m)
      expect(css).to match(/max-width:\s*639px[^{]*\)\s*\{[\s\S]*?\.counter-value\b[^{]*\{[^}]*font-size\s*:\s*44px/m)
    end

    it "R-YJ05-EQRH renders functional +/- forms posting to the mutation endpoints when signed in" do
      establish_web_session!(email: "signed-in@allowed.example")

      get "https://www.example.com/"

      expect(response).to have_http_status(:ok)
      body = response.body

      card = body[%r{<section[^>]*id=["']counter-card["'][^>]*>.*?</section>}m]
      expect(card).to match(%r{<form[^>]*action=["']/counter/increment["'][^>]*method=["']post["']}i)
      expect(card).to match(%r{<form[^>]*action=["']/counter/decrement["'][^>]*method=["']post["']}i)
      # Buttons are not disabled when signed in.
      expect(card).not_to match(/aria-label=["']Increment["'][^>]*disabled/i)
      expect(card).not_to match(/aria-label=["']Decrement["'][^>]*disabled/i)
    end

    it "R-YJ05-EQRH renders disabled +/- buttons and no mutation forms when not signed in" do
      get "/"

      body = response.body
      card = body[%r{<section[^>]*id=["']counter-card["'][^>]*>.*?</section>}m]

      # No mutation forms on the card.
      expect(card).not_to match(%r{<form[^>]*action=["'][^"']*counter[^"']*["']}i)
      # Both buttons carry the HTML disabled attribute.
      expect(card).to match(/<button[^>]*aria-label=["']Increment["'][^>]*disabled/i)
      expect(card).to match(/<button[^>]*aria-label=["']Decrement["'][^>]*disabled/i)
    end

    it "R-YJ05-EQRH shows a signed-in hint line when there is an active web session" do
      establish_web_session!(email: "signed-in@allowed.example")

      get "https://www.example.com/"
      expect(response.body).to include("Signed in. The MCP server can read")
    end

    it "R-YJ05-EQRH shows a sign-in hint line when there is no active web session" do
      get "/"
      expect(response.body).to include("Sign in to manipulate the counter from the browser.")
    end

    it "R-YJ05-EQRH a browser form submission to /counter/increment redirects to / with 303 and updates the counter" do
      establish_web_session!(email: "signed-in@allowed.example")
      Counter.current.update!(value: 7)

      post "https://www.example.com/counter/increment", params: { from: "index" }

      expect(response).to have_http_status(:see_other)
      expect(response.location).to match(%r{/\z})
      expect(Counter.current.value).to eq(8)
    end

    it "R-YJ05-EQRH a browser form submission to /counter/decrement redirects to / with 303 and updates the counter" do
      establish_web_session!(email: "signed-in@allowed.example")
      Counter.current.update!(value: 7)

      post "https://www.example.com/counter/decrement", params: { from: "index" }

      expect(response).to have_http_status(:see_other)
      expect(Counter.current.value).to eq(6)
    end

    it "R-YJ05-EQRH a browser form decrement against a zero counter redirects without changing the value" do
      establish_web_session!(email: "signed-in@allowed.example")
      Counter.current.update!(value: 0)

      post "https://www.example.com/counter/decrement", params: { from: "index" }

      expect(response).to have_http_status(:see_other)
      expect(Counter.current.value).to eq(0)
    end
  end

  describe "R-NIMH-0NZG signed-in +/- click drives a mutation" do
    let(:provider) { Rails.configuration.x.google_identity_provider }

    around do |example|
      previous_domain = Rails.configuration.x.auth.workspace_domain
      Rails.configuration.x.auth.workspace_domain = "allowed.example"
      example.run
      Rails.configuration.x.auth.workspace_domain = previous_domain
    end

    def establish_web_session!(email:)
      get "https://www.example.com/login"
      upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
      provider.stub_code(
        "code-h5ka-#{email}",
        sub: "google-#{email}",
        email: email,
        hosted_domain: "allowed.example"
      )
      get "https://www.example.com/oauth/google/callback",
          params: { code: "code-h5ka-#{email}", state: upstream_state },
          headers: { "X-Forwarded-Proto" => "https" }
    end

    it "R-NIMH-0NZG R-1LLM-Y4XF signed-in `+` click POSTs to /counter/increment and changes the canonical value" do
      establish_web_session!(email: "click@allowed.example")
      Counter.current.update!(value: 3)

      post "https://www.example.com/counter/increment", params: { from: "index" }

      expect(response).to have_http_status(:see_other)
      expect(response.location).to match(%r{/\z})
      expect(Counter.current.value).to eq(4)
    end

    it "R-NIMH-0NZG R-1LLM-Y4XF signed-in `−` click POSTs to /counter/decrement and changes the canonical value" do
      establish_web_session!(email: "click@allowed.example")
      Counter.current.update!(value: 3)

      post "https://www.example.com/counter/decrement", params: { from: "index" }

      expect(response).to have_http_status(:see_other)
      expect(Counter.current.value).to eq(2)
    end

    it "R-NIMH-0NZG R-1LLM-Y4XF a `−` click against a zero counter leaves the displayed value unchanged" do
      establish_web_session!(email: "click@allowed.example")
      Counter.current.update!(value: 0)

      post "https://www.example.com/counter/decrement", params: { from: "index" }

      expect(response).to have_http_status(:see_other)
      expect(Counter.current.value).to eq(0)
    end

    it "R-NIMH-0NZG session-cookie auth alone authenticates the JSON mutation path used by JS fetch" do
      establish_web_session!(email: "click@allowed.example")
      Counter.current.update!(value: 5)

      post "https://www.example.com/counter/increment"

      expect(response).to have_http_status(:ok)
      expect(JSON.parse(response.body)).to eq("value" => 6)
      expect(Counter.current.value).to eq(6)
    end

    it "R-NIMH-0NZG session-cookie JSON decrement at zero returns 409 so JS can convey rejection briefly" do
      establish_web_session!(email: "click@allowed.example")
      Counter.current.update!(value: 0)

      post "https://www.example.com/counter/decrement"

      expect(response).to have_http_status(:conflict)
      body = JSON.parse(response.body)
      expect(body["error"]).to eq("counter_at_zero")
      expect(Counter.current.value).to eq(0)
    end

    it "R-NIMH-0NZG renders JS that intercepts .counter-form submission and POSTs asynchronously via fetch" do
      get "/"

      body = response.body
      # The JS must prevent the default submit (no full reload),
      # target the counter forms, and issue an async POST via fetch.
      expect(body).to match(/querySelectorAll\(["']\.counter-form["']\)/)
      expect(body).to match(/addEventListener\(["']submit["']/)
      expect(body).to match(/preventDefault\(\)/)
      expect(body).to match(/fetch\(\s*url\s*,\s*\{[^}]*method:\s*["']POST["']/m)
      expect(body).to match(/credentials:\s*["']same-origin["']/)
      # On a 409 response, a brief rejection element flips visible.
      expect(body).to match(/id=["']counter-reject["']/)
      expect(body).to match(/r\.status\s*===\s*409/)
      expect(body).to match(/1400/)
    end
  end

  # R-052Y-EKE0
  describe "R-052Y-EKE0 anonymous visitors cannot post comments" do
    it "R-052Y-EKE0 returns 404 when an anonymous visitor attempts to post a comment" do
      post "/comments", params: { body: "test comment" }

      expect(response).to have_http_status(404)
    end
  end

  describe "R-YHS9-0Z0S instructions area structure" do
    it "R_YHS9_0Z0S renders an instructions header with an h2 and a right-side muted subhead pointing at <base-url>/mcp" do
      host! "hal.ai.metaspot.org"
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      head = body[%r{<div[^>]*class=["'][^"']*\binstructions-head\b[^"']*["'][^>]*>.*?</div>}m]
      expect(head).not_to be_nil
      expect(head).to match(%r{<h2[^>]*>\s*Connect an MCP client\s*</h2>}i)
      # The subhead lives in the same header, after the h2, and names the base URL.
      h2_idx = head.index("<h2")
      sub_idx = head.index(%r{class=["'][^"']*\binstructions-subhead\b})
      expect(sub_idx).not_to be_nil
      expect(sub_idx).to be > h2_idx
      expect(head).to include("http://hal.ai.metaspot.org/mcp")
    end

    it "R_YHS9_0Z0S renders two stacked section cards, one per supported client, with 01/02 numeric badges" do
      get "/"
      body = response.body

      code_section = body[%r{<article[^>]*class=["'][^"']*\bsection\b[^"']*["'][^>]*id=["']section-claude-code["'][^>]*>.*?</article>}m]
      desktop_section = body[%r{<article[^>]*class=["'][^"']*\bsection\b[^"']*["'][^>]*id=["']section-claude-desktop["'][^>]*>.*?</article>}m]
      expect(code_section).not_to be_nil, "expected the Claude Code section card"
      expect(desktop_section).not_to be_nil, "expected the Claude Desktop section card"

      # Each card has a header with a numeric badge in a small chip.
      expect(code_section).to match(%r{<span[^>]*class=["'][^"']*\bnum\b[^"']*["'][^>]*>\s*01\s*</span>}i)
      expect(desktop_section).to match(%r{<span[^>]*class=["'][^"']*\bnum\b[^"']*["'][^>]*>\s*02\s*</span>}i)

      # Each card's header includes the client title (h3) and a right-aligned kind description.
      expect(code_section).to match(%r{<h3[^>]*>\s*Claude Code\s*</h3>}i)
      expect(desktop_section).to match(%r{<h3[^>]*>\s*Claude Desktop\s*</h3>}i)
      expect(code_section).to match(%r{<span[^>]*class=["'][^"']*\bdesc\b[^"']*["'][^>]*>.*CLI.*</span>}im)
      expect(desktop_section).to match(%r{<span[^>]*class=["'][^"']*\bdesc\b[^"']*["'][^>]*>.*JSON.*</span>}im)

      # The Claude Code card comes before the Claude Desktop card in document order.
      code_idx = body.index('id="section-claude-code"')
      desktop_idx = body.index('id="section-claude-desktop"')
      expect(code_idx).to be < desktop_idx
    end

    it "R_YHS9_0Z0S renders dark code blocks with semantic token-coloring spans rather than undifferentiated monospace" do
      get "/"
      body = response.body

      # Every `<pre>` carrying an MCP-config id is marked as a dark `.code` block.
      %w[claude-code-config-project claude-code-config-user claude-desktop-config].each do |id|
        block = body[%r{<pre[^>]*id=["']#{Regexp.escape(id)}["'][^>]*>.*?</pre>}m]
        expect(block).not_to be_nil, "expected <pre id=#{id.inspect}>"
        expect(block).to match(%r{class=["'][^"']*\bcode\b[^"']*["']}), "expected #{id.inspect} to carry the .code class"
        # Some kind of semantic span (flag/url/key/str/arg/punct/prompt) is present.
        expect(block).to match(%r{<span[^>]*class=["'][^"']*\b(flag|url|key|str|arg|punct|prompt)\b}i),
          "expected #{id.inspect} to contain at least one semantic token-coloring span"
      end
    end

    it "R_YHS9_0Z0S the instructions area sits below the counter card and above the footer" do
      get "/"
      body = response.body

      head_idx = body.index(%r{<div[^>]*class=["'][^"']*\binstructions-head\b})
      footer_idx = body.index("<footer")
      expect(head_idx).not_to be_nil
      expect(footer_idx).not_to be_nil
      expect(head_idx).to be < footer_idx
    end

    it "R_YHS9_0Z0S section cards carry off-white fills and every code block exposes a visible copy button with clipboard JS + execCommand fallback + mint-green copied feedback" do
      get "/"
      body = response.body

      # Off-white section-card fill is pinned in CSS: the dark #1a1916 fill
      # is gone and the `.section` selector now keys off a light fill.
      css = File.read(Rails.root.join("app/assets/stylesheets/application.css"))
      expect(css).not_to match(/#mcp-config\s+\.section\s*\{[^}]*background:\s*#1a1916/m),
        "expected the dark #1a1916 section fill to be replaced with an off-white fill"
      expect(css).to match(/#mcp-config\s+\.section\s*\{[^}]*background:\s*#(?:f|F)[0-9a-fA-F]{5}/m),
        "expected #mcp-config .section to use an off-white (#f...) fill"

      # Every code block id has a sibling copy button that targets it by id.
      %w[claude-code-config-project claude-code-config-user claude-desktop-config].each do |id|
        expect(body).to match(%r{<button[^>]*class=["'][^"']*\bcopy\b[^"']*["'][^>]*data-copy-target=["']#{Regexp.escape(id)}["']}),
          "expected a copy button targeting #{id.inspect}"
        # Code block + button share a `.code-block` wrapper so the button can
        # be positioned over the <pre>.
        wrapper = body[%r{<div[^>]*class=["'][^"']*\bcode-block\b[^"']*["'][^>]*>(?:(?!</div>).)*?id=["']#{Regexp.escape(id)}["'].*?</div>}m]
        expect(wrapper).not_to be_nil, "expected #{id.inspect} to live inside a .code-block wrapper"
        expect(wrapper).to match(%r{<button[^>]*class=["'][^"']*\bcopy\b}),
          "expected the .code-block wrapping #{id.inspect} to contain its copy button"
      end

      # The copy-button JS uses the standard clipboard API with a textarea +
      # execCommand fallback and applies a `copied` state for ~1.4s.
      expect(body).to include("navigator.clipboard")
      expect(body).to include("writeText")
      expect(body).to match(/document\.execCommand\(\s*["']copy["']\s*\)/)
      expect(body).to match(/1400/)
      expect(body).to match(/classList\.add\(\s*["']copied["']\s*\)/)

      # Mint-green `copied` state is pinned in CSS.
      expect(css).to match(/\.copy\.copied\s*\{[^}]*color:\s*#79d4a9/m),
        "expected .copy.copied to render in mint-green #79d4a9"
    end
  end

  describe "R-K4XR-U91S motion + accessibility" do
    # Verifies the slice of R-K4XR-U91S that can land today: the
    # reduced-motion CSS suppressing the lens-dot pulse and the ARIA
    # affordances on elements already present on the index page.
    # The counter-flash and subtitle fade-swap suppression pieces
    # land alongside the counter card and live-channel work.

    it "suppresses the lens-dot pulse animation under prefers-reduced-motion: reduce" do
      css = File.read(Rails.root.join("app/assets/stylesheets/application.css"))

      # The @media block exists and references reduced motion.
      expect(css).to match(/@media\s*\(\s*prefers-reduced-motion\s*:\s*reduce\s*\)/i)

      # Inside that block, the lens animation is set to none.
      rm_block = css[/@media\s*\(\s*prefers-reduced-motion\s*:\s*reduce\s*\)\s*\{((?:[^{}]|\{[^{}]*\})*)\}/im, 1]
      expect(rm_block).not_to be_nil
      expect(rm_block).to match(/\.banner\s+\.lens\s*\{[^}]*animation\s*:\s*none/i)
    end

    it "marks the lens dot and footer status dot as aria-hidden decorative elements" do
      get "/"
      body = response.body

      lens = body[/<span[^>]*class=["'][^"']*\blens\b[^"']*["'][^>]*>/i]
      expect(lens).not_to be_nil
      expect(lens).to match(/aria-hidden=["']true["']/i)

      status_dot = body[/<span[^>]*class=["'][^"']*\bstatus-dot\b[^"']*["'][^>]*>/i]
      expect(status_dot).not_to be_nil
      expect(status_dot).to match(/aria-hidden=["']true["']/i)
    end

    it "labels the re-roll control with aria-label='New subtitle'" do
      get "/"
      body = response.body

      reroll = body[/<button[^>]*id=["']reroll["'][^>]*>/i]
      expect(reroll).not_to be_nil
      expect(reroll).to match(/aria-label=["']New subtitle["']/i)
    end
  end

  describe "R-YFCG-9FJE design-reference visual fidelity" do
    # The design reference (reqs/design/HAL.html) names two load-bearing
    # layout properties beyond the token palette R-JWEH-5UUX pins:
    # content is centered with a max content width around 880px, and the
    # page is visibly card-grouped — banner, counter, and MCP-client
    # sections render inside distinct bordered, rounded cards rather
    # than as flat full-width sections.

    let(:css) { File.read(Rails.root.join("app/assets/stylesheets/application.css")) }

    it "R_YFCG_9FJE centers page content with a max-width on the order of 880px" do
      get "/"
      body = response.body
      # The layout wraps yield in a .page container so every page picks up centering.
      expect(body).to match(%r{<main[^>]*class=["'][^"']*\bpage\b[^"']*["']}i)
      # The .page rule sets a max-width around 880px and centers via auto margins.
      page_rule = css[/\.page\s*\{[^}]*\}/m]
      expect(page_rule).not_to be_nil, "expected a .page CSS rule"
      expect(page_rule).to match(/max-width\s*:\s*8[0-9]{2}px/i)
      expect(page_rule).to match(/margin\s*:\s*0\s+auto/i)
    end

    it "R_YFCG_9FJE renders banner, counter, and instructions as visibly bordered cards" do
      get "/"
      body = response.body
      # The three load-bearing groupings are present on the page.
      expect(body).to match(%r{<section[^>]*class=["'][^"']*\bbanner\b}i)
      expect(body).to match(%r{<section[^>]*class=["'][^"']*\bcounter-card\b}i)
      expect(body).to match(%r{<section[^>]*id=["']mcp-config["']}i)
      # The banner and MCP-config section cards carry visible borders in CSS.
      banner_rule = css[/\.banner\s*\{[^}]*\}/m]
      expect(banner_rule).to match(/border\s*:\s*1px\s+solid/i)
      expect(banner_rule).to match(/border-radius\s*:/i)
      section_rule = css[/#mcp-config\s+\.section\s*\{[^}]*\}/m]
      expect(section_rule).to match(/border\s*:\s*1px\s+solid/i)
      expect(section_rule).to match(/border-radius\s*:/i)
    end
  end

  describe "R-JWEH-5UUX design-reference fidelity" do
    # The load-bearing design tokens pinned by R-JWEH-5UUX live in the
    # stylesheet. The reference checked into reqs/design/HAL.html names
    # them explicitly; this spec asserts each appears in application.css
    # so the index page renders against the same palette, type families,
    # and headline scale as the reference.

    let(:css) { File.read(Rails.root.join("app/assets/stylesheets/application.css")) }

    it "R_JWEH_5UUX pins the warm off-white background token" do
      expect(css).to match(/#f6f5f1/i)
    end

    it "R_JWEH_5UUX pins the dark-ink foreground token" do
      expect(css).to match(/#14130f/i)
    end

    it "R_JWEH_5UUX pins the HAL-lens red accent token" do
      expect(css).to match(/#d4361e/i)
    end

    it "R_JWEH_5UUX applies the page background and ink color to the body" do
      expect(css).to match(/body\s*\{[^}]*background\s*:\s*[^;]*(#f6f5f1|--bg)/im)
      expect(css).to match(/body\s*\{[^}]*color\s*:\s*[^;]*(#14130f|--ink)/im)
    end

    it "R_JWEH_5UUX selects Inter for UI text and JetBrains Mono for code" do
      expect(css).to match(/font-family\s*:\s*[^;]*["']?Inter["']?/i)
      expect(css).to match(/["']JetBrains Mono["']/)
    end

    it "R_JWEH_5UUX anchors the type scale at the 88px HAL title and the 11px mono badge" do
      expect(css).to match(/\.banner\s+\.title\s*\{[^}]*font-size\s*:\s*88px/im)
      expect(css).to match(/font\s*:\s*[^;]*\b11px\b[^;]*["']JetBrains Mono["']/i)
    end
  end

  describe "R-K3PV-GHB3 footer below the instructions area" do
    it "renders a footer with a decorative status dot and the 'MCP server live' phrase on the left" do
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to match(/<footer\b/i)
      # The status dot is decorative — the textual phrase carries the meaning.
      expect(body).to match(/aria-hidden=["']true["']/i)
      expect(body).to include("MCP server live")
    end

    it "deliberately omits the listening port from the live indicator" do
      get "/"

      footer = response.body[/<footer\b[^>]*>.*?<\/footer>/im]
      expect(footer).not_to be_nil
      # Strip HTML tags so CSS values inside style="..." don't trigger.
      footer_text = footer.gsub(/<[^>]+>/, " ")
      expect(footer_text).not_to match(/:\s*\d{2,5}\b/)
      expect(footer_text).not_to match(/listening/i)
    end

    it "renders the version + flavor line 'v0.1.0 · open my pod bay doors' on the right" do
      get "/"

      footer = response.body[/<footer\b[^>]*>.*?<\/footer>/im]
      expect(footer).to include("v0.1.0")
      expect(footer).to include("open my pod bay doors")
    end

    it "places the footer after the instructions / MCP config section" do
      get "/"
      body = response.body
      mcp_idx = body.index("mcp-config")
      footer_idx = body.index("<footer")
      expect(mcp_idx).not_to be_nil
      expect(footer_idx).not_to be_nil
      expect(footer_idx).to be > mcp_idx
    end
  end
end
