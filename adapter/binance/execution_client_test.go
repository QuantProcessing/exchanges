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

func TestSpotExecutionSubmitOrderUsesRESTAndRejectsPerp(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintSpot, Base: model.BTC, Quote: model.USDT},
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	spotClient := &fakeSpotExecution{}
	client := newSpotExecutionClient("binance-spot-master", provider, spotClient, nil)

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

	err = client.SubmitOrder(context.Background(), model.SubmitOrder{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		Side:         model.OrderSideBuy,
		Type:         model.OrderTypeMarket,
		Quantity:     decimal.RequireFromString("0.1"),
	})
	require.ErrorIs(t, err, model.ErrNotSupported)
}

func TestPerpExecutionSubmitOrderUsesRESTAndRejectsSpot(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintSpot, Base: model.BTC, Quote: model.USDT},
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	perpClient := &fakePerpExecution{}
	client := newPerpExecutionClient("binance-usdt-futures-master", provider, perpClient, nil)

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

	err = client.SubmitOrder(context.Background(), model.SubmitOrder{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		Side:         model.OrderSideBuy,
		Type:         model.OrderTypeMarket,
		Quantity:     decimal.RequireFromString("0.1"),
	})
	require.ErrorIs(t, err, model.ErrNotSupported)
}

func TestExecutionConnectStartsPrivateStream(t *testing.T) {
	stream := &fakePrivateExecutionStream{}
	client := newExecutionClient("acct", nil, stream)

	require.NoError(t, client.Connect(context.Background()))
	require.True(t, stream.connected)
	require.True(t, client.Health().Connected)

	require.NoError(t, client.Disconnect(context.Background()))
	require.True(t, stream.disconnected)
	require.False(t, client.Health().Connected)
}

func TestExecutionSubmitOrderRejectsUnknownInstrument(t *testing.T) {
	client := newPerpExecutionClient("acct", newInstrumentProviderForTest(nil), nil, nil)
	err := client.SubmitOrder(context.Background(), model.SubmitOrder{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		Side:         model.OrderSideBuy,
		Type:         model.OrderTypeMarket,
		Quantity:     decimal.RequireFromString("0.1"),
	})
	require.ErrorIs(t, err, model.ErrInstrumentNotLoaded)
}

func TestSpotExecutionGenerateOrderStatusReportsUsesAllOrders(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintSpot, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	spotClient := &fakeSpotExecution{
		allOrders: []spot.OrderResponse{{
			Symbol:        "BTCUSDT",
			OrderID:       123,
			ClientOrderID: "client-1",
			Status:        "FILLED",
			Type:          "LIMIT",
			Side:          "BUY",
			OrigQty:       "1",
			ExecutedQty:   "1",
		}},
	}
	client := newSpotExecutionClient("acct", provider, spotClient, nil)

	reports, err := client.GenerateOrderStatusReports(context.Background(), venue.OrderStatusQuery{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	})
	require.NoError(t, err)
	require.True(t, spotClient.allOrdersCalled)
	require.False(t, spotClient.openOrdersCalled)
	require.Len(t, reports, 1)
	require.Equal(t, model.OrderStatusFilled, reports[0].Status)
}

func TestPerpExecutionGenerateOrderStatusReportsUsesAllOrders(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	perpClient := &fakePerpExecution{
		allOrders: []perp.OrderResponse{{
			Symbol:        "BTCUSDT",
			OrderID:       456,
			ClientOrderID: "client-2",
			Status:        "CANCELED",
			Type:          "LIMIT",
			Side:          "SELL",
			OrigQty:       "1",
			ExecutedQty:   "0",
		}},
	}
	client := newPerpExecutionClient("acct", provider, perpClient, nil)

	reports, err := client.GenerateOrderStatusReports(context.Background(), venue.OrderStatusQuery{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
	})
	require.NoError(t, err)
	require.True(t, perpClient.allOrdersCalled)
	require.False(t, perpClient.openOrdersCalled)
	require.Len(t, reports, 1)
	require.Equal(t, model.OrderStatusCanceled, reports[0].Status)
}

func TestPerpExecutionGeneratePositionStatusReportsUsesPositionRisk(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	perpClient := &fakePerpExecution{
		positions: []perp.PositionRiskResponse{{
			Symbol:           "BTCUSDT",
			PositionAmt:      "-0.5",
			EntryPrice:       "64000",
			UnRealizedProfit: "12.5",
			PositionSide:     "SHORT",
			UpdateTime:       1234,
		}},
	}
	client := newPerpExecutionClient("acct", provider, perpClient, nil)

	reports, err := client.GeneratePositionStatusReports(context.Background(), venue.PositionQuery{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
	})
	require.NoError(t, err)
	require.True(t, perpClient.positionRiskCalled)
	require.Len(t, reports, 1)
	require.Equal(t, model.PositionSideShort, reports[0].Side)
	require.Equal(t, "0.5", reports[0].Quantity.String())
	require.Equal(t, model.USDT, reports[0].Unrealized.Currency)
}

type fakePrivateExecutionStream struct {
	connected    bool
	disconnected bool
}

func (f *fakePrivateExecutionStream) Connect(context.Context) error {
	f.connected = true
	return nil
}

func (f *fakePrivateExecutionStream) Disconnect(context.Context) error {
	f.disconnected = true
	return nil
}

type fakeSpotExecution struct {
	place            spot.PlaceOrderParams
	allOrders        []spot.OrderResponse
	allOrdersCalled  bool
	openOrdersCalled bool
}

func (f *fakeSpotExecution) PlaceOrder(ctx context.Context, p spot.PlaceOrderParams) (*spot.OrderResponse, error) {
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

func (f *fakeSpotExecution) CancelOrder(context.Context, string, int64, string) (*spot.CancelOrderResponse, error) {
	return nil, nil
}

func (f *fakeSpotExecution) GetOpenOrders(context.Context, string) ([]spot.OrderResponse, error) {
	f.openOrdersCalled = true
	return nil, nil
}

func (f *fakeSpotExecution) AllOrders(context.Context, string, int, int64, int64, int64) ([]spot.OrderResponse, error) {
	f.allOrdersCalled = true
	return f.allOrders, nil
}

func (f *fakeSpotExecution) MyTrades(context.Context, string, int, int64, int64, int64) ([]spot.Trade, error) {
	return nil, nil
}

func (f *fakeSpotExecution) GetAccount(context.Context) (*spot.AccountResponse, error) {
	return &spot.AccountResponse{}, nil
}

type fakePerpExecution struct {
	place              perp.PlaceOrderParams
	allOrders          []perp.OrderResponse
	positions          []perp.PositionRiskResponse
	allOrdersCalled    bool
	openOrdersCalled   bool
	positionRiskCalled bool
}

func (f *fakePerpExecution) PlaceOrder(ctx context.Context, p perp.PlaceOrderParams) (*perp.OrderResponse, error) {
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

func (f *fakePerpExecution) CancelOrder(context.Context, perp.CancelOrderParams) (*perp.OrderResponse, error) {
	return nil, nil
}

func (f *fakePerpExecution) CancelAllOpenOrders(context.Context, perp.CancelAllOrdersParams) error {
	return nil
}

func (f *fakePerpExecution) GetOpenOrders(context.Context, string) ([]perp.OrderResponse, error) {
	f.openOrdersCalled = true
	return nil, nil
}

func (f *fakePerpExecution) AllOrders(context.Context, string, int, int64, int64, int64) ([]perp.OrderResponse, error) {
	f.allOrdersCalled = true
	return f.allOrders, nil
}

func (f *fakePerpExecution) MyTrades(context.Context, string, int, int64, int64, int64) ([]perp.Trade, error) {
	return nil, nil
}

func (f *fakePerpExecution) GetPositionRisk(context.Context, string) ([]perp.PositionRiskResponse, error) {
	f.positionRiskCalled = true
	return f.positions, nil
}

func (f *fakePerpExecution) GetAccount(context.Context) (*perp.AccountResponse, error) {
	return &perp.AccountResponse{}, nil
}
