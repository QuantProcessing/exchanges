package nado

import (
	"time"

	"github.com/QuantProcessing/exchanges/ratelimit"
)

// Nado rate limits:
// - Query: IP-based, 2400/min or 400/10s
// - Execute: Wallet-based, 600/min or 100/10s
// - Each endpoint has specific weights
// Ref: https://docs.nado.xyz/developer-resources/api/rate-limits

var rateLimitRules = []ratelimit.RateLimitRule{
	// Query limits (IP-based)
	{
		Scope:    ratelimit.ScopeIP,
		Category: ratelimit.CategoryQuery,
		Windows: []ratelimit.Window{
			{Duration: 1 * time.Minute, Limit: 2400},
			{Duration: 10 * time.Second, Limit: 400},
		},
	},
	// Execute limits (Wallet-based)
	{
		Scope:    ratelimit.ScopeAccount,
		Category: ratelimit.CategoryTrade,
		Windows: []ratelimit.Window{
			{Duration: 1 * time.Minute, Limit: 600},
			{Duration: 10 * time.Second, Limit: 100},
		},
	},
}

var rateLimitWeights = map[string][]ratelimit.CategoryWeight{
	// Query endpoints (IP weight per docs)
	"FetchTicker":          {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOrderBook":       {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOrder":           {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOpenOrders":      {{Category: ratelimit.CategoryQuery, Weight: 2}}, // 2 * product_ids.length, use 2 as default
	"FetchKlines":          {{Category: ratelimit.CategoryQuery, Weight: 5}},
	"FetchTrades":          {{Category: ratelimit.CategoryQuery, Weight: 5}},
	"FetchAccount":         {{Category: ratelimit.CategoryQuery, Weight: 2}},
	"FetchBalance":         {{Category: ratelimit.CategoryQuery, Weight: 2}},
	"FetchPositions":       {{Category: ratelimit.CategoryQuery, Weight: 10}},
	"FetchSymbolDetails":   {{Category: ratelimit.CategoryQuery, Weight: 5}},
	"FetchFeeRate":         {{Category: ratelimit.CategoryQuery, Weight: 2}},
	"FetchFundingRate":     {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchAllFundingRates": {{Category: ratelimit.CategoryQuery, Weight: 5}},

	// Execute endpoints (Wallet weight)
	"PlaceOrder":      {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"CancelOrder":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"CancelAllOrders": {{Category: ratelimit.CategoryTrade, Weight: 50}},
	"ModifyOrder":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
}
