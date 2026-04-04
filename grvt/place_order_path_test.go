package grvt

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	sdkgrvt "github.com/QuantProcessing/exchanges/grvt/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type fakeTradeRPCClient struct {
	connectCalls int
	placeReq     *sdkgrvt.OrderRequest
	placeResp    *sdkgrvt.CreateOrderResponse
	cancelReq    *sdkgrvt.CancelOrderRequest
}

func (f *fakeTradeRPCClient) Connect() error {
	f.connectCalls++
	return nil
}

func (f *fakeTradeRPCClient) Close() {}

func (f *fakeTradeRPCClient) PlaceOrder(_ context.Context, req *sdkgrvt.OrderRequest) (*sdkgrvt.CreateOrderResponse, error) {
	f.placeReq = req
	return f.placeResp, nil
}

func (f *fakeTradeRPCClient) CancelOrder(_ context.Context, req *sdkgrvt.CancelOrderRequest) (*sdkgrvt.CancelOrderResponse, error) {
	f.cancelReq = req
	return &sdkgrvt.CancelOrderResponse{}, nil
}

func TestPlaceOrder_UsesTradeRPCPath(t *testing.T) {
	t.Parallel()

	rpc := &fakeTradeRPCClient{
		placeResp: &sdkgrvt.CreateOrderResponse{
			Result: sdkgrvt.Order{
				OrderID: "rpc-order",
				Legs: []sdkgrvt.OrderLeg{{
					Instrument:    "ETH_USDT_Perp",
					Size:          "0.25",
					LimitPrice:    "0",
					IsBuyintAsset: true,
				}},
				Metadata: sdkgrvt.OrderMetadata{
					ClientOrderID: "cli-1",
					CreatedTime:   "1710000000000000000",
				},
				State: sdkgrvt.OrderState{
					Status:       sdkgrvt.OrderStatusPending,
					TradedSize:   []string{},
					AvgFillPrice: []string{},
				},
			},
		},
	}

	adp := &Adapter{
		BaseAdapter:   exchanges.NewBaseAdapter("GRVT", exchanges.MarketTypePerp, exchanges.NopLogger),
		wsTradeRpc:    rpc,
		apiKey:        "api-key",
		privateKey:    "private-key",
		subAccountID:  42,
		quoteCurrency: "USDT",
	}
	adp.SetSymbolDetails(map[string]*exchanges.SymbolDetails{
		"ETH": {
			Symbol:            "ETH",
			MinQuantity:       decimal.RequireFromString("0.001"),
			PricePrecision:    1,
			QuantityPrecision: 3,
		},
	})

	order, err := adp.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("0.25"),
		ClientID: "cli-1",
	})
	require.NoError(t, err)
	require.Equal(t, 1, rpc.connectCalls)
	require.NotNil(t, rpc.placeReq)
	require.Equal(t, uint64(42), rpc.placeReq.SubAccountID)
	require.True(t, rpc.placeReq.IsMarket)
	require.Equal(t, "ETH_USDT_Perp", rpc.placeReq.Legs[0].Instrument)
	require.Equal(t, "0.25", rpc.placeReq.Legs[0].Size)
	require.Equal(t, "cli-1", rpc.placeReq.Metadata.ClientOrderID)
	require.Equal(t, "rpc-order", order.OrderID)
	require.Equal(t, exchanges.OrderStatusNew, order.Status)
}

func TestCancelOrder_UsesTradeRPCPath(t *testing.T) {
	t.Parallel()

	rpc := &fakeTradeRPCClient{}
	adp := &Adapter{
		BaseAdapter:   exchanges.NewBaseAdapter("GRVT", exchanges.MarketTypePerp, exchanges.NopLogger),
		wsTradeRpc:    rpc,
		apiKey:        "api-key",
		privateKey:    "private-key",
		subAccountID:  42,
		quoteCurrency: "USDT",
	}

	err := adp.CancelOrder(context.Background(), "order-1", "ETH")
	require.NoError(t, err)
	require.Equal(t, 1, rpc.connectCalls)
	require.NotNil(t, rpc.cancelReq)
	require.Equal(t, "42", rpc.cancelReq.SubAccountID)
	require.NotNil(t, rpc.cancelReq.OrderID)
	require.Equal(t, "order-1", *rpc.cancelReq.OrderID)
}
