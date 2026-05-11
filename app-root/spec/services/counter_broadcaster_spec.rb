require "rails_helper"

RSpec.describe CounterBroadcaster do
  before { described_class.reset! }
  after { described_class.reset! }

  it "R-K65O-80SH delivers each broadcast value to every active subscriber" do
    received_a = []
    received_b = []
    described_class.subscribe { |v| received_a << v }
    described_class.subscribe { |v| received_b << v }

    described_class.broadcast(1)
    described_class.broadcast(2)

    expect(received_a).to eq([ 1, 2 ])
    expect(received_b).to eq([ 1, 2 ])
  end

  it "R-K65O-80SH stops delivering once a subscription is unsubscribed" do
    received = []
    id = described_class.subscribe { |v| received << v }

    described_class.broadcast(1)
    described_class.unsubscribe(id)
    described_class.broadcast(2)

    expect(received).to eq([ 1 ])
  end

  it "R-K65O-80SH isolates subscriber failures from each other" do
    received = []
    described_class.subscribe { |_| raise "boom" }
    described_class.subscribe { |v| received << v }

    expect { described_class.broadcast(7) }.not_to raise_error
    expect(received).to eq([ 7 ])
  end

  it "R-K65O-80SH requires a block on subscribe" do
    expect { described_class.subscribe }.to raise_error(ArgumentError)
  end
end
