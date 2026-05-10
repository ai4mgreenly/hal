# R-I219-0C8A: out of scope — history, audit log, or reset operations
# on the counter. The counter supports only the three operations
# pinned by R-ECNJ-R09R (read, increment, decrement). Decrement is
# explicitly in scope; this spec must not forbid it.
require "rails_helper"

RSpec.describe "Counter out-of-scope posture (R-I219-0C8A)" do
  describe "Counter exposes no reset/history/audit operations" do
    it "defines none of the forbidden mutating methods on Counter" do
      forbidden = %i[reset reset! clear clear! set set! history audit
        rollback! rollback restore! restore versions revisions]
      defined = forbidden.select { |m| Counter.instance_methods(false).include?(m) }
      expect(defined).to eq([])
    end

    it "defines none of the forbidden class-level operations on Counter" do
      forbidden = %i[reset reset_all clear_all history audit_log rollback
        restore_all]
      defined = forbidden.select { |m| Counter.singleton_methods(false).include?(m) }
      expect(defined).to eq([])
    end
  end

  describe "Schema records no history/audit/version state" do
    it "stores no history/audit/version columns on the counters table" do
      columns = Counter.columns.map(&:name)
      forbidden = columns.grep(/\A(history|audit|version|previous|prior|log)(_.*)?\z/)
      expect(forbidden).to eq([])
    end

    it "creates no companion history/audit/version tables" do
      tables = ActiveRecord::Base.connection.tables
      forbidden = tables.grep(/(counter_history|counter_audit|counter_versions|counter_log|counter_revisions)/)
      expect(forbidden).to eq([])
    end
  end

  describe "Decrement remains in scope" do
    it "still defines decrement! on Counter (R-F5X4-XI2F)" do
      expect(Counter.instance_methods).to include(:decrement!)
    end
  end
end
