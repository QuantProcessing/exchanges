package kernel

import (
	"sort"
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
}

type LiveClock struct{}

func (LiveClock) Now() time.Time {
	return time.Now()
}

func (LiveClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

type TestClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []testTimer
}

type testTimer struct {
	at time.Time
	ch chan time.Time
}

func NewTestClock(start time.Time) *TestClock {
	return &TestClock{now: start}
}

func (c *TestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *TestClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	timer := testTimer{at: c.now.Add(d), ch: ch}
	c.timers = append(c.timers, timer)
	sort.Slice(c.timers, func(i, j int) bool {
		return c.timers[i].at.Before(c.timers[j].at)
	})
	return ch
}

func (c *TestClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	due := make([]testTimer, 0)
	pending := c.timers[:0]
	for _, timer := range c.timers {
		if !timer.at.After(now) {
			due = append(due, timer)
			continue
		}
		pending = append(pending, timer)
	}
	c.timers = pending
	c.mu.Unlock()

	for _, timer := range due {
		timer.ch <- timer.at
		close(timer.ch)
	}
}
