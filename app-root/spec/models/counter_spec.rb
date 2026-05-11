# R-UC3P-Z0IX: there is exactly one counter, shared by all callers.
# R-WD9O-X90L: on a fresh database the counter is zero.
# R-UZ9T-8NM4: the counter is a non-negative integer.
# R-XMDZ-2RGA: increment takes no arguments and adds exactly one.
# R-RQZQ-81ZC: increment returns the post-increment value.
# R-F5X4-XI2F: decrement takes no arguments; refuses at zero.
# R-VNNS-W2G0: the counter persists across process restarts.
# R-TOI0-0Z8X: concurrent increments do not lose updates.
require "rails_helper"
require "open3"

RSpec.describe Counter, type: :model do
  describe "R-UC3P-Z0IX exactly one counter, shared by all callers" do
    it "returns the same record on repeated Counter.current calls" do
      first = Counter.current
      second = Counter.current
      expect(second.id).to eq(first.id)
    end

    it "shares state across callers — increment via one handle is visible via another" do
      a = Counter.current
      b = Counter.current
      a.update!(value: 0)
      a.increment!
      expect(b.reload.value).to eq(1)
    end

    it "Counter.current still resolves to the lowest-id row even if a stray row exists" do
      original = Counter.current
      Counter.create!(value: 99)
      expect(Counter.current.id).to eq(original.id)
    end
  end

  describe "R-WD9O-X90L fresh database reads zero" do
    it "Counter.current.value is 0 with no prior increments" do
      expect(Counter.current.value).to eq(0)
    end
  end

  describe "R-UZ9T-8NM4 counter is a non-negative integer" do
    it "rejects a negative value" do
      counter = Counter.current
      counter.value = -1
      expect(counter).not_to be_valid
      expect(counter.errors[:value]).to be_present
    end

    it "rejects a non-integer value" do
      counter = Counter.current
      counter.value = 1.5
      expect(counter).not_to be_valid
      expect(counter.errors[:value]).to be_present
    end

    it "accepts zero and positive integers" do
      counter = Counter.current
      counter.value = 0
      expect(counter).to be_valid
      counter.value = 42
      expect(counter).to be_valid
    end
  end

  describe "R-XMDZ-2RGA increment takes no arguments and adds exactly one" do
    it "adds exactly one to the stored value per call" do
      counter = Counter.current
      counter.update!(value: 5)
      counter.increment!
      expect(counter.reload.value).to eq(6)
      counter.increment!
      counter.increment!
      expect(counter.reload.value).to eq(8)
    end

    it "takes no arguments" do
      expect(Counter.instance_method(:increment!).arity).to eq(0)
    end
  end

  describe "R-RQZQ-81ZC increment returns the post-increment value" do
    it "returns the value after the increment is applied" do
      counter = Counter.current
      counter.update!(value: 10)
      expect(counter.increment!).to eq(11)
    end
  end

  describe "R-F5X4-XI2F decrement takes no arguments; refuses at zero" do
    it "subtracts exactly one when value is greater than zero" do
      counter = Counter.current
      counter.update!(value: 5)
      counter.decrement!
      expect(counter.reload.value).to eq(4)
    end

    it "returns the post-decrement value" do
      counter = Counter.current
      counter.update!(value: 7)
      expect(counter.decrement!).to eq(6)
    end

    it "takes no arguments" do
      expect(Counter.instance_method(:decrement!).arity).to eq(0)
    end

    it "refuses to decrement below zero, raising and leaving the value unchanged" do
      counter = Counter.current
      counter.update!(value: 0)
      expect { counter.decrement! }.to raise_error(Counter::DecrementBelowZero)
      expect(counter.reload.value).to eq(0)
    end
  end

  describe "R-K65O-80SH change notifications go out on the live-update channel" do
    before { CounterBroadcaster.reset! }
    after { CounterBroadcaster.reset! }

    it "broadcasts the post-increment value on increment!" do
      counter = Counter.current
      counter.update!(value: 4)
      received = []
      CounterBroadcaster.subscribe { |v| received << v }

      counter.increment!

      expect(received).to eq([ 5 ])
    end

    it "broadcasts the post-decrement value on decrement!" do
      counter = Counter.current
      counter.update!(value: 4)
      received = []
      CounterBroadcaster.subscribe { |v| received << v }

      counter.decrement!

      expect(received).to eq([ 3 ])
    end

    it "does not broadcast when a decrement is refused at zero" do
      counter = Counter.current
      counter.update!(value: 0)
      received = []
      CounterBroadcaster.subscribe { |v| received << v }

      expect { counter.decrement! }.to raise_error(Counter::DecrementBelowZero)
      expect(received).to be_empty
    end
  end

  describe "R-VNNS-W2G0 the counter persists across process restarts" do
    # Disable transactional fixtures so the increment commits to disk
    # before the second "process" reads the same sqlite file.
    self.use_transactional_tests = false

    after { Counter.delete_all }

    it "a separately-launched ruby process reads the last incremented value from the same sqlite file" do
      counter = Counter.current
      counter.update!(value: 0)
      3.times { counter.increment! }
      expect(counter.reload.value).to eq(3)

      db_path = ActiveRecord::Base.connection_db_config.database
      absolute = Rails.root.join(db_path).to_s

      script = <<~RUBY
        require "sqlite3"
        db = SQLite3::Database.new(#{absolute.inspect})
        row = db.execute("SELECT value FROM counters ORDER BY id ASC LIMIT 1").first
        puts row[0]
      RUBY

      stdout, stderr, status = Open3.capture3(RbConfig.ruby, "-e", script)
      expect(status).to be_success, "subprocess failed: #{stderr}"
      expect(stdout.strip).to eq("3")
    end
  end

  describe "R-TOI0-0Z8X concurrent increments do not lose updates" do
    # Real concurrent writes need real connections — opt out of the
    # transactional fixture wrapping so each thread sees committed state.
    self.use_transactional_tests = false

    after { Counter.delete_all }

    it "N successful increments across T threads raise the value by exactly N" do
      Counter.current.update!(value: 0)

      threads_count = 5
      per_thread = 10
      total = threads_count * per_thread

      threads = Array.new(threads_count) do
        Thread.new do
          per_thread.times do
            Counter.current.increment!
          ensure
            ActiveRecord::Base.connection_handler.clear_active_connections!(:all)
          end
        end
      end
      threads.each(&:join)

      expect(Counter.current.value).to eq(total)
    end
  end
end
