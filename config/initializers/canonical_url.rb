# R-IS0W-S2H3: the service has a single configured resource identifier —
# the canonical external URL it is reached at. Issued access tokens are
# bound to this value via the RFC 8707 `resource` parameter, and the
# bearer-side check on protected endpoints rejects any token whose
# recorded binding does not equal it.
default = case Rails.env
when "test" then "http://www.example.com"
when "development" then "http://localhost:3000"
else "https://hal.ai.metaspot.org"
end
Rails.application.config.x.canonical_url = ENV.fetch("HAL_CANONICAL_URL", default)
