# R-QGB5-EMOO: every browser-session cookie the service issues
# carries Secure, HttpOnly, and SameSite=Lax. Lax (not Strict) is
# required because the Google callback (R-ETP6-60VA) is a cross-site
# top-level navigation that must carry the session cookie for the
# state-binding check.
Rails.application.config.session_store :cookie_store,
                                       key: "_hal_session",
                                       secure: true,
                                       httponly: true,
                                       same_site: :lax
