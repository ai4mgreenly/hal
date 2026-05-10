# R-KJ15-9P17: a web session is bounded by two ceilings beyond explicit
# revocation: 1 hour idle (clock restarts on each authenticated request),
# 12 hours absolute (regardless of activity). Earlier wins.
require "rails_helper"

RSpec.describe "Web session idle/absolute ceilings (R-KJ15-9P17)", type: :request do
  include ActiveSupport::Testing::TimeHelpers

  let(:provider) { Rails.configuration.x.google_identity_provider }
  let(:email) { "ceilings@allowed.example" }

  around do |example|
    previous_domain = Rails.configuration.x.auth.workspace_domain
    Rails.configuration.x.auth.workspace_domain = "allowed.example"
    example.run
    Rails.configuration.x.auth.workspace_domain = previous_domain
  end

  def complete_federation(code:, sub:)
    get "https://www.example.com/login"
    upstream_state = URI.decode_www_form(URI.parse(response.location).query).to_h["state"]
    provider.stub_code(code, sub: sub, email: email, hosted_domain: "allowed.example")
    get "https://www.example.com/oauth/google/callback",
        params: { code: code, state: upstream_state },
        headers: { "X-Forwarded-Proto" => "https" }
  end

  it "R-KJ15-9P17 a session idle for more than 1 hour no longer authenticates" do
    complete_federation(code: "code-idle", sub: "g-idle")
    expect(WebSession.find_by_presented_token(session[:web_session_id])&.owner).to eq(email)

    travel_to 61.minutes.from_now do
      get "https://www.example.com/"
      expect(response.body).not_to include(email)
    end
  end

  it "R-KJ15-9P17 activity within the idle window pushes expires_at forward" do
    complete_federation(code: "code-bump", sub: "g-bump")
    plaintext = session[:web_session_id]
    initial_expiry = WebSession.find_by(session_digest: WebSession.digest_for(plaintext)).expires_at

    travel_to 30.minutes.from_now do
      get "https://www.example.com/"
      expect(response).to have_http_status(:ok)
      bumped_expiry = WebSession.find_by(session_digest: WebSession.digest_for(plaintext)).expires_at
      expect(bumped_expiry).to be > initial_expiry
    end
  end

  it "R-KJ15-9P17 a session more than 12 hours old no longer authenticates regardless of activity" do
    complete_federation(code: "code-abs", sub: "g-abs")

    # Stay active well within the idle window for hours, then cross the absolute ceiling.
    11.times do |i|
      travel_to (i + 1).hours.from_now + 1.minute do
        get "https://www.example.com/"
      end
    end

    travel_to 12.hours.from_now + 1.minute do
      get "https://www.example.com/"
      expect(response.body).not_to include(email)
    end
  end

  it "R-KJ15-9P17 the effective deadline is capped at issued_at + 12h" do
    complete_federation(code: "code-cap", sub: "g-cap")
    plaintext = session[:web_session_id]
    row = WebSession.find_by(session_digest: WebSession.digest_for(plaintext))
    issued = row.issued_at

    # Keep the session alive with regular activity (every 30 minutes) up
    # until the absolute ceiling is near.
    23.times do |i|
      travel_to ((i + 1) * 30).minutes.from_now do
        get "https://www.example.com/"
      end
    end

    # Now at 11h30m past issuance, the most recent bump set expires_at to
    # now + 1h = issued + 12h30m, but the absolute cap pinches at issued
    # + 12h. Verify the cap.
    bumped = WebSession.find_by(session_digest: WebSession.digest_for(plaintext))
    expect(bumped.expires_at).to be_within(2.seconds).of(issued + 12.hours)
  end
end
