package sdk

import (
	"context"
	"fmt"
	"strconv"
)

func (c *Client) GetInstruments(ctx context.Context, category string) ([]Instrument, error) {
	var out []Instrument
	cursor := ""

	for {
		var resp responseEnvelope[InstrumentsResult]
		err := c.get(ctx, "/v5/market/instruments-info", map[string]string{
			"category": category,
			"limit":    strconv.Itoa(1000),
			"cursor":   cursor,
		}, &resp)
		if err != nil {
			return nil, err
		}
		if resp.RetCode != 0 {
			return nil, fmt.Errorf("bybit sdk: get instruments failed: %d %s", resp.RetCode, resp.RetMsg)
		}

		out = append(out, resp.Result.List...)
		if category == "spot" || resp.Result.NextPageCursor == "" {
			return out, nil
		}
		cursor = resp.Result.NextPageCursor
	}
}

func (c *Client) GetTicker(ctx context.Context, category, symbol string) (*Ticker, error) {
	var resp responseEnvelope[TickersResult]
	err := c.get(ctx, "/v5/market/tickers", map[string]string{
		"category": category,
		"symbol":   symbol,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit sdk: get ticker failed: %d %s", resp.RetCode, resp.RetMsg)
	}
	if len(resp.Result.List) == 0 {
		return nil, fmt.Errorf("bybit sdk: ticker not found for %s %s", category, symbol)
	}
	if resp.Result.List[0].Time == "" && resp.Time > 0 {
		resp.Result.List[0].Time = strconv.FormatInt(resp.Time, 10)
	}
	return &resp.Result.List[0], nil
}

func (c *Client) GetOrderBook(ctx context.Context, category, symbol string, limit int) (*OrderBook, error) {
	query := map[string]string{
		"category": category,
		"symbol":   symbol,
	}
	if limit > 0 {
		query["limit"] = strconv.Itoa(limit)
	}

	var resp responseEnvelope[OrderBook]
	err := c.get(ctx, "/v5/market/orderbook", query, &resp)
	if err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit sdk: get orderbook failed: %d %s", resp.RetCode, resp.RetMsg)
	}
	return &resp.Result, nil
}

func (c *Client) GetRecentTrades(ctx context.Context, category, symbol string, limit int) ([]PublicTrade, error) {
	query := map[string]string{
		"category": category,
		"symbol":   symbol,
	}
	if limit > 0 {
		query["limit"] = strconv.Itoa(limit)
	}

	var resp responseEnvelope[PublicTradesResult]
	err := c.get(ctx, "/v5/market/recent-trade", query, &resp)
	if err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit sdk: get recent trades failed: %d %s", resp.RetCode, resp.RetMsg)
	}
	return resp.Result.List, nil
}

func (c *Client) GetKlines(ctx context.Context, category, symbol, interval string, start, end int64, limit int) ([]Candle, error) {
	query := map[string]string{
		"category": category,
		"symbol":   symbol,
		"interval": interval,
	}
	if start > 0 {
		query["start"] = strconv.FormatInt(start, 10)
	}
	if end > 0 {
		query["end"] = strconv.FormatInt(end, 10)
	}
	if limit > 0 {
		query["limit"] = strconv.Itoa(limit)
	}

	var resp responseEnvelope[KlinesResult]
	err := c.get(ctx, "/v5/market/kline", query, &resp)
	if err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit sdk: get klines failed: %d %s", resp.RetCode, resp.RetMsg)
	}
	return resp.Result.List, nil
}
