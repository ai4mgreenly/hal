# R-3JCR-C810: Dynamic Client Registration (RFC 7591). Each registered
# client has a public client_id and (for confidential clients) a hashed
# client_secret. redirect_uris is the list of URIs the authorization
# endpoint will accept.
class CreateOauthClients < ActiveRecord::Migration[8.1]
  def change
    create_table :oauth_clients do |t|
      t.string :client_id, null: false
      t.string :client_secret_digest
      t.string :token_endpoint_auth_method, null: false, default: "none"
      t.string :client_name
      t.text :redirect_uris, null: false
      t.text :grant_types
      t.text :response_types
      t.timestamps
    end
    add_index :oauth_clients, :client_id, unique: true
  end
end
