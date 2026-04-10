package bybit

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/stretchr/testify/require"
)

func TestToTickerMapsBidAskAndMid(t *testing.T) {
	ticker := toTicker("BTC", &sdk.Ticker{
		Symbol:       "BTCUSDT",
		LastPrice:    "50000",
		Bid1Price:    "49999",
		Ask1Price:    "50001",
		Volume24h:    "10",
		Turnover24h:  "500000",
		HighPrice24h: "51000",
		LowPrice24h:  "49000",
		Time:         "1710000000000",
	})

	require.Equal(t, "BTC", ticker.Symbol)
	require.Equal(t, "50000", ticker.LastPrice.String())
	require.Equal(t, "50000", ticker.MidPrice.String())
	require.Equal(t, int64(1710000000000), ticker.Timestamp)
}

func TestPerpFetchTickerUsesClient(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
		getTickerFn: func(_ context.Context, category, symbol string) (*sdk.Ticker, error) {
			require.Equal(t, categoryLinear, category)
			require.Equal(t, "BTCUSDT", symbol)
			return &sdk.Ticker{
				Symbol:       "BTCUSDT",
				LastPrice:    "50000",
				Bid1Price:    "49999",
				Ask1Price:    "50001",
				Volume24h:    "10",
				Turnover24h:  "500000",
				HighPrice24h: "51000",
				LowPrice24h:  "49000",
				Time:         "1710000000000",
			}, nil
		},
	})
	require.NoError(t, err)

	ticker, err := adp.FetchTicker(context.Background(), "BTC")
	require.NoError(t, err)
	require.Equal(t, "BTC", ticker.Symbol)
	require.Equal(t, "50000", ticker.LastPrice.String())
}

func TestSpotFetchOrderBookUsesClient(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
		getOrderBookFn: func(_ context.Context, category, symbol string, limit int) (*sdk.OrderBook, error) {
			require.Equal(t, categorySpot, category)
			require.Equal(t, "BTCUSDT", symbol)
			require.Equal(t, 20, limit)
			return &sdk.OrderBook{
				Symbol: "BTCUSDT",
				Bids:   [][]sdk.NumberString{{"49999", "0.8"}},
				Asks:   [][]sdk.NumberString{{"50001", "1.2"}},
				TS:     1710000000001,
			}, nil
		},
	})
	require.NoError(t, err)

	book, err := adp.FetchOrderBook(context.Background(), "BTC", 20)
	require.NoError(t, err)
	require.Equal(t, "BTC", book.Symbol)
	require.Len(t, book.Bids, 1)
	require.Len(t, book.Asks, 1)
	require.Equal(t, "49999", book.Bids[0].Price.String())
}

func TestPerpFetchTradesUsesClient(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
		getRecentTradesFn: func(_ context.Context, category, symbol string, limit int) ([]sdk.PublicTrade, error) {
			require.Equal(t, categoryLinear, category)
			require.Equal(t, "BTCUSDT", symbol)
			require.Equal(t, 5, limit)
			return []sdk.PublicTrade{{
				ExecID: "trade-1",
				Symbol: "BTCUSDT",
				Price:  "50000",
				Size:   "0.25",
				Side:   "Buy",
				Time:   "1710000000002",
			}}, nil
		},
	})
	require.NoError(t, err)

	trades, err := adp.FetchTrades(context.Background(), "BTC", 5)
	require.NoError(t, err)
	require.Len(t, trades, 1)
	require.Equal(t, "trade-1", trades[0].ID)
	require.Equal(t, "buy", string(trades[0].Side))
}

func TestSpotFetchKlinesUsesClient(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
		getKlinesFn: func(_ context.Context, category, symbol, interval string, start, end int64, limit int) ([]sdk.Candle, error) {
			require.Equal(t, categorySpot, category)
			require.Equal(t, "BTCUSDT", symbol)
			require.Equal(t, "60", interval)
			require.Equal(t, 10, limit)
			return []sdk.Candle{{"1710000000000", "50000", "51000", "49000", "50500", "12", "600000"}}, nil
		},
	})
	require.NoError(t, err)

	klines, err := adp.FetchKlines(context.Background(), "BTC", exchanges.Interval1h, &exchanges.KlineOpts{Limit: 10})
	require.NoError(t, err)
	require.Len(t, klines, 1)
	require.Equal(t, "BTC", klines[0].Symbol)
	require.Equal(t, "50500", klines[0].Close.String())
}

func TestKlineIntervalStringMapsBybitIntervals(t *testing.T) {
	value, err := klineIntervalString(exchanges.Interval1h)
	require.NoError(t, err)
	require.Equal(t, "60", value)

	value, err = klineIntervalString(exchanges.Interval1d)
	require.NoError(t, err)
	require.Equal(t, "D", value)
}

func TestPerpFetchFeeRateUsesClient(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
		getFeeRatesFn: func(_ context.Context, category, symbol string) ([]sdk.FeeRateRecord, error) {
			require.Equal(t, categoryLinear, category)
			require.Equal(t, "BTCUSDT", symbol)
			return []sdk.FeeRateRecord{{
				Symbol:       "BTCUSDT",
				MakerFeeRate: "0.0001",
				TakerFeeRate: "0.0006",
			}}, nil
		},
	})
	require.NoError(t, err)

	feeRate, err := adp.FetchFeeRate(context.Background(), "BTC")
	require.NoError(t, err)
	require.Equal(t, "0.0001", feeRate.Maker.String())
	require.Equal(t, "0.0006", feeRate.Taker.String())
}

func TestPerpFetchPositionsUsesClient(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
		getPositionsFn: func(_ context.Context, category, symbol, settleCoin string) ([]sdk.PositionRecord, error) {
			require.Equal(t, categoryLinear, category)
			require.Empty(t, symbol)
			require.Equal(t, "USDT", settleCoin)
			return []sdk.PositionRecord{{
				Symbol:         "BTCUSDT",
				Side:           "Buy",
				Size:           "0.5",
				AvgPrice:       "50000",
				Leverage:       "10",
				UnrealisedPnl:  "100",
				CumRealisedPnl: "5",
				LiqPrice:       "45000",
			}}, nil
		},
	})
	require.NoError(t, err)

	positions, err := adp.FetchPositions(context.Background())
	require.NoError(t, err)
	require.Len(t, positions, 1)
	require.Equal(t, "BTC", positions[0].Symbol)
	require.Equal(t, exchanges.PositionSideLong, positions[0].Side)
}

func TestSpotFetchSpotBalancesUsesWalletBalance(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
		getWalletBalanceFn: func(_ context.Context, accountType, coin string) (*sdk.WalletBalanceResult, error) {
			require.Equal(t, "UNIFIED", accountType)
			require.Empty(t, coin)
			return &sdk.WalletBalanceResult{
				List: []sdk.WalletAccount{{
					AccountType: "UNIFIED",
					Coin: []sdk.WalletCoin{
						{Coin: "BTC", Equity: "1.2", WalletBalance: "1.0", Locked: "0.2"},
						{Coin: "USDT", Equity: "1200", WalletBalance: "1000", Locked: "200"},
					},
				}},
			}, nil
		},
	})
	require.NoError(t, err)

	balances, err := adp.FetchSpotBalances(context.Background())
	require.NoError(t, err)
	require.Len(t, balances, 2)
	require.Equal(t, "BTC", balances[0].Asset)
	require.Equal(t, "1", balances[0].Free.String())
}

func TestPerpFetchAccountAggregatesWalletPositionsAndOpenOrders(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
		getWalletBalanceFn: func(_ context.Context, accountType, coin string) (*sdk.WalletBalanceResult, error) {
			require.Equal(t, "UNIFIED", accountType)
			require.Equal(t, "USDT", coin)
			return &sdk.WalletBalanceResult{
				List: []sdk.WalletAccount{{
					AccountType:           "UNIFIED",
					TotalEquity:           "11",
					TotalAvailableBalance: "9",
					TotalPerpUPL:          "2",
					Coin: []sdk.WalletCoin{
						{Coin: "USDT", Equity: "11", WalletBalance: "9", Locked: "1", UnrealisedPnl: "2", CumRealisedPnl: "3"},
					},
				}},
			}, nil
		},
		getPositionsFn: func(_ context.Context, category, symbol, settleCoin string) ([]sdk.PositionRecord, error) {
			require.Equal(t, categoryLinear, category)
			require.Equal(t, "USDT", settleCoin)
			return []sdk.PositionRecord{{
				Symbol:         "BTCUSDT",
				Side:           "Buy",
				Size:           "0.5",
				AvgPrice:       "50000",
				Leverage:       "10",
				UnrealisedPnl:  "2",
				CumRealisedPnl: "3",
				LiqPrice:       "45000",
			}}, nil
		},
		getOpenOrdersFn: func(_ context.Context, category, symbol string) ([]sdk.OrderRecord, error) {
			require.Equal(t, categoryLinear, category)
			require.Empty(t, symbol)
			return []sdk.OrderRecord{{
				OrderID:     "1",
				OrderLinkID: "cid-1",
				Symbol:      "BTCUSDT",
				Side:        "Buy",
				OrderType:   "Limit",
				TimeInForce: "GTC",
				Price:       "50000",
				Qty:         "0.1",
				OrderStatus: "New",
				CreatedTime: "1710000000000",
				UpdatedTime: "1710000000001",
			}}, nil
		},
	})
	require.NoError(t, err)

	account, err := adp.FetchAccount(context.Background())
	require.NoError(t, err)
	require.Equal(t, "11", account.TotalBalance.String())
	require.Equal(t, "9", account.AvailableBalance.String())
	require.Len(t, account.Positions, 1)
	require.Len(t, account.Orders, 1)
}
