# R-DRX9-8WNY: in-process pub/sub for counter changes. Subscribers
# receive every value passed to .broadcast(value); the SSE controller
# uses one subscription per connected browser.
class CounterBroadcaster
  @mutex = Mutex.new
  @subscribers = {}
  @next_id = 0

  class << self
    def subscribe(&block)
      raise ArgumentError, "block required" unless block

      @mutex.synchronize do
        id = (@next_id += 1)
        @subscribers[id] = block
        id
      end
    end

    def unsubscribe(id)
      @mutex.synchronize { @subscribers.delete(id) }
    end

    def broadcast(value)
      callbacks = @mutex.synchronize { @subscribers.values.dup }
      callbacks.each do |cb|
        begin
          cb.call(value)
        rescue StandardError
          # A misbehaving subscriber must not stop other subscribers
          # from receiving the broadcast.
        end
      end
    end

    # Test-only: drop all subscribers so a test's expectations don't
    # leak into the next example.
    def reset!
      @mutex.synchronize { @subscribers.clear }
    end
  end
end
