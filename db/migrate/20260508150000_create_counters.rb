# R-WD9O-X90L: on a fresh database the counter is zero.
# R-UZ9T-8NM4: the counter is a non-negative integer.
class CreateCounters < ActiveRecord::Migration[8.1]
  def change
    create_table :counters do |t|
      t.integer :value, null: false, default: 0
      t.timestamps
    end
  end
end
