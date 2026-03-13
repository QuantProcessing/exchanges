// Package ratelimit provides a generic, declarative rate limiting engine.
//
// Each exchange declares its own RateLimitRule set and endpoint weights.
// The RateLimiter enforces these rules using a sliding window algorithm.
package ratelimit

import "time"

// ============================================================================
// Scope & Category
// ============================================================================

// LimitScope defines what entity the limit tracks against.
type LimitScope string

const (
	// ScopeIP tracks requests per IP address.
	ScopeIP LimitScope = "ip"
	// ScopeAccount tracks requests per account/wallet address.
	ScopeAccount LimitScope = "account"
)

// LimitCategory defines which request types share a bucket.
type LimitCategory string

const (
	// CategoryAll means all requests share one bucket.
	CategoryAll LimitCategory = "all"
	// CategoryQuery covers read/query requests (FetchTicker, FetchOrderBook, etc).
	CategoryQuery LimitCategory = "query"
	// CategoryTrade covers write/trade requests (PlaceOrder, CancelOrder, etc).
	CategoryTrade LimitCategory = "trade"
	// CategoryOrder adds an extra order-count bucket (e.g. Binance ORDER count).
	CategoryOrder LimitCategory = "order"
)

// ============================================================================
// Rule Declaration
// ============================================================================

// Window defines a single rate limit window.
type Window struct {
	Duration time.Duration // Window duration (e.g. 1*time.Minute, 10*time.Second)
	Limit    int           // Max total weight allowed within this window
}

// RateLimitRule describes one rate limit bucket for an exchanges.
// An exchange may declare multiple rules (e.g. IP query bucket + Account trade bucket).
type RateLimitRule struct {
	Scope    LimitScope    // What entity is tracked (IP / Account)
	Category LimitCategory // What request types count toward this bucket
	Windows  []Window      // One or more windows (requests must satisfy ALL windows)
}

// ============================================================================
// Weight Assignment
// ============================================================================

// CategoryWeight is the weight a single method consumes in a specific category.
type CategoryWeight struct {
	Category LimitCategory
	Weight   int
}

// ============================================================================
// Observability
// ============================================================================

// BucketStats reports current usage for one rate-limit bucket.
type BucketStats struct {
	Scope    LimitScope
	Category LimitCategory
	Windows  []WindowStats
}

// WindowStats reports usage within a single time window.
type WindowStats struct {
	Duration  time.Duration
	Limit     int
	Used      int
	Remaining int
}
