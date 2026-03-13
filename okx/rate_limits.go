package okx

import (
	"time"

	"github.com/QuantProcessing/exchanges/ratelimit"
)

// OKX rate limits:
// - Public endpoints: 20 requests / 2 seconds (IP-based)
// - Private endpoints: 6 requests / 2 seconds (User ID-based)
// - Order operations: per instrument ID, separate from data queries
// Ref: https://www.okx.com/docs-v5/en/#overview-rate-limit

var rateLimitRules = []ratelimit.RateLimitRule{
	// Public data: 20 req / 2s = 600 req/min (IP-based)
	{
		Scope:    ratelimit.ScopeIP,
		Category: ratelimit.CategoryQuery,
		Windows:  []ratelimit.Window{{Duration: 2 * time.Second, Limit: 20}},
	},
	// Private endpoints: 6 req / 2s (Account-based)
	{
		Scope:    ratelimit.ScopeAccount,
		Category: ratelimit.CategoryTrade,
		Windows:  []ratelimit.Window{{Duration: 2 * time.Second, Limit: 6}},
	},
}

var rateLimitWeights = map[string][]ratelimit.CategoryWeight{
	// Public data endpoints (IP-based)
	"FetchTicker":          {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOrderBook":       {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchKlines":          {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchTrades":          {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchSymbolDetails":   {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchFundingRate":     {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchAllFundingRates": {{Category: ratelimit.CategoryQuery, Weight: 1}},

	// Private data endpoints (also Account-based in OKX, counted as trade)
	"FetchAccount":    {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"FetchBalance":    {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"FetchPositions":  {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"FetchFeeRate":    {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"FetchOrder":      {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"FetchOpenOrders": {{Category: ratelimit.CategoryTrade, Weight: 1}},

	// Trading endpoints (Account-based)
	"PlaceOrder":      {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"CancelOrder":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"ModifyOrder":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"CancelAllOrders": {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"SetLeverage":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
}
