# Browser-facing index page (reqs/web.md).
class HomeController < ApplicationController
  # R-QY5R-PYDH: render the current counter value as plain server-
  # rendered HTML at the site root. No authentication required.
  def index
    @count = Counter.current.value
    # R-BZQY-DN3B / R-CO4Y-11X7: derive MCP base URL from the request.
    @base_url = request.base_url
  end
end
