package lighter

import (
	"time"

	"github.com/QuantProcessing/exchanges/ratelimit"
)

// Lighter rate limits (standard accounts):
// - REST: 60 requests / rolling minute (standard users)
// - WS: 100 connections, 100 subs/conn, 200 msg/min
// - sendTx/sendTxBatch: separate bucket for premium (not enforced here)
// Ref: https://apidocs.lighter.xyz/docs/rate-limits

var rateLimitRules = []ratelimit.RateLimitRule{
	{
		Scope:    ratelimit.ScopeIP,
		Category: ratelimit.CategoryAll,
		Windows:  []ratelimit.Window{{Duration: 1 * time.Minute, Limit: 60}},
	},
}

var rateLimitWeights = map[string][]ratelimit.CategoryWeight{
	// Standard weight = 1 for most endpoints
	"FetchTicker":          {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOrderBook":       {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchKlines":          {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchTrades":          {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOrder":           {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOpenOrders":      {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchSymbolDetails":   {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchFeeRate":         {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchFundingRate":     {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchAllFundingRates": {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchAccount":         {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchBalance":         {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchPositions":       {{Category: ratelimit.CategoryQuery, Weight: 1}},

	// Trade endpoints (sendTx) — also counted in the standard bucket
	"PlaceOrder":      {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"CancelOrder":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"ModifyOrder":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"CancelAllOrders": {{Category: ratelimit.CategoryTrade, Weight: 1}},
}
