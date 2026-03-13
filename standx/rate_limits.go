package standx

import (
	"time"

	"github.com/QuantProcessing/exchanges/ratelimit"
)

// StandX rate limits:
// - IP: 50 requests / second
// - Account: credit-based token bucket (replenishes at constant rate)
// Since we don't know the exact credit budget, we use the IP limit as the primary constraint.
// Ref: https://docs.standx.com/standx-api/rate-limits

var rateLimitRules = []ratelimit.RateLimitRule{
	{
		Scope:    ratelimit.ScopeIP,
		Category: ratelimit.CategoryAll,
		Windows:  []ratelimit.Window{{Duration: 1 * time.Second, Limit: 50}},
	},
}

var rateLimitWeights = map[string][]ratelimit.CategoryWeight{
	// All endpoints get weight 1 (simple count-based)
	"FetchTicker":          {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOrderBook":       {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchKlines":          {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchTrades":          {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOrder":           {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOpenOrders":      {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchAccount":         {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchBalance":         {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchPositions":       {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchSymbolDetails":   {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchFeeRate":         {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchFundingRate":     {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchAllFundingRates": {{Category: ratelimit.CategoryQuery, Weight: 1}},

	// Trade endpoints
	"PlaceOrder":      {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"CancelOrder":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"ModifyOrder":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"CancelAllOrders": {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"SetLeverage":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
}
