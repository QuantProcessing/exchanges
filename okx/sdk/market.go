package okx

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// GetTickers retrieves the tickers for a specific instrument type and family.
func (c *Client) GetTickers(ctx context.Context, instType string, instFamily *string) ([]Ticker, error) {
	params := url.Values{}
	params.Add("instType", instType)
	if instFamily != nil {
		params.Add("instFamily", *instFamily)
	}
	path := "/api/v5/market/tickers?" + params.Encode()

	return Request[Ticker](c, ctx, MethodGet, path, nil, false)
}

// GetTicker retrieves the ticker for a specific instrument.
func (c *Client) GetTicker(ctx context.Context, instId string) ([]Ticker, error) {
	params := url.Values{}
	params.Add("instId", instId)
	path := "/api/v5/market/ticker?" + params.Encode()

	return Request[Ticker](c, ctx, MethodGet, path, nil, false)
}

// GetOrderBook retrieves order book depth.
// sz: depth size, e.g. 400
func (c *Client) GetOrderBook(ctx context.Context, instId string, sz *int) ([]OrderBook, error) {
	params := url.Values{}
	params.Add("instId", instId)
	if sz != nil {
		params.Add("sz", fmt.Sprintf("%d", *sz))
	}
	path := "/api/v5/market/books?" + params.Encode()

	return Request[OrderBook](c, ctx, MethodGet, path, nil, false)
}

// GetInstruments retrieves instruments information.
// instType: SPOT, SWAP, FUTURES, OPTION, MARGIN
func (c *Client) GetInstruments(ctx context.Context, instType string) ([]Instrument, error) {
	params := url.Values{}
	params.Add("instType", instType)
	path := "/api/v5/public/instruments?" + params.Encode()

	return Request[Instrument](c, ctx, MethodGet, path, nil, false)
}

// GetCandles retrieves candles for a specific instrument.
// bar: 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h
// limit: default 100, max 300
func (c *Client) GetCandles(ctx context.Context, instId string, bar *string, after *string, before *string, limit *int) ([]Candle, error) {
	params := url.Values{}
	params.Add("instId", instId)
	if bar != nil {
		params.Add("bar", *bar)
	}
	if after != nil {
		params.Add("after", *after)
	}
	if before != nil {
		params.Add("before", *before)
	}
	if limit != nil {
		params.Add("limit", fmt.Sprintf("%d", *limit))
	}
	path := "/api/v5/market/candles?" + params.Encode()

	return Request[Candle](c, ctx, MethodGet, path, nil, false)
}

// GetFundingRate retrieves the current funding rate for a specific instrument
// Returns per-hour funding rate (standardized)
func (c *Client) GetFundingRate(ctx context.Context, instId string) (*FundingRateData, error) {
	params := url.Values{}
	params.Add("instId", instId)
	path := "/api/v5/public/funding-rate?" + params.Encode()

	resp, err := Request[FundingRate](c, ctx, MethodGet, path, nil, false)
	if err != nil {
		return nil, err
	}

	if len(resp) == 0 {
		return nil, fmt.Errorf("no funding rate data found for %s", instId)
	}

	// Convert to standardized format with hourly rate
	return convertOKXFundingToStandardized(&resp[0])
}

// GetAllFundingRates retrieves funding rates for all instruments
// Returns per-hour funding rates (standardized)
func (c *Client) GetAllFundingRates(ctx context.Context) ([]FundingRateData, error) {
	params := url.Values{}
	params.Add("instId", "ANY")
	path := "/api/v5/public/funding-rate?" + params.Encode()
	resp, err := Request[FundingRate](c, ctx, MethodGet, path, nil, false)
	if err != nil {
		return nil, err
	}

	var result []FundingRateData
	for i := range resp {
		converted, err := convertOKXFundingToStandardized(&resp[i])
		if err != nil {
			// Skip items that can't be converted
			continue
		}
		result = append(result, *converted)
	}

	return result, nil
}

// OpenInterest matches one element of /api/v5/public/open-interest's data array.
type OpenInterest struct {
	InstType string `json:"instType"`
	InstId   string `json:"instId"`
	OI       string `json:"oi"`              // open interest in contracts
	OICcy    string `json:"oiCcy"`           // OI in coin (base asset)
	OIUsd    string `json:"oiUsd,omitempty"` // OI in USD
	Ts       string `json:"ts"`
}

// GetOpenInterest returns the current open interest for a single instrument.
// Docs: https://www.okx.com/docs-v5/en/#public-data-rest-api-get-open-interest
func (c *Client) GetOpenInterest(ctx context.Context, instId string) (*OpenInterest, error) {
	// OKX requires instType; default SWAP covers the perp adapters in this repo.
	instType := "SWAP"
	path := fmt.Sprintf("/api/v5/public/open-interest?instType=%s&instId=%s", instType, instId)
	resp, err := Request[OpenInterest](c, ctx, MethodGet, path, nil, false)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("okx: empty open-interest response for %s", instId)
	}
	return &resp[0], nil
}

// FundingRateHistoryEntry matches one element of /api/v5/public/funding-rate-history.
type FundingRateHistoryEntry struct {
	InstType     string `json:"instType"`
	InstId       string `json:"instId"`
	FundingRate  string `json:"fundingRate"`
	RealizedRate string `json:"realizedRate"`
	FundingTime  string `json:"fundingTime"`
	Method       string `json:"method"`
}

// GetFundingRateHistory retrieves historical funding rates.
// beforeMillis / afterMillis are optional OKX timestamp cursors (pass 0 to omit).
// OKX `before` = newer than timestamp; `after` = older than timestamp.
// limit is capped at 100 by OKX; zero means exchange default.
// Docs: https://www.okx.com/docs-v5/en/#public-data-rest-api-get-funding-rate-history
func (c *Client) GetFundingRateHistory(ctx context.Context, instId string, beforeMillis, afterMillis int64, limit int) ([]FundingRateHistoryEntry, error) {
	u := url.Values{}
	u.Set("instId", instId)
	if beforeMillis > 0 {
		u.Set("before", strconv.FormatInt(beforeMillis, 10))
	}
	if afterMillis > 0 {
		u.Set("after", strconv.FormatInt(afterMillis, 10))
	}
	if limit > 0 {
		u.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/v5/public/funding-rate-history?" + u.Encode()
	return Request[FundingRateHistoryEntry](c, ctx, MethodGet, path, nil, false)
}

// HistoryTrade matches one element of /api/v5/market/history-trades.
type HistoryTrade struct {
	InstId  string `json:"instId"`
	TradeId string `json:"tradeId"`
	Px      string `json:"px"`
	Sz      string `json:"sz"`
	Side    string `json:"side"`
	Ts      string `json:"ts"`
}

// GetHistoryTrades returns paginated historical public trades.
// typ is OKX's pagination mode: 1=by tradeId, 2=by timestamp.
// before/after are cursors; either may be empty.
// Docs: https://www.okx.com/docs-v5/en/#public-data-rest-api-get-trades-history
func (c *Client) GetHistoryTrades(ctx context.Context, instId string, typ int, before, after string, limit int) ([]HistoryTrade, error) {
	u := url.Values{}
	u.Set("instId", instId)
	if typ > 0 {
		u.Set("type", strconv.Itoa(typ))
	}
	if before != "" {
		u.Set("before", before)
	}
	if after != "" {
		u.Set("after", after)
	}
	if limit > 0 {
		u.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/v5/market/history-trades?" + u.Encode()
	return Request[HistoryTrade](c, ctx, MethodGet, path, nil, false)
}

// convertOKXFundingToStandardized converts OKX funding rate to standardized hourly format
func convertOKXFundingToStandardized(funding *FundingRate) (*FundingRateData, error) {
	// Calculate interval from time difference (in milliseconds)
	fundingTime, err := strconv.ParseInt(funding.FundingTime, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse funding time: %w", err)
	}

	nextFundingTime, err := strconv.ParseInt(funding.NextFundingTime, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse next funding time: %w", err)
	}

	// Calculate interval in hours
	intervalMs := nextFundingTime - fundingTime
	intervalHours := intervalMs / (1000 * 3600)
	if intervalHours == 0 {
		intervalHours = 8 // default fallback
	}

	// Convert funding rate to hourly
	rate, err := strconv.ParseFloat(funding.FundingRate, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse funding rate: %w", err)
	}

	hourlyRate := rate / float64(intervalHours)

	return &FundingRateData{
		Symbol:               funding.InstrumentID,
		FundingRate:          fmt.Sprintf("%.10f", hourlyRate),
		FundingIntervalHours: intervalHours,
		FundingTime:          funding.FundingTime,
		NextFundingTime:      funding.NextFundingTime,
	}, nil
}
