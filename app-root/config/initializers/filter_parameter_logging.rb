# Be sure to restart your server when you modify this file.

# Configure parameters to be partially matched (e.g. passw matches password) and filtered from the log file.
# Use this to limit dissemination of sensitive information.
# See the ActiveSupport::ParameterFilter documentation for supported notations and behaviors.
# R-SAK8-WB9W: redact bearer-token-bearing parameter and header names from
# any logging that captures them. ParameterFilter substring-matches, so
# `:token` covers `access_token`/`refresh_token` and `:authorization`
# covers the `Authorization` header should it ever be logged.
Rails.application.config.filter_parameters += [
  :passw, :email, :secret, :token, :_key, :crypt, :salt, :certificate, :otp, :ssn, :cvv, :cvc,
  :authorization
]
