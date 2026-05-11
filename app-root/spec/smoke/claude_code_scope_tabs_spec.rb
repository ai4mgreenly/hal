# R-9T1D-KJ2W: the Claude Code section card renders its two scope
# examples inside a functional tab interface — two role="tab" triggers
# above one role="tabpanel" holding the active scope's snippet, with
# WAI-ARIA wiring and a client-side toggle that swaps which snippet is
# visible (no GET /, no navigation).
require "rails_helper"

RSpec.describe "R-9T1D-KJ2W Claude Code scope tabs", type: :request do
  def code_section
    body = response.body
    body[%r{<article[^>]*id=["']section-claude-code["'][^>]*>.*?</article>}m]
  end

  before do
    host! "hal.ai.metaspot.org"
    get "/"
    expect(response).to have_http_status(:ok)
  end

  it "R-9T1D-KJ2W renders a role=tablist with two role=tab buttons labelled by scope" do
    section = code_section
    expect(section).not_to be_nil
    expect(section).to match(/role=["']tablist["']/)

    project_tab = section[%r{<button[^>]*role=["']tab["'][^>]*data-scope=["']project["'][^>]*>.*?</button>}m]
    user_tab = section[%r{<button[^>]*role=["']tab["'][^>]*data-scope=["']user["'][^>]*>.*?</button>}m]
    expect(project_tab).not_to be_nil
    expect(user_tab).not_to be_nil

    expect(project_tab).to include("Project scope")
    expect(project_tab).to match(%r{<span[^>]*class=["'][^"']*\bmono\b[^"']*["'][^>]*>\.mcp\.json</span>})
    expect(user_tab).to include("User scope")
    expect(user_tab).to match(%r{<span[^>]*class=["'][^"']*\bmono\b[^"']*["'][^>]*>~/\.claude\.json</span>})
  end

  it "R-9T1D-KJ2W ships a single role=tabpanel with aria-labelledby pointing at the active trigger" do
    section = code_section
    panel = section[%r{<div[^>]*role=["']tabpanel["'][^>]*>.*?</div>\s*</div>\s*</article>}m]
    expect(panel).not_to be_nil
    expect(section).to match(/role=["']tabpanel["']/)
    expect(section).to match(/aria-controls=["']scope-tabpanel["']/)
    expect(section).to match(/aria-labelledby=["']scope-tab-project["']/)
  end

  it "R-9T1D-KJ2W Project scope is the default active tab with aria-selected=true and the User tab is aria-selected=false" do
    section = code_section
    project_tab = section[%r{<button[^>]*id=["']scope-tab-project["'][^>]*>.*?</button>}m]
    user_tab = section[%r{<button[^>]*id=["']scope-tab-user["'][^>]*>.*?</button>}m]
    expect(project_tab).to match(/aria-selected=["']true["']/)
    expect(user_tab).to match(/aria-selected=["']false["']/)
  end

  it "R-9T1D-KJ2W triggers are <button> elements (never <a href=…> that would navigate)" do
    section = code_section
    expect(section).not_to match(%r{<a[^>]*role=["']tab["']}i)
    expect(section).not_to match(%r{<a[^>]+id=["']scope-tab-})
  end

  it "R-9T1D-KJ2W both scope snippets ship in the rendered HTML at page load (client-side toggle, not server fetch)" do
    section = code_section
    expect(section).to match(%r{<pre[^>]*id=["']claude-code-config-project["']})
    expect(section).to match(%r{<pre[^>]*id=["']claude-code-config-user["']})
    text = section.gsub(/<[^>]+>/, "").gsub(/\s+/, " ")
    expect(text).to include("--scope project")
    expect(text).to include("--scope user")
  end

  it "R-9T1D-KJ2W only the active scope's code-block is visible at load (the inactive sibling is hidden)" do
    section = code_section
    project_block = section[%r{<div[^>]*class=["'][^"']*\bcode-block\b[^"']*["'][^>]*data-scope=["']project["'][^>]*>}]
    user_block = section[%r{<div[^>]*class=["'][^"']*\bcode-block\b[^"']*["'][^>]*data-scope=["']user["'][^>]*>}]
    expect(project_block).not_to be_nil
    expect(user_block).not_to be_nil
    expect(project_block).not_to include("hidden")
    expect(user_block).to include("hidden")
  end

  it "R-9T1D-KJ2W active trigger carries a 2px accent-red bottom border; inactive carries transparent" do
    css = File.read(Rails.root.join("app/assets/stylesheets/application.css"))
    expect(css).to match(/\.scope-tab\s*\{[^}]*border-bottom:\s*2px\s+solid\s+transparent/m)
    expect(css).to match(/\.scope-tab--active\s*\{[^}]*border-bottom-color:\s*var\(--accent\)/m)
  end

  it "R-9T1D-KJ2W ships a client-side toggle that swaps visibility without GET / or fetch(/)" do
    body = response.body
    # The toggle script lives near the Claude Code section.
    expect(body).to match(/querySelectorAll\(["']\.code-block["']\)/)
    expect(body).to match(/aria-selected/)
    # The toggle does not navigate or refetch the page.
    section_and_script = body[%r{<article[^>]*id=["']section-claude-code["'][^>]*>.*?</script>}m]
    expect(section_and_script).not_to be_nil
    expect(section_and_script).not_to match(/fetch\(\s*["']\/?["']\s*[,)]/)
    expect(section_and_script).not_to match(/location\s*\.\s*(?:href|assign|replace|reload)/)
  end

  it "R-9T1D-KJ2W copy button on the active scope's code-block strips the prompt to the executable form" do
    section = code_section
    # The active (project) code-block has its own copy button targeting the project <pre>.
    expect(section).to match(%r{<button[^>]*class=["'][^"']*\bcopy\b[^"']*["'][^>]*data-copy-target=["']claude-code-config-project["']})
    # The hidden (user) code-block has its own copy button targeting the user <pre>.
    expect(section).to match(%r{<button[^>]*class=["'][^"']*\bcopy\b[^"']*["'][^>]*data-copy-target=["']claude-code-config-user["']})
    # The shared copy JS strips the leading prompt prefix per R-9WP2-PUAZ.
    body = response.body
    expect(body).to match(%r{textContent\.replace\(/\^\[\\?\$>\]\\s\+/})
  end
end
