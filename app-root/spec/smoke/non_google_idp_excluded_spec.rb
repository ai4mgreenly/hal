# R-NAGM-EQAH: identity providers other than Google Workspace are out of scope.
require "rails_helper"

RSpec.describe "Non-Google IDP exclusion" do
  describe "R-NAGM-EQAH no third-party IDP gems are bundled" do
    it "Gemfile.lock contains no omniauth/devise/doorkeeper/saml/ldap/auth0/okta gems" do
      lockfile = Rails.root.join("Gemfile.lock").read
      gem_names = lockfile.scan(/^    ([a-z0-9][a-z0-9_-]*) \(/i).flatten.map(&:downcase).uniq
      forbidden_prefixes = %w[
        omniauth
        devise
        doorkeeper
        warden
        auth0
        okta
        saml
        ruby-saml
        ldap
        net-ldap
        cas
      ]
      offenders = gem_names.select do |name|
        forbidden_prefixes.any? { |p| name == p || name.start_with?("#{p}-") || name.start_with?("#{p}_") }
      end
      expect(offenders).to eq([])
    end
  end
end
