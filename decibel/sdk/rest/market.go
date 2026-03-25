package rest

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

func (c *Client) GetMarkets(ctx context.Context) ([]Market, error) {
	var markets []Market
	if err := c.get(ctx, "/api/v1/markets", nil, &markets); err != nil {
		return nil, err
	}
	return markets, nil
}

func (c *Client) GetTicker(ctx context.Context, market string) (*Ticker, error) {
	type priceSnapshot struct {
		Market            string          `json:"market"`
		OraclePrice       decimal.Decimal `json:"oracle_px"`
		MarkPrice         decimal.Decimal `json:"mark_px"`
		MidPrice          decimal.Decimal `json:"mid_px"`
		TransactionUnixMS int64           `json:"transaction_unix_ms"`
	}

	var prices []priceSnapshot
	if err := c.get(ctx, "/api/v1/prices", nil, &prices); err != nil {
		return nil, err
	}

	for _, price := range prices {
		if price.Market != market {
			continue
		}
		last := price.MarkPrice
		if last.IsZero() {
			last = price.OraclePrice
		}
		bid := price.MidPrice
		ask := price.MidPrice
		return &Ticker{
			Market:    price.Market,
			LastPrice: last,
			MarkPrice: price.MarkPrice,
			BidPrice:  bid,
			AskPrice:  ask,
			Timestamp: price.TransactionUnixMS,
		}, nil
	}

	return nil, exchanges.NewExchangeError("DECIBEL", "", fmt.Sprintf("market not found: %s", market), exchanges.ErrSymbolNotFound)
}

func (c *Client) GetOrderBook(ctx context.Context, market string, limit int) (*OrderBookSnapshot, error) {
	query := url.Values{}
	query.Set("market", market)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	var book OrderBookSnapshot
	if err := c.get(ctx, "/api/v1/orderbook", query, &book); err != nil {
		return nil, err
	}
	return &book, nil
}
