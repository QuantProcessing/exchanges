package backpack

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/stretchr/testify/require"
)

var _ adapterRESTClient = (*backpackStubClient)(nil)

func TestNewPerpAdapterWithClientAllowsConstruction(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDC, &backpackStubClient{
		marketsFn: func(context.Context) ([]sdk.Market, error) {
			return []sdk.Market{testBackpackPerpMarket()}, nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, adp)
}

func TestNewAdapterWithClientAllowsPublicOnlyConstruction(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDC, &backpackStubClient{
		marketsFn: func(context.Context) ([]sdk.Market, error) {
			return []sdk.Market{testBackpackPerpMarket()}, nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, adp)
}

func TestNewAdapterWithClientRejectsPartialCredentials(t *testing.T) {
	_, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{APIKey: "key"}, exchanges.QuoteCurrencyUSDC, &backpackStubClient{})
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func TestNewSpotAdapterWithClientAllowsConstruction(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDC, &backpackStubClient{
		marketsFn: func(context.Context) ([]sdk.Market, error) {
			return []sdk.Market{testBackpackSpotMarket()}, nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, adp)
}

func TestNewSpotAdapterWithClientRejectsPartialCredentials(t *testing.T) {
	_, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{PrivateKey: "private"}, exchanges.QuoteCurrencyUSDC, &backpackStubClient{})
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func testBackpackSpotMarket() sdk.Market {
	return sdk.Market{
		Symbol:      "BTC_USDC",
		BaseSymbol:  "BTC",
		QuoteSymbol: "USDC",
		MarketType:  "SPOT",
		Filters: sdk.MarketFilters{
			Price: sdk.PriceFilter{
				MinPrice: "1",
				TickSize: "0.1",
			},
			Quantity: sdk.QuantityFilter{
				MinQuantity: "0.0001",
				StepSize:    "0.0001",
			},
		},
		Visible: true,
	}
}

func testBackpackPerpMarket() sdk.Market {
	return sdk.Market{
		Symbol:      "BTC_USDC_PERP",
		BaseSymbol:  "BTC",
		QuoteSymbol: "USDC",
		MarketType:  "PERP",
		Filters: sdk.MarketFilters{
			Price: sdk.PriceFilter{
				MinPrice: "1",
				TickSize: "0.1",
			},
			Quantity: sdk.QuantityFilter{
				MinQuantity: "0.001",
				StepSize:    "0.001",
			},
		},
		Visible: true,
	}
}

type backpackStubClient struct {
	marketsFn          func(context.Context) ([]sdk.Market, error)
	tickerFn           func(context.Context, string) (*sdk.Ticker, error)
	orderBookFn        func(context.Context, string, int) (*sdk.Depth, error)
	tradesFn           func(context.Context, string, int) ([]sdk.Trade, error)
	fundingRatesFn     func(context.Context) ([]sdk.FundingRate, error)
	klinesFn           func(context.Context, string, string, int64, int64, string) ([]sdk.Kline, error)
	accountFn          func(context.Context) (*sdk.AccountSettings, error)
	balancesFn         func(context.Context) (map[string]sdk.CapitalBalance, error)
	openOrdersFn       func(context.Context, string, string) ([]sdk.Order, error)
	openPositionsFn    func(context.Context, string) ([]sdk.Position, error)
	placeOrderFn       func(context.Context, sdk.CreateOrderRequest) (*sdk.Order, error)
	cancelOrderFn      func(context.Context, sdk.CancelOrderRequest) (*sdk.Order, error)
	cancelOpenOrdersFn func(context.Context, string, string) error

	// Legacy fallback hooks keep older package tests compiling while the
	// preferred names above remain the primary constructor-test surface.
	getMarkets       func(context.Context) ([]sdk.Market, error)
	getTicker        func(context.Context, string) (*sdk.Ticker, error)
	getOrderBook     func(context.Context, string, int) (*sdk.Depth, error)
	getTrades        func(context.Context, string, int) ([]sdk.Trade, error)
	getFundingRates  func(context.Context) ([]sdk.FundingRate, error)
	getKlines        func(context.Context, string, string, int64, int64, string) ([]sdk.Kline, error)
	getAccount       func(context.Context) (*sdk.AccountSettings, error)
	getBalances      func(context.Context) (map[string]sdk.CapitalBalance, error)
	getOpenOrders    func(context.Context, string, string) ([]sdk.Order, error)
	getOpenPositions func(context.Context, string) ([]sdk.Position, error)
	placeOrder       func(context.Context, sdk.CreateOrderRequest) (*sdk.Order, error)
	cancelOrder      func(context.Context, sdk.CancelOrderRequest) (*sdk.Order, error)
	cancelOpenOrders func(context.Context, string, string) error
}

func (c *backpackStubClient) GetMarkets(ctx context.Context) ([]sdk.Market, error) {
	if c.marketsFn != nil {
		return c.marketsFn(ctx)
	}
	if c.getMarkets == nil {
		panic("unexpected GetMarkets call")
	}
	return c.getMarkets(ctx)
}

func (c *backpackStubClient) GetTicker(ctx context.Context, symbol string) (*sdk.Ticker, error) {
	if c.tickerFn != nil {
		return c.tickerFn(ctx, symbol)
	}
	if c.getTicker == nil {
		panic("unexpected GetTicker call")
	}
	return c.getTicker(ctx, symbol)
}

func (c *backpackStubClient) GetOrderBook(ctx context.Context, symbol string, limit int) (*sdk.Depth, error) {
	if c.orderBookFn != nil {
		return c.orderBookFn(ctx, symbol, limit)
	}
	if c.getOrderBook == nil {
		panic("unexpected GetOrderBook call")
	}
	return c.getOrderBook(ctx, symbol, limit)
}

func (c *backpackStubClient) GetTrades(ctx context.Context, symbol string, limit int) ([]sdk.Trade, error) {
	if c.tradesFn != nil {
		return c.tradesFn(ctx, symbol, limit)
	}
	if c.getTrades == nil {
		panic("unexpected GetTrades call")
	}
	return c.getTrades(ctx, symbol, limit)
}

func (c *backpackStubClient) GetFundingRates(ctx context.Context) ([]sdk.FundingRate, error) {
	if c.fundingRatesFn != nil {
		return c.fundingRatesFn(ctx)
	}
	if c.getFundingRates == nil {
		panic("unexpected GetFundingRates call")
	}
	return c.getFundingRates(ctx)
}

func (c *backpackStubClient) GetKlines(ctx context.Context, symbol, interval string, startTime, endTime int64, priceType string) ([]sdk.Kline, error) {
	if c.klinesFn != nil {
		return c.klinesFn(ctx, symbol, interval, startTime, endTime, priceType)
	}
	if c.getKlines == nil {
		panic("unexpected GetKlines call")
	}
	return c.getKlines(ctx, symbol, interval, startTime, endTime, priceType)
}

func (c *backpackStubClient) GetAccount(ctx context.Context) (*sdk.AccountSettings, error) {
	if c.accountFn != nil {
		return c.accountFn(ctx)
	}
	if c.getAccount == nil {
		panic("unexpected GetAccount call")
	}
	return c.getAccount(ctx)
}

func (c *backpackStubClient) GetBalances(ctx context.Context) (map[string]sdk.CapitalBalance, error) {
	if c.balancesFn != nil {
		return c.balancesFn(ctx)
	}
	if c.getBalances == nil {
		panic("unexpected GetBalances call")
	}
	return c.getBalances(ctx)
}

func (c *backpackStubClient) GetOpenOrders(ctx context.Context, marketType, symbol string) ([]sdk.Order, error) {
	if c.openOrdersFn != nil {
		return c.openOrdersFn(ctx, marketType, symbol)
	}
	if c.getOpenOrders == nil {
		panic("unexpected GetOpenOrders call")
	}
	return c.getOpenOrders(ctx, marketType, symbol)
}

func (c *backpackStubClient) GetOpenPositions(ctx context.Context, symbol string) ([]sdk.Position, error) {
	if c.openPositionsFn != nil {
		return c.openPositionsFn(ctx, symbol)
	}
	if c.getOpenPositions == nil {
		panic("unexpected GetOpenPositions call")
	}
	return c.getOpenPositions(ctx, symbol)
}

func (c *backpackStubClient) PlaceOrder(ctx context.Context, req sdk.CreateOrderRequest) (*sdk.Order, error) {
	if c.placeOrderFn != nil {
		return c.placeOrderFn(ctx, req)
	}
	if c.placeOrder == nil {
		panic("unexpected PlaceOrder call")
	}
	return c.placeOrder(ctx, req)
}

func (c *backpackStubClient) CancelOrder(ctx context.Context, req sdk.CancelOrderRequest) (*sdk.Order, error) {
	if c.cancelOrderFn != nil {
		return c.cancelOrderFn(ctx, req)
	}
	if c.cancelOrder == nil {
		panic("unexpected CancelOrder call")
	}
	return c.cancelOrder(ctx, req)
}

func (c *backpackStubClient) CancelOpenOrders(ctx context.Context, symbol, marketType string) error {
	if c.cancelOpenOrdersFn != nil {
		return c.cancelOpenOrdersFn(ctx, symbol, marketType)
	}
	if c.cancelOpenOrders == nil {
		panic("unexpected CancelOpenOrders call")
	}
	return c.cancelOpenOrders(ctx, symbol, marketType)
}
