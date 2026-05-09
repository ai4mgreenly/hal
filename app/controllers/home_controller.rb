# Browser-facing index page (reqs/web.md).
class HomeController < ApplicationController
  # R-TFIQ-6805: fixed list of acronym expansions for the banner subtitle.
  EXPANSIONS = [
    "Holistic Access Layer",
    "Human Augmentation Layer",
    "Heuristic Agent Liaison",
    "Home, APIs, Library",
    "Heuristically programmed ALgorithm",
    "Helpful Autonomous Liaison",
    "Hyperlocal Agent Layer",
    "Host Agent Liaison",
    "Has Always Listened",
    "House Always Loses"
  ].freeze

  # R-QY5R-PYDH: render the current counter value as plain server-
  # rendered HTML at the site root. No authentication required.
  def index
    @count = Counter.current.value
    # R-BZQY-DN3B / R-CO4Y-11X7: derive MCP base URL from the request.
    @base_url = request.base_url
    # R-TFIQ-6805: choose one expansion at random on each request.
    @subtitle = pick_subtitle
  end

  private

  # R-TFIQ-6805: seam for test stubbing without touching the frozen constant.
  def pick_subtitle
    EXPANSIONS.sample
  end
end
