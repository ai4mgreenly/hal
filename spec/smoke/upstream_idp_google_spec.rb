# R-K1E9-OR3T: upstream identity provider is Google (Workspace).
require "rails_helper"

RSpec.describe "Upstream IDP posture" do
  describe "R-K1E9-OR3T Google is the only identity provider" do
    it "ships a GoogleIdentityProvider service and no other *_identity_provider.rb" do
      services_dir = Rails.root.join("app/services")
      providers = Dir.children(services_dir).select { |n| n.end_with?("_identity_provider.rb") }
      expect(providers).to eq([ "google_identity_provider.rb" ])
    end
  end
end
