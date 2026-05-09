# R-CUUP-REQT: the token row stores a cryptographic hash of the token
# string, not the plaintext. Rename the column and its unique index so
# the schema reflects "this is a digest, not the bearer string".
class RenameOauthTokensTokenToTokenDigest < ActiveRecord::Migration[8.1]
  def change
    rename_column :oauth_tokens, :token, :token_digest
  end
end
