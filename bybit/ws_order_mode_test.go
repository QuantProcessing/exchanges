package bybit

import (
	"context"
	"errors"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSpotPlaceOrderWSRoutesToTradeWS(t *testing.T) {
	var restHits int

	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
		placeOrderFn: func(_ context.Context, req sdk.PlaceOrderRequest) (*sdk.OrderActionResponse, error) {
			restHits++
			return &sdk.OrderActionResponse{OrderID: "rest-order"}, nil
		},
	})
	require.NoError(t, err)

	adp.tradeWS = &stubTradeWSClient{
		placeOrderFn: func(_ context.Context, req sdk.PlaceOrderRequest) error {
			require.Equal(t, "BTCUSDT", req.Symbol)
			return nil
		},
	}

	err = adp.PlaceOrderWS(context.Background(), &exchanges.OrderParams{
		Symbol:      "BTC",
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.1"),
		Price:       decimal.RequireFromString("100"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "cid-1",
	})
	require.NoError(t, err)
	require.Equal(t, 0, restHits)
}

func TestSpotPlaceOrderWSDoesNotSilentlyFallbackToREST(t *testing.T) {
	var restHits int

	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
		placeOrderFn: func(_ context.Context, req sdk.PlaceOrderRequest) (*sdk.OrderActionResponse, error) {
			restHits++
			return &sdk.OrderActionResponse{OrderID: "rest-order"}, nil
		},
	})
	require.NoError(t, err)

	adp.tradeWS = &stubTradeWSClient{
		placeOrderFn: func(_ context.Context, req sdk.PlaceOrderRequest) error {
			return errors.New("ws trade failed")
		},
	}

	err = adp.PlaceOrderWS(context.Background(), &exchanges.OrderParams{
		Symbol:      "BTC",
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.1"),
		Price:       decimal.RequireFromString("100"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "cid-1",
	})
	require.Error(t, err)
	require.Equal(t, 0, restHits)
}

func TestSpotCancelOrderWSRoutesToTradeWS(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
	})
	require.NoError(t, err)

	adp.tradeWS = &stubTradeWSClient{
		cancelOrderFn: func(_ context.Context, req sdk.CancelOrderRequest) error {
			require.Equal(t, "BTCUSDT", req.Symbol)
			require.Equal(t, "1", req.OrderID)
			return nil
		},
	}

	require.NoError(t, adp.CancelOrderWS(context.Background(), "1", "BTC"))
}

func TestSpotCancelOrderWSDoesNotSilentlyFallbackToREST(t *testing.T) {
	var restHits int

	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
		cancelOrderFn: func(_ context.Context, req sdk.CancelOrderRequest) (*sdk.OrderActionResponse, error) {
			restHits++
			return &sdk.OrderActionResponse{OrderID: "rest-order"}, nil
		},
	})
	require.NoError(t, err)

	adp.tradeWS = &stubTradeWSClient{
		cancelOrderFn: func(_ context.Context, req sdk.CancelOrderRequest) error {
			return errors.New("ws cancel failed")
		},
	}

	err = adp.CancelOrderWS(context.Background(), "1", "BTC")
	require.Error(t, err)
	require.Equal(t, 0, restHits)
}

func TestPerpModifyOrderWSRoutesToTradeWS(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
	})
	require.NoError(t, err)

	adp.tradeWS = &stubTradeWSClient{
		amendOrderFn: func(_ context.Context, req sdk.AmendOrderRequest) error {
			require.Equal(t, "BTCUSDT", req.Symbol)
			require.Equal(t, "1", req.OrderID)
			require.Equal(t, "0.2", req.Qty)
			require.Equal(t, "101", req.Price)
			return nil
		},
	}

	err = adp.ModifyOrderWS(context.Background(), "1", "BTC", &exchanges.ModifyOrderParams{
		Quantity: decimal.RequireFromString("0.2"),
		Price:    decimal.RequireFromString("101"),
	})
	require.NoError(t, err)
}

func TestPerpModifyOrderWSDoesNotSilentlyFallbackToREST(t *testing.T) {
	var restHits int

	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
		amendOrderFn: func(_ context.Context, req sdk.AmendOrderRequest) (*sdk.OrderActionResponse, error) {
			restHits++
			return &sdk.OrderActionResponse{OrderID: "rest-order"}, nil
		},
	})
	require.NoError(t, err)

	adp.tradeWS = &stubTradeWSClient{
		amendOrderFn: func(_ context.Context, req sdk.AmendOrderRequest) error {
			return errors.New("ws amend failed")
		},
	}

	err = adp.ModifyOrderWS(context.Background(), "1", "BTC", &exchanges.ModifyOrderParams{
		Quantity: decimal.RequireFromString("0.2"),
		Price:    decimal.RequireFromString("101"),
	})
	require.Error(t, err)
	require.Equal(t, 0, restHits)
}

type stubTradeWSClient struct {
	placeOrderFn  func(context.Context, sdk.PlaceOrderRequest) error
	cancelOrderFn func(context.Context, sdk.CancelOrderRequest) error
	amendOrderFn  func(context.Context, sdk.AmendOrderRequest) error
}

func (c *stubTradeWSClient) PlaceOrder(ctx context.Context, req sdk.PlaceOrderRequest) error {
	if c.placeOrderFn == nil {
		panic("unexpected PlaceOrder call")
	}
	return c.placeOrderFn(ctx, req)
}

func (c *stubTradeWSClient) CancelOrder(ctx context.Context, req sdk.CancelOrderRequest) error {
	if c.cancelOrderFn == nil {
		panic("unexpected CancelOrder call")
	}
	return c.cancelOrderFn(ctx, req)
}

func (c *stubTradeWSClient) AmendOrder(ctx context.Context, req sdk.AmendOrderRequest) error {
	if c.amendOrderFn == nil {
		panic("unexpected AmendOrder call")
	}
	return c.amendOrderFn(ctx, req)
}

func (c *stubTradeWSClient) Close() error { return nil }
