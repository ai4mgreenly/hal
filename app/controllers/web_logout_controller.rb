# R-AE1P-Z1WC: `/logout` ends the current web session and redirects
# the user-agent to `/`. A request from a user-agent without an
# active web session is a no-op redirect to `/`, not an error.
# `/logout` does not touch any MCP token chain (R-93PJ-FRPY) — it
# revokes the matching row in the dedicated web-sessions table per
# R-SLGL-B5B4 and clears the session cookie.
class WebLogoutController < ApplicationController
  def show
    destroy_web_session! if current_web_session
    redirect_to "/"
  end
end
