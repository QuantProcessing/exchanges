package option

import (
	"context"
	"fmt"
	"strings"
)

func (c *Client) ExchangeInfo(ctx context.Context) (*ExchangeInfoResponse, error) {
	var res ExchangeInfoResponse
	if err := c.Get(ctx, "/eapi/v1/exchangeInfo", nil, false, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) Depth(ctx context.Context, symbol string, limit int) (*DepthResponse, error) {
	params := map[string]interface{}{"symbol": strings.ToUpper(symbol)}
	if limit > 0 {
		params["limit"] = limit
	}
	var res DepthResponse
	if err := c.Get(ctx, "/eapi/v1/depth", params, false, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) Ticker(ctx context.Context, symbol string) (*TickerResponse, error) {
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required for Ticker")
	}
	params := map[string]interface{}{"symbol": strings.ToUpper(symbol)}
	var res TickerResponse
	if err := c.Get(ctx, "/eapi/v1/ticker", params, false, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) Trades(ctx context.Context, symbol string, limit int) ([]TradeResponse, error) {
	params := map[string]interface{}{"symbol": strings.ToUpper(symbol)}
	if limit > 0 {
		params["limit"] = limit
	}
	var res []TradeResponse
	if err := c.Get(ctx, "/eapi/v1/trades", params, false, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) Klines(ctx context.Context, symbol, interval string, limit int, startTime, endTime int64) ([]KlineResponse, error) {
	params := map[string]interface{}{
		"symbol":   strings.ToUpper(symbol),
		"interval": interval,
	}
	if limit > 0 {
		params["limit"] = limit
	}
	if startTime > 0 {
		params["startTime"] = startTime
	}
	if endTime > 0 {
		params["endTime"] = endTime
	}
	var res []KlineResponse
	if err := c.Get(ctx, "/eapi/v1/klines", params, false, &res); err != nil {
		return nil, err
	}
	return res, nil
}
