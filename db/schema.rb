# This file is auto-generated from the current state of the database. Instead
# of editing this file, please use the migrations feature of Active Record to
# incrementally modify your database, and then regenerate this schema definition.
#
# This file is the source Rails uses to define your schema when running `bin/rails
# db:schema:load`. When creating a new database, `bin/rails db:schema:load` tends to
# be faster and is potentially less error prone than running all of your
# migrations from scratch. Old migrations may fail to apply correctly if those
# migrations use external dependencies or application code.
#
# It's strongly recommended that you check this file into your version control system.

ActiveRecord::Schema[8.1].define(version: 2026_05_08_200000) do
  create_table "counters", force: :cascade do |t|
    t.datetime "created_at", null: false
    t.datetime "updated_at", null: false
    t.integer "value", default: 0, null: false
  end

  create_table "oauth_authorization_codes", force: :cascade do |t|
    t.string "chain_id", null: false
    t.string "client_id", null: false
    t.string "code_challenge", null: false
    t.string "code_challenge_method", null: false
    t.string "code_digest", null: false
    t.datetime "created_at", null: false
    t.datetime "expires_at", null: false
    t.datetime "issued_at", null: false
    t.string "owner", null: false
    t.string "redirect_uri", null: false
    t.string "resource"
    t.datetime "revoked_at"
    t.datetime "updated_at", null: false
    t.datetime "used_at"
    t.index ["chain_id"], name: "index_oauth_authorization_codes_on_chain_id"
    t.index ["code_digest"], name: "index_oauth_authorization_codes_on_code_digest", unique: true
  end

  create_table "oauth_clients", force: :cascade do |t|
    t.string "client_id", null: false
    t.string "client_name"
    t.string "client_secret_digest"
    t.datetime "created_at", null: false
    t.text "grant_types"
    t.text "redirect_uris", null: false
    t.text "response_types"
    t.string "token_endpoint_auth_method", default: "none", null: false
    t.datetime "updated_at", null: false
    t.index ["client_id"], name: "index_oauth_clients_on_client_id", unique: true
  end

  create_table "oauth_tokens", force: :cascade do |t|
    t.string "chain_id", null: false
    t.datetime "created_at", null: false
    t.datetime "expires_at", null: false
    t.datetime "issued_at", null: false
    t.string "kind", null: false
    t.string "owner", null: false
    t.string "resource"
    t.datetime "revoked_at"
    t.string "token_digest", null: false
    t.datetime "updated_at", null: false
    t.datetime "used_at"
    t.index ["chain_id"], name: "index_oauth_tokens_on_chain_id"
    t.index ["token_digest"], name: "index_oauth_tokens_on_token_digest", unique: true
  end
end
