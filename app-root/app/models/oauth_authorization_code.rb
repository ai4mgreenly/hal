# R-ZPE1-0DV8: a server-side record of an issued authorization code,
# bound at issue time to client_id, redirect_uri, and PKCE
# code_challenge (with method). Codes are short-lived and single-use:
# expired or already-redeemed codes are rejected at redemption time.
# When the token endpoint detects redemption-time misuse (replay or
# expiry), it revokes the chain via revoke_chain!.
require "digest"

class OauthAuthorizationCode < ApplicationRecord
  validates :code_digest, :client_id, :redirect_uri, :code_challenge,
            :code_challenge_method, :owner, :chain_id, :issued_at,
            :expires_at, presence: true
  validates :code_digest, uniqueness: true

  def self.digest_for(plaintext)
    Digest::SHA256.hexdigest(plaintext)
  end

  def self.issue(client_id:, redirect_uri:, code_challenge:,
                 code_challenge_method:, owner:, chain_id: nil,
                 lifetime: nil, resource: :default)
    lifetime ||= Rails.configuration.x.auth.authorization_code_ttl
    # R-IS0W-S2H3: record the RFC 8707 resource binding at code-issue time
    # so the token endpoint can propagate it onto the issued tokens.
    resource = Rails.configuration.x.auth.canonical_url if resource == :default
    chain_id ||= SecureRandom.uuid
    now = Time.current
    plaintext = SecureRandom.urlsafe_base64(32)
    record = create!(
      code_digest: digest_for(plaintext),
      client_id: client_id,
      redirect_uri: redirect_uri,
      code_challenge: code_challenge,
      code_challenge_method: code_challenge_method,
      owner: owner,
      chain_id: chain_id,
      issued_at: now,
      expires_at: now + lifetime,
      resource: resource
    )
    [ record, plaintext ]
  end

  def self.find_by_presented_code(plaintext)
    find_by(code_digest: digest_for(plaintext))
  end

  def redeemable?
    used_at.nil? && revoked_at.nil? && Time.current < expires_at
  end

  def redeem!
    return false unless redeemable?

    update!(used_at: Time.current)
    true
  end
end
