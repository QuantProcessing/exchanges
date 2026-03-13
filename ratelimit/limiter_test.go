package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSlidingWindowBasic(t *testing.T) {
	sw := newSlidingWindow(Window{Duration: 100 * time.Millisecond, Limit: 3})

	now := time.Now()

	// Should have capacity for weight=1
	sw.mu.Lock()
	wait := sw.timeUntilCapacity(now, 1)
	sw.mu.Unlock()
	if wait != 0 {
		t.Fatalf("expected no wait, got %v", wait)
	}

	// Record 3 requests
	sw.mu.Lock()
	sw.record(now, 1)
	sw.record(now, 1)
	sw.record(now, 1)
	sw.mu.Unlock()

	// Should need to wait now
	sw.mu.Lock()
	wait = sw.timeUntilCapacity(now, 1)
	sw.mu.Unlock()
	if wait == 0 {
		t.Fatal("expected wait > 0 when at capacity")
	}
}

func TestSlidingWindowExpiry(t *testing.T) {
	sw := newSlidingWindow(Window{Duration: 50 * time.Millisecond, Limit: 2})

	now := time.Now()
	sw.mu.Lock()
	sw.record(now, 1)
	sw.record(now, 1)
	sw.mu.Unlock()

	// After the window expires, should have capacity again
	time.Sleep(60 * time.Millisecond)

	sw.mu.Lock()
	wait := sw.timeUntilCapacity(time.Now(), 1)
	sw.mu.Unlock()
	if wait != 0 {
		t.Fatalf("expected capacity after expiry, got wait=%v", wait)
	}
}

func TestRateLimiterAcquire(t *testing.T) {
	rules := []RateLimitRule{
		{
			Scope:    ScopeIP,
			Category: CategoryAll,
			Windows:  []Window{{Duration: 100 * time.Millisecond, Limit: 3}},
		},
	}
	rl := NewRateLimiter(rules, "test")

	ctx := context.Background()
	weights := []CategoryWeight{{Category: CategoryQuery, Weight: 1}}

	// Should be able to acquire 3 times quickly
	for i := 0; i < 3; i++ {
		if err := rl.Acquire(ctx, weights); err != nil {
			t.Fatalf("acquire %d failed: %v", i, err)
		}
	}

	// 4th acquire should block, use a short deadline
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	err := rl.Acquire(ctx2, weights)
	if err == nil {
		t.Fatal("expected timeout error on 4th acquire")
	}
}

func TestRateLimiterAcquireBlocks(t *testing.T) {
	rules := []RateLimitRule{
		{
			Scope:    ScopeIP,
			Category: CategoryAll,
			Windows:  []Window{{Duration: 50 * time.Millisecond, Limit: 1}},
		},
	}
	rl := NewRateLimiter(rules, "test")

	ctx := context.Background()
	weights := []CategoryWeight{{Category: CategoryQuery, Weight: 1}}

	// First acquire is instant
	if err := rl.Acquire(ctx, weights); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Second acquire should block for ~50ms
	start := time.Now()
	if err := rl.Acquire(ctx, weights); err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 30*time.Millisecond {
		t.Fatalf("expected blocking for at least 30ms, got %v", elapsed)
	}
}

func TestTryAcquire(t *testing.T) {
	rules := []RateLimitRule{
		{
			Scope:    ScopeIP,
			Category: CategoryAll,
			Windows:  []Window{{Duration: 100 * time.Millisecond, Limit: 1}},
		},
	}
	rl := NewRateLimiter(rules, "test")
	weights := []CategoryWeight{{Category: CategoryQuery, Weight: 1}}

	// First should succeed
	if err := rl.TryAcquire(weights); err != nil {
		t.Fatalf("first try failed: %v", err)
	}

	// Second should fail immediately
	if err := rl.TryAcquire(weights); err == nil {
		t.Fatal("expected error on second try")
	}
}

func TestMultipleWindows(t *testing.T) {
	rules := []RateLimitRule{
		{
			Scope:    ScopeIP,
			Category: CategoryAll,
			Windows: []Window{
				{Duration: 1 * time.Second, Limit: 10},
				{Duration: 100 * time.Millisecond, Limit: 2},
			},
		},
	}
	rl := NewRateLimiter(rules, "test")

	ctx := context.Background()
	weights := []CategoryWeight{{Category: CategoryQuery, Weight: 1}}

	// Can acquire 2 quickly (limited by shorter window)
	for i := 0; i < 2; i++ {
		if err := rl.Acquire(ctx, weights); err != nil {
			t.Fatalf("acquire %d failed: %v", i, err)
		}
	}

	// 3rd should block (short window at capacity even though long window has room)
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	if err := rl.Acquire(ctx2, weights); err == nil {
		t.Fatal("expected timeout on 3rd acquire (short window limit)")
	}
}

func TestMultipleBuckets(t *testing.T) {
	rules := []RateLimitRule{
		{
			Scope:    ScopeIP,
			Category: CategoryQuery,
			Windows:  []Window{{Duration: 100 * time.Millisecond, Limit: 2}},
		},
		{
			Scope:    ScopeAccount,
			Category: CategoryTrade,
			Windows:  []Window{{Duration: 100 * time.Millisecond, Limit: 1}},
		},
	}
	rl := NewRateLimiter(rules, "test")

	ctx := context.Background()

	// Query bucket allows 2
	qw := []CategoryWeight{{Category: CategoryQuery, Weight: 1}}
	if err := rl.Acquire(ctx, qw); err != nil {
		t.Fatal(err)
	}
	if err := rl.Acquire(ctx, qw); err != nil {
		t.Fatal(err)
	}

	// Trade bucket allows 1
	tw := []CategoryWeight{{Category: CategoryTrade, Weight: 1}}
	if err := rl.Acquire(ctx, tw); err != nil {
		t.Fatal(err)
	}

	// Extra trade should block
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	if err := rl.Acquire(ctx2, tw); err == nil {
		t.Fatal("expected timeout on extra trade")
	}

	// Query bucket is full too
	ctx3, cancel3 := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel3()
	if err := rl.Acquire(ctx3, qw); err == nil {
		t.Fatal("expected timeout on extra query")
	}
}

func TestWeightedRequests(t *testing.T) {
	rules := []RateLimitRule{
		{
			Scope:    ScopeIP,
			Category: CategoryAll,
			Windows:  []Window{{Duration: 100 * time.Millisecond, Limit: 10}},
		},
	}
	rl := NewRateLimiter(rules, "test")

	ctx := context.Background()

	// Use 8 out of 10 weight
	if err := rl.Acquire(ctx, []CategoryWeight{{Category: CategoryQuery, Weight: 8}}); err != nil {
		t.Fatal(err)
	}

	// Weight 3 should block (8 + 3 > 10)
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	if err := rl.Acquire(ctx2, []CategoryWeight{{Category: CategoryQuery, Weight: 3}}); err == nil {
		t.Fatal("expected timeout when weight would exceed limit")
	}

	// Weight 2 should pass (8 + 2 = 10)
	if err := rl.Acquire(ctx, []CategoryWeight{{Category: CategoryQuery, Weight: 2}}); err != nil {
		t.Fatalf("weight 2 should fit: %v", err)
	}
}

func TestStats(t *testing.T) {
	rules := []RateLimitRule{
		{
			Scope:    ScopeIP,
			Category: CategoryAll,
			Windows:  []Window{{Duration: 1 * time.Second, Limit: 10}},
		},
	}
	rl := NewRateLimiter(rules, "test")

	// Initially all capacity available
	stats := rl.Stats()
	if len(stats) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(stats))
	}
	if stats[0].Windows[0].Remaining != 10 {
		t.Fatalf("expected 10 remaining, got %d", stats[0].Windows[0].Remaining)
	}

	// After using some
	_ = rl.Acquire(context.Background(), []CategoryWeight{{Category: CategoryQuery, Weight: 3}})
	stats = rl.Stats()
	if stats[0].Windows[0].Used != 3 {
		t.Fatalf("expected 3 used, got %d", stats[0].Windows[0].Used)
	}
	if stats[0].Windows[0].Remaining != 7 {
		t.Fatalf("expected 7 remaining, got %d", stats[0].Windows[0].Remaining)
	}
}

func TestConcurrentAcquire(t *testing.T) {
	rules := []RateLimitRule{
		{
			Scope:    ScopeIP,
			Category: CategoryAll,
			Windows:  []Window{{Duration: 200 * time.Millisecond, Limit: 10}},
		},
	}
	rl := NewRateLimiter(rules, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var successCount atomic.Int64

	// Launch 20 goroutines each trying to acquire weight 1
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := rl.Acquire(ctx, []CategoryWeight{{Category: CategoryQuery, Weight: 1}})
			if err == nil {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// All 20 should eventually succeed (10 in first window, 10 in second)
	got := successCount.Load()
	if got != 20 {
		t.Fatalf("expected all 20 to succeed, got %d", got)
	}
}

func TestFindWeight(t *testing.T) {
	tests := []struct {
		name     string
		weights  []CategoryWeight
		category LimitCategory
		want     int
	}{
		{
			name:     "exact match",
			weights:  []CategoryWeight{{Category: CategoryQuery, Weight: 5}},
			category: CategoryQuery,
			want:     5,
		},
		{
			name:     "no match",
			weights:  []CategoryWeight{{Category: CategoryQuery, Weight: 5}},
			category: CategoryTrade,
			want:     0,
		},
		{
			name: "all bucket matches query",
			weights: []CategoryWeight{
				{Category: CategoryQuery, Weight: 5},
			},
			category: CategoryAll,
			want:     5,
		},
		{
			name: "all bucket sums query+trade",
			weights: []CategoryWeight{
				{Category: CategoryQuery, Weight: 3},
				{Category: CategoryTrade, Weight: 2},
			},
			category: CategoryAll,
			want:     5,
		},
		{
			name: "direct all weight matches any bucket",
			weights: []CategoryWeight{
				{Category: CategoryAll, Weight: 1},
			},
			category: CategoryQuery,
			want:     0, // CategoryAll weight only matches "all" bucket
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findWeight(tt.weights, tt.category)
			if got != tt.want {
				t.Errorf("findWeight() = %d, want %d", got, tt.want)
			}
		})
	}
}
