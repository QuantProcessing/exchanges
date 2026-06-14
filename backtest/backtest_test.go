package backtest

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestRunnerReplaysMarketEventsIntoStrategies(t *testing.T) {
	rec := &recordingStrategy{id: "bt"}
	runner := NewRunner(Config{
		Events: []Event{{
			At:    time.Unix(10, 0),
			Topic: strategy.TopicMarketData,
			Message: model.MarketEvent{Ticker: &model.Ticker{
				InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
				Last:         decimal.RequireFromString("100"),
			}},
		}},
		Strategies: []strategy.Strategy{rec},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, result.EventsProcessed)
	require.Eventually(t, func() bool {
		return len(rec.events) == 1 && rec.events[0].Topic == strategy.TopicMarketData
	}, time.Second, 10*time.Millisecond)
	require.True(t, rec.stopped)
}

func TestRunnerDispatchesTimersOnSimulatedClockBeforeNextEvent(t *testing.T) {
	start := time.Unix(1_000, 0).UTC()
	impl := &timerBacktestStrategy{}
	runner := NewRunner(Config{
		Events: []Event{
			tickerEventAt(start, decimal.RequireFromString("100")),
			tickerEventAt(start.Add(90*time.Second), decimal.RequireFromString("101")),
		},
		Strategies: []strategy.Strategy{strategy.NewTyped("timer-backtest", impl)},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, result.EventsProcessed)
	require.Equal(t, start, impl.startedAt)
	require.Equal(t, []string{"ticker", "timer:heartbeat", "ticker"}, impl.events)
	require.Equal(t, []time.Time{start, start.Add(time.Minute), start.Add(90 * time.Second)}, impl.clockTimes)
	require.Equal(t, []time.Time{start.Add(time.Minute)}, impl.timerTimes)
}

func TestBacktestMatchesMarketOrderAgainstTicker(t *testing.T) {
	trader := &submittingStrategy{id: "submitter"}
	runner := NewRunner(Config{
		Events:     []Event{tickerEvent(decimal.RequireFromString("100"))},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, result.EventsProcessed)

	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.True(t, decimal.RequireFromString("1").Equal(order.FilledQuantity))
	require.True(t, decimal.RequireFromString("100").Equal(order.AveragePrice))

	fills := result.Cache.FillsForOrder("backtest", order.OrderID)
	require.Len(t, fills, 1)
	require.True(t, decimal.RequireFromString("100").Equal(fills[0].Price))

	position, ok := result.Cache.PositionByInstrument("backtest", model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"))
	require.True(t, ok)
	require.True(t, decimal.RequireFromString("1").Equal(position.Quantity))
	require.True(t, decimal.RequireFromString("100").Equal(position.EntryPrice))
}

func TestBacktestAppliesInstrumentTakerFeeToMarketFills(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	c := cache.New()
	require.NoError(t, c.PutInstrument(model.Instrument{
		ID:        instID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		TakerFee:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}))
	trader := &submittingStrategy{id: "fee-submitter"}
	runner := NewRunner(Config{
		Cache:      c,
		Events:     []Event{tickerEvent(decimal.RequireFromString("100"))},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	fills := result.Cache.FillsForOrder("backtest", order.OrderID)
	require.Len(t, fills, 1)
	require.Equal(t, "0.1", fills[0].Fee.String())
	require.Equal(t, model.Currency("USDT"), fills[0].FeeCurrency)
	require.Equal(t, "0.1", result.Portfolio.Commission("backtest", "USDT").String())
}

func TestBacktestAppliesInstrumentMakerFeeToRestingLimitFills(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	c := cache.New()
	require.NoError(t, c.PutInstrument(model.Instrument{
		ID:        instID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		MakerFee:  decimal.RequireFromString("0.0002"),
		TakerFee:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}))
	trader := &submittingStrategy{
		id:        "maker-fee-submitter",
		orderType: model.OrderTypeLimit,
		price:     decimal.RequireFromString("99"),
		postOnly:  true,
	}
	runner := NewRunner(Config{
		Cache: c,
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			{
				At:    time.Unix(11, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{Quote: &model.QuoteTick{
					InstrumentID: instID,
					BidPrice:     decimal.RequireFromString("98.5"),
					AskPrice:     decimal.RequireFromString("99"),
					BidSize:      decimal.RequireFromString("1"),
					AskSize:      decimal.RequireFromString("1"),
					Timestamp:    time.Unix(11, 0),
					InitTime:     time.Unix(11, 0),
				}},
			},
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	fills := result.Cache.FillsForOrder("backtest", order.OrderID)
	require.Len(t, fills, 1)
	require.Equal(t, "0.0198", fills[0].Fee.String())
	require.Equal(t, model.Currency("USDT"), fills[0].FeeCurrency)
	require.Equal(t, "0.0198", result.Portfolio.Commission("backtest", "USDT").String())
}

func TestBacktestOrderLatencyDelaysOrderEligibility(t *testing.T) {
	trader := &submittingStrategy{id: "latency-submitter"}
	runner := NewRunner(Config{
		OrderLatency: time.Second,
		Events: []Event{
			tickerEventAt(time.Unix(10, 0), decimal.RequireFromString("100")),
			tickerEventAt(time.Unix(10, int64(500*time.Millisecond)), decimal.RequireFromString("101")),
			tickerEventAt(time.Unix(11, 0), decimal.RequireFromString("102")),
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "102", order.AveragePrice.String())
}

func TestBacktestFillModelCanRejectLimitTouchFills(t *testing.T) {
	fillModel, err := NewFillModel(FillModelConfig{
		ProbFillOnLimit:    0,
		ProbFillOnLimitSet: true,
		ProbSlippage:       0,
		RandomSeed:         42,
	})
	require.NoError(t, err)
	trader := &bookSubmittingStrategy{
		id:       "limit-touch-submitter",
		side:     model.OrderSideBuy,
		orderTyp: model.OrderTypeLimit,
		quantity: decimal.RequireFromString("1"),
		price:    decimal.RequireFromString("100"),
	}
	runner := NewRunner(Config{
		FillModel: fillModel,
		Events: []Event{bookEvent(
			nil,
			[]model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("1")}},
		)},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "book-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, order.Status)
	require.True(t, order.FilledQuantity.IsZero())
	require.Empty(t, result.Cache.FillsForOrder("backtest", order.OrderID))
}

func TestBacktestFillModelAppliesOneTickSlippageToL1TakerFill(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	c := cache.New()
	require.NoError(t, c.PutInstrument(model.Instrument{
		ID:        instID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.5"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}))
	fillModel, err := NewFillModel(FillModelConfig{
		ProbFillOnLimit: 1,
		ProbSlippage:    1,
		RandomSeed:      42,
	})
	require.NoError(t, err)
	trader := &quoteSubmittingStrategy{accountID: "backtest", instrumentID: instID}
	runner := NewRunner(Config{
		Cache:      c,
		FillModel:  fillModel,
		Events:     []Event{quoteEvent(decimal.RequireFromString("100"), decimal.RequireFromString("101"))},
		Strategies: []strategy.Strategy{strategy.NewTyped("quote-slip", trader)},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "quote-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "101.5", order.AveragePrice.String())
	fills := result.Cache.FillsForOrder("backtest", order.OrderID)
	require.Len(t, fills, 1)
	require.Equal(t, "101.5", fills[0].Price.String())
}

func TestBacktestResultPortfolioUpdatesFromMarketData(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	c := cache.New()
	require.NoError(t, c.PutInstrument(model.Instrument{
		ID:        instID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}))
	trader := &submittingStrategy{id: "portfolio-submitter"}
	runner := NewRunner(Config{
		Cache: c,
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			quoteEvent(decimal.RequireFromString("120"), decimal.RequireFromString("121")),
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result.Portfolio)
	require.Equal(t, "20", result.Portfolio.UnrealizedPnL("backtest", instID).String())
	require.Equal(t, "120", result.Portfolio.Exposure("backtest", "USDT").String())
}

func TestBacktestSharedPortfolioCacheDoesNotDoubleApplyFills(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	c := cache.New()
	require.NoError(t, c.PutInstrument(model.Instrument{
		ID:        instID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}))
	p := portfolio.New(c)
	trader := &submittingStrategy{id: "shared-portfolio-submitter"}
	runner := NewRunner(Config{
		Cache:      c,
		Portfolio:  p,
		Events:     []Event{tickerEvent(decimal.RequireFromString("100"))},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)

	position, ok := result.Cache.PositionByInstrument("backtest", instID)
	require.True(t, ok)
	require.Equal(t, "1", position.Quantity.String())
	require.Equal(t, "100", result.Portfolio.Exposure("backtest", "USDT").String())
}

func TestBacktestRuntimeExposesPortfolioToStrategies(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	c := cache.New()
	require.NoError(t, c.PutInstrument(model.Instrument{
		ID:        instID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}))
	reader := &portfolioRuntimeStrategy{}
	runner := NewRunner(Config{
		Cache:      c,
		Events:     []Event{tickerEvent(decimal.RequireFromString("100"))},
		Strategies: []strategy.Strategy{reader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)

	require.True(t, reader.sawPortfolio)
	require.Equal(t, "100", reader.exposure.String())
	require.Equal(t, "100", result.Portfolio.Exposure("backtest", "USDT").String())
}

func TestBacktestMatchesMarketOrderAgainstTradeTick(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	trader := &tradeSubmittingStrategy{accountID: "backtest", instrumentID: instID}
	runner := NewRunner(Config{
		Events:     []Event{tradeEvent(decimal.RequireFromString("100.25"), decimal.RequireFromString("0.8"))},
		Strategies: []strategy.Strategy{strategy.NewTyped("trade-submit", trader)},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.True(t, trader.seen)

	order, ok := result.Cache.OrderByClientID("backtest", "trade-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.True(t, decimal.RequireFromString("1").Equal(order.FilledQuantity))
	require.True(t, decimal.RequireFromString("100.25").Equal(order.AveragePrice))

	fills := result.Cache.FillsForOrder("backtest", order.OrderID)
	require.Len(t, fills, 1)
	require.True(t, decimal.RequireFromString("100.25").Equal(fills[0].Price))
	require.Equal(t, time.Unix(11, 0), fills[0].Timestamp)

	trade, ok := result.Cache.TradeTick(instID)
	require.True(t, ok)
	require.Equal(t, model.TradeID("venue-trade-1"), trade.TradeID)
}

func TestBacktestMatchesMarketOrderAgainstQuoteTick(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	trader := &quoteSubmittingStrategy{accountID: "backtest", instrumentID: instID}
	runner := NewRunner(Config{
		Events:     []Event{quoteEvent(decimal.RequireFromString("100"), decimal.RequireFromString("101"))},
		Strategies: []strategy.Strategy{strategy.NewTyped("quote-submit", trader)},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.True(t, trader.seen)

	order, ok := result.Cache.OrderByClientID("backtest", "quote-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.True(t, decimal.RequireFromString("1").Equal(order.FilledQuantity))
	require.True(t, decimal.RequireFromString("101").Equal(order.AveragePrice))

	quote, ok := result.Cache.QuoteTick(instID)
	require.True(t, ok)
	require.True(t, decimal.RequireFromString("101").Equal(quote.AskPrice))
}

func TestBacktestMatchesMarketOrderAgainstBarClose(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	barType := model.NewTimeBarType(instID, time.Minute)
	trader := &barSubmittingStrategy{accountID: "backtest", barType: barType}
	runner := NewRunner(Config{
		Events:     []Event{barEvent(barType, decimal.RequireFromString("101"))},
		Strategies: []strategy.Strategy{strategy.NewTyped("bar-submit", trader)},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.True(t, trader.seen)

	order, ok := result.Cache.OrderByClientID("backtest", "bar-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.True(t, decimal.RequireFromString("1").Equal(order.FilledQuantity))
	require.True(t, decimal.RequireFromString("101").Equal(order.AveragePrice))

	bar, ok := result.Cache.Bar(barType)
	require.True(t, ok)
	require.True(t, decimal.RequireFromString("101").Equal(bar.Close))
}

func TestBacktestMatchesLimitOrderAgainstBarIntrabarLow(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	barType := model.NewTimeBarType(instID, time.Minute)
	trader := &barSubmittingStrategy{
		accountID:     "backtest",
		barType:       barType,
		orderType:     model.OrderTypeLimit,
		price:         decimal.RequireFromString("99.5"),
		clientOrderID: "bar-limit-client-1",
	}
	runner := NewRunner(Config{
		Events:     []Event{barEvent(barType, decimal.RequireFromString("101"))},
		Strategies: []strategy.Strategy{strategy.NewTyped("bar-limit-submit", trader)},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.True(t, trader.seen)

	order, ok := result.Cache.OrderByClientID("backtest", "bar-limit-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.True(t, decimal.RequireFromString("1").Equal(order.FilledQuantity))
	require.True(t, decimal.RequireFromString("99.5").Equal(order.AveragePrice))

	fills := result.Cache.FillsForOrder("backtest", order.OrderID)
	require.Len(t, fills, 1)
	require.True(t, decimal.RequireFromString("99.5").Equal(fills[0].Price))
}

func TestBacktestMatchesSellLimitOrderAgainstBarIntrabarHigh(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	barType := model.NewTimeBarType(instID, time.Minute)
	trader := &barSubmittingStrategy{
		accountID:     "backtest",
		barType:       barType,
		side:          model.OrderSideSell,
		orderType:     model.OrderTypeLimit,
		price:         decimal.RequireFromString("101.5"),
		clientOrderID: "bar-sell-limit-client-1",
	}
	runner := NewRunner(Config{
		Events:     []Event{barEvent(barType, decimal.RequireFromString("100"))},
		Strategies: []strategy.Strategy{strategy.NewTyped("bar-sell-limit-submit", trader)},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.True(t, trader.seen)

	order, ok := result.Cache.OrderByClientID("backtest", "bar-sell-limit-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.True(t, decimal.RequireFromString("1").Equal(order.FilledQuantity))
	require.True(t, decimal.RequireFromString("101.5").Equal(order.AveragePrice))

	fills := result.Cache.FillsForOrder("backtest", order.OrderID)
	require.Len(t, fills, 1)
	require.True(t, decimal.RequireFromString("101.5").Equal(fills[0].Price))
}

func TestBacktestMatchesMarketOrderAgainstOrderBookLevels(t *testing.T) {
	trader := &bookSubmittingStrategy{
		id:       "book-submitter",
		side:     model.OrderSideBuy,
		orderTyp: model.OrderTypeMarket,
		quantity: decimal.RequireFromString("1"),
	}
	runner := NewRunner(Config{
		Events: []Event{bookEvent(
			[]model.OrderBookLevel{},
			[]model.OrderBookLevel{
				{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("0.4")},
				{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("0.6")},
			},
		)},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "book-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.True(t, decimal.RequireFromString("1").Equal(order.FilledQuantity))
	require.True(t, decimal.RequireFromString("100.6").Equal(order.AveragePrice))

	fills := result.Cache.FillsForOrder("backtest", order.OrderID)
	require.Len(t, fills, 2)
	require.True(t, decimal.RequireFromString("100").Equal(fills[0].Price))
	require.True(t, decimal.RequireFromString("0.4").Equal(fills[0].Quantity))
	require.True(t, decimal.RequireFromString("101").Equal(fills[1].Price))
	require.True(t, decimal.RequireFromString("0.6").Equal(fills[1].Quantity))
}

func TestBacktestDoesNotMutateHistoricalOrderBookAfterFills(t *testing.T) {
	trader := &bookSubmittingStrategy{
		id:       "immutable-book-submitter",
		side:     model.OrderSideBuy,
		orderTyp: model.OrderTypeMarket,
		quantity: decimal.RequireFromString("0.5"),
	}
	runner := NewRunner(Config{
		Events: []Event{bookEvent(
			[]model.OrderBookLevel{},
			[]model.OrderBookLevel{{
				Price: decimal.RequireFromString("100"),
				Size:  decimal.RequireFromString("1"),
			}},
		)},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	book, ok := result.Cache.OrderBook(model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"))
	require.True(t, ok)
	require.Len(t, book.Asks, 1)
	require.Equal(t, "1", book.Asks[0].Size.String())
}

func TestBacktestRestsLimitOrderAndMatchesOnLaterBook(t *testing.T) {
	trader := &submittingStrategy{
		id:        "submitter",
		orderType: model.OrderTypeLimit,
		price:     decimal.RequireFromString("99"),
	}
	runner := NewRunner(Config{
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			{
				At:    time.Unix(11, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{OrderBook: &model.OrderBook{
					InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
					Asks: []model.OrderBookLevel{{
						Price: decimal.RequireFromString("98"),
						Size:  decimal.RequireFromString("1"),
					}},
					Timestamp: time.Unix(11, 0),
				}},
			},
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.True(t, decimal.RequireFromString("98").Equal(order.AveragePrice))
}

func TestBacktestMatchesExistingOrdersBeforeStrategyReceivesNextMarketEvent(t *testing.T) {
	trader := &existingOrderObservationStrategy{}
	runner := NewRunner(Config{
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			{
				At:    time.Unix(11, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{OrderBook: &model.OrderBook{
					InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
					Asks: []model.OrderBookLevel{{
						Price: decimal.RequireFromString("98"),
						Size:  decimal.RequireFromString("1"),
					}},
					Timestamp: time.Unix(11, 0),
				}},
			},
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusFilled, trader.statusSeenOnSecondEvent)
	order, ok := result.Cache.OrderByClientID("backtest", "existing-before-callback")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
}

func TestBacktestTriggersStopMarketOrderOnLaterTick(t *testing.T) {
	trader := &submittingStrategy{
		id:        "stop-submitter",
		orderType: model.OrderTypeStopMarket,
		trigger:   decimal.RequireFromString("101"),
	}
	runner := NewRunner(Config{
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			{
				At:    time.Unix(11, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{Ticker: &model.Ticker{
					InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
					Bid:          decimal.RequireFromString("102"),
					Ask:          decimal.RequireFromString("102"),
					Last:         decimal.RequireFromString("102"),
					Timestamp:    time.Unix(11, 0),
				}},
			},
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "102", order.AveragePrice.String())
}

func TestBacktestTriggersBuyStopMarketOrderOnBarIntrabarHigh(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	barType := model.NewTimeBarType(instID, time.Minute)
	trader := &submittingStrategy{
		id:        "bar-stop-buy-submitter",
		orderType: model.OrderTypeStopMarket,
		trigger:   decimal.RequireFromString("101"),
	}
	runner := NewRunner(Config{
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			barEvent(barType, decimal.RequireFromString("100")),
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)

	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "101", order.AveragePrice.String())
}

func TestBacktestTriggersSellStopMarketOrderOnBarIntrabarLow(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	barType := model.NewTimeBarType(instID, time.Minute)
	trader := &submittingStrategy{
		id:        "bar-stop-sell-submitter",
		side:      model.OrderSideSell,
		orderType: model.OrderTypeStopMarket,
		trigger:   decimal.RequireFromString("99.5"),
	}
	runner := NewRunner(Config{
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			barEvent(barType, decimal.RequireFromString("100")),
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)

	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "99.5", order.AveragePrice.String())
}

func TestBacktestMarketToLimitRestsResidualAtFirstFillPrice(t *testing.T) {
	trader := &bookSubmittingStrategy{
		id:       "mtl-submitter",
		side:     model.OrderSideBuy,
		orderTyp: model.OrderTypeMarketToLimit,
		quantity: decimal.RequireFromString("1.5"),
	}
	runner := NewRunner(Config{
		Events: []Event{bookEvent(
			[]model.OrderBookLevel{},
			[]model.OrderBookLevel{{
				Price: decimal.RequireFromString("100"),
				Size:  decimal.RequireFromString("1"),
			}},
		)},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "book-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusPartiallyFilled, order.Status)
	require.Equal(t, "1", order.FilledQuantity.String())
	require.Equal(t, "0.5", order.LeavesQuantity.String())
	require.Equal(t, "100", order.Price.String())
}

func TestBacktestTriggersStopLimitThenMatchesLimitPrice(t *testing.T) {
	trader := &submittingStrategy{
		id:        "stop-limit-submitter",
		orderType: model.OrderTypeStopLimit,
		price:     decimal.RequireFromString("101"),
		trigger:   decimal.RequireFromString("101"),
	}
	runner := NewRunner(Config{
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			{
				At:    time.Unix(11, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{Ticker: &model.Ticker{
					InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
					Bid:          decimal.RequireFromString("102"),
					Ask:          decimal.RequireFromString("102"),
					Last:         decimal.RequireFromString("102"),
					Timestamp:    time.Unix(11, 0),
				}},
			},
			{
				At:    time.Unix(12, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{OrderBook: &model.OrderBook{
					InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
					Asks: []model.OrderBookLevel{{
						Price: decimal.RequireFromString("101"),
						Size:  decimal.RequireFromString("1"),
					}},
					Timestamp: time.Unix(12, 0),
				}},
			},
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "101", order.AveragePrice.String())
}

func TestBacktestTriggersTrailingStopMarketFromActivatedHighWatermark(t *testing.T) {
	trader := &submittingStrategy{
		id:         "trailing-submitter",
		side:       model.OrderSideSell,
		orderType:  model.OrderTypeTrailingStopMarket,
		activation: decimal.RequireFromString("100"),
		trailing:   decimal.RequireFromString("5"),
	}
	runner := NewRunner(Config{
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			{
				At:    time.Unix(11, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{Ticker: &model.Ticker{
					InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
					Bid:          decimal.RequireFromString("110"),
					Ask:          decimal.RequireFromString("110"),
					Last:         decimal.RequireFromString("110"),
					Timestamp:    time.Unix(11, 0),
				}},
			},
			{
				At:    time.Unix(12, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{Ticker: &model.Ticker{
					InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
					Bid:          decimal.RequireFromString("104"),
					Ask:          decimal.RequireFromString("104"),
					Last:         decimal.RequireFromString("104"),
					Timestamp:    time.Unix(12, 0),
				}},
			},
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "104", order.AveragePrice.String())
}

func TestBacktestTriggersTrailingStopMarketOnBarIntrabarHighLow(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	barType := model.NewTimeBarType(instID, time.Minute)
	trader := &submittingStrategy{
		id:         "bar-trailing-submitter",
		side:       model.OrderSideSell,
		orderType:  model.OrderTypeTrailingStopMarket,
		activation: decimal.RequireFromString("100"),
		trailing:   decimal.RequireFromString("5"),
	}
	runner := NewRunner(Config{
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			{
				At:    time.Unix(12, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{Bar: &model.Bar{
					BarType:   barType,
					Open:      decimal.RequireFromString("100"),
					High:      decimal.RequireFromString("110"),
					Low:       decimal.RequireFromString("104"),
					Close:     decimal.RequireFromString("106"),
					Volume:    decimal.RequireFromString("12.5"),
					Timestamp: time.Unix(12, 0),
					InitTime:  time.Unix(12, 0),
				}},
			},
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)

	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "105", order.AveragePrice.String())
	require.Equal(t, "105", order.TriggerPrice.String())
}

func TestBacktestTriggersTrailingStopLimitFromActivatedHighWatermark(t *testing.T) {
	trader := &submittingStrategy{
		id:         "trailing-limit-submitter",
		side:       model.OrderSideSell,
		orderType:  model.OrderTypeTrailingStopLimit,
		price:      decimal.RequireFromString("103"),
		activation: decimal.RequireFromString("100"),
		trailing:   decimal.RequireFromString("5"),
	}
	runner := NewRunner(Config{
		Events: []Event{
			tickerEvent(decimal.RequireFromString("100")),
			{
				At:    time.Unix(11, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{Ticker: &model.Ticker{
					InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
					Bid:          decimal.RequireFromString("110"),
					Ask:          decimal.RequireFromString("110"),
					Last:         decimal.RequireFromString("110"),
					Timestamp:    time.Unix(11, 0),
				}},
			},
			{
				At:    time.Unix(12, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{Ticker: &model.Ticker{
					InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
					Bid:          decimal.RequireFromString("104"),
					Ask:          decimal.RequireFromString("104"),
					Last:         decimal.RequireFromString("104"),
					Timestamp:    time.Unix(12, 0),
				}},
			},
			{
				At:    time.Unix(13, 0),
				Topic: strategy.TopicMarketData,
				Message: model.MarketEvent{OrderBook: &model.OrderBook{
					InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
					Bids: []model.OrderBookLevel{{
						Price: decimal.RequireFromString("104"),
						Size:  decimal.RequireFromString("1"),
					}},
					Timestamp: time.Unix(13, 0),
				}},
			},
		},
		Strategies: []strategy.Strategy{trader},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "bt-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "104", order.AveragePrice.String())
}

func TestBacktestPublishesFillAndPositionBeforeStop(t *testing.T) {
	b := bus.New()
	sub := b.Subscribe(strategy.TopicExecution, 8)
	defer sub.Close()
	trader := &submittingStrategy{id: "submitter"}
	runner := NewRunner(Config{
		Bus:        b,
		Events:     []Event{tickerEvent(decimal.RequireFromString("100"))},
		Strategies: []strategy.Strategy{trader},
	})

	_, err := runner.Run(context.Background())
	require.NoError(t, err)

	var sawFill bool
	var sawPosition bool
	deadline := time.After(time.Second)
	for !sawFill || !sawPosition {
		select {
		case env := <-sub.C():
			event := env.Message.(model.ExecutionEvent)
			sawFill = sawFill || event.Fill != nil
			sawPosition = sawPosition || event.Position != nil
		case <-deadline:
			require.Fail(t, "timed out waiting for fill and position events")
		}
	}
	require.True(t, sawFill)
	require.True(t, sawPosition)
	require.True(t, trader.stopped)
}

func TestEngineRunsTypedStrategyWithNautilusStyleAPI(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &typedBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	}
	engine.AddStrategy(strategy.NewTyped("bt-typed", impl))
	engine.AddData(bookEvent(
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("3")}},
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
	))

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, result.EventsProcessed)
	require.True(t, impl.wasFilled())

	order, ok := result.Cache.OrderByClientID("backtest", "bt-typed-client-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "0.01", order.FilledQuantity.String())
}

func TestBacktestSettlesCascadingOrdersSubmittedFromFillCallback(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &cascadingFillStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	}
	engine.AddStrategy(strategy.NewTyped("cascade", impl))
	engine.AddData(bookEvent(
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("1")}},
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
	))

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.True(t, impl.hedged)
	entry, ok := result.Cache.OrderByClientID("backtest", "cascade-entry")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, entry.Status)
	hedge, ok := result.Cache.OrderByClientID("backtest", "cascade-hedge")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, hedge.Status)
}

func TestBacktestBracketReleasesChildrenAndCancelsOcoSibling(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &bracketBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	}
	engine.AddStrategy(strategy.NewTyped("bracket", impl))
	engine.AddData(bookEvent(
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("2")}},
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("2")}},
	))
	engine.AddData(Event{
		At:    time.Unix(11, 0),
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{OrderBook: &model.OrderBook{
			InstrumentID: impl.instrumentID,
			Bids: []model.OrderBookLevel{{
				Price: decimal.RequireFromString("110"),
				Size:  decimal.RequireFromString("2"),
			}},
			Asks: []model.OrderBookLevel{{
				Price: decimal.RequireFromString("111"),
				Size:  decimal.RequireFromString("2"),
			}},
			Timestamp: time.Unix(11, 0),
		}},
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	entry, ok := result.Cache.OrderByClientID("backtest", "backtest-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, entry.Status)
	stopLoss, ok := result.Cache.OrderByClientID("backtest", "backtest-2")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, stopLoss.Status)
	takeProfit, ok := result.Cache.OrderByClientID("backtest", "backtest-3")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, takeProfit.Status)
}

func TestBacktestDispatchesPositionLifecycleCallbacks(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &positionLifecycleBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	}
	engine.AddStrategy(strategy.NewTyped("position-lifecycle", impl))
	engine.AddData(bookEvent(
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("2")}},
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("2")}},
	))

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.True(t, impl.opened)
	require.True(t, impl.closed)
	position, ok := result.Cache.PositionByInstrument("backtest", impl.instrumentID)
	require.True(t, ok)
	require.Equal(t, model.PositionSideFlat, position.Side)
	require.True(t, position.Quantity.IsZero())
}

func TestBacktestModifyOrderUpdatesRestingOrderAndMatches(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &modifyBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	}
	engine.AddStrategy(strategy.NewTyped("modify", impl))
	engine.AddData(bookEvent(
		nil,
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
	))
	engine.AddData(Event{
		At:    time.Unix(11, 0),
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{OrderBook: &model.OrderBook{
			InstrumentID: impl.instrumentID,
			Asks: []model.OrderBookLevel{{
				Price: decimal.RequireFromString("101"),
				Size:  decimal.RequireFromString("1"),
			}},
			Timestamp: time.Unix(11, 0),
		}},
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.True(t, impl.pendingUpdate)
	require.True(t, impl.updated)
	order, ok := result.Cache.OrderByClientID("backtest", "modify-client")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.Equal(t, "101", order.Price.String())
	require.Equal(t, "101", order.AveragePrice.String())
}

func TestBacktestSubmitOrderDispatchesSubmittedAndAcceptedLifecycle(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &submitLifecycleBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	}
	engine.AddStrategy(strategy.NewTyped("submit-lifecycle", impl))
	engine.AddData(bookEvent(
		nil,
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
	))

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.True(t, impl.submittedLifecycle)
	require.True(t, impl.acceptedLifecycle)
	order, ok := result.Cache.OrderByClientID("backtest", "submit-lifecycle-client")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, order.Status)
}

func TestBacktestQueryOrderReturnsCachedOrderToStrategy(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &queryOrderBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	}
	engine.AddStrategy(strategy.NewTyped("query-order", impl))
	engine.AddData(bookEvent(
		nil,
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
	))

	_, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.True(t, impl.queried)
	require.Equal(t, model.OrderStatusAccepted, impl.queryStatus)
}

func TestBacktestQueryAccountReturnsSnapshotToStrategy(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &queryAccountBacktestStrategy{accountID: "backtest"}
	engine.AddStrategy(strategy.NewTyped("query-account", impl))
	engine.AddData(tickerEvent(decimal.RequireFromString("100")))

	_, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.True(t, impl.queried)
	require.Equal(t, model.AccountID("backtest"), impl.snapshot.AccountID)
}

func TestBacktestCancelOrderDispatchesPendingCancelAndCanceled(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &cancelBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	}
	engine.AddStrategy(strategy.NewTyped("cancel", impl))
	engine.AddData(bookEvent(
		nil,
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
	))
	engine.AddData(Event{
		At:    time.Unix(11, 0),
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{OrderBook: &model.OrderBook{
			InstrumentID: impl.instrumentID,
			Asks: []model.OrderBookLevel{{
				Price: decimal.RequireFromString("101"),
				Size:  decimal.RequireFromString("1"),
			}},
			Timestamp: time.Unix(11, 0),
		}},
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.True(t, impl.pendingCancel)
	require.True(t, impl.canceled)
	order, ok := result.Cache.OrderByClientID("backtest", "cancel-client")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, order.Status)
}

func TestBacktestExpiresGTDOrdersOnSimulatedClock(t *testing.T) {
	start := time.Unix(2_000, 0).UTC()
	expire := start.Add(30 * time.Second)
	engine := NewEngine(EngineConfig{})
	impl := &gtdExpiryBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		expire:       expire,
	}
	engine.AddStrategy(strategy.NewTyped("gtd-expiry", impl))
	engine.AddData(bookEventAt(
		start,
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("1")}},
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
	))
	engine.AddData(bookEventAt(
		start.Add(time.Minute),
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("98"), Size: decimal.RequireFromString("1")}},
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("99"), Size: decimal.RequireFromString("1")}},
	))

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "gtd-expire-client")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusExpired, order.Status)
	require.Equal(t, expire, order.LastUpdatedTime)
	require.True(t, impl.expired)
}

func TestBacktestCancelsUnfilledIOCBeforeLaterMarketData(t *testing.T) {
	start := time.Unix(2_100, 0).UTC()
	engine := NewEngine(EngineConfig{})
	impl := &tifBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		clientID:     "ioc-client",
		tif:          model.TimeInForceIOC,
		price:        decimal.RequireFromString("99"),
		quantity:     decimal.RequireFromString("1"),
	}
	engine.AddStrategy(strategy.NewTyped("ioc", impl))
	engine.AddData(bookEventAt(
		start,
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("1")}},
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
	))
	engine.AddData(bookEventAt(
		start.Add(time.Minute),
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("98"), Size: decimal.RequireFromString("1")}},
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("99"), Size: decimal.RequireFromString("1")}},
	))

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "ioc-client")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, order.Status)
	require.True(t, order.FilledQuantity.IsZero())
	require.Empty(t, result.Cache.FillsForOrder("backtest", order.OrderID))
	require.True(t, impl.canceledLifecycle)
}

func TestBacktestCancelsFOKWhenFullQuantityUnavailable(t *testing.T) {
	start := time.Unix(2_200, 0).UTC()
	engine := NewEngine(EngineConfig{})
	impl := &tifBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		clientID:     "fok-client",
		tif:          model.TimeInForceFOK,
		price:        decimal.RequireFromString("101"),
		quantity:     decimal.RequireFromString("2"),
	}
	engine.AddStrategy(strategy.NewTyped("fok", impl))
	engine.AddData(bookEventAt(
		start,
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("1")}},
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
	))

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	order, ok := result.Cache.OrderByClientID("backtest", "fok-client")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, order.Status)
	require.True(t, order.FilledQuantity.IsZero())
	require.True(t, order.LeavesQuantity.IsZero())
	require.Empty(t, result.Cache.FillsForOrder("backtest", order.OrderID))
	require.True(t, impl.canceledLifecycle)
}

func TestBacktestCancelAllOrdersCancelsOpenOrdersForInstrument(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &cancelAllBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	}
	engine.AddStrategy(strategy.NewTyped("cancel-all", impl))
	engine.AddData(bookEvent(
		nil,
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("2")}},
	))
	engine.AddData(Event{
		At:    time.Unix(11, 0),
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{OrderBook: &model.OrderBook{
			InstrumentID: impl.instrumentID,
			Asks: []model.OrderBookLevel{{
				Price: decimal.RequireFromString("101"),
				Size:  decimal.RequireFromString("2"),
			}},
			Timestamp: time.Unix(11, 0),
		}},
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Len(t, impl.canceledReports, 2)
	require.Empty(t, result.Cache.OpenOrders("backtest"))
	for _, clientOrderID := range []model.ClientOrderID{"cancel-all-1", "cancel-all-2"} {
		order, ok := result.Cache.OrderByClientID("backtest", clientOrderID)
		require.True(t, ok)
		require.Equal(t, model.OrderStatusCanceled, order.Status)
	}
}

func TestBacktestCancelAllOrdersFiltersByOrderSide(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	impl := &cancelAllBacktestStrategy{
		accountID:    "backtest",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		orderSides:   []model.OrderSide{model.OrderSideBuy, model.OrderSideSell},
		cancelSide:   model.OrderSideBuy,
	}
	engine.AddStrategy(strategy.NewTyped("cancel-all-side", impl))
	engine.AddData(bookEvent(
		nil,
		[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("2")}},
	))
	engine.AddData(Event{
		At:    time.Unix(11, 0),
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{OrderBook: &model.OrderBook{
			InstrumentID: impl.instrumentID,
			Asks: []model.OrderBookLevel{{
				Price: decimal.RequireFromString("101"),
				Size:  decimal.RequireFromString("2"),
			}},
			Timestamp: time.Unix(11, 0),
		}},
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Len(t, impl.canceledReports, 1)
	require.Equal(t, model.ClientOrderID("cancel-all-1"), impl.canceledReports[0].ClientOrderID)

	buy, ok := result.Cache.OrderByClientID("backtest", "cancel-all-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, buy.Status)

	sell, ok := result.Cache.OrderByClientID("backtest", "cancel-all-2")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, sell.Status)
}

type recordingStrategy struct {
	id      string
	events  []bus.Envelope
	stopped bool
}

func (s *recordingStrategy) ID() string                                      { return s.id }
func (s *recordingStrategy) OnStart(context.Context, strategy.Runtime) error { return nil }
func (s *recordingStrategy) OnEvent(_ context.Context, env bus.Envelope) error {
	s.events = append(s.events, env)
	return nil
}
func (s *recordingStrategy) OnStop(context.Context) error {
	s.stopped = true
	return nil
}

type existingOrderObservationStrategy struct {
	runtime                 strategy.Runtime
	eventsSeen              int
	statusSeenOnSecondEvent model.OrderStatus
}

func (s *existingOrderObservationStrategy) ID() string { return "existing-order-observer" }
func (s *existingOrderObservationStrategy) OnStart(_ context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return nil
}
func (s *existingOrderObservationStrategy) OnEvent(ctx context.Context, env bus.Envelope) error {
	if env.Topic != strategy.TopicMarketData {
		return nil
	}
	s.eventsSeen++
	switch s.eventsSeen {
	case 1:
		event := env.Message.(model.MarketEvent)
		_, err := s.runtime.SubmitOrder(ctx, model.SubmitOrder{
			AccountID:     "backtest",
			InstrumentID:  event.InstrumentID(),
			ClientOrderID: "existing-before-callback",
			Side:          model.OrderSideBuy,
			Type:          model.OrderTypeLimit,
			TimeInForce:   model.TimeInForceGTC,
			Quantity:      decimal.RequireFromString("1"),
			Price:         decimal.RequireFromString("99"),
		})
		return err
	case 2:
		order, ok := s.runtime.Cache().OrderByClientID("backtest", "existing-before-callback")
		if ok {
			s.statusSeenOnSecondEvent = order.Status
		}
	}
	return nil
}
func (s *existingOrderObservationStrategy) OnStop(context.Context) error { return nil }

type submittingStrategy struct {
	id         string
	runtime    strategy.Runtime
	side       model.OrderSide
	orderType  model.OrderType
	price      decimal.Decimal
	trigger    decimal.Decimal
	activation decimal.Decimal
	trailing   decimal.Decimal
	postOnly   bool
	stopped    bool
	done       bool
}

func (s *submittingStrategy) ID() string { return s.id }
func (s *submittingStrategy) OnStart(_ context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return nil
}
func (s *submittingStrategy) OnEvent(ctx context.Context, env bus.Envelope) error {
	if s.done || env.Topic != strategy.TopicMarketData {
		return nil
	}
	event, ok := env.Message.(model.MarketEvent)
	if !ok || event.Ticker == nil {
		return nil
	}
	s.done = true
	orderType := s.orderType
	if orderType == "" {
		orderType = model.OrderTypeMarket
	}
	side := s.side
	if side == "" {
		side = model.OrderSideBuy
	}
	_, err := s.runtime.SubmitOrder(ctx, model.SubmitOrder{
		AccountID:       "backtest",
		InstrumentID:    event.Ticker.InstrumentID,
		ClientOrderID:   "bt-client-1",
		Side:            side,
		Type:            orderType,
		TimeInForce:     model.TimeInForceGTC,
		Quantity:        decimal.RequireFromString("1"),
		Price:           s.price,
		TriggerPrice:    s.trigger,
		ActivationPrice: s.activation,
		TrailingOffset:  s.trailing,
		PostOnly:        s.postOnly,
	})
	return err
}
func (s *submittingStrategy) OnStop(context.Context) error {
	s.stopped = true
	return nil
}

func tickerEvent(last decimal.Decimal) Event {
	return tickerEventAt(time.Unix(10, 0), last)
}

func tickerEventAt(at time.Time, last decimal.Decimal) Event {
	return Event{
		At:    at,
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{Ticker: &model.Ticker{
			InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
			Last:         last,
			Bid:          last,
			Ask:          last,
			Timestamp:    at,
		}},
	}
}

type timerBacktestStrategy struct {
	runtime    strategy.Runtime
	startedAt  time.Time
	events     []string
	clockTimes []time.Time
	timerTimes []time.Time
}

func (s *timerBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	s.startedAt = rt.Clock().Now()
	return rt.SetTimer(ctx, "heartbeat", time.Minute)
}

func (s *timerBacktestStrategy) OnTicker(_ context.Context, _ model.Ticker) error {
	s.events = append(s.events, "ticker")
	s.clockTimes = append(s.clockTimes, s.runtime.Clock().Now())
	return nil
}

func (s *timerBacktestStrategy) OnTimer(_ context.Context, event strategy.TimerEvent) error {
	s.events = append(s.events, "timer:"+event.Name)
	now := s.runtime.Clock().Now()
	s.clockTimes = append(s.clockTimes, now)
	s.timerTimes = append(s.timerTimes, event.Timestamp)
	return nil
}

func tradeEvent(price decimal.Decimal, size decimal.Decimal) Event {
	return Event{
		At:    time.Unix(11, 0),
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{Trade: &model.TradeTick{
			InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
			Price:         price,
			Size:          size,
			AggressorSide: model.AggressorSideBuyer,
			TradeID:       "venue-trade-1",
			Timestamp:     time.Unix(11, 0),
			InitTime:      time.Unix(11, 0),
		}},
	}
}

func quoteEvent(bid decimal.Decimal, ask decimal.Decimal) Event {
	return Event{
		At:    time.Unix(12, 0),
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
			BidPrice:     bid,
			AskPrice:     ask,
			BidSize:      decimal.RequireFromString("1.5"),
			AskSize:      decimal.RequireFromString("2.5"),
			Timestamp:    time.Unix(12, 0),
			InitTime:     time.Unix(12, 0),
		}},
	}
}

func barEvent(barType model.BarType, close decimal.Decimal) Event {
	return Event{
		At:    time.Unix(12, 0),
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{Bar: &model.Bar{
			BarType:   barType,
			Open:      decimal.RequireFromString("100"),
			High:      decimal.RequireFromString("102"),
			Low:       decimal.RequireFromString("99"),
			Close:     close,
			Volume:    decimal.RequireFromString("12.5"),
			Timestamp: time.Unix(12, 0),
			InitTime:  time.Unix(12, 0),
		}},
	}
}

type bookSubmittingStrategy struct {
	id       string
	runtime  strategy.Runtime
	side     model.OrderSide
	orderTyp model.OrderType
	quantity decimal.Decimal
	price    decimal.Decimal
	done     bool
}

func (s *bookSubmittingStrategy) ID() string { return s.id }
func (s *bookSubmittingStrategy) OnStart(_ context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return nil
}
func (s *bookSubmittingStrategy) OnEvent(ctx context.Context, env bus.Envelope) error {
	if s.done || env.Topic != strategy.TopicMarketData {
		return nil
	}
	event, ok := env.Message.(model.MarketEvent)
	if !ok || event.OrderBook == nil {
		return nil
	}
	s.done = true
	_, err := s.runtime.SubmitOrder(ctx, model.SubmitOrder{
		AccountID:     "backtest",
		InstrumentID:  event.OrderBook.InstrumentID,
		ClientOrderID: "book-client-1",
		Side:          s.side,
		Type:          s.orderTyp,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      s.quantity,
		Price:         s.price,
	})
	return err
}
func (s *bookSubmittingStrategy) OnStop(context.Context) error { return nil }

type portfolioRuntimeStrategy struct {
	runtime      strategy.Runtime
	sawPortfolio bool
	exposure     decimal.Decimal
	submitted    bool
}

func (s *portfolioRuntimeStrategy) ID() string { return "portfolio-runtime" }
func (s *portfolioRuntimeStrategy) OnStart(_ context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return nil
}
func (s *portfolioRuntimeStrategy) OnEvent(ctx context.Context, env bus.Envelope) error {
	if s.submitted || env.Topic != strategy.TopicMarketData {
		return nil
	}
	event, ok := env.Message.(model.MarketEvent)
	if !ok || event.Ticker == nil {
		return nil
	}
	s.submitted = true
	pf := s.runtime.Portfolio()
	s.sawPortfolio = pf != nil
	if _, err := s.runtime.SubmitOrder(ctx, model.SubmitOrder{
		AccountID:     "backtest",
		InstrumentID:  event.Ticker.InstrumentID,
		ClientOrderID: "portfolio-runtime-client",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("1"),
	}); err != nil {
		return err
	}
	if pf != nil {
		s.exposure = pf.Exposure("backtest", "USDT")
	}
	return nil
}
func (s *portfolioRuntimeStrategy) OnStop(context.Context) error { return nil }

func bookEvent(bids []model.OrderBookLevel, asks []model.OrderBookLevel) Event {
	return bookEventAt(time.Unix(10, 0), bids, asks)
}

func bookEventAt(at time.Time, bids []model.OrderBookLevel, asks []model.OrderBookLevel) Event {
	return Event{
		At:    at,
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{OrderBook: &model.OrderBook{
			InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
			Bids:         bids,
			Asks:         asks,
			Timestamp:    at,
		}},
	}
}

type typedBacktestStrategy struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	runtime      strategy.Runtime
	filled       bool
}

func (s *typedBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *typedBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	order := s.runtime.OrderFactory(s.accountID).Limit(
		book.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("0.01"),
		book.Asks[0].Price,
		model.WithClientOrderID("bt-typed-client-1"),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *typedBacktestStrategy) OnOrderFilled(context.Context, model.FillReport) error {
	s.filled = true
	return nil
}

func (s *typedBacktestStrategy) wasFilled() bool { return s.filled }

type cascadingFillStrategy struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	runtime      strategy.Runtime
	submitted    bool
	hedged       bool
}

type bracketBacktestStrategy struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	runtime      strategy.Runtime
	submitted    bool
}

type positionLifecycleBacktestStrategy struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	runtime      strategy.Runtime
	submitted    bool
	opened       bool
	closed       bool
}

type modifyBacktestStrategy struct {
	accountID     model.AccountID
	instrumentID  model.InstrumentID
	runtime       strategy.Runtime
	submitted     bool
	modified      bool
	pendingUpdate bool
	updated       bool
}

type cancelBacktestStrategy struct {
	accountID     model.AccountID
	instrumentID  model.InstrumentID
	runtime       strategy.Runtime
	submitted     bool
	canceledOnce  bool
	pendingCancel bool
	canceled      bool
}

type gtdExpiryBacktestStrategy struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	expire       time.Time
	runtime      strategy.Runtime
	submitted    bool
	expired      bool
}

type tifBacktestStrategy struct {
	accountID         model.AccountID
	instrumentID      model.InstrumentID
	clientID          model.ClientOrderID
	tif               model.TimeInForce
	price             decimal.Decimal
	quantity          decimal.Decimal
	runtime           strategy.Runtime
	submitted         bool
	canceledLifecycle bool
}

type submitLifecycleBacktestStrategy struct {
	accountID          model.AccountID
	instrumentID       model.InstrumentID
	runtime            strategy.Runtime
	submitted          bool
	submittedLifecycle bool
	acceptedLifecycle  bool
}

type queryOrderBacktestStrategy struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	runtime      strategy.Runtime
	submitted    bool
	queried      bool
	queryStatus  model.OrderStatus
}

type queryAccountBacktestStrategy struct {
	accountID model.AccountID
	runtime   strategy.Runtime
	queried   bool
	snapshot  model.AccountSnapshot
}

type tradeSubmittingStrategy struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	runtime      strategy.Runtime
	seen         bool
}

type quoteSubmittingStrategy struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	runtime      strategy.Runtime
	seen         bool
}

type barSubmittingStrategy struct {
	accountID     model.AccountID
	barType       model.BarType
	side          model.OrderSide
	orderType     model.OrderType
	price         decimal.Decimal
	clientOrderID model.ClientOrderID
	runtime       strategy.Runtime
	seen          bool
}

type cancelAllBacktestStrategy struct {
	accountID       model.AccountID
	instrumentID    model.InstrumentID
	orderSides      []model.OrderSide
	cancelSide      model.OrderSide
	runtime         strategy.Runtime
	submitted       bool
	canceledOnce    bool
	canceledReports []model.OrderStatusReport
}

func (s *modifyBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *modifyBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if !s.submitted {
		s.submitted = true
		order := s.runtime.OrderFactory(s.accountID).Limit(
			book.InstrumentID,
			model.OrderSideBuy,
			decimal.RequireFromString("1"),
			decimal.RequireFromString("99"),
			model.WithClientOrderID("modify-client"),
		)
		_, err := s.runtime.SubmitOrder(ctx, order)
		return err
	}
	if s.modified {
		return nil
	}
	s.modified = true
	_, err := s.runtime.ModifyOrder(ctx, model.ModifyOrder{
		AccountID:     s.accountID,
		InstrumentID:  book.InstrumentID,
		ClientOrderID: "modify-client",
		Price:         decimal.RequireFromString("101"),
	})
	return err
}

func (s *modifyBacktestStrategy) OnOrderPendingUpdate(context.Context, model.OrderLifecycleEvent) error {
	s.pendingUpdate = true
	return nil
}

func (s *modifyBacktestStrategy) OnOrderUpdated(context.Context, model.OrderLifecycleEvent) error {
	s.updated = true
	return nil
}

func (s *cancelBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *cancelBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if !s.submitted {
		s.submitted = true
		order := s.runtime.OrderFactory(s.accountID).Limit(
			book.InstrumentID,
			model.OrderSideBuy,
			decimal.RequireFromString("1"),
			decimal.RequireFromString("99"),
			model.WithClientOrderID("cancel-client"),
		)
		_, err := s.runtime.SubmitOrder(ctx, order)
		return err
	}
	if s.canceledOnce {
		return nil
	}
	s.canceledOnce = true
	_, err := s.runtime.CancelOrder(ctx, model.CancelOrder{
		AccountID:     s.accountID,
		InstrumentID:  book.InstrumentID,
		ClientOrderID: "cancel-client",
	})
	return err
}

func (s *cancelBacktestStrategy) OnOrderPendingCancel(context.Context, model.OrderLifecycleEvent) error {
	s.pendingCancel = true
	return nil
}

func (s *cancelBacktestStrategy) OnOrderCanceled(context.Context, model.OrderStatusReport) error {
	s.canceled = true
	return nil
}

func (s *gtdExpiryBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *gtdExpiryBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if s.submitted {
		return nil
	}
	s.submitted = true
	order := s.runtime.OrderFactory(s.accountID).Limit(
		book.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("1"),
		decimal.RequireFromString("99"),
		model.WithClientOrderID("gtd-expire-client"),
		model.WithTimeInForce(model.TimeInForceGTD),
		model.WithExpireTime(s.expire),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *gtdExpiryBacktestStrategy) OnOrderExpired(context.Context, model.OrderLifecycleEvent) error {
	s.expired = true
	return nil
}

func (s *tifBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *tifBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if s.submitted {
		return nil
	}
	s.submitted = true
	order := s.runtime.OrderFactory(s.accountID).Limit(
		book.InstrumentID,
		model.OrderSideBuy,
		s.quantity,
		s.price,
		model.WithClientOrderID(s.clientID),
		model.WithTimeInForce(s.tif),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *tifBacktestStrategy) OnOrderLifecycle(_ context.Context, event model.OrderLifecycleEvent) error {
	if event.Kind == model.OrderEventCanceled {
		s.canceledLifecycle = true
	}
	return nil
}

func (s *submitLifecycleBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *submitLifecycleBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if s.submitted {
		return nil
	}
	s.submitted = true
	order := s.runtime.OrderFactory(s.accountID).Limit(
		book.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("1"),
		decimal.RequireFromString("99"),
		model.WithClientOrderID("submit-lifecycle-client"),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *submitLifecycleBacktestStrategy) OnOrderLifecycle(_ context.Context, event model.OrderLifecycleEvent) error {
	switch event.Kind {
	case model.OrderEventSubmitted:
		s.submittedLifecycle = true
	case model.OrderEventAccepted:
		s.acceptedLifecycle = true
	}
	return nil
}

func (s *queryOrderBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *queryOrderBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if s.submitted {
		return nil
	}
	s.submitted = true
	order := s.runtime.OrderFactory(s.accountID).Limit(
		book.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("1"),
		decimal.RequireFromString("99"),
		model.WithClientOrderID("query-order-client"),
	)
	if _, err := s.runtime.SubmitOrder(ctx, order); err != nil {
		return err
	}
	report, err := s.runtime.QueryOrder(ctx, model.QueryOrder{
		AccountID:     s.accountID,
		InstrumentID:  book.InstrumentID,
		ClientOrderID: "query-order-client",
	})
	if err != nil {
		return err
	}
	s.queried = true
	s.queryStatus = report.Status
	return nil
}

func (s *queryAccountBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	snapshot, err := rt.QueryAccount(ctx, model.QueryAccount{AccountID: s.accountID})
	if err != nil {
		return err
	}
	s.snapshot = snapshot
	s.queried = true
	return nil
}

func (s *tradeSubmittingStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeTradeTicks(ctx, s.instrumentID)
}

func (s *tradeSubmittingStrategy) OnTradeTick(ctx context.Context, tick model.TradeTick) error {
	if s.seen {
		return nil
	}
	s.seen = true
	order := s.runtime.OrderFactory(s.accountID).Market(
		tick.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("1"),
		model.WithClientOrderID("trade-client-1"),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *quoteSubmittingStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeQuoteTicks(ctx, s.instrumentID)
}

func (s *quoteSubmittingStrategy) OnQuoteTick(ctx context.Context, quote model.QuoteTick) error {
	if s.seen {
		return nil
	}
	s.seen = true
	order := s.runtime.OrderFactory(s.accountID).Market(
		quote.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("1"),
		model.WithClientOrderID("quote-client-1"),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *barSubmittingStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeBars(ctx, s.barType)
}

func (s *barSubmittingStrategy) OnBar(ctx context.Context, bar model.Bar) error {
	if s.seen {
		return nil
	}
	s.seen = true
	clientOrderID := s.clientOrderID
	if clientOrderID == "" {
		clientOrderID = "bar-client-1"
	}
	side := s.side
	if side == "" {
		side = model.OrderSideBuy
	}
	factory := s.runtime.OrderFactory(s.accountID)
	order := factory.Market(
		bar.BarType.InstrumentID,
		side,
		decimal.RequireFromString("1"),
		model.WithClientOrderID(clientOrderID),
	)
	if s.orderType == model.OrderTypeLimit {
		order = factory.Limit(
			bar.BarType.InstrumentID,
			side,
			decimal.RequireFromString("1"),
			s.price,
			model.WithClientOrderID(clientOrderID),
		)
	}
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *cancelAllBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *cancelAllBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if !s.submitted {
		s.submitted = true
		sides := s.orderSides
		if len(sides) == 0 {
			sides = []model.OrderSide{model.OrderSideBuy, model.OrderSideBuy}
		}
		for i, clientOrderID := range []model.ClientOrderID{"cancel-all-1", "cancel-all-2"} {
			side := sides[i]
			price := decimal.RequireFromString("99")
			if side == model.OrderSideSell {
				price = decimal.RequireFromString("102")
			}
			order := s.runtime.OrderFactory(s.accountID).Limit(
				book.InstrumentID,
				side,
				decimal.RequireFromString("1"),
				price,
				model.WithClientOrderID(clientOrderID),
			)
			if _, err := s.runtime.SubmitOrder(ctx, order); err != nil {
				return err
			}
		}
		return nil
	}
	if s.canceledOnce {
		return nil
	}
	s.canceledOnce = true
	_, err := s.runtime.CancelAllOrders(ctx, model.CancelAllOrders{
		AccountID:    s.accountID,
		InstrumentID: book.InstrumentID,
		OrderSide:    s.cancelSide,
	})
	return err
}

func (s *cancelAllBacktestStrategy) OnOrderCanceled(_ context.Context, report model.OrderStatusReport) error {
	s.canceledReports = append(s.canceledReports, report)
	return nil
}

func (s *positionLifecycleBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *positionLifecycleBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if s.submitted {
		return nil
	}
	s.submitted = true
	order := s.runtime.OrderFactory(s.accountID).Market(
		book.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("0.1"),
		model.WithClientOrderID("position-entry"),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *positionLifecycleBacktestStrategy) OnPositionOpened(ctx context.Context, event model.PositionLifecycleEvent) error {
	s.opened = true
	order := s.runtime.OrderFactory(s.accountID).Market(
		event.InstrumentID,
		model.OrderSideSell,
		event.Quantity,
		model.WithClientOrderID("position-exit"),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *positionLifecycleBacktestStrategy) OnPositionClosed(context.Context, model.PositionLifecycleEvent) error {
	s.closed = true
	return nil
}

func (s *bracketBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *bracketBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if s.submitted {
		return nil
	}
	s.submitted = true
	list := s.runtime.OrderFactory(s.accountID).Bracket(model.BracketOrderRequest{
		InstrumentID: book.InstrumentID,
		Side:         model.OrderSideBuy,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("101"),
		TakeProfit:   decimal.RequireFromString("103"),
		StopLoss:     decimal.RequireFromString("99"),
	})
	_, err := s.runtime.SubmitOrderList(ctx, list)
	return err
}

func (s *cascadingFillStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *cascadingFillStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if s.submitted {
		return nil
	}
	s.submitted = true
	order := s.runtime.OrderFactory(s.accountID).Market(
		book.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("0.1"),
		model.WithClientOrderID("cascade-entry"),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *cascadingFillStrategy) OnOrderFilled(ctx context.Context, fill model.FillReport) error {
	if fill.ClientOrderID != "cascade-entry" || s.hedged {
		return nil
	}
	order := s.runtime.OrderFactory(s.accountID).Market(
		fill.InstrumentID,
		model.OrderSideSell,
		fill.Quantity,
		model.WithClientOrderID("cascade-hedge"),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	if err == nil {
		s.hedged = true
	}
	return err
}
