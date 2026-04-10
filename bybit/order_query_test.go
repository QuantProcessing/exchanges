package bybit

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestFetchOrderByIDFallsBackToHistory(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
		getOpenOrdersFn: func(_ context.Context, category, symbol string) ([]sdk.OrderRecord, error) {
			return nil, nil
		},
		getOrderHistoryFn: func(_ context.Context, category, symbol string) ([]sdk.OrderRecord, error) {
			return []sdk.OrderRecord{{
				OrderID:     "1",
				OrderLinkID: "cid-1",
				Symbol:      "BTCUSDT",
				Side:        "Sell",
				OrderType:   "Market",
				TimeInForce: "IOC",
				Qty:         "0.1",
				CumExecQty:  "0.1",
				AvgPrice:    "50010",
				OrderStatus: "Filled",
				ReduceOnly:  true,
				CreatedTime: "1710000000000",
				UpdatedTime: "1710000000002",
			}}, nil
		},
	})
	require.NoError(t, err)

	order, err := adp.FetchOrderByID(context.Background(), "1", "BTC")
	require.NoError(t, err)
	require.Equal(t, "1", order.OrderID)
	require.Equal(t, exchanges.OrderStatusFilled, order.Status)
}

func TestFetchOrderByIDReturnsErrOrderNotFoundWhenMissing(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
		getOpenOrdersFn: func(_ context.Context, category, symbol string) ([]sdk.OrderRecord, error) {
			return nil, nil
		},
		getOrderHistoryFn: func(_ context.Context, category, symbol string) ([]sdk.OrderRecord, error) {
			return nil, nil
		},
	})
	require.NoError(t, err)

	_, err = adp.FetchOrderByID(context.Background(), "missing", "BTC")
	require.ErrorIs(t, err, exchanges.ErrOrderNotFound)
}

func TestSpotCancelAllOrdersUsesClient(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
		cancelAllOrdersFn: func(_ context.Context, req sdk.CancelAllOrdersRequest) error {
			require.Equal(t, categorySpot, req.Category)
			require.Equal(t, "BTCUSDT", req.Symbol)
			return nil
		},
	})
	require.NoError(t, err)

	require.NoError(t, adp.CancelAllOrders(context.Background(), "BTC"))
}

func TestPerpModifyOrderUsesClient(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
		amendOrderFn: func(_ context.Context, req sdk.AmendOrderRequest) (*sdk.OrderActionResponse, error) {
			require.Equal(t, categoryLinear, req.Category)
			require.Equal(t, "BTCUSDT", req.Symbol)
			require.Equal(t, "1", req.OrderID)
			require.Equal(t, "0.2", req.Qty)
			require.Equal(t, "101", req.Price)
			return &sdk.OrderActionResponse{OrderID: "1", OrderLinkID: "cid-1"}, nil
		},
		getOpenOrdersFn: func(_ context.Context, category, symbol string) ([]sdk.OrderRecord, error) {
			return []sdk.OrderRecord{{
				OrderID:     "1",
				OrderLinkID: "cid-1",
				Symbol:      "BTCUSDT",
				Side:        "Buy",
				OrderType:   "Limit",
				TimeInForce: "GTC",
				Price:       "101",
				Qty:         "0.2",
				OrderStatus: "New",
				CreatedTime: "1710000000000",
				UpdatedTime: "1710000000001",
			}}, nil
		},
	})
	require.NoError(t, err)

	order, err := adp.ModifyOrder(context.Background(), "1", "BTC", &exchanges.ModifyOrderParams{
		Quantity: decimal.RequireFromString("0.2"),
		Price:    decimal.RequireFromString("101"),
	})
	require.NoError(t, err)
	require.Equal(t, "1", order.OrderID)
	require.Equal(t, "101", order.Price.String())
}
