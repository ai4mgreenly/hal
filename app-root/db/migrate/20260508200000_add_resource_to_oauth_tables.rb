# R-IS0W-S2H3: add the RFC 8707 `resource` binding column to oauth_tokens
# and oauth_authorization_codes so each issued credential records the
# resource identifier it was bound to at issue time.
class AddResourceToOauthTables < ActiveRecord::Migration[8.1]
  def change
    add_column :oauth_authorization_codes, :resource, :string
    add_column :oauth_tokens, :resource, :string
  end
end
