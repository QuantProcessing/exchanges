package hyperliquid

import (
	"time"

	"github.com/QuantProcessing/exchanges/ratelimit"
)

// Hyperliquid rate limits:
// - IP: 1200 weight / minute (exchange actions=1, info requests=2/20/60)
// - Max 10 WS connections, 1000 subscriptions, 2000 msg/min
// - Address-based limits are volume-linked, not enforced client-side
// Ref: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/rate-limits-and-user-limits

var rateLimitRules = []ratelimit.RateLimitRule{
	{
		Scope:    ratelimit.ScopeIP,
		Category: ratelimit.CategoryAll,
		Windows:  []ratelimit.Window{{Duration: 1 * time.Minute, Limit: 1200}},
	},
}

var rateLimitWeights = map[string][]ratelimit.CategoryWeight{
	// info requests with weight 2
	"FetchTicker":    {{Category: ratelimit.CategoryQuery, Weight: 2}},
	"FetchOrderBook": {{Category: ratelimit.CategoryQuery, Weight: 2}},
	"FetchOrder":     {{Category: ratelimit.CategoryQuery, Weight: 2}},

	// info requests with weight 20
	"FetchKlines":          {{Category: ratelimit.CategoryQuery, Weight: 20}},
	"FetchTrades":          {{Category: ratelimit.CategoryQuery, Weight: 20}},
	"FetchOpenOrders":      {{Category: ratelimit.CategoryQuery, Weight: 20}},
	"FetchFeeRate":         {{Category: ratelimit.CategoryQuery, Weight: 20}},
	"FetchFundingRate":     {{Category: ratelimit.CategoryQuery, Weight: 20}},
	"FetchAllFundingRates": {{Category: ratelimit.CategoryQuery, Weight: 20}},
	"FetchSymbolDetails":   {{Category: ratelimit.CategoryQuery, Weight: 20}},

	// clearinghouseState = weight 2
	"FetchAccount":   {{Category: ratelimit.CategoryQuery, Weight: 2}},
	"FetchBalance":   {{Category: ratelimit.CategoryQuery, Weight: 2}},
	"FetchPositions": {{Category: ratelimit.CategoryQuery, Weight: 2}},

	// exchange actions = weight 1
	"PlaceOrder":      {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"CancelOrder":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"ModifyOrder":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"CancelAllOrders": {{Category: ratelimit.CategoryTrade, Weight: 1}},
	"SetLeverage":     {{Category: ratelimit.CategoryTrade, Weight: 1}},
}
