# R-UC3P-Z0IX: there is exactly one counter, shared by all callers.
# R-WD9O-X90L: on a fresh database the counter is zero.
# R-UZ9T-8NM4: the counter is a non-negative integer.
# R-XMDZ-2RGA: increment takes no arguments and adds exactly one.
# R-RQZQ-81ZC: increment returns the post-increment value.
# R-F5X4-XI2F: decrement takes no arguments; refuses at zero rather than clamping.
class Counter < ApplicationRecord
  class DecrementBelowZero < StandardError; end

  validates :value, numericality: { only_integer: true, greater_than_or_equal_to: 0 }

  # Returns the singleton counter row, creating it on first access so a
  # fresh database reads as zero (R-WD9O-X90L).
  def self.current
    first_or_create!
  end

  def increment!
    new_value = with_lock do
      update!(value: value + 1)
      value
    end
    # R-K65O-80SH: notify any subscribed live-update channels.
    CounterBroadcaster.broadcast(new_value)
    new_value
  end

  def decrement!
    new_value = with_lock do
      raise DecrementBelowZero if value.zero?
      update!(value: value - 1)
      value
    end
    # R-K65O-80SH: notify any subscribed live-update channels.
    CounterBroadcaster.broadcast(new_value)
    new_value
  end
end
