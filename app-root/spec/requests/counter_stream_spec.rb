require "rails_helper"

RSpec.describe "GET /counter/stream", type: :request do
  before { CounterBroadcaster.reset! }
  after { CounterBroadcaster.reset! }

  # The endpoint is a long-lived SSE stream; in tests we stub the
  # blocking hold-open phase so the action returns once the test has
  # observed what it cares about.
  def stub_hold_open!(&block)
    allow_any_instance_of(CounterStreamController)
      .to receive(:hold_open) do |controller, queue|
        block&.call(controller, queue)
      end
  end

  it "R-DRX9-8WNY responds with text/event-stream and emits the current value on subscribe" do
    Counter.current.update!(value: 17)
    stub_hold_open!

    get "/counter/stream"

    expect(response).to have_http_status(:ok)
    expect(response.headers["Content-Type"]).to start_with("text/event-stream")
    expect(response.body).to include('data: {"value":17}')
  end

  it "R-DRX9-8WNY requires no authentication" do
    Counter.current.update!(value: 3)
    stub_hold_open!

    get "/counter/stream"

    expect(response).to have_http_status(:ok)
    expect(response.body).to include('data: {"value":3}')
  end

  it "R-DRX9-8WNY emits later counter changes to a connected subscriber" do
    Counter.current.update!(value: 0)
    stub_hold_open! do |controller, queue|
      CounterBroadcaster.broadcast(42)
      controller.send(:drain_queue, queue)
    end

    get "/counter/stream"

    expect(response.body).to include('data: {"value":0}')
    expect(response.body).to include('data: {"value":42}')
  end

end
