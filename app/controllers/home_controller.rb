# Browser-facing index page (reqs/web.md).
class HomeController < ApplicationController
  # R-HALR-NDM1: the configured list of acronym expansions for the
  # banner subtitle. The list is deliberately a mix of plausible
  # readings and obvious jokes; no entry is canonical.
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
    # R-HALR-NDM1: banner subtitle picked server-side per request.
    @expansion = EXPANSIONS.sample
  end
end
