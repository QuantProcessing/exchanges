package binance

import (
	"time"

	"github.com/QuantProcessing/exchanges/ratelimit"
)

// Binance rate limits (perp/futures):
// - IP: 2400 weight / 1 minute (per exchangeInfo rateLimits)
// - Order count: per account, per 10s and per day windows
// Ref: https://developers.binance.com/docs/zh-CN/binance-spot-api-docs/rest-api/limits

var rateLimitRules = []ratelimit.RateLimitRule{
	{
		Scope:    ratelimit.ScopeIP,
		Category: ratelimit.CategoryAll,
		Windows:  []ratelimit.Window{{Duration: 1 * time.Minute, Limit: 2400}},
	},
	{
		Scope:    ratelimit.ScopeAccount,
		Category: ratelimit.CategoryOrder,
		Windows: []ratelimit.Window{
			{Duration: 10 * time.Second, Limit: 300},
			{Duration: 1 * time.Minute, Limit: 1200},
		},
	},
}

var rateLimitWeights = map[string][]ratelimit.CategoryWeight{
	// Market Data (query weight)
	"FetchTicker":    {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOrderBook": {{Category: ratelimit.CategoryQuery, Weight: 5}},
	"FetchKlines":    {{Category: ratelimit.CategoryQuery, Weight: 5}},
	"FetchTrades":    {{Category: ratelimit.CategoryQuery, Weight: 5}},

	// Account (query weight)
	"FetchAccount":       {{Category: ratelimit.CategoryQuery, Weight: 5}},
	"FetchBalance":       {{Category: ratelimit.CategoryQuery, Weight: 5}},
	"FetchSymbolDetails": {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchFeeRate":       {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchPositions":     {{Category: ratelimit.CategoryQuery, Weight: 5}},
	"FetchFundingRate":   {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOrder":         {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"FetchOpenOrders":    {{Category: ratelimit.CategoryQuery, Weight: 1}},
	"SetLeverage":        {{Category: ratelimit.CategoryQuery, Weight: 1}},

	// Trading (query weight + order count)
	"PlaceOrder": {
		{Category: ratelimit.CategoryQuery, Weight: 1},
		{Category: ratelimit.CategoryOrder, Weight: 1},
	},
	"CancelOrder": {
		{Category: ratelimit.CategoryQuery, Weight: 1},
	},
	"ModifyOrder": {
		{Category: ratelimit.CategoryQuery, Weight: 1},
		{Category: ratelimit.CategoryOrder, Weight: 1},
	},
	"CancelAllOrders": {
		{Category: ratelimit.CategoryQuery, Weight: 1},
	},
}
