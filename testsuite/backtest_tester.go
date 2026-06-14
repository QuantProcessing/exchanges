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
	instrumentID  model.InstrumentID
	clientOrderID model.ClientOrderID
	orderType     model.OrderType
	quantity      decimal.Decimal
	price         decimal.Decimal
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
	_, err := s.runtime.SubmitOrder(ctx, model.SubmitOrder{
		AccountID:     "backtest",
		InstrumentID:  s.instrumentID,
		ClientOrderID: s.clientOrderID,
		Side:          model.OrderSideBuy,
		Type:          orderType,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      quantity,
		Price:         s.price,
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
