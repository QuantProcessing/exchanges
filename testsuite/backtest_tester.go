package testsuite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/backtest"
	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/shopspring/decimal"
)

type BacktestTesterConfig struct {
	InstrumentID model.InstrumentID
}

type BacktestTester struct {
	cfg BacktestTesterConfig
}

func NewBacktestTester(cfg BacktestTesterConfig) *BacktestTester {
	return &BacktestTester{cfg: cfg}
}

func (b *BacktestTester) Run(ctx context.Context, t *testing.T) ContractReport {
	t.Helper()
	return runContractCases(t, "backtest", []contractCase{
		{id: "TC-B01", name: "Replay market data into strategy", run: func() error {
			instID := b.instrumentID()
			rec := &backtestRecordingStrategy{id: "bt-recorder"}
			result, err := backtest.NewRunner(backtest.Config{
				Cache:      backtestCache(instID),
				Events:     []backtest.Event{backtestTickerEvent(instID, time.Unix(10, 0), decimal.RequireFromString("100"))},
				Strategies: []strategy.Strategy{rec},
			}).Run(ctx)
			if err != nil {
				return err
			}
			if result.EventsProcessed != 1 || rec.marketEvents != 1 {
				return fmt.Errorf("market replay mismatch: processed=%d seen=%d", result.EventsProcessed, rec.marketEvents)
			}
			return nil
		}},
		{id: "TC-B02", name: "Match existing orders before strategy callback", run: func() error {
			instID := b.instrumentID()
			rec := &backtestExistingOrderStrategy{instrumentID: instID}
			result, err := backtest.NewRunner(backtest.Config{
				Cache: backtestCache(instID),
				Events: []backtest.Event{
					backtestTickerEvent(instID, time.Unix(10, 0), decimal.RequireFromString("100")),
					backtestBookEvent(instID, time.Unix(11, 0), nil, []model.OrderBookLevel{{
						Price: decimal.RequireFromString("98"),
						Size:  decimal.RequireFromString("1"),
					}}),
				},
				Strategies: []strategy.Strategy{rec},
			}).Run(ctx)
			if err != nil {
				return err
			}
			if rec.statusSeenOnSecondEvent != model.OrderStatusFilled {
				return fmt.Errorf("strategy saw %s before callback", rec.statusSeenOnSecondEvent)
			}
			order, ok := result.Cache.OrderByClientID("backtest", "tc-b02-limit")
			if !ok || order.Status != model.OrderStatusFilled {
				return fmt.Errorf("expected filled existing order, got %#v", order)
			}
			return nil
		}},
		{id: "TC-B03", name: "Market fills update portfolio", run: func() error {
			instID := b.instrumentID()
			trader := &backtestSubmitStrategy{
				id:            "bt-market",
				instrumentID:  instID,
				clientOrderID: "tc-b03-market",
				orderType:     model.OrderTypeMarket,
				quantity:      decimal.RequireFromString("1"),
			}
			result, err := backtest.NewRunner(backtest.Config{
				Cache:      backtestCache(instID),
				Events:     []backtest.Event{backtestTickerEvent(instID, time.Unix(10, 0), decimal.RequireFromString("100"))},
				Strategies: []strategy.Strategy{trader},
			}).Run(ctx)
			if err != nil {
				return err
			}
			order, ok := result.Cache.OrderByClientID("backtest", "tc-b03-market")
			if !ok || order.Status != model.OrderStatusFilled {
				return fmt.Errorf("expected filled market order, got %#v", order)
			}
			position, ok := result.Cache.PositionByInstrument("backtest", instID)
			if !ok || !position.Quantity.Equal(decimal.RequireFromString("1")) {
				return fmt.Errorf("position mismatch: %#v", position)
			}
			if got := result.Portfolio.Exposure("backtest", "USDT"); !got.Equal(decimal.RequireFromString("100")) {
				return fmt.Errorf("portfolio exposure mismatch: %s", got)
			}
			return nil
		}},
		{id: "TC-B04", name: "Order book liquidity consumption", run: func() error {
			instID := b.instrumentID()
			trader := &backtestSubmitStrategy{
				id:            "bt-book",
				instrumentID:  instID,
				clientOrderID: "tc-b04-book",
				orderType:     model.OrderTypeMarket,
				quantity:      decimal.RequireFromString("1"),
			}
			result, err := backtest.NewRunner(backtest.Config{
				Cache: backtestCache(instID),
				Events: []backtest.Event{backtestBookEvent(instID, time.Unix(10, 0), nil, []model.OrderBookLevel{
					{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("0.4")},
					{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("0.6")},
				})},
				Strategies: []strategy.Strategy{trader},
			}).Run(ctx)
			if err != nil {
				return err
			}
			order, ok := result.Cache.OrderByClientID("backtest", "tc-b04-book")
			if !ok || order.Status != model.OrderStatusFilled {
				return fmt.Errorf("expected filled book order, got %#v", order)
			}
			if !order.AveragePrice.Equal(decimal.RequireFromString("100.6")) {
				return fmt.Errorf("average price mismatch: %s", order.AveragePrice)
			}
			fills := result.Cache.FillsForOrder("backtest", order.OrderID)
			if len(fills) != 2 {
				return fmt.Errorf("expected two fills, got %d", len(fills))
			}
			return nil
		}},
		{id: "TC-B05", name: "Strategy command metadata propagation", run: func() error {
			instID := b.instrumentID()
			trader := &backtestSubmitStrategy{
				id:            "bt-metadata",
				instrumentID:  instID,
				clientOrderID: "tc-b05-metadata",
				orderType:     model.OrderTypeMarket,
				quantity:      decimal.RequireFromString("1"),
			}
			result, err := backtest.NewRunner(backtest.Config{
				Cache:      backtestCache(instID),
				Events:     []backtest.Event{backtestTickerEvent(instID, time.Unix(10, 0), decimal.RequireFromString("100"))},
				Strategies: []strategy.Strategy{trader},
			}).Run(ctx)
			if err != nil {
				return err
			}
			order, ok := result.Cache.OrderByClientID("backtest", "tc-b05-metadata")
			if !ok {
				return fmt.Errorf("metadata order not found")
			}
			if order.Metadata.StrategyID != "bt-metadata" || order.Metadata.TsInit.IsZero() {
				return fmt.Errorf("metadata mismatch: %#v", order.Metadata)
			}
			return nil
		}},
		{id: "TC-B06", name: "Reusable matching core", run: func() error {
			inst := model.Instrument{
				ID:        b.instrumentID(),
				RawSymbol: "BTCUSDT",
				Type:      model.InstrumentTypeSpot,
				Base:      "BTC",
				Quote:     "USDT",
				PriceTick: decimal.RequireFromString("0.01"),
				SizeTick:  decimal.RequireFromString("0.0001"),
				Status:    model.InstrumentStatusTrading,
			}
			core := backtest.NewMatchingCore(backtest.MatchingCoreConfig{
				Instrument: inst,
				FillModel:  backtest.DefaultFillModel(),
			})
			matches := core.MatchOrderBook(backtest.OrderBookMatchRequest{
				Order: model.OrderStatusReport{
					AccountID:      "backtest",
					InstrumentID:   inst.ID,
					OrderID:        "tc-b06-market",
					ClientOrderID:  "tc-b06-market-client",
					Status:         model.OrderStatusAccepted,
					Side:           model.OrderSideBuy,
					Type:           model.OrderTypeMarket,
					Quantity:       decimal.RequireFromString("1"),
					LeavesQuantity: decimal.RequireFromString("1"),
				},
				Book: model.OrderBook{
					InstrumentID: inst.ID,
					Asks: []model.OrderBookLevel{
						{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("0.4")},
						{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("0.8")},
					},
					Timestamp: time.Unix(10, 0),
				},
				Consumed: map[string]decimal.Decimal{"101": decimal.RequireFromString("0.2")},
			})
			if len(matches) != 2 {
				return fmt.Errorf("expected two core matches, got %d", len(matches))
			}
			if !matches[0].Price.Equal(decimal.RequireFromString("100")) || !matches[0].Quantity.Equal(decimal.RequireFromString("0.4")) {
				return fmt.Errorf("first match mismatch: %#v", matches[0])
			}
			if !matches[1].Price.Equal(decimal.RequireFromString("101")) || !matches[1].Quantity.Equal(decimal.RequireFromString("0.6")) {
				return fmt.Errorf("second match mismatch: %#v", matches[1])
			}
			core = backtest.NewMatchingCore(backtest.MatchingCoreConfig{
				Instrument: inst,
				FillModel:  backtestRejectLimitTouchFillModel{},
			})
			matches = core.MatchOrderBook(backtest.OrderBookMatchRequest{
				Order: model.OrderStatusReport{
					AccountID:      "backtest",
					InstrumentID:   inst.ID,
					OrderID:        "tc-b06-limit",
					ClientOrderID:  "tc-b06-limit-client",
					Status:         model.OrderStatusAccepted,
					Side:           model.OrderSideBuy,
					Type:           model.OrderTypeLimit,
					Quantity:       decimal.RequireFromString("1"),
					LeavesQuantity: decimal.RequireFromString("1"),
					Price:          decimal.RequireFromString("100"),
				},
				Book: model.OrderBook{
					InstrumentID: inst.ID,
					Asks: []model.OrderBookLevel{
						{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("0.4")},
						{Price: decimal.RequireFromString("99"), Size: decimal.RequireFromString("0.6")},
					},
					Timestamp: time.Unix(11, 0),
				},
			})
			if len(matches) != 1 || !matches[0].Price.Equal(decimal.RequireFromString("99")) {
				return fmt.Errorf("limit-touch fill model mismatch: %#v", matches)
			}
			return nil
		}},
		{id: "TC-B07", name: "Post-only limit rests before filling", run: func() error {
			instID := b.instrumentID()
			start := time.Unix(10, 0).UTC()
			trader := &backtestSubmitStrategy{
				id:            "bt-post-only",
				instrumentID:  instID,
				clientOrderID: "tc-b07-post-only",
				orderType:     model.OrderTypeLimit,
				quantity:      decimal.RequireFromString("1"),
				price:         decimal.RequireFromString("100"),
				postOnly:      true,
			}
			result, err := backtest.NewRunner(backtest.Config{
				Cache: backtestCache(instID),
				Events: []backtest.Event{
					backtestBookEvent(instID, start, nil, []model.OrderBookLevel{{
						Price: decimal.RequireFromString("100"),
						Size:  decimal.RequireFromString("1"),
					}}),
					backtestBookEvent(instID, start.Add(time.Second), nil, []model.OrderBookLevel{{
						Price: decimal.RequireFromString("100"),
						Size:  decimal.RequireFromString("1"),
					}}),
				},
				Strategies: []strategy.Strategy{trader},
			}).Run(ctx)
			if err != nil {
				return err
			}
			order, ok := result.Cache.OrderByClientID("backtest", "tc-b07-post-only")
			if !ok || order.Status != model.OrderStatusFilled {
				return fmt.Errorf("expected filled post-only order after resting, got %#v", order)
			}
			fills := result.Cache.FillsForOrder("backtest", order.OrderID)
			if len(fills) != 1 {
				return fmt.Errorf("expected one post-only fill, got %d", len(fills))
			}
			if !fills[0].Timestamp.Equal(start.Add(time.Second)) {
				return fmt.Errorf("post-only filled before resting: %s", fills[0].Timestamp)
			}
			return nil
		}},
		{id: "TC-B08", name: "Reduce-only cannot open a position", run: func() error {
			instID := b.instrumentID()
			trader := &backtestSubmitStrategy{
				id:            "bt-reduce-only",
				instrumentID:  instID,
				clientOrderID: "tc-b08-reduce-only",
				side:          model.OrderSideSell,
				orderType:     model.OrderTypeLimit,
				quantity:      decimal.RequireFromString("1"),
				price:         decimal.RequireFromString("100"),
				reduceOnly:    true,
			}
			result, err := backtest.NewRunner(backtest.Config{
				Cache: backtestCache(instID),
				Events: []backtest.Event{backtestBookEvent(instID, time.Unix(10, 0), []model.OrderBookLevel{{
					Price: decimal.RequireFromString("100"),
					Size:  decimal.RequireFromString("1"),
				}}, nil)},
				Strategies: []strategy.Strategy{trader},
			}).Run(ctx)
			if err != nil {
				return err
			}
			order, ok := result.Cache.OrderByClientID("backtest", "tc-b08-reduce-only")
			if !ok || order.Status != model.OrderStatusCanceled {
				return fmt.Errorf("expected canceled reduce-only order, got %#v", order)
			}
			if !order.FilledQuantity.IsZero() {
				return fmt.Errorf("reduce-only order filled without position: %s", order.FilledQuantity)
			}
			if fills := result.Cache.FillsForOrder("backtest", order.OrderID); len(fills) != 0 {
				return fmt.Errorf("expected no reduce-only fills, got %d", len(fills))
			}
			if position, ok := result.Cache.PositionByInstrument("backtest", instID); ok && position.Quantity.IsPositive() {
				return fmt.Errorf("reduce-only opened position: %#v", position)
			}
			return nil
		}},
		{id: "TC-B09", name: "Market-if-touched triggers on favorable touch", run: func() error {
			instID := b.instrumentID()
			trader := &backtestSubmitStrategy{
				id:            "bt-mit",
				instrumentID:  instID,
				clientOrderID: "tc-b09-mit",
				orderType:     model.OrderTypeMarketIfTouched,
				quantity:      decimal.RequireFromString("1"),
				trigger:       decimal.RequireFromString("99"),
			}
			result, err := backtest.NewRunner(backtest.Config{
				Cache: backtestCache(instID),
				Events: []backtest.Event{
					backtestTickerEvent(instID, time.Unix(10, 0), decimal.RequireFromString("101")),
					backtestTickerEvent(instID, time.Unix(11, 0), decimal.RequireFromString("99")),
				},
				Strategies: []strategy.Strategy{trader},
			}).Run(ctx)
			if err != nil {
				return err
			}
			order, ok := result.Cache.OrderByClientID("backtest", "tc-b09-mit")
			if !ok || order.Status != model.OrderStatusFilled {
				return fmt.Errorf("expected filled market-if-touched order, got %#v", order)
			}
			if !order.AveragePrice.Equal(decimal.RequireFromString("99")) {
				return fmt.Errorf("market-if-touched average price mismatch: %s", order.AveragePrice)
			}
			return nil
		}},
		{id: "TC-B10", name: "Limit-if-touched triggers then rests as limit", run: func() error {
			instID := b.instrumentID()
			trader := &backtestSubmitStrategy{
				id:            "bt-lit",
				instrumentID:  instID,
				clientOrderID: "tc-b10-lit",
				orderType:     model.OrderTypeLimitIfTouched,
				quantity:      decimal.RequireFromString("1"),
				price:         decimal.RequireFromString("99"),
				trigger:       decimal.RequireFromString("100"),
			}
			result, err := backtest.NewRunner(backtest.Config{
				Cache: backtestCache(instID),
				Events: []backtest.Event{
					backtestTickerEvent(instID, time.Unix(10, 0), decimal.RequireFromString("105")),
					backtestTickerEvent(instID, time.Unix(11, 0), decimal.RequireFromString("100")),
					backtestBookEvent(instID, time.Unix(12, 0), nil, []model.OrderBookLevel{{
						Price: decimal.RequireFromString("99"),
						Size:  decimal.RequireFromString("1"),
					}}),
				},
				Strategies: []strategy.Strategy{trader},
			}).Run(ctx)
			if err != nil {
				return err
			}
			order, ok := result.Cache.OrderByClientID("backtest", "tc-b10-lit")
			if !ok || order.Status != model.OrderStatusFilled {
				return fmt.Errorf("expected filled limit-if-touched order, got %#v", order)
			}
			if !order.AveragePrice.Equal(decimal.RequireFromString("99")) {
				return fmt.Errorf("limit-if-touched average price mismatch: %s", order.AveragePrice)
			}
			return nil
		}},
		{id: "TC-B11", name: "OUO partial fill reduces linked sibling quantity", run: func() error {
			instID := b.instrumentID()
			c := backtestCache(instID)
			if err := c.PutPosition(model.PositionStatusReport{
				AccountID:    "backtest",
				InstrumentID: instID,
				PositionID:   model.PositionID(instID.String()),
				Side:         model.PositionSideLong,
				Quantity:     decimal.RequireFromString("1"),
				EntryPrice:   decimal.RequireFromString("101"),
				Timestamp:    time.Unix(9, 0),
			}); err != nil {
				return err
			}
			trader := &backtestOUOListStrategy{instrumentID: instID}
			result, err := backtest.NewRunner(backtest.Config{
				Cache: c,
				Events: []backtest.Event{
					backtestBookEvent(instID, time.Unix(10, 0), []model.OrderBookLevel{{
						Price: decimal.RequireFromString("100"),
						Size:  decimal.RequireFromString("1"),
					}}, nil),
					backtestBookEvent(instID, time.Unix(11, 0), []model.OrderBookLevel{{
						Price: decimal.RequireFromString("103"),
						Size:  decimal.RequireFromString("0.4"),
					}}, nil),
				},
				Strategies: []strategy.Strategy{trader},
			}).Run(ctx)
			if err != nil {
				return err
			}
			firstExit, ok := result.Cache.OrderByClientID("backtest", "tc-b11-ouo-1")
			if !ok || firstExit.Status != model.OrderStatusPartiallyFilled {
				return fmt.Errorf("expected partially filled first OUO exit, got %#v", firstExit)
			}
			if !firstExit.FilledQuantity.Equal(decimal.RequireFromString("0.4")) {
				return fmt.Errorf("first OUO fill quantity mismatch: %s", firstExit.FilledQuantity)
			}
			peerExit, ok := result.Cache.OrderByClientID("backtest", "tc-b11-ouo-2")
			if !ok || peerExit.Status != model.OrderStatusAccepted {
				return fmt.Errorf("expected accepted peer OUO exit, got %#v", peerExit)
			}
			if !peerExit.Quantity.Equal(decimal.RequireFromString("0.6")) || !peerExit.LeavesQuantity.Equal(decimal.RequireFromString("0.6")) {
				return fmt.Errorf("peer OUO quantity mismatch: quantity=%s leaves=%s", peerExit.Quantity, peerExit.LeavesQuantity)
			}
			return nil
		}},
		{id: "TC-B12", name: "OTO child releases and resizes on partial parent fills", run: func() error {
			instID := b.instrumentID()
			trader := &backtestOTOPartialListStrategy{instrumentID: instID}
			result, err := backtest.NewRunner(backtest.Config{
				Cache: backtestCache(instID),
				Events: []backtest.Event{
					backtestBookEvent(instID, time.Unix(10, 0),
						[]model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("1")}},
						[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("0.4")}},
					),
					backtestBookEvent(instID, time.Unix(11, 0),
						[]model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("1")}},
						[]model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("0.6")}},
					),
				},
				Strategies: []strategy.Strategy{trader},
			}).Run(ctx)
			if err != nil {
				return err
			}
			if !trader.childQuantityAfterFirstFill.Equal(decimal.RequireFromString("0.4")) {
				return fmt.Errorf("first OTO child quantity mismatch: %s", trader.childQuantityAfterFirstFill)
			}
			if !trader.childQuantityAfterSecondFill.Equal(decimal.RequireFromString("1")) {
				return fmt.Errorf("second OTO child quantity mismatch: %s", trader.childQuantityAfterSecondFill)
			}
			parent, ok := result.Cache.OrderByClientID("backtest", "tc-b12-oto-parent")
			if !ok || parent.Status != model.OrderStatusFilled {
				return fmt.Errorf("expected filled OTO parent, got %#v", parent)
			}
			child, ok := result.Cache.OrderByClientID("backtest", "tc-b12-oto-child")
			if !ok || child.Status != model.OrderStatusAccepted {
				return fmt.Errorf("expected accepted OTO child, got %#v", child)
			}
			if !child.Quantity.Equal(decimal.RequireFromString("1")) || !child.LeavesQuantity.Equal(decimal.RequireFromString("1")) {
				return fmt.Errorf("OTO child final quantity mismatch: quantity=%s leaves=%s", child.Quantity, child.LeavesQuantity)
			}
			return nil
		}},
		{id: "TC-B13", name: "Deterministic result summary JSON", run: func() error {
			instID := b.instrumentID()
			run := func() (backtest.Result, []byte, error) {
				result, err := backtest.NewRunner(backtest.Config{
					Cache: backtestCache(instID),
					Events: []backtest.Event{
						backtestTickerEvent(instID, time.Unix(10, 0), decimal.RequireFromString("100")),
					},
					Strategies: []strategy.Strategy{&backtestSubmitStrategy{
						id:            "bt-summary",
						instrumentID:  instID,
						clientOrderID: "tc-b13-summary",
						orderType:     model.OrderTypeMarket,
						quantity:      decimal.RequireFromString("1"),
					}},
				}).Run(ctx)
				if err != nil {
					return result, nil, err
				}
				payload, err := result.DeterministicJSON("backtest")
				return result, payload, err
			}
			firstResult, firstPayload, err := run()
			if err != nil {
				return err
			}
			_, secondPayload, err := run()
			if err != nil {
				return err
			}
			if string(firstPayload) != string(secondPayload) {
				return fmt.Errorf("deterministic summaries differ:\n%s\n%s", firstPayload, secondPayload)
			}
			summary := firstResult.Summary("backtest")
			if summary.EventsProcessed != 1 || len(summary.Accounts) != 1 {
				return fmt.Errorf("summary envelope mismatch: %#v", summary)
			}
			account := summary.Accounts[0]
			if len(account.Orders) != 1 || len(account.Fills) != 1 || len(account.Positions) != 1 {
				return fmt.Errorf("summary content mismatch: orders=%d fills=%d positions=%d", len(account.Orders), len(account.Fills), len(account.Positions))
			}
			return nil
		}},
		{id: "TC-B14", name: "Multi-account result summary defaults", run: func() error {
			instID := b.instrumentID()
			result, err := backtest.NewRunner(backtest.Config{
				Cache: backtestCache(instID),
				Events: []backtest.Event{
					backtestTickerEvent(instID, time.Unix(10, 0), decimal.RequireFromString("100")),
				},
				Strategies: []strategy.Strategy{&backtestMultiAccountSubmitStrategy{instrumentID: instID}},
			}).Run(ctx)
			if err != nil {
				return err
			}
			summary := result.Summary()
			if len(summary.Accounts) != 2 {
				return fmt.Errorf("expected two account summaries, got %d", len(summary.Accounts))
			}
			if summary.Accounts[0].AccountID != "tc-b14-a" || summary.Accounts[1].AccountID != "tc-b14-b" {
				return fmt.Errorf("account summary order mismatch: %#v", summary.Accounts)
			}
			for _, account := range summary.Accounts {
				if len(account.Orders) != 1 || len(account.Fills) != 1 || len(account.Positions) != 1 {
					return fmt.Errorf("account %s summary mismatch: orders=%d fills=%d positions=%d", account.AccountID, len(account.Orders), len(account.Fills), len(account.Positions))
				}
			}
			return nil
		}},
		{id: "TC-B15", name: "Catalog-backed engine run", run: func() error {
			instID := b.instrumentID()
			catalog := backtest.NewMemoryCatalog(backtestTickerEvent(instID, time.Unix(10, 0), decimal.RequireFromString("100")))
			engine := backtest.NewEngine(backtest.EngineConfig{Catalog: catalog})
			engine.AddStrategy(&backtestSubmitStrategy{
				id:            "bt-catalog",
				instrumentID:  instID,
				clientOrderID: "tc-b15-catalog",
				orderType:     model.OrderTypeMarket,
				quantity:      decimal.RequireFromString("1"),
			})
			result, err := engine.Run(ctx)
			if err != nil {
				return err
			}
			if result.EventsProcessed != 1 {
				return fmt.Errorf("expected one catalog event, got %d", result.EventsProcessed)
			}
			order, ok := result.Cache.OrderByClientID("backtest", "tc-b15-catalog")
			if !ok || order.Status != model.OrderStatusFilled {
				return fmt.Errorf("expected catalog-backed order fill, got %#v", order)
			}
			return nil
		}},
		{id: "TC-B16", name: "Multi-strategy run preserves strategy metadata", run: func() error {
			instID := b.instrumentID()
			result, err := backtest.NewRunner(backtest.Config{
				Cache: backtestCache(instID),
				Events: []backtest.Event{
					backtestTickerEvent(instID, time.Unix(10, 0), decimal.RequireFromString("100")),
				},
				Strategies: []strategy.Strategy{
					&backtestSubmitStrategy{
						id:            "tc-b16-alpha",
						accountID:     "tc-b16-alpha",
						instrumentID:  instID,
						clientOrderID: "tc-b16-alpha-order",
						orderType:     model.OrderTypeMarket,
						quantity:      decimal.RequireFromString("1"),
					},
					&backtestSubmitStrategy{
						id:            "tc-b16-beta",
						accountID:     "tc-b16-beta",
						instrumentID:  instID,
						clientOrderID: "tc-b16-beta-order",
						orderType:     model.OrderTypeMarket,
						quantity:      decimal.RequireFromString("1"),
					},
				},
			}).Run(ctx)
			if err != nil {
				return err
			}
			summary := result.Summary()
			if len(summary.Accounts) != 2 {
				return fmt.Errorf("expected two strategy accounts, got %d", len(summary.Accounts))
			}
			if summary.Accounts[0].Orders[0].Metadata.StrategyID != "tc-b16-alpha" ||
				summary.Accounts[1].Orders[0].Metadata.StrategyID != "tc-b16-beta" {
				return fmt.Errorf("strategy metadata mismatch: %#v", summary.Accounts)
			}
			return nil
		}},
	})
}

func (b *BacktestTester) instrumentID() model.InstrumentID {
	if b.cfg.InstrumentID != (model.InstrumentID{}) {
		return b.cfg.InstrumentID
	}
	return model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
}

func backtestCache(instID model.InstrumentID) *cache.Cache {
	c := cache.New()
	_ = c.PutInstrument(model.Instrument{
		ID:        instID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	})
	return c
}

func backtestTickerEvent(instID model.InstrumentID, at time.Time, price decimal.Decimal) backtest.Event {
	return backtest.Event{
		At:    at,
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{Ticker: &model.Ticker{
			InstrumentID: instID,
			Last:         price,
			Timestamp:    at,
		}},
	}
}

func backtestBookEvent(instID model.InstrumentID, at time.Time, bids []model.OrderBookLevel, asks []model.OrderBookLevel) backtest.Event {
	return backtest.Event{
		At:    at,
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{OrderBook: &model.OrderBook{
			InstrumentID: instID,
			Bids:         bids,
			Asks:         asks,
			Timestamp:    at,
		}},
	}
}

type backtestRecordingStrategy struct {
	id           string
	marketEvents int
}

func (s *backtestRecordingStrategy) ID() string { return s.id }
func (s *backtestRecordingStrategy) OnStart(context.Context, strategy.Runtime) error {
	return nil
}
func (s *backtestRecordingStrategy) OnEvent(_ context.Context, env bus.Envelope) error {
	if env.Topic == strategy.TopicMarketData {
		s.marketEvents++
	}
	return nil
}
func (s *backtestRecordingStrategy) OnStop(context.Context) error { return nil }

type backtestSubmitStrategy struct {
	id            string
	accountID     model.AccountID
	instrumentID  model.InstrumentID
	clientOrderID model.ClientOrderID
	side          model.OrderSide
	orderType     model.OrderType
	quantity      decimal.Decimal
	price         decimal.Decimal
	trigger       decimal.Decimal
	postOnly      bool
	reduceOnly    bool
	submitted     bool
	runtime       strategy.Runtime
}

func (s *backtestSubmitStrategy) ID() string { return s.id }
func (s *backtestSubmitStrategy) OnStart(_ context.Context, runtime strategy.Runtime) error {
	s.runtime = runtime
	return nil
}
func (s *backtestSubmitStrategy) OnEvent(ctx context.Context, env bus.Envelope) error {
	if s.submitted || env.Topic != strategy.TopicMarketData {
		return nil
	}
	s.submitted = true
	orderType := s.orderType
	if orderType == "" {
		orderType = model.OrderTypeMarket
	}
	quantity := s.quantity
	if !quantity.IsPositive() {
		quantity = decimal.RequireFromString("1")
	}
	side := s.side
	if side == "" {
		side = model.OrderSideBuy
	}
	accountID := s.accountID
	if accountID == "" {
		accountID = "backtest"
	}
	_, err := s.runtime.SubmitOrder(ctx, model.SubmitOrder{
		AccountID:     accountID,
		InstrumentID:  s.instrumentID,
		ClientOrderID: s.clientOrderID,
		Side:          side,
		Type:          orderType,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      quantity,
		Price:         s.price,
		TriggerPrice:  s.trigger,
		PostOnly:      s.postOnly,
		ReduceOnly:    s.reduceOnly,
	})
	return err
}
func (s *backtestSubmitStrategy) OnStop(context.Context) error { return nil }

type backtestExistingOrderStrategy struct {
	instrumentID            model.InstrumentID
	events                  int
	statusSeenOnSecondEvent model.OrderStatus
	runtime                 strategy.Runtime
}

func (s *backtestExistingOrderStrategy) ID() string { return "bt-existing-order" }
func (s *backtestExistingOrderStrategy) OnStart(_ context.Context, runtime strategy.Runtime) error {
	s.runtime = runtime
	return nil
}
func (s *backtestExistingOrderStrategy) OnEvent(ctx context.Context, env bus.Envelope) error {
	if env.Topic != strategy.TopicMarketData {
		return nil
	}
	s.events++
	switch s.events {
	case 1:
		_, err := s.runtime.SubmitOrder(ctx, model.SubmitOrder{
			AccountID:     "backtest",
			InstrumentID:  s.instrumentID,
			ClientOrderID: "tc-b02-limit",
			Side:          model.OrderSideBuy,
			Type:          model.OrderTypeLimit,
			TimeInForce:   model.TimeInForceGTC,
			Quantity:      decimal.RequireFromString("1"),
			Price:         decimal.RequireFromString("99"),
		})
		return err
	case 2:
		order, ok := s.runtime.Cache().OrderByClientID("backtest", "tc-b02-limit")
		if !ok {
			return fmt.Errorf("existing order not found")
		}
		s.statusSeenOnSecondEvent = order.Status
	}
	return nil
}
func (s *backtestExistingOrderStrategy) OnStop(context.Context) error { return nil }

type backtestOUOListStrategy struct {
	instrumentID model.InstrumentID
	submitted    bool
	runtime      strategy.Runtime
}

func (s *backtestOUOListStrategy) ID() string { return "bt-ouo-list" }
func (s *backtestOUOListStrategy) OnStart(_ context.Context, runtime strategy.Runtime) error {
	s.runtime = runtime
	return nil
}
func (s *backtestOUOListStrategy) OnEvent(ctx context.Context, env bus.Envelope) error {
	if s.submitted || env.Topic != strategy.TopicMarketData {
		return nil
	}
	s.submitted = true
	listID := model.OrderListID("tc-b11-ouo-list")
	_, err := s.runtime.SubmitOrderList(ctx, model.OrderList{
		ID: listID,
		Orders: []model.SubmitOrder{
			{
				AccountID:     "backtest",
				InstrumentID:  s.instrumentID,
				ClientOrderID: "tc-b11-ouo-1",
				OrderListID:   listID,
				Side:          model.OrderSideSell,
				Type:          model.OrderTypeLimit,
				Contingency:   model.ContingencyTypeOUO,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("103"),
				ReduceOnly:    true,
			},
			{
				AccountID:     "backtest",
				InstrumentID:  s.instrumentID,
				ClientOrderID: "tc-b11-ouo-2",
				OrderListID:   listID,
				Side:          model.OrderSideSell,
				Type:          model.OrderTypeLimit,
				Contingency:   model.ContingencyTypeOUO,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("104"),
				ReduceOnly:    true,
			},
		},
	})
	return err
}
func (s *backtestOUOListStrategy) OnStop(context.Context) error { return nil }

type backtestOTOPartialListStrategy struct {
	instrumentID                 model.InstrumentID
	events                       int
	childQuantityAfterFirstFill  decimal.Decimal
	childQuantityAfterSecondFill decimal.Decimal
	runtime                      strategy.Runtime
}

func (s *backtestOTOPartialListStrategy) ID() string { return "bt-oto-partial-list" }
func (s *backtestOTOPartialListStrategy) OnStart(_ context.Context, runtime strategy.Runtime) error {
	s.runtime = runtime
	return nil
}
func (s *backtestOTOPartialListStrategy) OnEvent(ctx context.Context, env bus.Envelope) error {
	if env.Topic != strategy.TopicMarketData {
		return nil
	}
	s.events++
	if s.events == 1 {
		listID := model.OrderListID("tc-b12-oto-list")
		_, err := s.runtime.SubmitOrderList(ctx, model.OrderList{
			ID: listID,
			Orders: []model.SubmitOrder{
				{
					AccountID:     "backtest",
					InstrumentID:  s.instrumentID,
					ClientOrderID: "tc-b12-oto-parent",
					OrderListID:   listID,
					Side:          model.OrderSideBuy,
					Type:          model.OrderTypeLimit,
					Contingency:   model.ContingencyTypeOTO,
					TimeInForce:   model.TimeInForceGTC,
					Quantity:      decimal.RequireFromString("1"),
					Price:         decimal.RequireFromString("101"),
				},
				{
					AccountID:           "backtest",
					InstrumentID:        s.instrumentID,
					ClientOrderID:       "tc-b12-oto-child",
					ParentClientOrderID: "tc-b12-oto-parent",
					OrderListID:         listID,
					Side:                model.OrderSideSell,
					Type:                model.OrderTypeLimit,
					Contingency:         model.ContingencyTypeOTO,
					TimeInForce:         model.TimeInForceGTC,
					Quantity:            decimal.RequireFromString("1"),
					Price:               decimal.RequireFromString("110"),
					ReduceOnly:          true,
				},
			},
		})
		if err != nil {
			return err
		}
		if child, ok := s.runtime.Cache().OrderByClientID("backtest", "tc-b12-oto-child"); ok {
			s.childQuantityAfterFirstFill = child.Quantity
		}
		return nil
	}
	if child, ok := s.runtime.Cache().OrderByClientID("backtest", "tc-b12-oto-child"); ok {
		s.childQuantityAfterSecondFill = child.Quantity
	}
	return nil
}
func (s *backtestOTOPartialListStrategy) OnStop(context.Context) error { return nil }

type backtestMultiAccountSubmitStrategy struct {
	instrumentID model.InstrumentID
	submitted    bool
	runtime      strategy.Runtime
}

func (s *backtestMultiAccountSubmitStrategy) ID() string { return "bt-multi-account-submit" }
func (s *backtestMultiAccountSubmitStrategy) OnStart(_ context.Context, runtime strategy.Runtime) error {
	s.runtime = runtime
	return nil
}
func (s *backtestMultiAccountSubmitStrategy) OnEvent(ctx context.Context, env bus.Envelope) error {
	if s.submitted || env.Topic != strategy.TopicMarketData {
		return nil
	}
	s.submitted = true
	for _, accountID := range []model.AccountID{"tc-b14-a", "tc-b14-b"} {
		_, err := s.runtime.SubmitOrder(ctx, model.SubmitOrder{
			AccountID:     accountID,
			InstrumentID:  s.instrumentID,
			ClientOrderID: model.ClientOrderID(string(accountID) + "-order"),
			Side:          model.OrderSideBuy,
			Type:          model.OrderTypeMarket,
			TimeInForce:   model.TimeInForceGTC,
			Quantity:      decimal.RequireFromString("1"),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *backtestMultiAccountSubmitStrategy) OnStop(context.Context) error { return nil }

type backtestRejectLimitTouchFillModel struct{}

func (backtestRejectLimitTouchFillModel) ShouldFillLimitTouch(ctx backtest.FillContext) bool {
	return !ctx.LimitTouch
}

func (backtestRejectLimitTouchFillModel) ApplySlippage(_ backtest.FillContext, price decimal.Decimal) decimal.Decimal {
	return price
}
