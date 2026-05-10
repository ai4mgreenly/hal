# R-3JCR-C810: server-side record of a Dynamic Client Registration.
# Stores the public client_id, optional hashed client_secret, the
# advertised redirect URIs, and the standard RFC 7591 metadata fields
# this service cares about. The plaintext client_secret is returned to
# the registering client exactly once at registration time and is
# never persisted (mirrors the OauthToken digest pattern).
require "digest"

class OauthClient < ApplicationRecord
  AUTH_METHODS = %w[none client_secret_basic].freeze
  DEFAULT_GRANT_TYPES = %w[authorization_code refresh_token].freeze
  DEFAULT_RESPONSE_TYPES = %w[code].freeze

  serialize :redirect_uris, coder: JSON, type: Array
  serialize :grant_types, coder: JSON, type: Array
  serialize :response_types, coder: JSON, type: Array

  validates :client_id, presence: true, uniqueness: true
  validates :token_endpoint_auth_method, inclusion: { in: AUTH_METHODS }
  validate :redirect_uris_must_be_present_https_or_loopback

  def self.digest_for(plaintext)
    Digest::SHA256.hexdigest(plaintext)
  end

  def self.register(params)
    redirect_uris = Array(params[:redirect_uris] || params["redirect_uris"])
    auth_method = (params[:token_endpoint_auth_method] || params["token_endpoint_auth_method"] || "none").to_s
    raise ArgumentError, "invalid_redirect_uri" if redirect_uris.empty?

    client_id = SecureRandom.urlsafe_base64(24)
    secret_plain = nil
    secret_digest = nil
    if auth_method != "none"
      secret_plain = SecureRandom.urlsafe_base64(32)
      secret_digest = digest_for(secret_plain)
    end

    grants = Array(params[:grant_types] || params["grant_types"])
    grants = DEFAULT_GRANT_TYPES if grants.empty?
    responses = Array(params[:response_types] || params["response_types"])
    responses = DEFAULT_RESPONSE_TYPES if responses.empty?

    record = create!(
      client_id: client_id,
      client_secret_digest: secret_digest,
      token_endpoint_auth_method: auth_method,
      client_name: params[:client_name] || params["client_name"],
      redirect_uris: redirect_uris,
      grant_types: grants,
      response_types: responses
    )
    [ record, secret_plain ]
  end

  private

  def redirect_uris_must_be_present_https_or_loopback
    uris = redirect_uris
    if uris.blank?
      errors.add(:redirect_uris, "must be present")
      return
    end
    uris.each do |u|
      parsed = URI.parse(u.to_s)
      next if parsed.scheme == "https"
      next if parsed.scheme == "http" && %w[localhost 127.0.0.1 ::1].include?(parsed.host)
      errors.add(:redirect_uris, "must be https or loopback http")
    rescue URI::InvalidURIError
      errors.add(:redirect_uris, "must be a valid URI")
    end
  end
end
