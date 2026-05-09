# R-ZPE1-0DV8: server-side row for an issued authorization code. The
# row stores a SHA-256 digest of the code string (mirroring OauthToken
# and OauthClient secret patterns); the plaintext is returned to the
# user-agent exactly once, embedded in the redirect back to the client.
# At issue time the code is bound to three values from the originating
# authorize request — client_id, redirect_uri, and the PKCE
# code_challenge (with its method) — so the token endpoint can refuse a
# code presented by anyone but the original client. used_at marks
# single-use redemption; expires_at enforces short-lived rejection.
class CreateOauthAuthorizationCodes < ActiveRecord::Migration[8.1]
  def change
    create_table :oauth_authorization_codes do |t|
      t.string :code_digest, null: false
      t.string :client_id, null: false
      t.string :redirect_uri, null: false
      t.string :code_challenge, null: false
      t.string :code_challenge_method, null: false
      t.string :owner, null: false
      t.string :chain_id, null: false
      t.datetime :issued_at, null: false
      t.datetime :expires_at, null: false
      t.datetime :used_at
      t.datetime :revoked_at
      t.timestamps
    end
    add_index :oauth_authorization_codes, :code_digest, unique: true
    add_index :oauth_authorization_codes, :chain_id
  end
end
