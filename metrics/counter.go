package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

type counter struct {
	mu           sync.Mutex
	generation   uint64
	up           atomic.Int64
	down         atomic.Int64
	rateUp       atomic.Int64
	rateDown     atomic.Int64
	peakUp       atomic.Int64
	peakDown     atomic.Int64
	active       atomic.Int64
	total        atomic.Int64
	dials        atomic.Int64
	dialFailures atomic.Int64
	latency      atomic.Int64
	latencySum   atomic.Int64
	lastDial     atomic.Int64
	lastActive   atomic.Int64
	lastUp       atomic.Int64
	lastDown     atomic.Int64
	prevUp       int64
	prevDown     int64
	prevAt       time.Time
}

func newCounter(now time.Time) *counter {
	return &counter{prevAt: now}
}

func (c *counter) open() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total.Add(1)
	c.active.Add(1)
	c.touch(time.Now().UnixNano())
	return c.generation
}

func (c *counter) close(generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation == generation {
		c.active.Add(-1)
	}
	c.touch(time.Now().UnixNano())
}

func (c *counter) dial(elapsed time.Duration, now time.Time, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dials.Add(1)
	if err != nil {
		c.dialFailures.Add(1)
	} else {
		if now.UnixNano() >= c.lastDial.Load() {
			c.latency.Store(int64(elapsed))
			c.lastDial.Store(now.UnixNano())
		}
		c.latencySum.Add(int64(elapsed))
	}
	c.touch(now.UnixNano())
}

func (c *counter) touch(now int64) {
	touchMax(&c.lastActive, now)
}

func (c *counter) addBytes(up bool, size int64, now int64) {
	if up {
		c.up.Add(size)
		touchMax(&c.lastUp, now)
	} else {
		c.down.Add(size)
		touchMax(&c.lastDown, now)
	}
	c.touch(now)
}

func touchMax(field *atomic.Int64, now int64) {
	for previous := field.Load(); now > previous; previous = field.Load() {
		if field.CompareAndSwap(previous, now) {
			return
		}
	}
}

func (c *counter) tick(now time.Time) {
	up, down := c.up.Load(), c.down.Load()
	if c.prevAt.IsZero() || now.Before(c.prevAt) {
		c.rateUp.Store(0)
		c.rateDown.Store(0)
	} else {
		elapsed := now.Sub(c.prevAt).Seconds()
		if elapsed == 0 {
			return
		}
		c.rateUp.Store(int64(float64(up-c.prevUp) / elapsed))
		c.rateDown.Store(int64(float64(down-c.prevDown) / elapsed))
	}
	touchMax(&c.peakUp, c.rateUp.Load())
	touchMax(&c.peakDown, c.rateDown.Load())
	c.prevUp, c.prevDown, c.prevAt = up, down, now
}

func (c *counter) reset(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	c.up.Store(0)
	c.down.Store(0)
	c.rateUp.Store(0)
	c.rateDown.Store(0)
	c.peakUp.Store(0)
	c.peakDown.Store(0)
	c.active.Store(0)
	c.total.Store(0)
	c.dials.Store(0)
	c.dialFailures.Store(0)
	c.latency.Store(0)
	c.latencySum.Store(0)
	c.lastDial.Store(0)
	c.lastActive.Store(0)
	c.lastUp.Store(0)
	c.lastDown.Store(0)
	c.prevUp, c.prevDown, c.prevAt = 0, 0, now
}
