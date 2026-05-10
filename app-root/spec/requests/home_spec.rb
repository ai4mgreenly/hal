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

      # The R-DRX9-8WNY live-update bootstrap is the only permitted
      # script and is a progressive enhancement: the count above is
      # already rendered by the server, the +/- forms function via
      # POST + 303 redirect (R-NRQS-QC4F), and the script merely
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
      # The block carries semantic token-coloring spans (R-PLLD-DY5X); strip inner
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

    it "R-JV1V-OF3W shows two scope-flagged Claude Code examples, each labeled and copy-pasteable" do
      host! "hal.ai.metaspot.org"
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      # Spans for semantic token coloring (R-PLLD-DY5X) live inside <code>; capture
      # the full inner block then strip tags + the leading `$ ` prompt.
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

      # Each example is a complete `claude mcp add` line with the MCP URL.
      expect(project_cmd).to start_with("claude mcp add")
      expect(project_cmd).to include("--scope project")
      expect(project_cmd).to include("http://hal.ai.metaspot.org/mcp")

      expect(user_cmd).to start_with("claude mcp add")
      expect(user_cmd).to include("--scope user")
      expect(user_cmd).to include("http://hal.ai.metaspot.org/mcp")

      # Each scope is labeled near its block so a visitor can pick by intent.
      project_section = body[/Project scope.*?<\/pre>/m]
      user_section = body[/User scope.*?<\/pre>/m]
      expect(project_section).to include("claude-code-config-project")
      expect(user_section).to include("claude-code-config-user")

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

    context "R-TFIQ-6805 banner and subtitle" do
      let(:expansions) { HomeController::EXPANSIONS }

      it "R-TFIQ-6805 shows the project name as the banner heading" do
        # R-N3CT-2XAJ replaces the earlier H.A.L. spelling with bare HAL.
        get "/"

        expect(response).to have_http_status(:ok)
        expect(response.body).to match(%r{<h1[^>]*class=["'][^"']*\btitle\b[^"']*["'][^>]*>\s*HAL\s*</h1>}i)
      end

      it "R-TFIQ-6805 shows exactly one expansion from the fixed list as the subtitle" do
        expansion = expansions.first
        allow_any_instance_of(HomeController).to receive(:pick_subtitle).and_return(expansion)

        get "/"

        expect(response.body).to include(expansion)
        # Only the one chosen expansion appears; the others do not.
        (expansions - [ expansion ]).each do |other|
          expect(response.body).not_to include(other)
        end
      end

      it "R-TFIQ-6805 every expansion in the list can be shown as the subtitle" do
        expansions.each do |expansion|
          allow_any_instance_of(HomeController).to receive(:pick_subtitle).and_return(expansion)
          get "/"
          expect(response.body).to include(expansion), "expected #{expansion.inspect} to appear when sampled"
        end
      end

      it "R-TFIQ-6805 successive requests can produce different subtitles" do
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

      it "R-TFIQ-6805 provides a re-roll control that is a link to the root path" do
        get "/"

        expect(response).to have_http_status(:ok)
        reroll_pattern = %r{<a[^>]*href=["']/["'][^>]*id=["']reroll["']|<a[^>]*id=["']reroll["'][^>]*href=["']/["']}
        expect(response.body).to match(reroll_pattern)
      end

      it "R-TFIQ-6805 the re-roll control works without JavaScript (plain link)" do
        get "/"

        # No JavaScript needed: the control is an <a> link, not a JS-driven
        # widget. (The page may carry the R-DRX9-8WNY SSE bootstrap script
        # for live counter updates, but the re-roll itself does not depend
        # on JS — it is a plain anchor.)
        expect(response.body).to match(/<a[^>]*id=["']reroll["']/)
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
        reroll = section[%r{<a[^>]*id=["']reroll["'][^>]*>.*?</a>}m]
        expect(reroll).not_to be_nil
        expect(reroll).to match(/aria-label=["']New subtitle["']/i)
        # Still works without JS — it is an <a href="/"> link, not a JS-only widget.
        expect(reroll).to match(%r{href=["']/["']})
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

  describe "R-AZZW-UX8U index page reflects web-session state" do
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

    it "R-AZZW-UX8U shows the visitor's email and a /logout affordance when signed in" do
      email = "signed-in@allowed.example"
      establish_web_session(email: email)

      get "https://www.example.com/"

      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to include(email)
      expect(body).to match(%r{<a[^>]*href=["']/logout["']}i)
      expect(body).not_to match(%r{<a[^>]*href=["']/login["']}i)
    end

    it "R-AZZW-UX8U shows a /login affordance and no placeholder identity when not signed in" do
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to match(%r{<a[^>]*href=["']/login["']}i)
      expect(body).not_to match(%r{<a[^>]*href=["']/logout["']}i)
      expect(body).not_to match(/guest/i)
      expect(body).not_to match(/anonymous/i)
    end
  end

  describe "R-NRQS-QC4F counter card with +/- buttons" do
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

    it "R-NRQS-QC4F renders a counter card with CURRENT COUNT label, the value, and decrement/increment buttons" do
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
    end

    it "R-NRQS-QC4F renders functional +/- forms posting to the mutation endpoints when signed in" do
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

    it "R-NRQS-QC4F renders disabled +/- buttons and no mutation forms when not signed in" do
      get "/"

      body = response.body
      card = body[%r{<section[^>]*id=["']counter-card["'][^>]*>.*?</section>}m]

      # No mutation forms on the card.
      expect(card).not_to match(%r{<form[^>]*action=["'][^"']*counter[^"']*["']}i)
      # Both buttons carry the HTML disabled attribute.
      expect(card).to match(/<button[^>]*aria-label=["']Increment["'][^>]*disabled/i)
      expect(card).to match(/<button[^>]*aria-label=["']Decrement["'][^>]*disabled/i)
    end

    it "R-NRQS-QC4F shows a signed-in hint line when there is an active web session" do
      establish_web_session!(email: "signed-in@allowed.example")

      get "https://www.example.com/"
      expect(response.body).to include("Signed in. The MCP server can read")
    end

    it "R-NRQS-QC4F shows a sign-in hint line when there is no active web session" do
      get "/"
      expect(response.body).to include("Sign in to manipulate the counter from the browser.")
    end

    it "R-NRQS-QC4F a browser form submission to /counter/increment redirects to / with 303 and updates the counter" do
      establish_web_session!(email: "signed-in@allowed.example")
      Counter.current.update!(value: 7)

      post "https://www.example.com/counter/increment", params: { from: "index" }

      expect(response).to have_http_status(:see_other)
      expect(response.location).to match(%r{/\z})
      expect(Counter.current.value).to eq(8)
    end

    it "R-NRQS-QC4F a browser form submission to /counter/decrement redirects to / with 303 and updates the counter" do
      establish_web_session!(email: "signed-in@allowed.example")
      Counter.current.update!(value: 7)

      post "https://www.example.com/counter/decrement", params: { from: "index" }

      expect(response).to have_http_status(:see_other)
      expect(Counter.current.value).to eq(6)
    end

    it "R-NRQS-QC4F a browser form decrement against a zero counter redirects without changing the value" do
      establish_web_session!(email: "signed-in@allowed.example")
      Counter.current.update!(value: 0)

      post "https://www.example.com/counter/decrement", params: { from: "index" }

      expect(response).to have_http_status(:see_other)
      expect(Counter.current.value).to eq(0)
    end
  end

  # R-052Y-EKE0
  describe "R-052Y-EKE0 anonymous visitors cannot post comments" do
    it "R-052Y-EKE0 returns 404 when an anonymous visitor attempts to post a comment" do
      post "/comments", params: { body: "test comment" }

      expect(response).to have_http_status(404)
    end
  end

  describe "R-OZN6-I2TF Claude Code scope tabs" do
    it "R-OZN6-I2TF renders a WAI-ARIA tablist with two tabs labeled Project scope and User scope" do
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      expect(body).to match(/role=["']tablist["']/i)

      # Two tabs, each with role=tab, aria-selected, aria-controls.
      project_tab = body[%r{<a[^>]*role=["']tab["'][^>]*id=["']claude-code-tab-project["'][^>]*>.*?</a>}m]
      user_tab = body[%r{<a[^>]*role=["']tab["'][^>]*id=["']claude-code-tab-user["'][^>]*>.*?</a>}m]
      expect(project_tab).not_to be_nil, "expected a Project-scope tab"
      expect(user_tab).not_to be_nil, "expected a User-scope tab"

      # Project tab is the default-active one.
      expect(project_tab).to match(/aria-selected=["']true["']/i)
      expect(user_tab).to match(/aria-selected=["']false["']/i)

      # Tab triggers carry the documented labels with the mono filenames.
      expect(project_tab).to include("Project scope")
      expect(project_tab).to match(%r{<code[^>]*>\s*\.mcp\.json\s*</code>}m)
      expect(user_tab).to include("User scope")
      expect(user_tab).to match(%r{<code[^>]*>\s*~/\.claude\.json\s*</code>}m)

      # aria-controls wires each tab to its matching panel.
      expect(project_tab).to match(/aria-controls=["']claude-code-panel-project["']/i)
      expect(user_tab).to match(/aria-controls=["']claude-code-panel-user["']/i)
    end

    it "R-OZN6-I2TF renders two tabpanels labelled-by their tabs, each containing the scope-meta description and the existing code block" do
      get "/"
      body = response.body

      project_panel = body[%r{<div[^>]*role=["']tabpanel["'][^>]*id=["']claude-code-panel-project["'][^>]*>.*?</div>\s*<div[^>]*role=["']tabpanel["']}m] ||
                      body[%r{<div[^>]*role=["']tabpanel["'][^>]*id=["']claude-code-panel-project["'][^>]*>.*?</pre>\s*</div>}m]
      user_panel = body[%r{<div[^>]*role=["']tabpanel["'][^>]*id=["']claude-code-panel-user["'][^>]*>.*?</pre>\s*</div>}m]
      expect(project_panel).not_to be_nil
      expect(user_panel).not_to be_nil

      expect(project_panel).to match(/aria-labelledby=["']claude-code-tab-project["']/i)
      expect(user_panel).to match(/aria-labelledby=["']claude-code-tab-user["']/i)

      # Scope-meta description from the spec, verbatim phrasing.
      expect(project_panel).to include("Commits the server to")
      expect(project_panel).to include(".mcp.json")
      expect(user_panel).to include("Records the server in")
      expect(user_panel).to include("~/.claude.json")

      # Each panel still contains its existing-id `<pre>` code block.
      expect(project_panel).to include("claude-code-config-project")
      expect(user_panel).to include("claude-code-config-user")
    end

    it "R-OZN6-I2TF degrades to both panels visible with JS disabled" do
      get "/"
      body = response.body

      # With no JS and no hiding CSS, both panels (and their code blocks) are present in the body.
      expect(body).to match(%r{<pre[^>]*id=["']claude-code-config-project["']})
      expect(body).to match(%r{<pre[^>]*id=["']claude-code-config-user["']})
    end
  end

  describe "R-PLLD-DY5X instructions area structure" do
    it "R_PLLD_DY5X renders an instructions header with an h2 and a right-side muted subhead pointing at <base-url>/mcp" do
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

    it "R_PLLD_DY5X renders two stacked section cards, one per supported client, with 01/02 numeric badges" do
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

    it "R_PLLD_DY5X renders dark code blocks with semantic token-coloring spans rather than undifferentiated monospace" do
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

    it "R_PLLD_DY5X the instructions area sits below the counter card and above the footer" do
      get "/"
      body = response.body

      head_idx = body.index(%r{<div[^>]*class=["'][^"']*\binstructions-head\b})
      footer_idx = body.index("<footer")
      expect(head_idx).not_to be_nil
      expect(footer_idx).not_to be_nil
      expect(head_idx).to be < footer_idx
    end
  end

  describe "R-RLJF-YEWW motion + accessibility" do
    # Verifies the slice of R-RLJF-YEWW that can land today: the
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

      reroll = body[/<a[^>]*id=["']reroll["'][^>]*>/i]
      expect(reroll).not_to be_nil
      expect(reroll).to match(/aria-label=["']New subtitle["']/i)
    end
  end

  describe "R-LRSQ-5VDG visual fidelity to design reference" do
    # The design tokens pinned by R-LRSQ-5VDG live in the stylesheet.
    # The reference checked into reqs/design/HAL.html names them
    # explicitly; this spec asserts each load-bearing token appears
    # in application.css so the index page renders against the same
    # palette, type families, and headline scale as the reference.

    let(:css) { File.read(Rails.root.join("app/assets/stylesheets/application.css")) }

    it "pins the warm off-white background token" do
      expect(css).to match(/#f6f5f1/i)
    end

    it "pins the dark-ink foreground token" do
      expect(css).to match(/#14130f/i)
    end

    it "pins the HAL-lens red accent token" do
      expect(css).to match(/#d4361e/i)
    end

    it "applies the page background and ink color to the body" do
      expect(css).to match(/body\s*\{[^}]*background\s*:\s*[^;]*(#f6f5f1|--bg)/im)
      expect(css).to match(/body\s*\{[^}]*color\s*:\s*[^;]*(#14130f|--ink)/im)
    end

    it "selects Inter for UI text and JetBrains Mono for code" do
      expect(css).to match(/font-family\s*:\s*[^;]*["']?Inter["']?/i)
      expect(css).to match(/["']JetBrains Mono["']/)
    end

    it "anchors the type scale at the 88px HAL title and the 11px mono badge" do
      expect(css).to match(/\.banner\s+\.title\s*\{[^}]*font-size\s*:\s*88px/im)
      expect(css).to match(/font\s*:\s*[^;]*\b11px\b[^;]*["']JetBrains Mono["']/i)
    end
  end

  describe "R-QKYG-HAO2 footer below the instructions area" do
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
