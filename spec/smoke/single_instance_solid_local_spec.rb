# R-MOIF-IUXZ: high availability, multi-instance, or clustered
# deployment is out of scope. The supported topology is one process,
# and the solid_* backends all live on the local SQLite filesystem so
# state cannot be shared across instances.
require "rails_helper"

RSpec.describe "Deployment posture" do
  describe "R-MOIF-IUXZ single-instance solid_* backends" do
    it "binds solid_cache, solid_queue, and solid_cable to local SQLite databases in production" do
      database_yml = Rails.root.join("config/database.yml").read
      production = YAML.safe_load(ERB.new(database_yml).result, aliases: true)["production"]

      %w[primary cache queue cable].each do |slot|
        expect(production).to have_key(slot)
        expect(production[slot]["adapter"]).to eq("sqlite3")
        expect(production[slot]["database"]).to start_with("storage/")
      end
    end
  end
end
