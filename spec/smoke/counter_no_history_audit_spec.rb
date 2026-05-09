require "rails_helper"

RSpec.describe "Counter mutation posture" do
  describe "R-LBQG-81A7 no history, audit log, decrement, or reset" do
    it "exposes no decrement/reset/audit/history instance methods on Counter" do
      forbidden = %i[decrement decrement! reset reset! reset_value audit history rollback undo]
      defined_methods = forbidden.select { |m| Counter.instance_methods(false).include?(m) }
      expect(defined_methods).to eq([])
    end

    it "stores no history/audit/version columns on the counters table" do
      columns = Counter.columns.map(&:name)
      forbidden = columns.grep(/\A(history|audit|version|previous_value|last_value|log)\z/)
      expect(forbidden).to eq([])
    end

    it "has no audit/history-style tables in the schema" do
      tables = ActiveRecord::Base.connection.tables
      forbidden = tables.grep(/(history|audit|version|paper_trail|log)/i)
      expect(forbidden).to eq([])
    end
  end
end
