// Package counter owns the shared counter capability.
package counter

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Ticker is the small timer surface StreamHTTP needs from the host
// application clock.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// Counter is the shared non-negative counter, optionally backed by SQLite.
type Counter struct {
	mu    sync.Mutex
	value uint64
	db    *sql.DB
	bcast *Broadcaster
}

// New returns an in-memory counter initialized to zero.
func New() *Counter {
	return &Counter{}
}

// Broadcaster returns the counter-owned live-update broadcaster.
func (c *Counter) Broadcaster() *Broadcaster {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bcast == nil {
		c.bcast = &Broadcaster{}
	}
	return c.bcast
}

// Read returns the current counter value.
func (c *Counter) Read() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// Increment adds one and returns the post-increment value.
func (c *Counter) Increment() uint64 {
	c.mu.Lock()
	c.value++
	v := c.value
	if c.db != nil {
		_, _ = c.db.Exec(`UPDATE counter SET value = ? WHERE id = 1`, int64(v))
	}
	bcast := c.bcast
	c.mu.Unlock()
	if bcast != nil {
		bcast.Broadcast(v)
	}
	return v
}

// Decrement subtracts one when possible. It returns false at zero.
func (c *Counter) Decrement() (uint64, bool) {
	c.mu.Lock()
	if c.value == 0 {
		c.mu.Unlock()
		return 0, false
	}
	c.value--
	v := c.value
	if c.db != nil {
		_, _ = c.db.Exec(`UPDATE counter SET value = ? WHERE id = 1`, int64(v))
	}
	bcast := c.bcast
	c.mu.Unlock()
	if bcast != nil {
		bcast.Broadcast(v)
	}
	return v, true
}

// Attach binds a backing database and loads the persisted singleton value.
func (c *Counter) Attach(db *sql.DB) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var v int64
	if err := db.QueryRow(
		`SELECT value FROM counter WHERE id = 1`).Scan(&v); err != nil {
		return err
	}
	if v < 0 {
		return fmt.Errorf("counter: stored value %d is negative", v)
	}
	c.value = uint64(v)
	c.db = db
	return nil
}

// DetachDBIf clears the backing database if it is currently db.
func (c *Counter) DetachDBIf(db *sql.DB) {
	c.mu.Lock()
	if c.db == db {
		c.db = nil
	}
	c.mu.Unlock()
}

// Broadcaster fans out counter values to live subscribers.
type Broadcaster struct {
	mu   sync.Mutex
	subs map[*Subscriber]struct{}
}

// Subscriber is a live counter stream subscription.
type Subscriber struct {
	ch chan uint64
}

// Subscribe registers a live counter stream subscriber.
func (b *Broadcaster) Subscribe() *Subscriber {
	sub := &Subscriber{ch: make(chan uint64, 1)}
	b.mu.Lock()
	if b.subs == nil {
		b.subs = make(map[*Subscriber]struct{})
	}
	b.subs[sub] = struct{}{}
	b.mu.Unlock()
	return sub
}

// Unsubscribe releases a subscriber.
func (b *Broadcaster) Unsubscribe(sub *Subscriber) {
	b.mu.Lock()
	delete(b.subs, sub)
	b.mu.Unlock()
}

// SubscriberCount returns the number of live subscribers.
func (b *Broadcaster) SubscriberCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

// Broadcast delivers the latest counter value to subscribers.
func (b *Broadcaster) Broadcast(v uint64) {
	b.mu.Lock()
	targets := make([]*Subscriber, 0, len(b.subs))
	for s := range b.subs {
		targets = append(targets, s)
	}
	b.mu.Unlock()
	for _, s := range targets {
		select {
		case <-s.ch:
		default:
		}
		select {
		case s.ch <- v:
		default:
		}
	}
}

// StreamHTTP serves the counter live-update SSE endpoint.
func StreamHTTP(
	c *Counter, now func() time.Time, newTicker func(time.Duration) Ticker,
	heartbeatInterval, writeTimeout time.Duration, w http.ResponseWriter, r *http.Request,
) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported",
			http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-"+"stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // R-QHMK-0MIK: disables nginx proxy response buffering
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)

	bcast := c.Broadcaster()
	sub := bcast.Subscribe()
	defer bcast.Unsubscribe(sub)

	writeBytes := func(p []byte) error {
		_ = rc.SetWriteDeadline(now().Add(writeTimeout))
		if _, err := w.Write(p); err != nil {
			return err
		}
		flusher.Flush()
		_ = rc.SetWriteDeadline(time.Time{})
		return nil
	}
	writeValue := func(v uint64) error {
		return writeBytes([]byte(fmt.Sprintf(
			"data: {\"value\":%d}\n\n", v)))
	}

	if err := writeValue(c.Read()); err != nil {
		return
	}

	hb := newTicker(heartbeatInterval)
	defer hb.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-sub.ch:
			if err := writeValue(v); err != nil {
				return
			}
		case <-hb.C():
			if err := writeBytes([]byte(":hb\n\n")); err != nil {
				return
			}
		}
	}
}
