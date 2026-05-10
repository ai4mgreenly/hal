# R-DFMI-JQHO: the seam between the service's Google client and Google
# is narrow — `#authorization_url` and `#exchange_code` are the only
# operations on it — so the test double and the real-Google
# implementation are substitutable behind that interface.
require "rails_helper"

RSpec.describe "Google identity provider seam shape" do
  describe "R-DFMI-JQHO narrow seam between service and Google" do
    let(:seam_methods) { %i[authorization_url exchange_code] }

    it "GoogleIdentityProvider defines exactly #authorization_url and #exchange_code as its seam methods" do
      defined = GoogleIdentityProvider.instance_methods(false).sort
      expect(defined).to eq(seam_methods.sort)
    end

    it "GoogleIdentityProvider::Fake overrides exactly the seam methods (substitutable behind that interface)" do
      overridden = GoogleIdentityProvider::Fake.instance_methods(false) &
                   GoogleIdentityProvider.instance_methods(false)
      expect(overridden.sort).to eq(seam_methods.sort)
    end

    it "the fake's seam method signatures match the base's, so callers can swap implementations" do
      seam_methods.each do |name|
        base_params = GoogleIdentityProvider.instance_method(name).parameters
        fake_params = GoogleIdentityProvider::Fake.instance_method(name).parameters
        expect(fake_params).to eq(base_params), "parameters for ##{name} differ between base and fake"
      end
    end
  end
end
