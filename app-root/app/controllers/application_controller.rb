class ApplicationController < ActionController::Base
  # Only allow modern browsers supporting webp images, web push, badges, import maps, CSS nesting, and CSS :has.
  allow_browser versions: :modern

  # Changes to the importmap will invalidate the etag for HTML responses
  stale_when_importmap_changes

  helper_method :current_web_email, :current_web_session

  private

  # R-SLGL-B5B4: validation of an inbound web-session cookie is a single
  # lookup against the dedicated table. The opaque identifier travels in
  # the Rails-encrypted session cookie under :web_session_id; the row's
  # digest column is the SHA-256 of that identifier.
  # R-KJ15-9P17: each successful resolution bumps the row's expires_at to
  # min(now + 1h, issued_at + 12h); a row past its absolute ceiling is
  # treated as no session.
  def current_web_session
    return @current_web_session if defined?(@current_web_session)
    row = WebSession.find_by_presented_token(session[:web_session_id])
    if row && !row.touch_expiry!
      row = nil
    end
    @current_web_session = row
  end

  def current_web_email
    current_web_session&.owner
  end

  # R-SLGL-B5B4 / R-CXJ2-R3BN: only this method establishes a web session.
  # It rotates the session id (R-AYLJ-8SYX-style), inserts the row, and
  # places the opaque identifier in the encrypted session cookie.
  def establish_web_session(email:)
    reset_session
    _, plaintext = WebSession.issue(owner: email)
    session[:web_session_id] = plaintext
    @current_web_session = nil
  end

  # R-AE1P-Z1WC: logout writes revoked_at on the matching row and clears
  # the session cookie; the same cookie value cannot be redeemed again.
  def destroy_web_session!
    current_web_session&.revoke!
    reset_session
    @current_web_session = nil
  end
end
