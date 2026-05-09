# R-AYLJ-8SYX: every browser-session cookie the service
# issues carries `HttpOnly` and `SameSite=Lax`. `Lax` (not `Strict`) is
# required because the Google callback (R-ETP6-60VA) is a cross-site
# top-level navigation that must carry the session cookie for the
# state-binding check. The `Secure` attribute is added conditionally by
# `ConditionalSecureSessionCookie` (registered in `config/application.rb`)
# based on the same forwarded-protocol signal R-ID5L-BSJM uses for HSTS,
# so production HTTPS responses set `Secure` while local plain-HTTP dev
# (R-PVA6-Q6OB) omits it.
SESSION_COOKIE_KEY = "_hal_session".freeze

Rails.application.config.session_store :cookie_store,
                                       key: SESSION_COOKIE_KEY,
                                       secure: false,
                                       httponly: true,
                                       same_site: :lax
