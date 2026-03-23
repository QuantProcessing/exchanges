package sdk

import (
	"context"
	"strconv"
)

func (c *Client) GetMarkets(ctx context.Context) ([]Market, error) {
	var out []Market
	err := c.get(ctx, "/api/v1/markets", nil, &out)
	return out, err
}

func (c *Client) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	var out Ticker
	err := c.get(ctx, "/api/v1/ticker", map[string]string{"symbol": symbol}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDepth(ctx context.Context, symbol string, limit int) (*Depth, error) {
	var out Depth
	query := map[string]string{"symbol": symbol}
	if limit > 0 {
		query["limit"] = strconv.Itoa(limit)
	}
	err := c.get(ctx, "/api/v1/depth", query, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetOrderBook(ctx context.Context, symbol string, limit int) (*Depth, error) {
	return c.GetDepth(ctx, symbol, limit)
}

func (c *Client) GetTrades(ctx context.Context, symbol string, limit int) ([]Trade, error) {
	query := map[string]string{"symbol": symbol}
	if limit > 0 {
		query["limit"] = strconv.Itoa(limit)
	}
	var out []Trade
	err := c.get(ctx, "/api/v1/trades", query, &out)
	return out, err
}

func (c *Client) GetFundingRates(ctx context.Context) ([]FundingRate, error) {
	var out []FundingRate
	err := c.get(ctx, "/api/v1/fundingRates", nil, &out)
	return out, err
}

func (c *Client) GetKlines(ctx context.Context, symbol, interval string, startTime, endTime int64, priceType string) ([]Kline, error) {
	query := map[string]string{
		"symbol":    symbol,
		"interval":  interval,
		"startTime": strconv.FormatInt(startTime, 10),
	}
	if endTime > 0 {
		query["endTime"] = strconv.FormatInt(endTime, 10)
	}
	if priceType != "" {
		query["priceType"] = priceType
	}
	var out []Kline
	err := c.get(ctx, "/api/v1/klines", query, &out)
	return out, err
}
