package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Logger is the logging interface for the ratelimit package.
type Logger interface {
	Debugw(msg string, keysAndValues ...any)
}

// nopLogger discards all log output.
type nopLogger struct{}

func (nopLogger) Debugw(string, ...any) {}

// ============================================================================
// Sliding Window
// ============================================================================

// entry records a single request's weight and timestamp.
type entry struct {
	ts     time.Time
	weight int
}

// slidingWindow implements a sliding window rate limiter for a single window.
type slidingWindow struct {
	duration time.Duration
	limit    int

	mu      sync.Mutex
	entries []entry
}

// newSlidingWindow creates a new sliding window.
func newSlidingWindow(w Window) *slidingWindow {
	return &slidingWindow{
		duration: w.Duration,
		limit:    w.Limit,
		entries:  make([]entry, 0, 64),
	}
}

// evictExpired removes entries outside the current window. Must be called under lock.
func (sw *slidingWindow) evictExpired(now time.Time) {
	cutoff := now.Add(-sw.duration)
	i := 0
	for i < len(sw.entries) && sw.entries[i].ts.Before(cutoff) {
		i++
	}
	if i > 0 {
		// Shift remaining entries to front
		copy(sw.entries, sw.entries[i:])
		sw.entries = sw.entries[:len(sw.entries)-i]
	}
}

// usedWeight returns total weight used in the current window. Must be called under lock.
func (sw *slidingWindow) usedWeight(now time.Time) int {
	sw.evictExpired(now)
	total := 0
	for _, e := range sw.entries {
		total += e.weight
	}
	return total
}

// record adds a request to the window. Must be called under lock.
func (sw *slidingWindow) record(now time.Time, weight int) {
	sw.entries = append(sw.entries, entry{ts: now, weight: weight})
}

// timeUntilCapacity returns how long to wait before 'weight' capacity is available.
// Returns 0 if capacity is already available. Must be called under lock.
func (sw *slidingWindow) timeUntilCapacity(now time.Time, weight int) time.Duration {
	sw.evictExpired(now)
	used := 0
	for _, e := range sw.entries {
		used += e.weight
	}

	if used+weight <= sw.limit {
		return 0
	}

	// Walk entries from oldest; each one's expiry frees capacity.
	needed := used + weight - sw.limit
	for _, e := range sw.entries {
		needed -= e.weight
		if needed <= 0 {
			// This entry's expiry time is when we'll have enough capacity.
			expiresAt := e.ts.Add(sw.duration)
			wait := expiresAt.Sub(now)
			if wait < 0 {
				return 0
			}
			return wait
		}
	}

	// Shouldn't happen if weight <= limit, but be safe.
	return sw.duration
}

// stats returns current window stats.
func (sw *slidingWindow) stats(now time.Time) WindowStats {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	used := sw.usedWeight(now)
	remaining := sw.limit - used
	if remaining < 0 {
		remaining = 0
	}
	return WindowStats{
		Duration:  sw.duration,
		Limit:     sw.limit,
		Used:      used,
		Remaining: remaining,
	}
}

// ============================================================================
// Bucket
// ============================================================================

// bucket groups one or more sliding windows for a single Scope+Category pair.
// A request must satisfy ALL windows in the bucket.
type bucket struct {
	scope    LimitScope
	category LimitCategory
	windows  []*slidingWindow
}

// newBucket creates a bucket from a rule.
func newBucket(rule RateLimitRule) *bucket {
	windows := make([]*slidingWindow, len(rule.Windows))
	for i, w := range rule.Windows {
		windows[i] = newSlidingWindow(w)
	}
	return &bucket{
		scope:    rule.Scope,
		category: rule.Category,
		windows:  windows,
	}
}

// ============================================================================
// RateLimiter
// ============================================================================

// RateLimiter manages one or more rate limit buckets for a single exchange adapter.
type RateLimiter struct {
	buckets []*bucket
	logger  Logger
}

// NewRateLimiter creates a limiter from the exchange's declared rules.
func NewRateLimiter(rules []RateLimitRule, name string) *RateLimiter {
	return NewRateLimiterWithLogger(rules, name, nopLogger{})
}

// NewRateLimiterWithLogger creates a limiter with a custom logger.
func NewRateLimiterWithLogger(rules []RateLimitRule, name string, logger Logger) *RateLimiter {
	buckets := make([]*bucket, len(rules))
	for i, r := range rules {
		buckets[i] = newBucket(r)
	}
	if logger == nil {
		logger = nopLogger{}
	}
	return &RateLimiter{
		buckets: buckets,
		logger:  logger,
	}
}

// Acquire blocks until all applicable buckets have enough capacity
// for the given weights, or until ctx is cancelled.
//
// If no bucket matches a given weight's category, that weight is skipped.
func (rl *RateLimiter) Acquire(ctx context.Context, weights []CategoryWeight) error {
	for {
		now := time.Now()
		maxWait := time.Duration(0)

		// Check all buckets, compute max wait time across all.
		for _, b := range rl.buckets {
			weight := findWeight(weights, b.category)
			if weight <= 0 {
				continue
			}

			for _, sw := range b.windows {
				sw.mu.Lock()
				wait := sw.timeUntilCapacity(now, weight)
				sw.mu.Unlock()
				if wait > maxWait {
					maxWait = wait
				}
			}
		}

		// If no wait needed, record and return.
		if maxWait == 0 {
			for _, b := range rl.buckets {
				weight := findWeight(weights, b.category)
				if weight <= 0 {
					continue
				}
				for _, sw := range b.windows {
					sw.mu.Lock()
					sw.record(now, weight)
					sw.mu.Unlock()
				}
			}
			return nil
		}

		// Wait and retry.
		rl.logger.Debugw("[ratelimit] waiting", "wait", maxWait)
		timer := time.NewTimer(maxWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("rate limit wait cancelled: %w", ctx.Err())
		case <-timer.C:
			// Retry the loop.
		}
	}
}

// TryAcquire is a non-blocking version. Returns ErrRateLimited if insufficient capacity.
func (rl *RateLimiter) TryAcquire(weights []CategoryWeight) error {
	now := time.Now()

	// Check all buckets first.
	for _, b := range rl.buckets {
		weight := findWeight(weights, b.category)
		if weight <= 0 {
			continue
		}
		for _, sw := range b.windows {
			sw.mu.Lock()
			wait := sw.timeUntilCapacity(now, weight)
			sw.mu.Unlock()
			if wait > 0 {
				return fmt.Errorf("rate limited: bucket %s/%s needs %v wait", b.scope, b.category, wait)
			}
		}
	}

	// All buckets have capacity. Record.
	for _, b := range rl.buckets {
		weight := findWeight(weights, b.category)
		if weight <= 0 {
			continue
		}
		for _, sw := range b.windows {
			sw.mu.Lock()
			sw.record(now, weight)
			sw.mu.Unlock()
		}
	}
	return nil
}

// Stats returns current usage for all buckets.
func (rl *RateLimiter) Stats() []BucketStats {
	now := time.Now()
	stats := make([]BucketStats, len(rl.buckets))
	for i, b := range rl.buckets {
		wStats := make([]WindowStats, len(b.windows))
		for j, sw := range b.windows {
			wStats[j] = sw.stats(now)
		}
		stats[i] = BucketStats{
			Scope:    b.scope,
			Category: b.category,
			Windows:  wStats,
		}
	}
	return stats
}

// findWeight looks up the weight for a given category in the weights list.
// Also matches CategoryAll: if the bucket category is CategoryAll, it accepts
// any CategoryQuery or CategoryTrade weight; if the weight category is
// CategoryAll, it applies to any bucket category.
func findWeight(weights []CategoryWeight, bucketCategory LimitCategory) int {
	total := 0
	for _, w := range weights {
		if w.Category == bucketCategory {
			total += w.Weight
		} else if bucketCategory == CategoryAll {
			// "All" bucket accumulates query + trade weights
			total += w.Weight
		}
	}
	return total
}
