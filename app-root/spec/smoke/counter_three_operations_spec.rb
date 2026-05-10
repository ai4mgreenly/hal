# R-ECNJ-R09R: umbrella claim — exactly three operations on the
# counter (read, increment, decrement) across all transports. No
# other counter operations exist.
require "rails_helper"

RSpec.describe "Counter operations posture (R-ECNJ-R09R)" do
  describe "HTTP API exposes exactly the three operations" do
    let(:counter_routes) do
      Rails.application.routes.routes.map do |r|
        { verb: r.verb, path: r.path.spec.to_s.sub("(.:format)", "") }
      end.select { |r| r[:path] == "/counter" || r[:path].start_with?("/counter/") }
       # R-DRX9-8WNY's /counter/stream is a live-update notification
       # channel, not a counter operation R-ECNJ-R09R counts.
       .reject { |r| r[:path] == "/counter/stream" }
    end

    it "defines exactly three counter routes (read, increment, decrement)" do
      expect(counter_routes).to contain_exactly(
        { verb: "GET", path: "/counter" },
        { verb: "POST", path: "/counter/increment" },
        { verb: "POST", path: "/counter/decrement" }
      )
    end

    it "exposes exactly the three public actions on CounterController" do
      action_names = CounterController.action_methods.to_a.sort
      expect(action_names).to eq(%w[decrement increment show])
    end
  end

  describe "MCP advertises exactly three counter tools", type: :request do
    it "returns three tools from tools/list, one per operation" do
      post "/mcp",
        params: { jsonrpc: "2.0", id: 1, method: "tools/list" }.to_json,
        headers: { "Content-Type" => "application/json", "Accept" => "application/json" }
      body = JSON.parse(response.body)
      names = body.fetch("result").fetch("tools").map { |t| t["name"] }
      expect(names).to contain_exactly("counter_read", "counter_increment", "counter_decrement")
    end
  end
end
