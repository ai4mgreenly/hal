# Browser-facing index page (reqs/web.md).
class HomeController < ApplicationController
  # R-MG6P-TA7C: merged subtitle bank — acronym expansions for the
  # project name. One entry is chosen uniformly at random per render;
  # the re-roll control (R-N3CT-2XAJ) yields a freshly-picked entry on
  # each activation. This list supersedes the earlier 10-entry bank.
  EXPANSIONS = [
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
  ].freeze

  # R-QY5R-PYDH: render the current counter value as plain server-
  # rendered HTML at the site root. No authentication required.
  def index
    @count = Counter.current.value
    # R-BZQY-DN3B / R-CO4Y-11X7: derive MCP base URL from the request.
    @base_url = request.base_url
    # R-MG6P-TA7C: choose one expansion uniformly at random per render.
    @subtitle = pick_subtitle
    # R-AZZW-UX8U: reflect web-session state. When a web session is
    # active, the page identifies the visitor by the recorded Google
    # email and exposes a /logout affordance; otherwise it exposes a
    # /login affordance and renders no placeholder identity.
    @web_email = current_web_email.presence
  end

  private

  # R-MG6P-TA7C: seam for test stubbing without touching the frozen
  # constant. EXPANSIONS.sample is Ruby's uniform-random sampler.
  def pick_subtitle
    EXPANSIONS.sample
  end
end
