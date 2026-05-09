# R-KPS9-C5XP: out of scope — per-user counters or any namespacing.
# Exactly one counter, shared by every caller.
require "rails_helper"

RSpec.describe "Counter scope posture" do
  describe "R-KPS9-C5XP no per-user or namespaced counters" do
    it "stores no user/owner/scope/namespace columns on the counters table" do
      columns = Counter.columns.map(&:name)
      forbidden = columns.grep(/\A(user|owner|account|tenant|scope|namespace|client)(_id)?\z/)
      expect(forbidden).to eq([])
    end

    it "treats Counter.current as a singleton row shared by all callers" do
      Counter.delete_all
      a = Counter.current
      b = Counter.current
      expect(a.id).to eq(b.id)
      expect(Counter.count).to eq(1)
    end
  end
end
