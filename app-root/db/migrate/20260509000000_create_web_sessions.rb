# R-SLGL-B5B4: web sessions are persisted as rows in a dedicated table
# distinct from the OAuth token store. Each row records the owner email,
# a SHA-256 digest of the opaque session identifier the cookie carries,
# issued_at, expires_at (R-KJ15-9P17 idle/absolute ceiling), and revoked_at.
class CreateWebSessions < ActiveRecord::Migration[8.1]
  def change
    create_table :web_sessions do |t|
      t.string :owner, null: false
      t.string :session_digest, null: false
      t.datetime :issued_at, null: false
      t.datetime :expires_at
      t.datetime :revoked_at
      t.timestamps
    end
    add_index :web_sessions, :session_digest, unique: true
    add_index :web_sessions, :owner
  end
end
