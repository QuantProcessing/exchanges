package bybit

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/stretchr/testify/require"
)

var _ marketClient = (*bybitStubClient)(nil)

func TestNewPerpAdapterWithClientAllowsConstruction(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, adp)
	require.Equal(t, "BTCUSDT", adp.FormatSymbol("BTC"))
}

func TestNewPerpAdapterWithClientRejectsPartialCredentials(t *testing.T) {
	_, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{APIKey: "key"}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{})
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func TestNewSpotAdapterWithClientAllowsConstruction(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, adp)
	require.Equal(t, "BTCUSDT", adp.FormatSymbol("BTC"))
}

func TestNewSpotAdapterWithClientRejectsPartialCredentials(t *testing.T) {
	_, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{SecretKey: "secret"}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{})
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func testSpotInstrument() sdk.Instrument {
	return sdk.Instrument{
		Symbol:     "BTCUSDT",
		BaseCoin:   "BTC",
		QuoteCoin:  "USDT",
		Status:     instrumentStatusTrading,
		PriceScale: "2",
		PriceFilter: sdk.PriceFilter{
			TickSize: "0.01",
		},
		LotSizeFilter: sdk.LotSizeFilter{
			BasePrecision:    "0.0001",
			MinOrderQty:      "0.0001",
			MinNotionalValue: "5",
		},
	}
}

func testLinearInstrument() sdk.Instrument {
	return sdk.Instrument{
		Symbol:     "BTCUSDT",
		BaseCoin:   "BTC",
		QuoteCoin:  "USDT",
		Status:     instrumentStatusTrading,
		PriceScale: "2",
		PriceFilter: sdk.PriceFilter{
			TickSize: "0.1",
		},
		LotSizeFilter: sdk.LotSizeFilter{
			QtyStep:          "0.001",
			MinOrderQty:      "0.001",
			MinNotionalValue: "5",
		},
	}
}

type bybitStubClient struct {
	getInstrumentsFn    func(context.Context, string) ([]sdk.Instrument, error)
	getTickerFn         func(context.Context, string, string) (*sdk.Ticker, error)
	getOrderBookFn      func(context.Context, string, string, int) (*sdk.OrderBook, error)
	getRecentTradesFn   func(context.Context, string, string, int) ([]sdk.PublicTrade, error)
	getKlinesFn         func(context.Context, string, string, string, int64, int64, int) ([]sdk.Candle, error)
	getWalletBalanceFn  func(context.Context, string, string) (*sdk.WalletBalanceResult, error)
	getFeeRatesFn       func(context.Context, string, string) ([]sdk.FeeRateRecord, error)
	getPositionsFn      func(context.Context, string, string, string) ([]sdk.PositionRecord, error)
	setLeverageFn       func(context.Context, sdk.SetLeverageRequest) error
	placeOrderFn        func(context.Context, sdk.PlaceOrderRequest) (*sdk.OrderActionResponse, error)
	cancelOrderFn       func(context.Context, sdk.CancelOrderRequest) (*sdk.OrderActionResponse, error)
	cancelAllOrdersFn   func(context.Context, sdk.CancelAllOrdersRequest) error
	amendOrderFn        func(context.Context, sdk.AmendOrderRequest) (*sdk.OrderActionResponse, error)
	getOpenOrdersFn     func(context.Context, string, string) ([]sdk.OrderRecord, error)
	getOrderHistoryFn   func(context.Context, string, string) ([]sdk.OrderRecord, error)
	getRealtimeOrdersFn func(context.Context, string, string, string, string, string, int) ([]sdk.OrderRecord, error)
}

func (c *bybitStubClient) GetInstruments(ctx context.Context, category string) ([]sdk.Instrument, error) {
	if c.getInstrumentsFn == nil {
		panic("unexpected GetInstruments call")
	}
	return c.getInstrumentsFn(ctx, category)
}

func (c *bybitStubClient) HasCredentials() bool {
	return true
}

func (c *bybitStubClient) GetTicker(ctx context.Context, category, symbol string) (*sdk.Ticker, error) {
	if c.getTickerFn == nil {
		panic("unexpected GetTicker call")
	}
	return c.getTickerFn(ctx, category, symbol)
}

func (c *bybitStubClient) GetOrderBook(ctx context.Context, category, symbol string, limit int) (*sdk.OrderBook, error) {
	if c.getOrderBookFn == nil {
		panic("unexpected GetOrderBook call")
	}
	return c.getOrderBookFn(ctx, category, symbol, limit)
}

func (c *bybitStubClient) GetRecentTrades(ctx context.Context, category, symbol string, limit int) ([]sdk.PublicTrade, error) {
	if c.getRecentTradesFn == nil {
		panic("unexpected GetRecentTrades call")
	}
	return c.getRecentTradesFn(ctx, category, symbol, limit)
}

func (c *bybitStubClient) GetKlines(ctx context.Context, category, symbol, interval string, start, end int64, limit int) ([]sdk.Candle, error) {
	if c.getKlinesFn == nil {
		panic("unexpected GetKlines call")
	}
	return c.getKlinesFn(ctx, category, symbol, interval, start, end, limit)
}

func (c *bybitStubClient) GetWalletBalance(ctx context.Context, accountType, coin string) (*sdk.WalletBalanceResult, error) {
	if c.getWalletBalanceFn == nil {
		panic("unexpected GetWalletBalance call")
	}
	return c.getWalletBalanceFn(ctx, accountType, coin)
}

func (c *bybitStubClient) GetFeeRates(ctx context.Context, category, symbol string) ([]sdk.FeeRateRecord, error) {
	if c.getFeeRatesFn == nil {
		panic("unexpected GetFeeRates call")
	}
	return c.getFeeRatesFn(ctx, category, symbol)
}

func (c *bybitStubClient) GetPositions(ctx context.Context, category, symbol, settleCoin string) ([]sdk.PositionRecord, error) {
	if c.getPositionsFn == nil {
		panic("unexpected GetPositions call")
	}
	return c.getPositionsFn(ctx, category, symbol, settleCoin)
}

func (c *bybitStubClient) SetLeverage(ctx context.Context, req sdk.SetLeverageRequest) error {
	if c.setLeverageFn == nil {
		panic("unexpected SetLeverage call")
	}
	return c.setLeverageFn(ctx, req)
}

func (c *bybitStubClient) PlaceOrder(ctx context.Context, req sdk.PlaceOrderRequest) (*sdk.OrderActionResponse, error) {
	if c.placeOrderFn == nil {
		panic("unexpected PlaceOrder call")
	}
	return c.placeOrderFn(ctx, req)
}

func (c *bybitStubClient) CancelOrder(ctx context.Context, req sdk.CancelOrderRequest) (*sdk.OrderActionResponse, error) {
	if c.cancelOrderFn == nil {
		panic("unexpected CancelOrder call")
	}
	return c.cancelOrderFn(ctx, req)
}

func (c *bybitStubClient) CancelAllOrders(ctx context.Context, req sdk.CancelAllOrdersRequest) error {
	if c.cancelAllOrdersFn == nil {
		panic("unexpected CancelAllOrders call")
	}
	return c.cancelAllOrdersFn(ctx, req)
}

func (c *bybitStubClient) AmendOrder(ctx context.Context, req sdk.AmendOrderRequest) (*sdk.OrderActionResponse, error) {
	if c.amendOrderFn == nil {
		panic("unexpected AmendOrder call")
	}
	return c.amendOrderFn(ctx, req)
}

func (c *bybitStubClient) GetOpenOrders(ctx context.Context, category, symbol string) ([]sdk.OrderRecord, error) {
	if c.getOpenOrdersFn == nil {
		panic("unexpected GetOpenOrders call")
	}
	return c.getOpenOrdersFn(ctx, category, symbol)
}

func (c *bybitStubClient) GetOrderHistory(ctx context.Context, category, symbol string) ([]sdk.OrderRecord, error) {
	if c.getOrderHistoryFn == nil {
		panic("unexpected GetOrderHistory call")
	}
	return c.getOrderHistoryFn(ctx, category, symbol)
}

func (c *bybitStubClient) GetOrderHistoryFiltered(ctx context.Context, category, symbol, orderID, orderLinkID string) ([]sdk.OrderRecord, error) {
	if c.getOrderHistoryFn == nil {
		panic("unexpected GetOrderHistoryFiltered call")
	}
	return c.getOrderHistoryFn(ctx, category, symbol)
}

func (c *bybitStubClient) GetRealtimeOrders(ctx context.Context, category, symbol, settleCoin, orderID, orderLinkID string, openOnly int) ([]sdk.OrderRecord, error) {
	if c.getRealtimeOrdersFn != nil {
		return c.getRealtimeOrdersFn(ctx, category, symbol, settleCoin, orderID, orderLinkID, openOnly)
	}
	if c.getOpenOrdersFn == nil {
		panic("unexpected GetRealtimeOrders call")
	}
	return c.getOpenOrdersFn(ctx, category, symbol)
}
