package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestV2ExecutionSubmitOrderRoutesSpot(t *testing.T) {
	provider := newV2InstrumentProviderForTest([]v2InstrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintSpot, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	spotClient := &fakeV2SpotExecution{}
	client := newV2ExecutionClient("acct", provider, spotClient, nil)

	err := client.SubmitOrder(context.Background(), model.SubmitOrder{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		Side:         model.OrderSideBuy,
		Type:         model.OrderTypeMarket,
		Quantity:     decimal.RequireFromString("0.1"),
		ClientID:     "client-1",
	})
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", spotClient.place.Symbol)
	require.Equal(t, "BUY", spotClient.place.Side)
	require.Equal(t, "MARKET", spotClient.place.Type)
	require.Equal(t, "0.1", spotClient.place.Quantity)
	require.Equal(t, "client-1", spotClient.place.NewClientOrderID)
}

func TestV2ExecutionSubmitOrderRoutesPerp(t *testing.T) {
	provider := newV2InstrumentProviderForTest([]v2InstrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	perpClient := &fakeV2PerpExecution{}
	client := newV2ExecutionClient("acct", provider, nil, perpClient)

	err := client.SubmitOrder(context.Background(), model.SubmitOrder{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		Side:         model.OrderSideSell,
		Type:         model.OrderTypeLimit,
		Quantity:     decimal.RequireFromString("0.2"),
		Price:        decimal.RequireFromString("100"),
		ClientID:     "client-2",
		ReduceOnly:   true,
	})
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", perpClient.place.Symbol)
	require.Equal(t, "SELL", perpClient.place.Side)
	require.Equal(t, "LIMIT", perpClient.place.Type)
	require.Equal(t, "0.2", perpClient.place.Quantity)
	require.Equal(t, "100", perpClient.place.Price)
	require.Equal(t, "client-2", perpClient.place.NewClientOrderID)
	require.True(t, perpClient.place.ReduceOnly)
}

func TestV2ExecutionSubmitOrderRejectsUnknownInstrument(t *testing.T) {
	client := newV2ExecutionClient("acct", newV2InstrumentProviderForTest(nil), nil, nil)
	err := client.SubmitOrder(context.Background(), model.SubmitOrder{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		Side:         model.OrderSideBuy,
		Type:         model.OrderTypeMarket,
		Quantity:     decimal.RequireFromString("0.1"),
	})
	require.ErrorIs(t, err, model.ErrInstrumentNotLoaded)
}

type fakeV2SpotExecution struct {
	place spot.PlaceOrderParams
}

func (f *fakeV2SpotExecution) PlaceOrder(ctx context.Context, p spot.PlaceOrderParams) (*spot.OrderResponse, error) {
	f.place = p
	return &spot.OrderResponse{
		Symbol:        p.Symbol,
		OrderID:       123,
		ClientOrderID: p.NewClientOrderID,
		Status:        "NEW",
		Type:          p.Type,
		Side:          p.Side,
		OrigQty:       p.Quantity,
		Price:         p.Price,
	}, nil
}

func (f *fakeV2SpotExecution) CancelOrder(context.Context, string, int64, string) (*spot.CancelOrderResponse, error) {
	return nil, nil
}

func (f *fakeV2SpotExecution) GetOpenOrders(context.Context, string) ([]spot.OrderResponse, error) {
	return nil, nil
}

func (f *fakeV2SpotExecution) MyTrades(context.Context, string, int, int64, int64, int64) ([]spot.Trade, error) {
	return nil, nil
}

func (f *fakeV2SpotExecution) GetAccount(context.Context) (*spot.AccountResponse, error) {
	return &spot.AccountResponse{}, nil
}

type fakeV2PerpExecution struct {
	place perp.PlaceOrderParams
}

func (f *fakeV2PerpExecution) PlaceOrder(ctx context.Context, p perp.PlaceOrderParams) (*perp.OrderResponse, error) {
	f.place = p
	return &perp.OrderResponse{
		Symbol:        p.Symbol,
		OrderID:       456,
		ClientOrderID: p.NewClientOrderID,
		Status:        "NEW",
		Type:          p.Type,
		Side:          p.Side,
		OrigQty:       p.Quantity,
		Price:         p.Price,
		ReduceOnly:    p.ReduceOnly,
	}, nil
}

func (f *fakeV2PerpExecution) CancelOrder(context.Context, perp.CancelOrderParams) (*perp.OrderResponse, error) {
	return nil, nil
}

func (f *fakeV2PerpExecution) CancelAllOpenOrders(context.Context, perp.CancelAllOrdersParams) error {
	return nil
}

func (f *fakeV2PerpExecution) GetOpenOrders(context.Context, string) ([]perp.OrderResponse, error) {
	return nil, nil
}

func (f *fakeV2PerpExecution) MyTrades(context.Context, string, int, int64, int64, int64) ([]perp.Trade, error) {
	return nil, nil
}

func (f *fakeV2PerpExecution) GetAccount(context.Context) (*perp.AccountResponse, error) {
	return &perp.AccountResponse{}, nil
}
