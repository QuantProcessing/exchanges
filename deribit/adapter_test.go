package deribit

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/deribit/sdk"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

var _ exchanges.PerpExchange = (*Adapter)(nil)
var _ exchanges.OptionExchange = (*OptionAdapter)(nil)

func TestDeribitPerpAdapterMapsPublicMarketData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := &deribitStubClient{
		getInstrumentsFn: func(_ context.Context, currency, kind string, expired bool) ([]sdk.Instrument, error) {
			require.Equal(t, "any", currency)
			require.Equal(t, kindFuture, kind)
			require.False(t, expired)
			return []sdk.Instrument{testPerpInstrument()}, nil
		},
		getTickerFn: func(_ context.Context, instrumentName string) (*sdk.Ticker, error) {
			require.Equal(t, "BTC-PERPETUAL", instrumentName)
			return &sdk.Ticker{
				InstrumentName: "BTC-PERPETUAL",
				LastPrice:      79632,
				BestBidPrice:   79631.5,
				BestAskPrice:   79632,
				IndexPrice:     79613.18,
				MarkPrice:      79628.55,
				CurrentFunding: 0.000001,
				Funding8h:      0.0000226,
				Timestamp:      1778726187516,
				Stats: sdk.Stats{
					High:      81325.5,
					Low:       78754,
					Volume:    4760.80052028,
					VolumeUSD: 380489130,
				},
			}, nil
		},
		getOrderBookFn: func(_ context.Context, instrumentName string, depth int) (*sdk.OrderBook, error) {
			require.Equal(t, "BTC-PERPETUAL", instrumentName)
			require.Equal(t, 5, depth)
			return &sdk.OrderBook{
				InstrumentName: "BTC-PERPETUAL",
				Timestamp:      1778726186839,
				Bids:           [][]float64{{79631.5, 35040}},
				Asks:           [][]float64{{79632, 485860}},
			}, nil
		},
		getTradesFn: func(_ context.Context, instrumentName string, count int) (*sdk.TradesResult, error) {
			require.Equal(t, "BTC-PERPETUAL", instrumentName)
			require.Equal(t, 2, count)
			return &sdk.TradesResult{Trades: []sdk.Trade{
				{TradeID: "415305279", InstrumentName: "BTC-PERPETUAL", Direction: "buy", Price: 79632, Amount: 10, Timestamp: 1778726187000},
			}}, nil
		},
		getChartFn: func(_ context.Context, instrumentName string, start, end int64, resolution string) (*sdk.TradingViewChartData, error) {
			require.Equal(t, "BTC-PERPETUAL", instrumentName)
			require.Equal(t, int64(1778722800000), start)
			require.Equal(t, int64(1778726400000), end)
			require.Equal(t, "60", resolution)
			return &sdk.TradingViewChartData{
				Status: "ok",
				Ticks:  []int64{1778722800000},
				Open:   []float64{79000},
				High:   []float64{80000},
				Low:    []float64{78900},
				Close:  []float64{79632},
				Volume: []float64{12.5},
				Cost:   []float64{995400},
			}, nil
		},
	}

	adp, err := newPerpAdapterWithClient(ctx, Options{}, client)
	require.NoError(t, err)

	require.Equal(t, "BTC-PERPETUAL", adp.FormatSymbol("BTC"))
	require.Equal(t, "BTC", adp.ExtractSymbol("BTC-PERPETUAL"))

	ticker, err := adp.FetchTicker(ctx, "BTC")
	require.NoError(t, err)
	require.Equal(t, "BTC", ticker.Symbol)
	require.Equal(t, "79632", ticker.LastPrice.String())
	require.Equal(t, "79631.75", ticker.MidPrice.String())
	require.Equal(t, "380489130", ticker.QuoteVol.String())

	book, err := adp.FetchOrderBook(ctx, "BTC", 5)
	require.NoError(t, err)
	require.Equal(t, "BTC", book.Symbol)
	require.Equal(t, "79631.5", book.Bids[0].Price.String())
	require.Equal(t, "485860", book.Asks[0].Quantity.String())

	trades, err := adp.FetchTrades(ctx, "BTC", 2)
	require.NoError(t, err)
	require.Len(t, trades, 1)
	require.Equal(t, exchanges.TradeSideBuy, trades[0].Side)

	start := time.UnixMilli(1778722800000)
	end := time.UnixMilli(1778726400000)
	klines, err := adp.FetchKlines(ctx, "BTC", exchanges.Interval1h, &exchanges.KlineOpts{Start: &start, End: &end, Limit: 1})
	require.NoError(t, err)
	require.Len(t, klines, 1)
	require.Equal(t, "995400", klines[0].QuoteVol.String())

	funding, err := adp.FetchFundingRate(ctx, "BTC")
	require.NoError(t, err)
	require.Equal(t, "BTC", funding.Symbol)
	require.Equal(t, "0.0000226", funding.FundingRate.String())
	require.Equal(t, int64(8), funding.FundingIntervalHours)

	requireUnsupportedExchangeMethods(t, ctx, adp)
	require.ErrorIs(t, adp.SetLeverage(ctx, "BTC", 5), exchanges.ErrNotSupported)
	_, err = adp.FetchPositions(ctx)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}

func TestDeribitOptionAdapterListsContractsAndMapsSymbols(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := &deribitStubClient{
		getInstrumentsFn: func(_ context.Context, currency, kind string, expired bool) ([]sdk.Instrument, error) {
			require.Equal(t, kindOption, kind)
			require.False(t, expired)
			switch currency {
			case "any":
				return []sdk.Instrument{testOptionInstrument()}, nil
			default:
				return nil, nil
			}
		},
		getTickerFn: func(_ context.Context, instrumentName string) (*sdk.Ticker, error) {
			require.Equal(t, "BTC-14MAY26-72000-C", instrumentName)
			return &sdk.Ticker{
				InstrumentName: "BTC-14MAY26-72000-C",
				LastPrice:      0.0525,
				BestBidPrice:   0.052,
				BestAskPrice:   0.053,
				IndexPrice:     66930.31,
				MarkPrice:      0.05253883,
				Timestamp:      1770984454552,
				Stats: sdk.Stats{
					High:   0.06,
					Low:    0.04,
					Volume: 3,
				},
			}, nil
		},
		getOrderBookFn: func(_ context.Context, instrumentName string, depth int) (*sdk.OrderBook, error) {
			require.Equal(t, "BTC-14MAY26-72000-C", instrumentName)
			require.Equal(t, 1, depth)
			return &sdk.OrderBook{
				InstrumentName: "BTC-14MAY26-72000-C",
				Timestamp:      1770984454552,
				Bids:           [][]float64{{0.052, 3}},
				Asks:           [][]float64{{0.053, 5}},
			}, nil
		},
		getTradesFn: func(_ context.Context, instrumentName string, count int) (*sdk.TradesResult, error) {
			require.Equal(t, "BTC-14MAY26-72000-C", instrumentName)
			require.Equal(t, 1, count)
			return &sdk.TradesResult{Trades: []sdk.Trade{
				{TradeID: "option-trade-1", InstrumentName: "BTC-14MAY26-72000-C", Direction: "sell", Price: 0.0525, Amount: 3, Timestamp: 1770984454552},
			}}, nil
		},
		getChartFn: func(_ context.Context, instrumentName string, start, end int64, resolution string) (*sdk.TradingViewChartData, error) {
			require.Equal(t, "BTC-14MAY26-72000-C", instrumentName)
			require.Equal(t, "1D", resolution)
			return &sdk.TradingViewChartData{
				Status: "ok",
				Ticks:  []int64{start},
				Open:   []float64{0.05},
				High:   []float64{0.06},
				Low:    []float64{0.04},
				Close:  []float64{0.0525},
				Volume: []float64{3},
				Cost:   []float64{0.1575},
			}, nil
		},
	}

	adp, err := newOptionAdapterWithClient(ctx, Options{}, client)
	require.NoError(t, err)

	contracts, err := adp.ListOptionContracts(ctx, "BTC")
	require.NoError(t, err)
	require.Len(t, contracts, 1)

	contract := contracts[0]
	require.Equal(t, "BTC-BTC-BTC-20260514-72000-C", contract.Symbol)
	require.Equal(t, "BTC-14MAY26-72000-C", contract.ExchangeSymbol)
	require.Equal(t, exchanges.OptionTypeCall, contract.Type)
	require.Equal(t, "72000", contract.StrikePrice.String())
	require.Equal(t, int64(1778745600000), contract.ExpiryTime)
	require.Equal(t, "1", contract.ContractSize.String())
	require.Equal(t, "0.0001", contract.TickSize.String())
	require.Equal(t, "0.1", contract.MinQuantity.String())

	require.Equal(t, "BTC-14MAY26-72000-C", adp.FormatSymbol("BTC-BTC-BTC-20260514-72000-C"))
	require.Equal(t, "BTC-BTC-BTC-20260514-72000-C", adp.ExtractSymbol("BTC-14MAY26-72000-C"))

	fetched, err := adp.FetchOptionContract(ctx, "BTC-14MAY26-72000-C")
	require.NoError(t, err)
	require.Equal(t, contract.Symbol, fetched.Symbol)

	ticker, err := adp.FetchTicker(ctx, "BTC-BTC-BTC-20260514-72000-C")
	require.NoError(t, err)
	require.Equal(t, "BTC-BTC-BTC-20260514-72000-C", ticker.Symbol)
	require.Equal(t, "0.0525", ticker.MidPrice.String())

	book, err := adp.FetchOrderBook(ctx, "BTC-BTC-BTC-20260514-72000-C", 1)
	require.NoError(t, err)
	require.Equal(t, "BTC-BTC-BTC-20260514-72000-C", book.Symbol)
	require.Equal(t, "0.052", book.Bids[0].Price.String())

	trades, err := adp.FetchTrades(ctx, "BTC-BTC-BTC-20260514-72000-C", 1)
	require.NoError(t, err)
	require.Len(t, trades, 1)
	require.Equal(t, exchanges.TradeSideSell, trades[0].Side)

	start := time.UnixMilli(1770940800000)
	end := time.UnixMilli(1771027200000)
	klines, err := adp.FetchKlines(ctx, "BTC-BTC-BTC-20260514-72000-C", exchanges.Interval1d, &exchanges.KlineOpts{Start: &start, End: &end, Limit: 1})
	require.NoError(t, err)
	require.Len(t, klines, 1)
	require.Equal(t, "0.1575", klines[0].QuoteVol.String())

	details, err := adp.FetchSymbolDetails(ctx, "BTC-14MAY26-72000-C")
	require.NoError(t, err)
	require.Equal(t, "BTC-BTC-BTC-20260514-72000-C", details.Symbol)
	require.Equal(t, int32(4), details.PricePrecision)

	testsuite.RunOptionPublicDataSuite(t, adp, testsuite.OptionPublicDataConfig{
		Underlying: "BTC",
		Symbol:     contract.Symbol,
		Native:     contract.ExchangeSymbol,
		Interval:   exchanges.Interval1d,
		KlineOpts:  &exchanges.KlineOpts{Start: &start, End: &end, Limit: 1},
		BookDepth:  1,
	})
}

func TestDeribitOptionAdapterLoadsSpecificUnderlyingOnDemand(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var requested []string
	client := &deribitStubClient{
		getInstrumentsFn: func(_ context.Context, currency, kind string, expired bool) ([]sdk.Instrument, error) {
			require.Equal(t, kindOption, kind)
			require.False(t, expired)
			requested = append(requested, currency)
			switch currency {
			case "any":
				return nil, nil
			case "SOL":
				inst := testOptionInstrument()
				inst.InstrumentName = "SOL-14MAY26-200-C"
				inst.BaseCurrency = "SOL"
				inst.QuoteCurrency = "SOL"
				inst.SettlementCurrency = "SOL"
				inst.Strike = 200
				return []sdk.Instrument{inst}, nil
			default:
				return nil, nil
			}
		},
	}

	adp, err := newOptionAdapterWithClient(ctx, Options{}, client)
	require.NoError(t, err)

	contracts, err := adp.ListOptionContracts(ctx, "SOL")
	require.NoError(t, err)
	require.Len(t, contracts, 1)
	require.Equal(t, "SOL-SOL-SOL-20260514-200-C", contracts[0].Symbol)
	require.Equal(t, []string{"any", "SOL"}, requested)
}

func TestDeribitOptionAdapterRESTTradingUsesPrivateTradingMethods(t *testing.T) {
	ctx := context.Background()
	contract := "BTC-BTC-BTC-20260514-72000-C"
	native := "BTC-14MAY26-72000-C"

	client := &deribitStubClient{
		getInstrumentsFn: func(_ context.Context, currency, kind string, expired bool) ([]sdk.Instrument, error) {
			require.Equal(t, "any", currency)
			require.Equal(t, kindOption, kind)
			require.False(t, expired)
			return []sdk.Instrument{testOptionInstrument()}, nil
		},
		buyFn: func(_ context.Context, req sdk.OrderRequest) (*sdk.OrderResult, error) {
			require.Equal(t, native, req.InstrumentName)
			require.Equal(t, "0.1", req.Amount)
			require.Equal(t, "limit", req.Type)
			require.Equal(t, "0.0525", req.Price)
			require.Equal(t, "good_til_cancelled", req.TimeInForce)
			require.Equal(t, "option-client-1", req.Label)
			return &sdk.OrderResult{Order: sdk.OrderRecord{
				OrderID:        "BTC-123",
				InstrumentName: native,
				Direction:      "buy",
				OrderType:      "limit",
				OrderState:     "open",
				Amount:         0.1,
				Price:          0.0525,
				TimeInForce:    "good_til_cancelled",
				Label:          "option-client-1",
				CreationTime:   1778745600000,
			}}, nil
		},
		cancelOrderFn: func(_ context.Context, orderID string) (*sdk.OrderRecord, error) {
			require.Equal(t, "BTC-123", orderID)
			return &sdk.OrderRecord{OrderID: "BTC-123", InstrumentName: native, OrderState: "cancelled"}, nil
		},
		cancelAllByInstrumentFn: func(_ context.Context, instrumentName string) (int64, error) {
			require.Equal(t, native, instrumentName)
			return 1, nil
		},
		getOrderStateFn: func(_ context.Context, orderID string) (*sdk.OrderRecord, error) {
			require.Equal(t, "BTC-123", orderID)
			return &sdk.OrderRecord{
				OrderID:        "BTC-123",
				InstrumentName: native,
				Direction:      "buy",
				OrderType:      "limit",
				OrderState:     "open",
				Amount:         0.1,
				FilledAmount:   0.02,
				Price:          0.0525,
				AveragePrice:   0.0524,
				Label:          "option-client-1",
				TimeInForce:    "good_til_cancelled",
				UpdateTime:     1778745600001,
			}, nil
		},
		getOpenOrdersByInstrumentFn: func(_ context.Context, instrumentName string) ([]sdk.OrderRecord, error) {
			require.Equal(t, native, instrumentName)
			return []sdk.OrderRecord{{
				OrderID:        "BTC-123",
				InstrumentName: native,
				Direction:      "buy",
				OrderType:      "limit",
				OrderState:     "open",
				Amount:         0.1,
				FilledAmount:   0.02,
				Price:          0.0525,
				AveragePrice:   0.0524,
				Label:          "option-client-1",
				TimeInForce:    "good_til_cancelled",
				UpdateTime:     1778745600001,
			}}, nil
		},
		getOrderHistoryByInstrumentFn: func(_ context.Context, instrumentName string, count int) ([]sdk.OrderRecord, error) {
			require.Equal(t, native, instrumentName)
			require.Equal(t, 100, count)
			return []sdk.OrderRecord{{
				OrderID:        "BTC-124",
				InstrumentName: native,
				Direction:      "sell",
				OrderType:      "limit",
				OrderState:     "filled",
				Amount:         0.1,
				FilledAmount:   0.1,
				Price:          0.06,
				AveragePrice:   0.06,
				Label:          "option-client-2",
				TimeInForce:    "good_til_cancelled",
				UpdateTime:     1778745600002,
			}}, nil
		},
	}

	adp, err := newOptionAdapterWithClient(ctx, Options{}, client)
	require.NoError(t, err)

	order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:      contract,
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.1"),
		Price:       decimal.RequireFromString("0.0525"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "option-client-1",
	})
	require.NoError(t, err)
	require.Equal(t, "BTC-123", order.OrderID)
	require.Equal(t, contract, order.Symbol)

	fetched, err := adp.FetchOrderByID(ctx, "BTC-123", contract)
	require.NoError(t, err)
	require.Equal(t, exchanges.OrderStatusPartiallyFilled, fetched.Status)
	require.Equal(t, "0.02", fetched.FilledQuantity.String())

	openOrders, err := adp.FetchOpenOrders(ctx, contract)
	require.NoError(t, err)
	require.Len(t, openOrders, 1)
	require.Equal(t, contract, openOrders[0].Symbol)

	allOrders, err := adp.FetchOrders(ctx, contract)
	require.NoError(t, err)
	require.Len(t, allOrders, 1)
	require.Equal(t, exchanges.OrderStatusFilled, allOrders[0].Status)

	require.NoError(t, adp.CancelOrder(ctx, "BTC-123", contract))
	require.NoError(t, adp.CancelAllOrders(ctx, contract))
	require.ErrorIs(t, adp.PlaceOrderWS(ctx, &exchanges.OrderParams{Symbol: contract}), exchanges.ErrNotSupported)
}

func testPerpInstrument() sdk.Instrument {
	return sdk.Instrument{
		InstrumentName:      "BTC-PERPETUAL",
		Kind:                kindFuture,
		BaseCurrency:        "BTC",
		QuoteCurrency:       "USD",
		CounterCurrency:     "USD",
		SettlementCurrency:  "BTC",
		SettlementPeriod:    "perpetual",
		TickSize:            0.5,
		MinTradeAmount:      10,
		ContractSize:        10,
		ExpirationTimestamp: 32503708800000,
		State:               "open",
		IsActive:            true,
	}
}

func testOptionInstrument() sdk.Instrument {
	return sdk.Instrument{
		InstrumentName:      "BTC-14MAY26-72000-C",
		Kind:                kindOption,
		BaseCurrency:        "BTC",
		QuoteCurrency:       "BTC",
		CounterCurrency:     "USD",
		SettlementCurrency:  "BTC",
		SettlementPeriod:    "day",
		OptionType:          "call",
		Strike:              72000,
		TickSize:            0.0001,
		MinTradeAmount:      0.1,
		ContractSize:        1,
		ExpirationTimestamp: 1778745600000,
		State:               "open",
		IsActive:            true,
	}
}

type deribitStubClient struct {
	getInstrumentsFn              func(context.Context, string, string, bool) ([]sdk.Instrument, error)
	getTickerFn                   func(context.Context, string) (*sdk.Ticker, error)
	getOrderBookFn                func(context.Context, string, int) (*sdk.OrderBook, error)
	getTradesFn                   func(context.Context, string, int) (*sdk.TradesResult, error)
	getChartFn                    func(context.Context, string, int64, int64, string) (*sdk.TradingViewChartData, error)
	buyFn                         func(context.Context, sdk.OrderRequest) (*sdk.OrderResult, error)
	sellFn                        func(context.Context, sdk.OrderRequest) (*sdk.OrderResult, error)
	cancelOrderFn                 func(context.Context, string) (*sdk.OrderRecord, error)
	cancelAllFn                   func(context.Context) (int64, error)
	cancelAllByInstrumentFn       func(context.Context, string) (int64, error)
	getOrderStateFn               func(context.Context, string) (*sdk.OrderRecord, error)
	getOpenOrdersByInstrumentFn   func(context.Context, string) ([]sdk.OrderRecord, error)
	getOrderHistoryByInstrumentFn func(context.Context, string, int) ([]sdk.OrderRecord, error)
}

func (c *deribitStubClient) GetInstruments(ctx context.Context, currency, kind string, expired bool) ([]sdk.Instrument, error) {
	if c.getInstrumentsFn == nil {
		panic("unexpected GetInstruments call")
	}
	return c.getInstrumentsFn(ctx, currency, kind, expired)
}

func (c *deribitStubClient) GetTicker(ctx context.Context, instrumentName string) (*sdk.Ticker, error) {
	if c.getTickerFn == nil {
		panic("unexpected GetTicker call")
	}
	return c.getTickerFn(ctx, instrumentName)
}

func (c *deribitStubClient) GetOrderBook(ctx context.Context, instrumentName string, depth int) (*sdk.OrderBook, error) {
	if c.getOrderBookFn == nil {
		panic("unexpected GetOrderBook call")
	}
	return c.getOrderBookFn(ctx, instrumentName, depth)
}

func (c *deribitStubClient) GetLastTradesByInstrument(ctx context.Context, instrumentName string, count int) (*sdk.TradesResult, error) {
	if c.getTradesFn == nil {
		panic("unexpected GetLastTradesByInstrument call")
	}
	return c.getTradesFn(ctx, instrumentName, count)
}

func (c *deribitStubClient) GetTradingViewChartData(ctx context.Context, instrumentName string, start, end int64, resolution string) (*sdk.TradingViewChartData, error) {
	if c.getChartFn == nil {
		panic("unexpected GetTradingViewChartData call")
	}
	return c.getChartFn(ctx, instrumentName, start, end, resolution)
}

func (c *deribitStubClient) Buy(ctx context.Context, req sdk.OrderRequest) (*sdk.OrderResult, error) {
	if c.buyFn == nil {
		panic("unexpected Buy call")
	}
	return c.buyFn(ctx, req)
}

func (c *deribitStubClient) Sell(ctx context.Context, req sdk.OrderRequest) (*sdk.OrderResult, error) {
	if c.sellFn == nil {
		panic("unexpected Sell call")
	}
	return c.sellFn(ctx, req)
}

func (c *deribitStubClient) CancelOrder(ctx context.Context, orderID string) (*sdk.OrderRecord, error) {
	if c.cancelOrderFn == nil {
		panic("unexpected CancelOrder call")
	}
	return c.cancelOrderFn(ctx, orderID)
}

func (c *deribitStubClient) CancelAll(ctx context.Context) (int64, error) {
	if c.cancelAllFn == nil {
		panic("unexpected CancelAll call")
	}
	return c.cancelAllFn(ctx)
}

func (c *deribitStubClient) CancelAllByInstrument(ctx context.Context, instrumentName string) (int64, error) {
	if c.cancelAllByInstrumentFn == nil {
		panic("unexpected CancelAllByInstrument call")
	}
	return c.cancelAllByInstrumentFn(ctx, instrumentName)
}

func (c *deribitStubClient) GetOrderState(ctx context.Context, orderID string) (*sdk.OrderRecord, error) {
	if c.getOrderStateFn == nil {
		panic("unexpected GetOrderState call")
	}
	return c.getOrderStateFn(ctx, orderID)
}

func (c *deribitStubClient) GetOpenOrdersByInstrument(ctx context.Context, instrumentName string) ([]sdk.OrderRecord, error) {
	if c.getOpenOrdersByInstrumentFn == nil {
		panic("unexpected GetOpenOrdersByInstrument call")
	}
	return c.getOpenOrdersByInstrumentFn(ctx, instrumentName)
}

func (c *deribitStubClient) GetOrderHistoryByInstrument(ctx context.Context, instrumentName string, count int) ([]sdk.OrderRecord, error) {
	if c.getOrderHistoryByInstrumentFn == nil {
		panic("unexpected GetOrderHistoryByInstrument call")
	}
	return c.getOrderHistoryByInstrumentFn(ctx, instrumentName, count)
}

func (c *deribitStubClient) HasCredentials() bool {
	return true
}

func requireUnsupportedExchangeMethods(t *testing.T, ctx context.Context, adp exchanges.Exchange) {
	t.Helper()

	_, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{Symbol: "BTC"})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.PlaceOrderWS(ctx, &exchanges.OrderParams{Symbol: "BTC"}), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.CancelOrder(ctx, "order-id", "BTC"), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.CancelOrderWS(ctx, "order-id", "BTC"), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.CancelAllOrders(ctx, "BTC"), exchanges.ErrNotSupported)
	_, err = adp.FetchOrderByID(ctx, "order-id", "BTC")
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	_, err = adp.FetchOrders(ctx, "BTC")
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	_, err = adp.FetchOpenOrders(ctx, "BTC")
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	_, err = adp.FetchAccount(ctx)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	_, err = adp.FetchBalance(ctx)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	_, err = adp.FetchFeeRate(ctx, "BTC")
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchOrderBook(ctx, "BTC", 0, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchOrderBook(ctx, "BTC"), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchOrders(ctx, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchFills(ctx, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchPositions(ctx, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchTicker(ctx, "BTC", nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchTrades(ctx, "BTC", nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchKlines(ctx, "BTC", exchanges.Interval1m, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchOrders(ctx), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchFills(ctx), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchPositions(ctx), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchTicker(ctx, "BTC"), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchTrades(ctx, "BTC"), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchKlines(ctx, "BTC", exchanges.Interval1m), exchanges.ErrNotSupported)
}
