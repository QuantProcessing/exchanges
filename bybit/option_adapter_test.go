package bybit

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

var _ exchanges.OptionExchange = (*OptionAdapter)(nil)

func TestBybitOptionAdapterListsContracts(t *testing.T) {
	ctx := context.Background()
	adp, err := newOptionAdapterWithClient(ctx, func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryOption, category)
			return []sdk.Instrument{testOptionInstrument()}, nil
		},
		getTickerFn: func(_ context.Context, category, symbol string) (*sdk.Ticker, error) {
			require.Equal(t, categoryOption, category)
			require.Equal(t, "BTC-26MAR27-78000-P", symbol)
			return &sdk.Ticker{
				Symbol:       symbol,
				LastPrice:    "0.0525",
				Bid1Price:    "0.052",
				Ask1Price:    "0.053",
				Volume24h:    "10",
				Turnover24h:  "0.525",
				HighPrice24h: "0.06",
				LowPrice24h:  "0.04",
				Time:         "1806019200000",
			}, nil
		},
		getOrderBookFn: func(_ context.Context, category, symbol string, limit int) (*sdk.OrderBook, error) {
			require.Equal(t, categoryOption, category)
			require.Equal(t, "BTC-26MAR27-78000-P", symbol)
			require.Equal(t, 2, limit)
			return &sdk.OrderBook{
				Symbol: symbol,
				TS:     1806019200000,
				Bids:   [][]sdk.NumberString{{"0.052", "3"}},
				Asks:   [][]sdk.NumberString{{"0.053", "5"}},
			}, nil
		},
		getRecentTradesFn: func(_ context.Context, category, symbol string, limit int) ([]sdk.PublicTrade, error) {
			require.Equal(t, categoryOption, category)
			require.Equal(t, "BTC-26MAR27-78000-P", symbol)
			require.Equal(t, 1, limit)
			return []sdk.PublicTrade{{ExecID: "trade-1", Symbol: symbol, Price: "0.0525", Size: "3", Side: "Sell", Time: "1806019200000"}}, nil
		},
		getKlinesFn: func(_ context.Context, category, symbol, interval string, _ int64, _ int64, limit int) ([]sdk.Candle, error) {
			require.Equal(t, categoryOption, category)
			require.Equal(t, "BTC-26MAR27-78000-P", symbol)
			require.Equal(t, "60", interval)
			require.Equal(t, 200, limit)
			return []sdk.Candle{{"1806019200000", "0.05", "0.06", "0.04", "0.0525", "3", "0.1575"}}, nil
		},
	})
	require.NoError(t, err)

	contracts, err := adp.ListOptionContracts(ctx, "BTC")
	require.NoError(t, err)
	require.Len(t, contracts, 1)

	contract := contracts[0]
	require.Equal(t, "BTC-USDT-USDT-20270326-78000-P", contract.Symbol)
	require.Equal(t, "BTC-26MAR27-78000-P", contract.ExchangeSymbol)
	require.Equal(t, exchanges.OptionTypePut, contract.Type)
	require.Equal(t, "78000", contract.StrikePrice.String())
	require.Equal(t, int64(1806019200000), contract.ExpiryTime)
	require.Equal(t, "1", contract.ContractSize.String())
	require.Equal(t, "0.5", contract.TickSize.String())
	require.Equal(t, "0.01", contract.LotSize.String())
	require.Equal(t, "0.01", contract.MinQuantity.String())

	require.Equal(t, "BTC-26MAR27-78000-P", adp.FormatSymbol("BTC-USDT-USDT-20270326-78000-P"))
	require.Equal(t, "BTC-USDT-USDT-20270326-78000-P", adp.ExtractSymbol("BTC-26MAR27-78000-P"))

	fetched, err := adp.FetchOptionContract(ctx, "BTC-26MAR27-78000-P")
	require.NoError(t, err)
	require.Equal(t, contract.Symbol, fetched.Symbol)

	testsuite.RunOptionPublicDataSuite(t, adp, testsuite.OptionPublicDataConfig{
		Underlying: "BTC",
		Symbol:     contract.Symbol,
		Native:     contract.ExchangeSymbol,
		Interval:   exchanges.Interval1h,
	})
}

func TestBybitOptionAdapterPreloadsDocumentedDefaultUnderlyings(t *testing.T) {
	ctx := context.Background()
	var requested []string
	adp, err := newOptionAdapterWithClient(ctx, func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsForBaseFn: func(_ context.Context, category, baseCoin string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryOption, category)
			requested = append(requested, baseCoin)
			switch baseCoin {
			case "BTC":
				return []sdk.Instrument{testOptionInstrument()}, nil
			case "ETH":
				return []sdk.Instrument{testETHOptionInstrument()}, nil
			case "SOL":
				return nil, nil
			default:
				return nil, nil
			}
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"BTC", "ETH", "SOL"}, requested)

	contracts, err := adp.ListOptionContracts(ctx, "ETH")
	require.NoError(t, err)
	require.Len(t, contracts, 1)
	require.Equal(t, "ETH-USDT-USDT-20270326-8000-C", contracts[0].Symbol)
	require.Equal(t, "ETH-26MAR27-8000-C", contracts[0].ExchangeSymbol)
}

func TestBybitOptionAdapterKeepsUSDQuotedOptions(t *testing.T) {
	ctx := context.Background()
	adp, err := newOptionAdapterWithClient(ctx, func() {}, Options{OptionUnderlyings: []string{"BTC"}}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsForBaseFn: func(_ context.Context, category, baseCoin string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryOption, category)
			require.Equal(t, "BTC", baseCoin)
			return []sdk.Instrument{{
				Symbol:        "BTC-14MAY26-75000-C",
				BaseCoin:      "BTC",
				QuoteCoin:     "USD",
				SettleCoin:    "USDC",
				Status:        instrumentStatusTrading,
				OptionsType:   "Call",
				DeliveryTime:  "1778745600000",
				PriceFilter:   sdk.PriceFilter{TickSize: "0.5"},
				LotSizeFilter: sdk.LotSizeFilter{MinOrderQty: "0.1", QtyStep: "0.1"},
			}}, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"BTC-USD-USDC-20260514-75000-C"}, adp.ListSymbols())
}

func TestBybitOptionAdapterRESTTradingUsesOptionCategory(t *testing.T) {
	ctx := context.Background()
	contract := "BTC-USDT-USDT-20270326-78000-P"
	native := "BTC-26MAR27-78000-P"

	client := &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryOption, category)
			return []sdk.Instrument{testOptionInstrument()}, nil
		},
		placeOrderFn: func(_ context.Context, req sdk.PlaceOrderRequest) (*sdk.OrderActionResponse, error) {
			require.Equal(t, categoryOption, req.Category)
			require.Equal(t, native, req.Symbol)
			require.Equal(t, "Buy", req.Side)
			require.Equal(t, "Limit", req.OrderType)
			require.Equal(t, "0.12", req.Qty)
			require.Equal(t, "0.5", req.Price)
			require.Equal(t, "GTC", req.TimeInForce)
			require.Equal(t, "option-client-1", req.OrderLinkID)
			return &sdk.OrderActionResponse{OrderID: "order-1", OrderLinkID: req.OrderLinkID}, nil
		},
		cancelOrderFn: func(_ context.Context, req sdk.CancelOrderRequest) (*sdk.OrderActionResponse, error) {
			require.Equal(t, categoryOption, req.Category)
			require.Equal(t, native, req.Symbol)
			require.Equal(t, "order-1", req.OrderID)
			return &sdk.OrderActionResponse{OrderID: req.OrderID}, nil
		},
		cancelAllOrdersFn: func(_ context.Context, req sdk.CancelAllOrdersRequest) error {
			require.Equal(t, categoryOption, req.Category)
			require.Empty(t, req.Symbol)
			return nil
		},
		getRealtimeOrdersFn: func(_ context.Context, category, symbol, settleCoin, orderID, orderLinkID string, openOnly int) ([]sdk.OrderRecord, error) {
			require.Equal(t, categoryOption, category)
			require.Empty(t, settleCoin)
			require.Empty(t, orderLinkID)
			record := sdk.OrderRecord{
				OrderID:     "order-1",
				OrderLinkID: "option-client-1",
				Symbol:      native,
				Side:        "Buy",
				OrderType:   "Limit",
				TimeInForce: "GTC",
				Price:       "0.05",
				Qty:         "0.12",
				CumExecQty:  "0.02",
				OrderStatus: "PartiallyFilled",
				UpdatedTime: "1806019200000",
			}
			if orderID != "" {
				require.Equal(t, native, symbol)
				require.Equal(t, "order-1", orderID)
				return []sdk.OrderRecord{record}, nil
			}
			if openOnly == 0 {
				require.Contains(t, []string{"", native}, symbol)
				return []sdk.OrderRecord{record}, nil
			}
			require.Equal(t, 1, openOnly)
			return []sdk.OrderRecord{{
				OrderID:     "order-closed",
				Symbol:      native,
				Side:        "Sell",
				OrderType:   "Limit",
				TimeInForce: "GTC",
				Price:       "0.06",
				Qty:         "0.1",
				OrderStatus: "Cancelled",
				UpdatedTime: "1806019200001",
			}}, nil
		},
		getOrderHistoryFn: func(_ context.Context, category, symbol string) ([]sdk.OrderRecord, error) {
			require.Equal(t, categoryOption, category)
			require.Equal(t, native, symbol)
			return []sdk.OrderRecord{{
				OrderID:     "order-filled",
				Symbol:      native,
				Side:        "Buy",
				OrderType:   "Limit",
				TimeInForce: "GTC",
				Price:       "0.05",
				Qty:         "0.1",
				CumExecQty:  "0.1",
				OrderStatus: "Filled",
				UpdatedTime: "1806019200002",
			}}, nil
		},
	}

	adp, err := newOptionAdapterWithClient(ctx, func() {}, Options{OptionUnderlyings: []string{"BTC"}}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:      contract,
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.12"),
		Price:       decimal.RequireFromString("0.5"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "option-client-1",
	})
	require.NoError(t, err)
	require.Equal(t, "order-1", order.OrderID)
	require.Equal(t, contract, order.Symbol)

	require.NoError(t, adp.CancelOrder(ctx, "order-1", contract))
	require.NoError(t, adp.CancelAllOrders(ctx, ""))

	openOrders, err := adp.FetchOpenOrders(ctx, "")
	require.NoError(t, err)
	require.Len(t, openOrders, 1)
	require.Equal(t, contract, openOrders[0].Symbol)
	require.Equal(t, exchanges.OrderStatusPartiallyFilled, openOrders[0].Status)

	fetched, err := adp.FetchOrderByID(ctx, "order-1", contract)
	require.NoError(t, err)
	require.Equal(t, "order-1", fetched.OrderID)

	allOrders, err := adp.FetchOrders(ctx, contract)
	require.NoError(t, err)
	require.Len(t, allOrders, 3)
}

func testOptionInstrument() sdk.Instrument {
	return sdk.Instrument{
		Symbol:       "BTC-26MAR27-78000-P",
		BaseCoin:     "BTC",
		QuoteCoin:    "USDT",
		SettleCoin:   "USDT",
		Status:       instrumentStatusTrading,
		OptionsType:  "Put",
		DeliveryTime: "1806019200000",
		PriceFilter: sdk.PriceFilter{
			TickSize: "0.5",
		},
		LotSizeFilter: sdk.LotSizeFilter{
			QtyStep:     "0.01",
			MinOrderQty: "0.01",
		},
	}
}

func testETHOptionInstrument() sdk.Instrument {
	inst := testOptionInstrument()
	inst.Symbol = "ETH-26MAR27-8000-C"
	inst.BaseCoin = "ETH"
	inst.OptionsType = "Call"
	return inst
}
