# R-SLGL-B5B4: web sessions live in a dedicated table. The plaintext
# session identifier is returned to the user-agent exactly once via the
# Set-Cookie response that establishes the session; the row stores only
# its SHA-256 digest. Validation of an inbound cookie is a single
# lookup by digest, accepted iff the row is un-revoked and un-expired.
# R-KJ15-9P17: a session is bounded by two ceilings — 1 hour idle, 12
# hours absolute. The earlier wins. The expires_at column materializes
# the effective deadline; on each successful request it is recomputed
# as min(now + 1h, issued_at + 12h).
require "digest"

class WebSession < ApplicationRecord
  validates :owner, :session_digest, :issued_at, presence: true

  def self.idle_ttl
    Rails.configuration.x.auth.web_session_idle_ttl
  end

  def self.absolute_ttl
    Rails.configuration.x.auth.web_session_absolute_ttl
  end

  def self.digest_for(plaintext)
    Digest::SHA256.hexdigest(plaintext)
  end

  def self.issue(owner:)
    plaintext = SecureRandom.urlsafe_base64(32)
    now = Time.current
    record = create!(
      owner: owner,
      session_digest: digest_for(plaintext),
      issued_at: now,
      expires_at: now + idle_ttl
    )
    [ record, plaintext ]
  end

  # R-KJ15-9P17: validation rejects rows past their effective deadline.
  def self.find_by_presented_token(plaintext)
    return nil if plaintext.blank?
    row = find_by(session_digest: digest_for(plaintext), revoked_at: nil)
    return nil if row.nil?
    return nil if row.expires_at && row.expires_at <= Time.current
    row
  end

  # R-KJ15-9P17: bump the effective deadline on a successful authenticated
  # request. The new deadline is min(now + 1h, issued_at + 12h); if that
  # value is already in the past, the absolute ceiling has been crossed
  # and the session is expired.
  def touch_expiry!
    now = Time.current
    new_deadline = [ now + self.class.idle_ttl, issued_at + self.class.absolute_ttl ].min
    if new_deadline <= now
      update!(expires_at: new_deadline)
      false
    else
      update!(expires_at: new_deadline)
      true
    end
  end

  def revoke!
    update!(revoked_at: Time.current)
  end
end
