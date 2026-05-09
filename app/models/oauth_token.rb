# R-Z955-CD0I: tokens are opaque cryptographically-random strings; each
# issued token has a server-side row recording kind, owner, chain
# membership, issued-at, expires-at, used-at, and revoked-at. Validation
# of an inbound bearer token is a single lookup against this store.
# R-CUUP-REQT: the row stores a SHA-256 digest of the token string. The
# plaintext is returned to the client exactly once at issue time and is
# never persisted. Inbound tokens are validated by hashing the presented
# string and looking up the row by that digest.
require "digest"

class OauthToken < ApplicationRecord
  KINDS = %w[access refresh].freeze

  validates :kind, inclusion: { in: KINDS }
  validates :owner, :chain_id, :token_digest, :issued_at, :expires_at, presence: true

  def self.digest_for(plaintext)
    Digest::SHA256.hexdigest(plaintext)
  end

  def self.issue(kind:, owner:, lifetime:, chain_id: nil, resource: :default)
    resource = Rails.configuration.x.canonical_url if resource == :default
    chain_id ||= SecureRandom.uuid
    now = Time.current
    plaintext = SecureRandom.urlsafe_base64(32)
    record = create!(
      kind: kind,
      owner: owner,
      chain_id: chain_id,
      token_digest: digest_for(plaintext),
      issued_at: now,
      expires_at: now + lifetime,
      resource: resource
    )
    [ record, plaintext ]
  end

  def self.find_by_presented_token(plaintext)
    find_by(token_digest: digest_for(plaintext))
  end
end
