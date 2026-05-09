# R-Z955-CD0I: tokens have a server-side row recording kind, owner, chain
# membership, issued_at, expires_at, used_at, and revoked_at.
class CreateOauthTokens < ActiveRecord::Migration[8.1]
  def change
    create_table :oauth_tokens do |t|
      t.string :kind, null: false
      t.string :owner, null: false
      t.string :chain_id, null: false
      t.string :token, null: false
      t.datetime :issued_at, null: false
      t.datetime :expires_at, null: false
      t.datetime :used_at
      t.datetime :revoked_at
      t.timestamps
    end
    add_index :oauth_tokens, :token, unique: true
    add_index :oauth_tokens, :chain_id
  end
end
