# R-DRX9-8WNY: SSE endpoint streaming counter values to any connected
# browser. No authentication required (the counter value is already
# public per R-SE5T-HP2J / R-3R73-2TN9 / R-0CQ7-DSBQ); the connection
# carries only the integer counter value.
class CounterStreamController < ActionController::Base
  include ActionController::Live

  # How often (seconds) the streaming loop wakes up to check for a
  # broadcast or a closed client. Test code stubs +hold_open+ so the
  # action returns after the initial event without waiting.
  POLL_INTERVAL = 0.5

  def show
    response.headers["Content-Type"] = "text/event-stream"
    response.headers["Cache-Control"] = "no-cache"
    response.headers["X-Accel-Buffering"] = "no"

    queue = Thread::Queue.new
    queue << Counter.current.value
    sub_id = CounterBroadcaster.subscribe { |v| queue << v }

    begin
      drain_queue(queue)
      hold_open(queue)
    rescue IOError, ActionController::Live::ClientDisconnected
      # Client went away; nothing more to do.
    ensure
      CounterBroadcaster.unsubscribe(sub_id)
      response.stream.close
    end
  end

  private

  def drain_queue(queue)
    until queue.empty?
      value = queue.pop(true)
      response.stream.write("data: #{ { value: value }.to_json }\n\n")
    end
  rescue ThreadError
    # Queue emptied between empty? and pop; safe to ignore.
  end

  def hold_open(queue)
    loop do
      sleep POLL_INTERVAL
      drain_queue(queue)
    end
  end
end
