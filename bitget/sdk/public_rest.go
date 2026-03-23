package sdk

import (
	"context"
	"fmt"
	"strconv"
)

func (c *Client) GetInstruments(ctx context.Context, category, symbol string) ([]Instrument, error) {
	var out responseEnvelope[[]Instrument]
	err := c.get(ctx, "/api/v3/market/instruments", map[string]string{
		"category": category,
		"symbol":   symbol,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: get instruments failed: %s %s", out.Code, out.Msg)
	}
	return out.Data, nil
}

func (c *Client) GetTicker(ctx context.Context, category, symbol string) (*Ticker, error) {
	var out responseEnvelope[[]Ticker]
	err := c.get(ctx, "/api/v3/market/tickers", map[string]string{
		"category": category,
		"symbol":   symbol,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: get ticker failed: %s %s", out.Code, out.Msg)
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("bitget sdk: ticker not found for %s %s", category, symbol)
	}
	return &out.Data[0], nil
}

func (c *Client) GetOrderBook(ctx context.Context, category, symbol string, limit int) (*OrderBook, error) {
	query := map[string]string{
		"category": category,
		"symbol":   symbol,
	}
	if limit > 0 {
		query["limit"] = strconv.Itoa(limit)
	}

	var out responseEnvelope[OrderBook]
	err := c.get(ctx, "/api/v3/market/orderbook", query, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: get orderbook failed: %s %s", out.Code, out.Msg)
	}
	return &out.Data, nil
}

func (c *Client) GetRecentFills(ctx context.Context, category, symbol string, limit int) ([]PublicFill, error) {
	query := map[string]string{
		"category": category,
		"symbol":   symbol,
	}
	if limit > 0 {
		query["limit"] = strconv.Itoa(limit)
	}

	var out responseEnvelope[[]PublicFill]
	err := c.get(ctx, "/api/v3/market/fills", query, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: get fills failed: %s %s", out.Code, out.Msg)
	}
	return out.Data, nil
}

func (c *Client) GetCandles(ctx context.Context, category, symbol, interval, candleType string, startTime, endTime int64, limit int) ([]Candle, error) {
	query := map[string]string{
		"category": category,
		"symbol":   symbol,
		"interval": interval,
	}
	if candleType != "" {
		query["type"] = candleType
	}
	if startTime > 0 {
		query["startTime"] = strconv.FormatInt(startTime, 10)
	}
	if endTime > 0 {
		query["endTime"] = strconv.FormatInt(endTime, 10)
	}
	if limit > 0 {
		query["limit"] = strconv.Itoa(limit)
	}

	var out responseEnvelope[[]Candle]
	err := c.get(ctx, "/api/v3/market/candles", query, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: get candles failed: %s %s", out.Code, out.Msg)
	}
	return out.Data, nil
}
