# R-CL63-P202: in the test environment the service's Google client is
# replaced with the in-memory fake. R-DBZW-40BC: callers reach the
# provider through `Rails.configuration.x.google_identity_provider`,
# so swapping the fake for a real Google client later is one line.
require Rails.root.join("app/services/google_identity_provider")
require Rails.root.join("app/services/google_identity_provider/fake")

Rails.application.config.x.google_identity_provider =
  if Rails.env.test?
    GoogleIdentityProvider::Fake.new
  else
    GoogleIdentityProvider.new
  end
