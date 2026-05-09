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

      # No <script> tags on the page — the index page does not depend on JS.
      expect(body).not_to match(/<script\b/i)
      # No JS module imports or importmap references.
      expect(body).not_to match(/type=["']module["']/i)
      expect(body).not_to match(/importmap/i)
      # No data-turbo-driven mutation hooks tied to the count.
      expect(body).not_to match(/data-turbo-stream/i)
    end

    it "R-BZQY-DN3B displays MCP client config for Claude Code and Desktop with copy-pasteable instructions" do
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      # Both clients are covered.
      expect(body).to match(/Claude Code/i)
      expect(body).to match(/Claude Desktop/i)

      # Each block is rendered in a <pre> for copy-paste.
      expect(body).to match(%r{<pre[^>]*id=["']claude-code-config["'][^>]*>.*</pre>}m)
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
      host! "ouroboros.ai.metaspot.org"
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      # Claude Code: the exact `claude mcp add --transport http <name> <base>/mcp` line.
      expect(body).to match(
        %r{<pre[^>]*id=["']claude-code-config["'][^>]*>\s*<code[^>]*>\s*claude mcp add --transport http ouroboros http://ouroboros\.ai\.metaspot\.org/mcp\s*</code>\s*</pre>}m
      )

      # Claude Desktop: a `mcpServers` JSON block with type "http" and the URL.
      desktop_match = body.match(
        %r{<pre[^>]*id=["']claude-desktop-config["'][^>]*>\s*<code[^>]*>(?<json>.*?)</code>\s*</pre>}m
      )
      expect(desktop_match).not_to be_nil
      json = JSON.parse(desktop_match[:json])
      expect(json).to have_key("mcpServers")
      expect(json["mcpServers"]).to have_key("ouroboros")
      expect(json["mcpServers"]["ouroboros"]).to eq(
        "type" => "http",
        "url" => "http://ouroboros.ai.metaspot.org/mcp"
      )
    end

    it "R-CO4Y-11X7 derives the MCP base URL from the request host" do
      host! "localhost:3000"
      get "/"
      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to include("http://localhost:3000/mcp")
      expect(body).not_to include("www.example.com")

      host! "ouroboros.ai.metaspot.org"
      get "/"
      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to include("http://ouroboros.ai.metaspot.org/mcp")
      expect(body).not_to include("localhost:3000")
      expect(body).not_to include("www.example.com")
    end

    it "R-DA34-WX9P honors X-Forwarded-Proto from a TLS-terminating proxy" do
      host! "ouroboros.ai.metaspot.org"
      get "/", headers: { "X-Forwarded-Proto" => "https" }

      expect(response).to have_http_status(:ok)
      body = response.body
      expect(body).to include("https://ouroboros.ai.metaspot.org/mcp")
      expect(body).not_to include("http://ouroboros.ai.metaspot.org/mcp")
    end

    it "R-SY3U-AF4G offers no in-page control to mutate the count" do
      get "/"

      expect(response).to have_http_status(:ok)
      body = response.body

      # No <form> targeting any counter mutation route.
      expect(body).not_to match(/<form[^>]*action=["'][^"']*counter[^"']*["']/i)
      # No method=post forms at all on the index page.
      expect(body).not_to match(/<form[^>]*method=["']post["']/i)
      # No buttons or links pointing at the increment endpoint.
      expect(body).not_to match(%r{counter/increment}i)
      # No <button> elements (mutation controls).
      expect(body).not_to match(/<button\b/i)
      # No <input type=submit> either.
      expect(body).not_to match(/<input[^>]*type=["']submit["']/i)
    end
  end
end
