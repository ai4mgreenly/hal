# R-Z955-CD0I: tokens are opaque cryptographically-random strings backed
# by a server-side row recording kind, owner, chain, issued_at,
# expires_at, used_at, and revoked_at. Validation is a single lookup.
require "rails_helper"

RSpec.describe OauthToken, type: :model do
  describe "R-Z955-CD0I tokens are opaque random strings backed by a server-side row" do
    it "issues an access token whose plaintext is opaque, random, and unique per call" do
      _, t1 = OauthToken.issue(kind: "access", owner: "alice@example.com", lifetime: 1.hour)
      _, t2 = OauthToken.issue(kind: "access", owner: "alice@example.com", lifetime: 1.hour)

      expect(t1).to be_a(String)
      expect(t1.length).to be >= 32
      expect(t1).to match(/\A[A-Za-z0-9_\-]+\z/)
      expect(t1).not_to eq(t2)
    end

    it "records kind, owner, chain_id, issued_at, and expires_at on the row at issue time" do
      record, _ = OauthToken.issue(kind: "access", owner: "alice@example.com", lifetime: 1.hour)
      expect(record.kind).to eq("access")
      expect(record.owner).to eq("alice@example.com")
      expect(record.chain_id).to be_present
      expect(record.issued_at).to be_present
      expect(record.expires_at).to be > record.issued_at
      expect((record.expires_at - record.issued_at).to_i).to be_within(2).of(3600)
    end

    it "exposes used_at and revoked_at columns, both nil at issue time" do
      record, _ = OauthToken.issue(kind: "refresh", owner: "alice@example.com", lifetime: 30.days)
      expect(record.kind).to eq("refresh")
      expect(record.used_at).to be_nil
      expect(record.revoked_at).to be_nil
      [ :kind, :owner, :chain_id, :issued_at, :expires_at, :used_at, :revoked_at ].each do |col|
        expect(record).to respond_to(col)
      end
    end

    it "shares chain_id when an explicit chain_id is passed in (sibling rotation)" do
      access, _ = OauthToken.issue(kind: "access", owner: "alice@example.com", lifetime: 1.hour)
      refresh, _ = OauthToken.issue(
        kind: "refresh", owner: "alice@example.com", lifetime: 30.days, chain_id: access.chain_id
      )
      expect(refresh.chain_id).to eq(access.chain_id)
    end

    it "validates an inbound bearer with a single lookup keyed by the presented string" do
      _, plaintext = OauthToken.issue(kind: "access", owner: "alice@example.com", lifetime: 1.hour)
      expect(OauthToken.find_by_presented_token(plaintext)).to be_present
      expect(OauthToken.find_by_presented_token("not-a-real-token")).to be_nil
    end
  end

  describe "R-CUUP-REQT the row stores a SHA-256 digest, not the plaintext" do
    it "persists the SHA-256 digest of the plaintext, never the plaintext itself" do
      record, plaintext = OauthToken.issue(kind: "access", owner: "alice@example.com", lifetime: 1.hour)
      expect(record.token_digest).not_to eq(plaintext)
      expect(record.token_digest).to eq(Digest::SHA256.hexdigest(plaintext))
      expect(record).not_to respond_to(:token)
    end

    it "does not leave the plaintext anywhere on the row's attributes" do
      record, plaintext = OauthToken.issue(kind: "refresh", owner: "alice@example.com", lifetime: 30.days)
      record.reload
      record.attributes.each_value do |value|
        expect(value.to_s).not_to include(plaintext)
      end
    end

    it "validates inbound bearers by hashing then looking up by digest" do
      _, plaintext = OauthToken.issue(kind: "access", owner: "alice@example.com", lifetime: 1.hour)
      digest = Digest::SHA256.hexdigest(plaintext)
      expect(OauthToken.find_by(token_digest: digest)).to be_present
      expect(OauthToken.find_by_presented_token(plaintext)).to be_present
    end
  end
end
