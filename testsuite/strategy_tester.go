package testsuite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/shopspring/decimal"
)

type StrategyTesterConfig struct{}

type StrategyTester struct {
	cfg StrategyTesterConfig
}

func NewStrategyTester(cfg StrategyTesterConfig) *StrategyTester {
	return &StrategyTester{cfg: cfg}
}

func (s *StrategyTester) Run(ctx context.Context, t *testing.T) ContractReport {
	t.Helper()
	return runContractCases(t, "strategy", []contractCase{
		{id: "TC-S01", name: "Typed market callbacks", run: func() error {
			impl := &strategyCallbackProbe{}
			wrapped := strategy.NewTyped("market", impl)
			instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
			if err := wrapped.OnEvent(ctx, bus.Envelope{Topic: strategy.TopicMarketData, Message: model.MarketEvent{Ticker: &model.Ticker{
				InstrumentID: instID,
				Last:         decimal.RequireFromString("100"),
				Timestamp:    time.Unix(1, 0),
			}}}); err != nil {
				return err
			}
			if err := wrapped.OnEvent(ctx, bus.Envelope{Topic: strategy.TopicMarketData, Message: model.MarketEvent{Custom: &model.CustomData{
				InstrumentID: instID,
				Type:         "funding_rate",
				Fields:       map[string]string{"rate": "0.0001"},
				Timestamp:    time.Unix(1, 0),
			}}}); err != nil {
				return err
			}
			if impl.tickers != 1 || impl.customData != 1 {
				return fmt.Errorf("market callback counts mismatch: %#v", impl)
			}
			return nil
		}},
		{id: "TC-S02", name: "Typed execution callbacks", run: func() error {
			impl := &strategyCallbackProbe{}
			wrapped := strategy.NewTyped("execution", impl)
			instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
			events := []model.ExecutionEvent{
				{Account: &model.AccountSnapshot{AccountID: "acct", Venue: "BINANCE"}},
				{Order: &model.OrderStatusReport{AccountID: "acct", InstrumentID: instID, OrderID: "order-1", Status: model.OrderStatusAccepted}},
				{Lifecycle: &model.OrderLifecycleEvent{AccountID: "acct", InstrumentID: instID, OrderID: "order-1", Kind: model.OrderEventFilled, Status: model.OrderStatusFilled}},
				{Fill: &model.FillReport{AccountID: "acct", InstrumentID: instID, OrderID: "order-1", TradeID: "trade-1", Quantity: decimal.RequireFromString("1")}},
				{Position: &model.PositionStatusReport{AccountID: "acct", InstrumentID: instID, PositionID: "pos-1", Side: model.PositionSideLong, Quantity: decimal.RequireFromString("1")}},
			}
			for i := range events {
				if err := wrapped.OnEvent(ctx, bus.Envelope{Topic: strategy.TopicExecution, Message: events[i]}); err != nil {
					return err
				}
			}
			if impl.accounts != 1 || impl.orders != 1 || impl.orderLifecycle != 1 || impl.fills != 1 || impl.positions != 1 {
				return fmt.Errorf("execution callback counts mismatch: %#v", impl)
			}
			return nil
		}},
		{id: "TC-S03", name: "Typed timer callbacks", run: func() error {
			impl := &strategyCallbackProbe{}
			wrapped := strategy.NewTyped("timer", impl)
			if err := wrapped.OnEvent(ctx, bus.Envelope{Topic: strategy.TopicTimer, Message: strategy.TimerEvent{Name: "heartbeat", Timestamp: time.Unix(2, 0)}}); err != nil {
				return err
			}
			if impl.timers != 1 {
				return errCase("timer callback was not dispatched")
			}
			return nil
		}},
		{id: "TC-S04", name: "Typed error callbacks", run: func() error {
			impl := &strategyCallbackProbe{}
			wrapped := strategy.NewTyped("errors", impl)
			cause := errors.New("risk denied order")
			if err := wrapped.OnEvent(ctx, bus.Envelope{Topic: strategy.TopicError, Message: strategy.ErrorEvent{Source: "risk", Err: cause}}); err != nil {
				return err
			}
			if impl.errors != 1 || !errors.Is(impl.lastError, cause) {
				return errCase("error callback was not dispatched with the original cause")
			}
			return nil
		}},
		{id: "TC-S05", name: "Async engine errors are observable", run: func() error {
			b := bus.New()
			engine := strategy.NewEngine(b)
			requireErr := errors.New("strategy handler failed")
			if err := engine.Add(strategy.NewTyped("failing", strategyFailingHandler{err: requireErr})); err != nil {
				return err
			}
			if err := engine.Start(ctx); err != nil {
				return err
			}
			defer engine.Stop(context.Background())
			if err := b.Publish(ctx, strategy.TopicTimer, strategy.TimerEvent{Name: "fatal", Timestamp: time.Unix(3, 0)}); err != nil {
				return err
			}
			select {
			case err := <-engine.Errors():
				if !errors.Is(err, requireErr) {
					return fmt.Errorf("engine error mismatch: %v", err)
				}
				return nil
			case <-time.After(time.Second):
				return errCase("engine did not surface async strategy error")
			}
		}},
		{id: "TC-S09", name: "Strategy actor faults do not skip peers", run: func() error {
			b := bus.New()
			requireErr := errors.New("strategy actor failed")
			healthy := &strategyTimerSignal{seen: make(chan struct{}, 1)}
			engine := strategy.NewEngine(b)
			if err := engine.Add(strategy.NewTyped("failing-actor", strategyFailingHandler{err: requireErr})); err != nil {
				return err
			}
			if err := engine.Add(strategy.NewTyped("healthy-actor", healthy)); err != nil {
				return err
			}
			if err := engine.Start(ctx); err != nil {
				return err
			}
			defer engine.Stop(context.Background())
			if err := b.Publish(ctx, strategy.TopicTimer, strategy.TimerEvent{Name: "actor-isolation", Timestamp: time.Unix(4, 0)}); err != nil {
				return err
			}
			var sawError bool
			var sawPeer bool
			deadline := time.After(time.Second)
			for !sawError || !sawPeer {
				select {
				case err := <-engine.Errors():
					if !errors.Is(err, requireErr) {
						return fmt.Errorf("engine error mismatch: %v", err)
					}
					sawError = true
				case <-healthy.seen:
					sawPeer = true
				case <-deadline:
					return fmt.Errorf("actor isolation did not surface both error=%t and peer=%t", sawError, sawPeer)
				}
			}
			return nil
		}},
		{id: "TC-S06", name: "Strategy config validates and freezes runtime identity", run: func() error {
			wrapped, err := strategy.NewTypedWithConfig(strategy.StrategyConfig{ID: "configured"}, &strategyCallbackProbe{})
			if err != nil {
				return err
			}
			if wrapped.ID() != "configured" {
				return errCase("configured strategy id was not applied")
			}
			if _, err := strategy.NewTypedWithConfig(strategy.StrategyConfig{}, &strategyCallbackProbe{}); err == nil {
				return errCase("empty strategy id should be rejected")
			}
			rt := &strategyRuntimeProbe{cache: cache.New()}
			mutable := &strategyMutableIDProbe{id: "tc-s06"}
			engine := strategy.NewEngine(bus.New(), strategy.WithRuntime(rt), strategy.WithTraderID("TRADER-001"))
			if err := engine.Add(mutable); err != nil {
				return err
			}
			mutable.id = "mutated"
			if err := engine.Start(ctx); err != nil {
				return err
			}
			defer engine.Stop(context.Background())
			if _, err := mutable.runtime.SubmitOrder(ctx, model.SubmitOrder{
				AccountID:     "acct",
				InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
				ClientOrderID: "tc-s06-client",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeMarket,
				Quantity:      decimal.RequireFromString("1"),
			}); err != nil {
				return err
			}
			if rt.lastSubmit.Metadata.TraderID != "TRADER-001" || rt.lastSubmit.Metadata.StrategyID != "tc-s06" {
				return fmt.Errorf("runtime identity was not frozen: %#v", rt.lastSubmit.Metadata)
			}
			if err := engine.Add(&strategyMutableIDProbe{id: "tc-s06"}); err == nil {
				return errCase("duplicate strategy id should be rejected")
			}
			return nil
		}},
		{id: "TC-S07", name: "Built-in indicators initialize and update", run: func() error {
			ema, err := strategy.NewExponentialMovingAverage(3)
			if err != nil {
				return err
			}
			for _, value := range []string{"10", "12", "14", "16"} {
				if err := ema.Update(decimal.RequireFromString(value)); err != nil {
					return err
				}
			}
			if !ema.Initialized() || ema.Count() != 4 || !ema.Value().Equal(decimal.RequireFromString("14.25")) {
				return fmt.Errorf("EMA parity mismatch: initialized=%t count=%d value=%s", ema.Initialized(), ema.Count(), ema.Value())
			}

			atr, err := strategy.NewAverageTrueRange(3)
			if err != nil {
				return err
			}
			for _, bar := range []model.Bar{
				strategyTesterBar("10", "8", "9"),
				strategyTesterBar("12", "9", "11"),
				strategyTesterBar("13", "10", "12"),
				strategyTesterBar("14", "11", "13"),
			} {
				if err := atr.UpdateBar(bar); err != nil {
					return err
				}
			}
			if !atr.Initialized() || atr.Count() != 4 || !atr.Value().Round(4).Equal(decimal.RequireFromString("2.7778")) {
				return fmt.Errorf("ATR parity mismatch: initialized=%t count=%d value=%s", atr.Initialized(), atr.Count(), atr.Value())
			}
			return nil
		}},
		{id: "TC-S08", name: "Runtime helpers include data request metadata and strategy-scoped logging", run: func() error {
			var buf bytes.Buffer
			rt := &strategyRuntimeProbe{
				cache:  cache.New(),
				logger: slog.New(slog.NewJSONHandler(&buf, nil)),
			}
			mutable := &strategyMutableIDProbe{id: "tc-s08"}
			engine := strategy.NewEngine(bus.New(), strategy.WithRuntime(rt), strategy.WithTraderID("TRADER-LOG"))
			if err := engine.Add(mutable); err != nil {
				return err
			}
			mutable.id = "mutated"
			if err := engine.Start(ctx); err != nil {
				return err
			}
			defer engine.Stop(context.Background())

			if _, err := mutable.runtime.RequestData(ctx, model.DataRequest{
				Metadata:     model.CommandMetadata{CommandID: "tc-s08-request"},
				RequestID:    "tc-s08-data-request",
				InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
				Type:         model.MarketDataTypeTicker,
			}); err != nil {
				return err
			}
			if rt.lastDataRequest.Metadata.TraderID != "TRADER-LOG" ||
				rt.lastDataRequest.Metadata.StrategyID != "tc-s08" ||
				rt.lastDataRequest.Metadata.CommandID != "tc-s08-request" ||
				rt.lastDataRequest.Metadata.TsInit.IsZero() {
				return fmt.Errorf("data request metadata was not decorated: %#v", rt.lastDataRequest.Metadata)
			}

			mutable.runtime.Logger().Info("runtime helper ready", "kind", "logger")
			line := strings.TrimSpace(buf.String())
			for _, fragment := range []string{
				`"msg":"runtime helper ready"`,
				`"trader_id":"TRADER-LOG"`,
				`"strategy_id":"tc-s08"`,
				`"kind":"logger"`,
			} {
				if !strings.Contains(line, fragment) {
					return fmt.Errorf("strategy scoped log missing %s in %s", fragment, line)
				}
			}
			return nil
		}},
	})
}

func strategyTesterBar(high string, low string, close string) model.Bar {
	return model.Bar{
		BarType:   model.NewTimeBarType(model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"), time.Minute),
		Open:      decimal.RequireFromString(close),
		High:      decimal.RequireFromString(high),
		Low:       decimal.RequireFromString(low),
		Close:     decimal.RequireFromString(close),
		Volume:    decimal.RequireFromString("1"),
		Timestamp: time.Unix(1, 0),
	}
}

type strategyCallbackProbe struct {
	tickers        int
	customData     int
	accounts       int
	orders         int
	orderLifecycle int
	fills          int
	positions      int
	timers         int
	errors         int
	lastError      error
}

func (p *strategyCallbackProbe) OnTicker(context.Context, model.Ticker) error {
	p.tickers++
	return nil
}

func (p *strategyCallbackProbe) OnCustomData(context.Context, model.CustomData) error {
	p.customData++
	return nil
}

func (p *strategyCallbackProbe) OnAccount(context.Context, model.AccountSnapshot) error {
	p.accounts++
	return nil
}

func (p *strategyCallbackProbe) OnOrderLifecycle(context.Context, model.OrderLifecycleEvent) error {
	p.orderLifecycle++
	return nil
}

func (p *strategyCallbackProbe) OnOrderStatus(context.Context, model.OrderStatusReport) error {
	p.orders++
	return nil
}

func (p *strategyCallbackProbe) OnOrderFilled(context.Context, model.FillReport) error {
	p.fills++
	return nil
}

func (p *strategyCallbackProbe) OnPosition(context.Context, model.PositionStatusReport) error {
	p.positions++
	return nil
}

func (p *strategyCallbackProbe) OnTimer(context.Context, strategy.TimerEvent) error {
	p.timers++
	return nil
}

func (p *strategyCallbackProbe) OnError(_ context.Context, event strategy.ErrorEvent) error {
	p.errors++
	p.lastError = event.Err
	return nil
}

type strategyFailingHandler struct {
	err error
}

func (h strategyFailingHandler) OnTimer(context.Context, strategy.TimerEvent) error {
	return h.err
}

type strategyTimerSignal struct {
	seen chan struct{}
}

func (h *strategyTimerSignal) OnTimer(context.Context, strategy.TimerEvent) error {
	select {
	case h.seen <- struct{}{}:
	default:
	}
	return nil
}

type strategyMutableIDProbe struct {
	id      string
	runtime strategy.Runtime
}

func (p *strategyMutableIDProbe) ID() string { return p.id }

func (p *strategyMutableIDProbe) OnStart(_ context.Context, runtime strategy.Runtime) error {
	p.runtime = runtime
	return nil
}

func (p *strategyMutableIDProbe) OnEvent(context.Context, bus.Envelope) error { return nil }
func (p *strategyMutableIDProbe) OnStop(context.Context) error                { return nil }

type strategyRuntimeProbe struct {
	cache           *cache.Cache
	logger          *slog.Logger
	lastDataRequest model.DataRequest
	lastSubmit      model.SubmitOrder
}

func (r *strategyRuntimeProbe) Cache() *cache.Cache { return r.cache }
func (r *strategyRuntimeProbe) Portfolio() *portfolio.Portfolio {
	return nil
}
func (r *strategyRuntimeProbe) Clock() strategy.Clock { return strategy.WallClock{} }
func (r *strategyRuntimeProbe) Logger() *slog.Logger {
	return r.logger
}
func (r *strategyRuntimeProbe) SetTimer(context.Context, string, time.Duration) error {
	return nil
}
func (r *strategyRuntimeProbe) CancelTimer(context.Context, string) error {
	return nil
}
func (r *strategyRuntimeProbe) OrderFactory(accountID model.AccountID) *model.OrderFactory {
	return model.NewOrderFactory(accountID)
}
func (r *strategyRuntimeProbe) SubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (r *strategyRuntimeProbe) UnsubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (r *strategyRuntimeProbe) SubscribeTicker(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeTicker})
}
func (r *strategyRuntimeProbe) UnsubscribeTicker(context.Context, model.InstrumentID) error {
	return nil
}
func (r *strategyRuntimeProbe) SubscribeTradeTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeTradeTick})
}
func (r *strategyRuntimeProbe) UnsubscribeTradeTicks(context.Context, model.InstrumentID) error {
	return nil
}
func (r *strategyRuntimeProbe) SubscribeQuoteTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeQuoteTick})
}
func (r *strategyRuntimeProbe) UnsubscribeQuoteTicks(context.Context, model.InstrumentID) error {
	return nil
}
func (r *strategyRuntimeProbe) SubscribeFundingRates(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeFundingRate})
}
func (r *strategyRuntimeProbe) UnsubscribeFundingRates(context.Context, model.InstrumentID) error {
	return nil
}
func (r *strategyRuntimeProbe) SubscribeBars(ctx context.Context, barType model.BarType) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: barType.InstrumentID, Type: model.MarketDataTypeBar, BarType: barType})
}
func (r *strategyRuntimeProbe) UnsubscribeBars(context.Context, model.BarType) error { return nil }
func (r *strategyRuntimeProbe) SubscribeOrderBookDepth(ctx context.Context, instrumentID model.InstrumentID, depth int) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeOrderBook, Depth: depth})
}
func (r *strategyRuntimeProbe) UnsubscribeOrderBookDepth(context.Context, model.InstrumentID, int) error {
	return nil
}
func (r *strategyRuntimeProbe) RequestData(_ context.Context, request model.DataRequest) (model.DataResponse, error) {
	r.lastDataRequest = request
	return model.DataResponse{
		Metadata:     request.Metadata,
		RequestID:    request.RequestID,
		InstrumentID: request.InstrumentID,
		Type:         request.Type,
		BarType:      request.BarType,
		IsFinal:      true,
	}, nil
}
func (r *strategyRuntimeProbe) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	r.lastSubmit = order
	return model.OrderStatusReport{AccountID: order.AccountID, InstrumentID: order.InstrumentID, OrderID: "tc-s06-order", ClientOrderID: order.ClientOrderID, Status: model.OrderStatusAccepted}, nil
}
func (r *strategyRuntimeProbe) SubmitOrderList(ctx context.Context, list model.OrderList) ([]model.OrderStatusReport, error) {
	reports := make([]model.OrderStatusReport, 0, len(list.Orders))
	for _, order := range list.Orders {
		report, err := r.SubmitOrder(ctx, order)
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}
func (r *strategyRuntimeProbe) ModifyOrder(context.Context, model.ModifyOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (r *strategyRuntimeProbe) CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (r *strategyRuntimeProbe) BatchCancelOrders(context.Context, model.BatchCancelOrders) ([]model.OrderStatusReport, error) {
	return nil, nil
}
func (r *strategyRuntimeProbe) CancelAllOrders(context.Context, model.CancelAllOrders) ([]model.OrderStatusReport, error) {
	return nil, nil
}
func (r *strategyRuntimeProbe) QueryOrder(context.Context, model.QueryOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (r *strategyRuntimeProbe) QueryAccount(_ context.Context, query model.QueryAccount) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{AccountID: query.AccountID}, nil
}
