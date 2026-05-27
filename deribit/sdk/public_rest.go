package sdk

import (
	"context"
	"strconv"
)

func (c *Client) GetInstruments(ctx context.Context, currency, kind string, expired bool) ([]Instrument, error) {
	query := map[string]string{
		"currency": currency,
		"kind":     kind,
	}
	if expired {
		query["expired"] = "true"
	}
	var out []Instrument
	err := c.get(ctx, "/api/v2/public/get_instruments", query, &out)
	return out, err
}

func (c *Client) GetTicker(ctx context.Context, instrumentName string) (*Ticker, error) {
	var out Ticker
	err := c.get(ctx, "/api/v2/public/ticker", map[string]string{"instrument_name": instrumentName}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetOrderBook(ctx context.Context, instrumentName string, depth int) (*OrderBook, error) {
	query := map[string]string{"instrument_name": instrumentName}
	if depth > 0 {
		query["depth"] = strconv.Itoa(depth)
	}
	var out OrderBook
	err := c.get(ctx, "/api/v2/public/get_order_book", query, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetLastTradesByInstrument(ctx context.Context, instrumentName string, count int) (*TradesResult, error) {
	query := map[string]string{"instrument_name": instrumentName}
	if count > 0 {
		query["count"] = strconv.Itoa(count)
	}
	var out TradesResult
	err := c.get(ctx, "/api/v2/public/get_last_trades_by_instrument", query, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetTradingViewChartData(ctx context.Context, instrumentName string, start, end int64, resolution string) (*TradingViewChartData, error) {
	query := map[string]string{
		"instrument_name": instrumentName,
		"start_timestamp": strconv.FormatInt(start, 10),
		"end_timestamp":   strconv.FormatInt(end, 10),
		"resolution":      resolution,
	}
	var out TradingViewChartData
	err := c.get(ctx, "/api/v2/public/get_tradingview_chart_data", query, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
