# R-K8LG-ZK9V: when more than one implementation approach can satisfy
# a requirement, the build agent prefers the canonical-Rails approach
# (what `rails new` ships in-box, what the Rails guides recommend) over
# a third-party gem or a hand-rolled implementation. This is a
# tiebreaker, applied only when the spec doesn't constrain the choice.
require "rails_helper"

RSpec.describe "Canonical-Rails preference" do
  describe "R-K8LG-ZK9V no third-party gem displaces an in-box Rails facility" do
    it "Gemfile.lock contains no gem from the displacers list" do
      # Each entry is a third-party gem whose role is already covered by
      # an in-box Rails facility on the pinned Rails 8.1 line:
      #
      #   Background jobs   -> Active Job + Solid Queue
      #   Cache             -> Active Support cache + Solid Cache
      #   WebSockets / pubsub -> Action Cable + Solid Cable
      #   Authentication    -> has_secure_password
      #   HTTP client       -> Net::HTTP
      #   JSON              -> stdlib JSON
      #   ORM / migrations  -> Active Record
      #   File upload       -> Active Storage
      #   Forms             -> Action View form helpers
      #
      # Requirements that name a specific third-party choice (e.g.
      # rspec-rails, rubocop-rails-omakase) take precedence and are
      # not listed here.
      displacers = %w[
        sidekiq
        resque
        delayed_job
        good_job
        que
        sucker_punch
        redis-rails
        redis-store
        dalli
        anycable
        anycable-rails
        devise
        clearance
        sorcery
        authlogic
        httparty
        faraday
        rest-client
        typhoeus
        excon
        http
        oj
        multi_json
        yajl-ruby
        sequel
        rom
        rom-rb
        ridgepole
        carrierwave
        paperclip
        shrine
        simple_form
        formtastic
      ]
      lockfile = Rails.root.join("Gemfile.lock").read
      gem_names = lockfile.scan(/^    ([a-z0-9][a-z0-9_-]*) \(/i).flatten.map(&:downcase).uniq
      offenders = gem_names & displacers
      expect(offenders).to eq([])
    end
  end

  describe "R-K8LG-ZK9V the in-box Rails facilities `rails new` generates are present" do
    it "Gemfile.lock contains rails, propshaft, puma, importmap-rails, turbo-rails, stimulus-rails, solid_cache, solid_queue, solid_cable" do
      canonical = %w[
        rails
        propshaft
        puma
        importmap-rails
        turbo-rails
        stimulus-rails
        solid_cache
        solid_queue
        solid_cable
      ]
      lockfile = Rails.root.join("Gemfile.lock").read
      gem_names = lockfile.scan(/^    ([a-z0-9][a-z0-9_-]*) \(/i).flatten.map(&:downcase).uniq
      missing = canonical - gem_names
      expect(missing).to eq([])
    end
  end
end
