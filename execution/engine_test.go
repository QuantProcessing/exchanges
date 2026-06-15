package execution

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestEngineConnectsRoutesSubmitsAndCachesReports(t *testing.T) {
	client := newExecutionFakeClient("acct")
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c})
	require.NoError(t, engine.AddClient(client))

	require.NoError(t, engine.Start(context.Background()))
	order := executionTestSubmit("client-1", model.OrderSideBuy, oneDecimal())
	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusAccepted, report.Status)
	require.Equal(t, []string{"connect", "submit:client-1"}, client.Calls())

	cached, ok := c.OrderByClientID("acct", "client-1")
	require.True(t, ok)
	require.Equal(t, report.OrderID, cached.OrderID)
	require.NoError(t, engine.Stop(context.Background()))
	require.Contains(t, client.Calls(), "disconnect")
}

func TestEngineHealthTracksKernelLifecycleState(t *testing.T) {
	client := newExecutionFakeClient("acct")
	engine := NewEngine(EngineConfig{})
	require.NoError(t, engine.AddClient(client))

	health := engine.Health()
	require.Equal(t, kernel.ComponentStateInitialized, health.State)
	require.Equal(t, int64(0), health.Starts)
	require.Equal(t, int64(0), health.Stops)

	require.NoError(t, engine.Start(context.Background()))
	health = engine.Health()
	require.Equal(t, kernel.ComponentStateRunning, health.State)
	require.Equal(t, int64(1), health.Starts)
	require.Equal(t, int64(0), health.Stops)

	require.NoError(t, engine.Stop(context.Background()))
	health = engine.Health()
	require.Equal(t, kernel.ComponentStateStopped, health.State)
	require.Equal(t, int64(1), health.Starts)
	require.Equal(t, int64(1), health.Stops)
}

func TestEngineRoutesCancelModifyAndQuery(t *testing.T) {
	client := newExecutionFakeClient("acct")
	engine := NewEngine(EngineConfig{})
	require.NoError(t, engine.AddClient(client))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())
	order := executionTestSubmit("client-1", model.OrderSideBuy, oneDecimal())
	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)

	modifyReport, err := engine.ModifyOrder(context.Background(), model.ModifyOrder{
		AccountID:     "acct",
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		OrderID:       report.OrderID,
		Quantity:      oneDecimal(),
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusAccepted, modifyReport.Status)

	queryReport, err := engine.QueryOrder(context.Background(), model.QueryOrder{
		AccountID:     "acct",
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		OrderID:       report.OrderID,
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderID("query-client-1"), queryReport.OrderID)

	cancelReport, err := engine.CancelOrder(context.Background(), model.CancelOrder{
		AccountID:     "acct",
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		OrderID:       report.OrderID,
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusCanceled, cancelReport.Status)
	require.Contains(t, client.Calls(), "cancel:client-1")
	require.Contains(t, client.Calls(), "modify:client-1")
	require.Contains(t, client.Calls(), "query:client-1")
}

func TestEngineRejectsMissingClient(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	_, err := engine.SubmitOrder(context.Background(), executionTestSubmit("client-1", model.OrderSideBuy, oneDecimal()))
	require.ErrorIs(t, err, ErrClientNotFound)
}

func TestEngineRoutesExecAlgorithmOrdersBeforeVenueSubmission(t *testing.T) {
	client := newExecutionFakeClient("acct")
	algorithm := &executionFakeAlgorithm{id: "twap"}
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c})
	require.NoError(t, engine.AddClient(client))
	require.NoError(t, engine.AddAlgorithm(algorithm))

	order := executionTestSubmit("client-algo", model.OrderSideBuy, oneDecimal())
	order.Metadata.ExecAlgorithmID = "twap"
	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	require.Equal(t, []model.ClientOrderID{"client-algo"}, algorithm.ClientOrderIDs())
	require.NotContains(t, client.Calls(), "submit:client-algo")

	cached, ok := c.OrderByClientID("acct", "client-algo")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusEmulated, cached.Status)
	require.Equal(t, model.ExecAlgorithmID("twap"), cached.Metadata.ExecAlgorithmID)
	require.Equal(t, 1, engine.Health().Algorithms)

	missing := executionTestSubmit("client-missing-algo", model.OrderSideBuy, oneDecimal())
	missing.Metadata.ExecAlgorithmID = "missing"
	_, err = engine.SubmitOrder(context.Background(), missing)
	require.ErrorIs(t, err, ErrAlgorithmNotFound)
}

func TestEngineEmulatesTriggerOrderUntilQuoteReleasesToVenue(t *testing.T) {
	client := newExecutionFakeClient("acct")
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c, Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-emulated-stop", model.OrderSideBuy, oneDecimal())
	order.Type = model.OrderTypeStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	emulated := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventEmulated, emulated.Kind)
	require.Equal(t, model.OrderStatusEmulated, emulated.Status)
	require.Equal(t, order.ClientOrderID, emulated.ClientOrderID)
	require.NotContains(t, client.Calls(), "submit:client-emulated-stop")
	cached, ok := c.OrderByClientID("acct", "client-emulated-stop")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusEmulated, cached.Status)

	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("100.5"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Empty(t, reports)
	requireNoExecutionEvent(t, engine.Events())
	require.NotContains(t, client.Calls(), "submit:client-emulated-stop")

	reports, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	triggered := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventTriggered, triggered.Kind)
	require.Equal(t, model.OrderStatusTriggered, triggered.Status)
	require.Equal(t, model.OrderStatusEmulated, triggered.PreviousStatus)
	released := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventReleased, released.Kind)
	require.Equal(t, model.OrderStatusReleased, released.Status)
	require.Equal(t, model.OrderStatusTriggered, released.PreviousStatus)
	require.Equal(t, model.OrderStatusAccepted, reports[0].Status)
	require.Contains(t, client.Calls(), "submit:client-emulated-stop")
	cached, ok = c.OrderByClientID("acct", "client-emulated-stop")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, cached.Status)
	require.Len(t, c.OpenOrders("acct"), 1)

	reports, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("101"),
		AskPrice:     decimal.RequireFromString("101.5"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Empty(t, reports)
	requireNoExecutionEvent(t, engine.Events())
	require.Equal(t, 1, countString(client.Calls(), "submit:client-emulated-stop"))
}

func TestEngineCancelsEmulatedOrderLocally(t *testing.T) {
	client := newExecutionFakeClient("acct")
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c, Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-local-cancel", model.OrderSideBuy, oneDecimal())
	order.Type = model.OrderTypeStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	cancelReport, err := engine.CancelOrder(context.Background(), model.CancelOrder{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusCanceled, cancelReport.Status)
	require.Equal(t, report.OrderID, cancelReport.OrderID)
	require.NotContains(t, client.Calls(), "cancel:client-local-cancel")

	canceled := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventCanceled, canceled.Kind)
	require.Equal(t, model.OrderStatusCanceled, canceled.Status)
	require.Equal(t, model.OrderStatusEmulated, canceled.PreviousStatus)
	requireNoExecutionEvent(t, engine.Events())

	_, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.NotContains(t, client.Calls(), "submit:client-local-cancel")
}

func TestEngineLocalEmulatedModifyRematchesTrigger(t *testing.T) {
	client := newExecutionFakeClient("acct")
	c := cache.New()
	quote := model.QuoteTick{
		InstrumentID: executionTestInstrumentID(),
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Quote: &quote}))
	engine := NewEngine(EngineConfig{Cache: c, Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-local-modify", model.OrderSideBuy, oneDecimal())
	order.Type = model.OrderTypeStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("105")
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	modifyReport, err := engine.ModifyOrder(context.Background(), model.ModifyOrder{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		OrderID:       report.OrderID,
		TriggerPrice:  decimal.RequireFromString("100.5"),
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusAccepted, modifyReport.Status)
	require.NotContains(t, client.Calls(), "modify:client-local-modify")
	require.Contains(t, client.Calls(), "submit:client-local-modify")

	updated := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventUpdated, updated.Kind)
	require.Equal(t, model.OrderStatusEmulated, updated.PreviousStatus)
	require.Equal(t, model.OrderStatusEmulated, updated.Status)
	require.Equal(t, decimal.RequireFromString("100.5"), updated.Report.TriggerPrice)

	triggered := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventTriggered, triggered.Kind)
	require.Equal(t, model.OrderStatusEmulated, triggered.PreviousStatus)
	require.Equal(t, decimal.RequireFromString("100.5"), triggered.Report.TriggerPrice)

	released := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventReleased, released.Kind)
	require.Equal(t, model.OrderStatusTriggered, released.PreviousStatus)
	requireNoExecutionEvent(t, engine.Events())

	submitted := client.SubmittedOrders()
	require.Len(t, submitted, 1)
	require.Equal(t, model.OrderTypeMarket, submitted[0].Type)
	require.Equal(t, model.TriggerTypeNoTrigger, submitted[0].EmulationTrigger)
}

func TestEngineCancelAllCancelsHeldEmulatedOrdersLocally(t *testing.T) {
	client := &executionCancelAllFakeClient{executionFakeClient: newExecutionFakeClient("acct")}
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c, Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-local-cancel-all", model.OrderSideBuy, oneDecimal())
	order.Type = model.OrderTypeStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	cancelReports, err := engine.CancelAllOrders(context.Background(), model.CancelAllOrders{
		AccountID:    order.AccountID,
		InstrumentID: order.InstrumentID,
		OrderSide:    model.OrderSideBuy,
	})
	require.NoError(t, err)
	require.Len(t, cancelReports, 1)
	require.Equal(t, model.OrderStatusCanceled, cancelReports[0].Status)
	require.Equal(t, report.OrderID, cancelReports[0].OrderID)
	require.NotContains(t, client.Calls(), "cancel-all:BINANCE:BTCUSDT")
	require.NotContains(t, client.Calls(), "cancel:client-local-cancel-all")

	canceled := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventCanceled, canceled.Kind)
	require.Equal(t, model.OrderStatusEmulated, canceled.PreviousStatus)
	requireNoExecutionEvent(t, engine.Events())

	_, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.NotContains(t, client.Calls(), "submit:client-local-cancel-all")
}

func TestEngineEmulatesTriggerOrderWithTriggerInstrument(t *testing.T) {
	client := newExecutionFakeClient("acct")
	engine := NewEngine(EngineConfig{Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-trigger-instrument", model.OrderSideBuy, oneDecimal())
	order.Type = model.OrderTypeStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("201")
	order.TriggerInstrumentID = executionTestTriggerInstrumentID()
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	require.Equal(t, executionTestTriggerInstrumentID(), report.TriggerInstrumentID)
	_ = readExecutionLifecycle(t, engine.Events())

	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("200"),
		AskPrice:     decimal.RequireFromString("201"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Empty(t, reports)
	requireNoExecutionEvent(t, engine.Events())
	require.NotContains(t, client.Calls(), "submit:client-trigger-instrument")

	reports, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: executionTestTriggerInstrumentID(),
		BidPrice:     decimal.RequireFromString("200"),
		AskPrice:     decimal.RequireFromString("201"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	triggered := readExecutionLifecycle(t, engine.Events())
	require.NotNil(t, triggered.Report)
	require.Equal(t, executionTestTriggerInstrumentID(), triggered.Report.TriggerInstrumentID)
	require.Equal(t, model.OrderStatusTriggered, triggered.Status)
	_ = readExecutionLifecycle(t, engine.Events())
	require.Contains(t, client.Calls(), "submit:client-trigger-instrument")
}

func TestEngineEmulatesTriggerOrderImmediatelyWithCachedMarketData(t *testing.T) {
	client := newExecutionFakeClient("acct")
	c := cache.New()
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: executionTestInstrumentID(),
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}}))
	engine := NewEngine(EngineConfig{Cache: c, Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-initial-trigger", model.OrderSideBuy, oneDecimal())
	order.Type = model.OrderTypeStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusAccepted, report.Status)
	require.Contains(t, client.Calls(), "submit:client-initial-trigger")

	triggered := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventTriggered, triggered.Kind)
	require.Equal(t, model.OrderStatusTriggered, triggered.Status)
	require.Equal(t, model.OrderStatusInitialized, triggered.PreviousStatus)
	released := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventReleased, released.Kind)
	require.Equal(t, model.OrderStatusReleased, released.Status)
	require.Equal(t, model.OrderStatusTriggered, released.PreviousStatus)
	requireNoExecutionEvent(t, engine.Events())
}

func TestEngineEmulatesBidAskTriggerFromOrderBook(t *testing.T) {
	client := newExecutionFakeClient("acct")
	engine := NewEngine(EngineConfig{Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-book-trigger", model.OrderSideBuy, oneDecimal())
	order.Type = model.OrderTypeStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("201")
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{OrderBook: &model.OrderBook{
		InstrumentID: order.InstrumentID,
		Bids: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("200"),
			Size:  oneDecimal(),
		}},
		Asks: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("201"),
			Size:  oneDecimal(),
		}},
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	triggered := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderStatusTriggered, triggered.Status)
	_ = readExecutionLifecycle(t, engine.Events())
	require.Contains(t, client.Calls(), "submit:client-book-trigger")
}

func TestEngineEmulatesLimitOrderUntilOrderBookIsMarketable(t *testing.T) {
	client := newExecutionFakeClient("acct")
	engine := NewEngine(EngineConfig{Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-emulated-limit", model.OrderSideBuy, oneDecimal())
	order.Type = model.OrderTypeLimit
	order.Price = decimal.RequireFromString("100")
	order.TriggerPrice = decimal.Zero
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	require.Empty(t, client.SubmittedOrders())
	_ = readExecutionLifecycle(t, engine.Events())

	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{OrderBook: &model.OrderBook{
		InstrumentID: order.InstrumentID,
		Asks: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("101"),
			Size:  oneDecimal(),
		}},
		Timestamp: time.Unix(10, 0),
	}})
	require.NoError(t, err)
	require.Empty(t, reports)
	require.Empty(t, client.SubmittedOrders())

	reports, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{OrderBook: &model.OrderBook{
		InstrumentID: order.InstrumentID,
		Asks: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("100"),
			Size:  oneDecimal(),
		}},
		Timestamp: time.Unix(11, 0),
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.OrderStatusAccepted, reports[0].Status)
	require.Equal(t, model.OrderTypeLimit, reports[0].Type)
	submitted := client.SubmittedOrders()
	require.Len(t, submitted, 1)
	require.Equal(t, model.OrderTypeLimit, submitted[0].Type)
	require.Equal(t, model.TriggerTypeNoTrigger, submitted[0].EmulationTrigger)
	require.True(t, submitted[0].TriggerPrice.IsZero())
}

func TestEngineEmulatesTrailingStopMarketUntilTrailingQuoteTriggers(t *testing.T) {
	client := newExecutionFakeClient("acct")
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c, Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-emulated-trailing-stop", model.OrderSideSell, oneDecimal())
	order.Type = model.OrderTypeTrailingStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.Zero
	order.ActivationPrice = decimal.RequireFromString("100")
	order.TrailingOffset = decimal.RequireFromString("5")
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Empty(t, reports)
	requireNoExecutionEvent(t, engine.Events())

	reports, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("110"),
		AskPrice:     decimal.RequireFromString("111"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Empty(t, reports)
	requireNoExecutionEvent(t, engine.Events())
	require.NotContains(t, client.Calls(), "submit:client-emulated-trailing-stop")

	reports, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("104"),
		AskPrice:     decimal.RequireFromString("105"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	triggered := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventTriggered, triggered.Kind)
	require.NotNil(t, triggered.Report)
	require.Equal(t, "105", triggered.Report.TriggerPrice.String())
	released := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventReleased, released.Kind)
	require.Equal(t, model.OrderStatusAccepted, reports[0].Status)
	require.Contains(t, client.Calls(), "submit:client-emulated-trailing-stop")
}

func TestEngineEmulatesTrailingStopMarketWithTickOffset(t *testing.T) {
	client := newExecutionFakeClient("acct")
	c := cache.New()
	putExecutionTestInstrument(t, c, "0.5")
	engine := NewEngine(EngineConfig{Cache: c, Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-emulated-trailing-ticks", model.OrderSideSell, oneDecimal())
	order.Type = model.OrderTypeTrailingStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.Zero
	order.ActivationPrice = decimal.RequireFromString("100")
	order.TrailingOffset = decimal.RequireFromString("10")
	order.TrailingOffsetType = model.TrailingOffsetTypeTicks
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	_, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)

	_, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("110"),
		AskPrice:     decimal.RequireFromString("111"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)

	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("104"),
		AskPrice:     decimal.RequireFromString("105"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	triggered := readExecutionLifecycle(t, engine.Events())
	require.NotNil(t, triggered.Report)
	require.Equal(t, "105", triggered.Report.TriggerPrice.String())
	require.Equal(t, model.TrailingOffsetTypeTicks, triggered.Report.TrailingOffsetType)
	_ = readExecutionLifecycle(t, engine.Events())
	require.Contains(t, client.Calls(), "submit:client-emulated-trailing-ticks")
}

func TestEngineEmulatesTrailingStopMarketWithSyntheticTriggerTickOffset(t *testing.T) {
	client := newExecutionFakeClient("acct")
	c := cache.New()
	synthID := model.MustInstrumentID("BTC-ETH-SPREAD.SYNTH")
	require.NoError(t, c.PutSyntheticInstrument(model.SyntheticInstrument{
		ID:             synthID,
		PricePrecision: 2,
		PriceTick:      decimal.RequireFromString("0.25"),
		Components: []model.InstrumentID{
			executionTestInstrumentID(),
			executionTestTriggerInstrumentID(),
		},
		Formula: "BTC-USDT-SPOT.BINANCE - ETH-USDT-SPOT.BINANCE",
	}))
	engine := NewEngine(EngineConfig{Cache: c, Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-emulated-synthetic-trailing", model.OrderSideSell, oneDecimal())
	order.Type = model.OrderTypeTrailingStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.Zero
	order.TriggerInstrumentID = synthID
	order.ActivationPrice = decimal.RequireFromString("100")
	order.TrailingOffset = decimal.RequireFromString("10")
	order.TrailingOffsetType = model.TrailingOffsetTypeTicks
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	_, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: synthID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)

	_, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: synthID,
		BidPrice:     decimal.RequireFromString("105"),
		AskPrice:     decimal.RequireFromString("106"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)

	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: synthID,
		BidPrice:     decimal.RequireFromString("102.5"),
		AskPrice:     decimal.RequireFromString("103.5"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	triggered := readExecutionLifecycle(t, engine.Events())
	require.NotNil(t, triggered.Report)
	require.Equal(t, "102.5", triggered.Report.TriggerPrice.String())
	require.Equal(t, synthID, triggered.Report.TriggerInstrumentID)
	_ = readExecutionLifecycle(t, engine.Events())
	require.Contains(t, client.Calls(), "submit:client-emulated-synthetic-trailing")
}

func TestEngineEmulatesTrailingStopMarketWithBasisPointOffset(t *testing.T) {
	client := newExecutionFakeClient("acct")
	engine := NewEngine(EngineConfig{Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-emulated-trailing-bps", model.OrderSideSell, oneDecimal())
	order.Type = model.OrderTypeTrailingStopMarket
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.Zero
	order.ActivationPrice = decimal.RequireFromString("100")
	order.TrailingOffset = decimal.RequireFromString("500")
	order.TrailingOffsetType = model.TrailingOffsetTypeBasisPoints
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	_, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)

	_, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("110"),
		AskPrice:     decimal.RequireFromString("111"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)

	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("104.5"),
		AskPrice:     decimal.RequireFromString("105.5"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	triggered := readExecutionLifecycle(t, engine.Events())
	require.NotNil(t, triggered.Report)
	require.Equal(t, "104.5", triggered.Report.TriggerPrice.String())
	require.Equal(t, model.TrailingOffsetTypeBasisPoints, triggered.Report.TrailingOffsetType)
	_ = readExecutionLifecycle(t, engine.Events())
	require.Contains(t, client.Calls(), "submit:client-emulated-trailing-bps")
}

func TestEngineEmulatesTrailingStopLimitReleasesLimitOrder(t *testing.T) {
	client := newExecutionFakeClient("acct")
	engine := NewEngine(EngineConfig{Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-emulated-trailing-limit", model.OrderSideSell, oneDecimal())
	order.Type = model.OrderTypeTrailingStopLimit
	order.TimeInForce = model.TimeInForceGTC
	order.Price = decimal.RequireFromString("104")
	order.TriggerPrice = decimal.Zero
	order.ActivationPrice = decimal.RequireFromString("100")
	order.TrailingOffset = decimal.RequireFromString("5")
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	_, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	_, err = engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("110"),
		AskPrice:     decimal.RequireFromString("111"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("104"),
		AskPrice:     decimal.RequireFromString("105"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.OrderTypeLimit, reports[0].Type)
	require.Equal(t, "104", reports[0].Price.String())

	triggered := readExecutionLifecycle(t, engine.Events())
	require.NotNil(t, triggered.Report)
	require.Equal(t, model.OrderTypeTrailingStopLimit, triggered.Report.Type)
	require.Equal(t, "105", triggered.Report.TriggerPrice.String())
	released := readExecutionLifecycle(t, engine.Events())
	require.Equal(t, model.OrderEventReleased, released.Kind)
	submitted := client.SubmittedOrders()
	require.Len(t, submitted, 1)
	require.Equal(t, model.OrderTypeLimit, submitted[0].Type)
	require.Equal(t, "104", submitted[0].Price.String())
	require.True(t, submitted[0].TriggerPrice.IsZero())
	require.True(t, submitted[0].TrailingOffset.IsZero())
}

func TestEngineTransformsReleasedStopMarketToMarketOrder(t *testing.T) {
	client := newExecutionFakeClient("acct")
	engine := NewEngine(EngineConfig{Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-release-market", model.OrderSideBuy, oneDecimal())
	order.Type = model.OrderTypeStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.OrderTypeMarket, reports[0].Type)
	submitted := client.SubmittedOrders()
	require.Len(t, submitted, 1)
	require.Equal(t, model.OrderTypeMarket, submitted[0].Type)
	require.Equal(t, model.TriggerTypeNoTrigger, submitted[0].EmulationTrigger)
	require.True(t, submitted[0].TriggerPrice.IsZero())
}

func TestEngineTransformsReleasedStopLimitToLimitOrder(t *testing.T) {
	client := newExecutionFakeClient("acct")
	engine := NewEngine(EngineConfig{Emulator: NewEmulator(EmulatorConfig{})})
	require.NoError(t, engine.AddClient(client))

	order := executionTestSubmit("client-release-limit", model.OrderSideBuy, oneDecimal())
	order.Type = model.OrderTypeStopLimit
	order.Price = decimal.RequireFromString("102")
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk

	report, err := engine.SubmitOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	_ = readExecutionLifecycle(t, engine.Events())

	reports, err := engine.ProcessMarketEvent(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      oneDecimal(),
		AskSize:      oneDecimal(),
	}})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.OrderTypeLimit, reports[0].Type)
	submitted := client.SubmittedOrders()
	require.Len(t, submitted, 1)
	require.Equal(t, model.OrderTypeLimit, submitted[0].Type)
	require.Equal(t, "102", submitted[0].Price.String())
	require.Equal(t, model.TriggerTypeNoTrigger, submitted[0].EmulationTrigger)
	require.True(t, submitted[0].TriggerPrice.IsZero())
}

func TestEngineRoutesCompositeCommandsAndAccountQuery(t *testing.T) {
	client := newExecutionFakeClient("acct")
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c})
	require.NoError(t, engine.AddClient(client))

	list := executionTestOrderList("list-1", "list-client-1", "list-client-2")
	reports, err := engine.SubmitOrderList(context.Background(), model.SubmitOrderList{
		Metadata:  model.CommandMetadata{CommandID: "submit-list"},
		AccountID: "acct",
		List:      list,
	})
	require.NoError(t, err)
	require.Len(t, reports, 2)
	require.Contains(t, client.Calls(), "submit:list-client-1")
	require.Contains(t, client.Calls(), "submit:list-client-2")

	batchReports, err := engine.BatchCancelOrders(context.Background(), model.BatchCancelOrders{
		Metadata:     model.CommandMetadata{CommandID: "batch-cancel"},
		AccountID:    "acct",
		InstrumentID: executionTestInstrumentID(),
		Cancels: []model.CancelOrder{
			{ClientOrderID: "list-client-1"},
			{ClientOrderID: "list-client-2"},
		},
	})
	require.NoError(t, err)
	require.Len(t, batchReports, 2)
	require.Contains(t, client.Calls(), "cancel:list-client-1")
	require.Contains(t, client.Calls(), "cancel:list-client-2")

	buy := executionTestSubmit("cancel-all-buy", model.OrderSideBuy, oneDecimal())
	sell := executionTestSubmit("cancel-all-sell", model.OrderSideSell, oneDecimal())
	_, err = engine.SubmitOrder(context.Background(), buy)
	require.NoError(t, err)
	_, err = engine.SubmitOrder(context.Background(), sell)
	require.NoError(t, err)
	cancelAllReports, err := engine.CancelAllOrders(context.Background(), model.CancelAllOrders{
		AccountID:    "acct",
		InstrumentID: executionTestInstrumentID(),
		OrderSide:    model.OrderSideBuy,
	})
	require.NoError(t, err)
	require.Len(t, cancelAllReports, 1)
	require.Equal(t, model.ClientOrderID("cancel-all-buy"), cancelAllReports[0].ClientOrderID)

	snapshot, err := engine.QueryAccount(context.Background(), model.QueryAccount{AccountID: "acct"})
	require.NoError(t, err)
	require.Equal(t, model.AccountID("acct"), snapshot.AccountID)
	_, ok := c.Account("acct")
	require.True(t, ok)
	health := engine.Health()
	require.Equal(t, int64(4), health.Submits)
	require.Equal(t, int64(3), health.Cancels)
}

func TestEngineGeneratesReportsAndMassStatus(t *testing.T) {
	client := newExecutionFakeClient("acct")
	order := executionTestOrderReport("order-report-1", "client-report-1", model.OrderStatusAccepted)
	fill := executionTestFill("trade-report-1", "order-report-1", "client-report-1", "1", "101")
	position := executionTestPosition("position-report-1")
	client.orderReports = []model.OrderStatusReport{order}
	client.fillReports = []model.FillReport{fill}
	client.positionReports = []model.PositionStatusReport{position}
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c})
	require.NoError(t, engine.AddClient(client))

	orders, err := engine.GenerateOrderStatusReports(context.Background(), model.GenerateOrderStatusReports{
		AccountID:    "acct",
		InstrumentID: executionTestInstrumentID(),
	})
	require.NoError(t, err)
	require.Equal(t, []model.OrderStatusReport{order}, orders)
	_, ok := c.Order("acct", "order-report-1")
	require.True(t, ok)

	fills, err := engine.GenerateFillReports(context.Background(), model.GenerateFillReports{
		AccountID:    "acct",
		InstrumentID: executionTestInstrumentID(),
	})
	require.NoError(t, err)
	require.Equal(t, []model.FillReport{fill}, fills)
	_, ok = c.FillByTradeID("acct", "trade-report-1")
	require.True(t, ok)

	positions, err := engine.GeneratePositionStatusReports(context.Background(), model.GeneratePositionStatusReports{
		AccountID:    "acct",
		InstrumentID: executionTestInstrumentID(),
	})
	require.NoError(t, err)
	require.Equal(t, []model.PositionStatusReport{position}, positions)
	_, ok = c.Position("acct", "position-report-1")
	require.True(t, ok)

	massCache := cache.New()
	massEngine := NewEngine(EngineConfig{Cache: massCache})
	require.NoError(t, massEngine.AddClient(client))
	massStatus, err := massEngine.GenerateExecutionMassStatus(context.Background(), model.GenerateExecutionMassStatus{
		AccountID:    "acct",
		InstrumentID: executionTestInstrumentID(),
	})
	require.NoError(t, err)
	require.Equal(t, model.AccountID("acct"), massStatus.AccountID)
	require.Len(t, massStatus.Accounts, 1)
	require.Len(t, massStatus.Orders, 1)
	require.Len(t, massStatus.Fills, 1)
	require.Len(t, massStatus.Positions, 1)
}

func TestEngineQueryOrderFallsBackToReportsByVenueOrderID(t *testing.T) {
	client := newExecutionReportOnlyClient("acct")
	report := executionTestOrderReport("venue-query-order", "", model.OrderStatusAccepted)
	report.AccountID = "acct"
	report.InstrumentID = executionTestInstrumentID()
	report.VenueOrderID = "venue-only-1"
	client.orderReports = []model.OrderStatusReport{report}
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c})
	require.NoError(t, engine.AddClient(client))

	got, err := engine.QueryOrder(context.Background(), model.QueryOrder{
		AccountID:    "acct",
		InstrumentID: executionTestInstrumentID(),
		VenueOrderID: "venue-only-1",
	})
	require.NoError(t, err)
	require.Equal(t, report.OrderID, got.OrderID)
	require.Equal(t, report.VenueOrderID, got.VenueOrderID)
	generateCall := "generate-orders:" + executionTestInstrumentID().String()
	require.Contains(t, client.Calls(), generateCall)
	cached, ok := c.OrderByVenueID("acct", "venue-only-1")
	require.True(t, ok)
	require.Equal(t, report.OrderID, cached.OrderID)

	again, err := engine.QueryOrder(context.Background(), model.QueryOrder{
		AccountID:    "acct",
		InstrumentID: executionTestInstrumentID(),
		VenueOrderID: "venue-only-1",
	})
	require.NoError(t, err)
	require.Equal(t, report.OrderID, again.OrderID)
	require.Equal(t, 1, countString(client.Calls(), generateCall))
}

func TestEngineClaimsExternalOrderReportsByInstrument(t *testing.T) {
	client := newExecutionFakeClient("acct")
	report := executionTestOrderReport("external-order-1", "", model.OrderStatusAccepted)
	report.Metadata = model.CommandMetadata{}
	client.orderReports = []model.OrderStatusReport{report}
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c})
	require.NoError(t, engine.AddClient(client))
	require.NoError(t, engine.RegisterExternalOrderClaim(executionTestInstrumentID(), "strategy-claim"))
	require.Error(t, engine.RegisterExternalOrderClaim(executionTestInstrumentID(), "other-strategy"))

	claim, ok := engine.ExternalOrderClaim(executionTestInstrumentID())
	require.True(t, ok)
	require.Equal(t, model.StrategyID("strategy-claim"), claim)
	require.Equal(t, []model.InstrumentID{executionTestInstrumentID()}, engine.ExternalOrderClaimInstruments())

	reports, err := engine.GenerateOrderStatusReports(context.Background(), model.GenerateOrderStatusReports{
		AccountID:    "acct",
		InstrumentID: executionTestInstrumentID(),
	})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.StrategyID("strategy-claim"), reports[0].Metadata.StrategyID)
	require.Len(t, c.OrdersByStrategy("acct", "strategy-claim"), 1)
}

func TestEngineSnapshotsAndPurgesExecutionState(t *testing.T) {
	c := cache.New()
	engine := NewEngine(EngineConfig{Cache: c})
	c.PutAccount(model.AccountSnapshot{AccountID: "acct", Venue: "BINANCE"})
	openOrder := executionTestOrderReport("open-order", "open-client", model.OrderStatusAccepted)
	closedOrder := executionTestOrderReport("closed-order", "closed-client", model.OrderStatusFilled)
	closedOrder.FilledQuantity = closedOrder.Quantity
	closedOrder.LeavesQuantity = decimal.Zero
	require.NoError(t, engine.Manager().ApplyOrderReport(openOrder))
	require.NoError(t, engine.Manager().ApplyOrderReport(closedOrder))
	openPosition := executionTestPosition("open-position")
	require.NoError(t, c.PutPosition(openPosition))
	closedPosition := executionTestPosition("closed-position")
	closedPosition.Side = model.PositionSideFlat
	closedPosition.Quantity = decimal.Zero
	require.NoError(t, c.PutPosition(closedPosition))

	snapshot := engine.Snapshot("acct")
	require.Len(t, snapshot.OpenOrders, 1)
	require.Len(t, snapshot.ClosedOrders, 1)
	require.Len(t, snapshot.OpenPositions, 1)
	require.Len(t, snapshot.ClosedPositions, 1)

	result := engine.Purge("acct", cache.PurgePolicy{
		ClosedOrdersLimit:    0,
		ClosedPositionsLimit: 0,
	})
	require.Equal(t, 1, result.ClosedOrders)
	require.Equal(t, 1, result.ClosedPositions)
	snapshot = engine.Snapshot("acct")
	require.Len(t, snapshot.OpenOrders, 1)
	require.Empty(t, snapshot.ClosedOrders)
	require.Len(t, snapshot.OpenPositions, 1)
	require.Empty(t, snapshot.ClosedPositions)
}

type executionFakeClient struct {
	accountID       model.AccountID
	calls           []string
	submitted       []model.SubmitOrder
	orderReports    []model.OrderStatusReport
	fillReports     []model.FillReport
	positionReports []model.PositionStatusReport
}

func newExecutionFakeClient(accountID model.AccountID) *executionFakeClient {
	return &executionFakeClient{accountID: accountID}
}

type executionReportOnlyClient struct {
	accountID    model.AccountID
	calls        []string
	orderReports []model.OrderStatusReport
}

func newExecutionReportOnlyClient(accountID model.AccountID) *executionReportOnlyClient {
	return &executionReportOnlyClient{accountID: accountID}
}

type executionFakeAlgorithm struct {
	id     model.ExecAlgorithmID
	orders []model.SubmitOrder
}

func (a *executionFakeAlgorithm) ID() model.ExecAlgorithmID { return a.id }

func (a *executionFakeAlgorithm) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	a.orders = append(a.orders, order)
	report := executionTestOrderReport(model.OrderID("algo-"+string(order.ClientOrderID)), order.ClientOrderID, model.OrderStatusEmulated)
	report.Metadata = order.Metadata
	report.AccountID = order.AccountID
	report.InstrumentID = order.InstrumentID
	report.Side = order.Side
	report.Type = order.Type
	report.TimeInForce = order.TimeInForce
	report.Quantity = order.Quantity
	report.LeavesQuantity = order.Quantity
	report.Price = order.Price
	return report, nil
}

func (a *executionFakeAlgorithm) ClientOrderIDs() []model.ClientOrderID {
	ids := make([]model.ClientOrderID, 0, len(a.orders))
	for _, order := range a.orders {
		ids = append(ids, order.ClientOrderID)
	}
	return ids
}

func (c *executionFakeClient) Venue() model.Venue         { return "BINANCE" }
func (c *executionFakeClient) AccountID() model.AccountID { return c.accountID }
func (c *executionFakeClient) Connect(context.Context) error {
	c.calls = append(c.calls, "connect")
	return nil
}
func (c *executionFakeClient) Disconnect(context.Context) error {
	c.calls = append(c.calls, "disconnect")
	return nil
}
func (c *executionFakeClient) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true}
}
func (c *executionFakeClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	c.calls = append(c.calls, "query-account")
	return model.AccountSnapshot{AccountID: c.accountID, Venue: c.Venue()}, nil
}
func (c *executionFakeClient) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "submit:"+string(order.ClientOrderID))
	c.submitted = append(c.submitted, order)
	report := executionTestOrderReport(model.OrderID("accepted-"+string(order.ClientOrderID)), order.ClientOrderID, model.OrderStatusAccepted)
	report.AccountID = order.AccountID
	report.InstrumentID = order.InstrumentID
	report.Side = order.Side
	report.Type = order.Type
	report.TimeInForce = order.TimeInForce
	report.Quantity = order.Quantity
	report.LeavesQuantity = order.Quantity
	report.Price = order.Price
	return report, nil
}
func (c *executionFakeClient) SubmittedOrders() []model.SubmitOrder {
	return append([]model.SubmitOrder(nil), c.submitted...)
}
func (c *executionFakeClient) CancelOrder(_ context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "cancel:"+string(cancel.ClientOrderID))
	orderID := cancel.OrderID
	if orderID == "" {
		orderID = model.OrderID("accepted-" + string(cancel.ClientOrderID))
	}
	report := executionTestOrderReport(model.OrderID("canceled-"+string(cancel.ClientOrderID)), cancel.ClientOrderID, model.OrderStatusCanceled)
	report.AccountID = cancel.AccountID
	report.InstrumentID = cancel.InstrumentID
	report.OrderID = orderID
	report.LeavesQuantity = zeroDecimal()
	return report, nil
}
func (c *executionFakeClient) GenerateOrderStatusReports(_ context.Context, instrumentID model.InstrumentID) ([]model.OrderStatusReport, error) {
	c.calls = append(c.calls, "generate-orders:"+instrumentID.String())
	return append([]model.OrderStatusReport(nil), c.orderReports...), nil
}
func (c *executionFakeClient) Events() <-chan model.ExecutionEvent { return nil }
func (c *executionFakeClient) ModifyOrder(_ context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "modify:"+string(modify.ClientOrderID))
	report := executionTestOrderReport(model.OrderID("modified-"+string(modify.ClientOrderID)), modify.ClientOrderID, model.OrderStatusAccepted)
	report.AccountID = modify.AccountID
	report.InstrumentID = modify.InstrumentID
	report.OrderID = modify.OrderID
	return report, nil
}
func (c *executionFakeClient) QueryOrder(_ context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "query:"+string(query.ClientOrderID))
	report := executionTestOrderReport(model.OrderID("query-"+string(query.ClientOrderID)), query.ClientOrderID, model.OrderStatusAccepted)
	report.AccountID = query.AccountID
	report.InstrumentID = query.InstrumentID
	report.OrderID = model.OrderID("query-" + string(query.ClientOrderID))
	return report, nil
}
func (c *executionFakeClient) Calls() []string { return append([]string(nil), c.calls...) }

type executionCancelAllFakeClient struct {
	*executionFakeClient
}

func (c *executionCancelAllFakeClient) CancelAllOrders(_ context.Context, cancelAll model.CancelAllOrders) ([]model.OrderStatusReport, error) {
	c.calls = append(c.calls, "cancel-all:"+cancelAll.InstrumentID.String())
	return []model.OrderStatusReport{{
		AccountID:     cancelAll.AccountID,
		InstrumentID:  cancelAll.InstrumentID,
		OrderID:       "venue-cancel-all",
		ClientOrderID: "venue-cancel-all",
		Status:        model.OrderStatusCanceled,
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		Quantity:      oneDecimal(),
		TimeInForce:   model.TimeInForceGTC,
	}}, nil
}

func (c *executionFakeClient) GenerateFillReports(_ context.Context, instrumentID model.InstrumentID) ([]model.FillReport, error) {
	c.calls = append(c.calls, "generate-fills:"+instrumentID.String())
	return append([]model.FillReport(nil), c.fillReports...), nil
}

func (c *executionFakeClient) GeneratePositionStatusReports(_ context.Context, instrumentID model.InstrumentID) ([]model.PositionStatusReport, error) {
	c.calls = append(c.calls, "generate-positions:"+instrumentID.String())
	return append([]model.PositionStatusReport(nil), c.positionReports...), nil
}

func (c *executionReportOnlyClient) Venue() model.Venue         { return "BINANCE" }
func (c *executionReportOnlyClient) AccountID() model.AccountID { return c.accountID }
func (c *executionReportOnlyClient) Connect(context.Context) error {
	c.calls = append(c.calls, "connect")
	return nil
}
func (c *executionReportOnlyClient) Disconnect(context.Context) error {
	c.calls = append(c.calls, "disconnect")
	return nil
}
func (c *executionReportOnlyClient) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true}
}
func (c *executionReportOnlyClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	c.calls = append(c.calls, "query-account")
	return model.AccountSnapshot{AccountID: c.accountID, Venue: c.Venue()}, nil
}
func (c *executionReportOnlyClient) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "submit:"+string(order.ClientOrderID))
	report := executionTestOrderReport(model.OrderID("accepted-"+string(order.ClientOrderID)), order.ClientOrderID, model.OrderStatusAccepted)
	report.AccountID = order.AccountID
	report.InstrumentID = order.InstrumentID
	return report, nil
}
func (c *executionReportOnlyClient) CancelOrder(_ context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "cancel:"+string(cancel.ClientOrderID))
	report := executionTestOrderReport(cancel.OrderID, cancel.ClientOrderID, model.OrderStatusCanceled)
	report.AccountID = cancel.AccountID
	report.InstrumentID = cancel.InstrumentID
	return report, nil
}
func (c *executionReportOnlyClient) GenerateOrderStatusReports(_ context.Context, instrumentID model.InstrumentID) ([]model.OrderStatusReport, error) {
	c.calls = append(c.calls, "generate-orders:"+instrumentID.String())
	return append([]model.OrderStatusReport(nil), c.orderReports...), nil
}
func (c *executionReportOnlyClient) Events() <-chan model.ExecutionEvent { return nil }
func (c *executionReportOnlyClient) Calls() []string {
	return append([]string(nil), c.calls...)
}

func executionTestOrderList(listID model.OrderListID, clientOrderIDs ...model.ClientOrderID) model.OrderList {
	list := model.OrderList{ID: listID}
	for _, clientOrderID := range clientOrderIDs {
		order := executionTestSubmit(clientOrderID, model.OrderSideBuy, oneDecimal())
		order.OrderListID = listID
		list.Orders = append(list.Orders, order)
	}
	return list
}

func executionTestPosition(positionID model.PositionID) model.PositionStatusReport {
	return model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: executionTestInstrumentID(),
		PositionID:   positionID,
		Side:         model.PositionSideLong,
		Quantity:     oneDecimal(),
		EntryPrice:   decimal.RequireFromString("100"),
	}
}

func oneDecimal() decimal.Decimal {
	return decimal.RequireFromString("1")
}

func zeroDecimal() decimal.Decimal {
	return decimal.Zero
}

func putExecutionTestInstrument(t *testing.T, c *cache.Cache, priceTick string) {
	t.Helper()
	require.NoError(t, c.PutInstrument(model.Instrument{
		ID:        executionTestInstrumentID(),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString(priceTick),
		SizeTick:  oneDecimal(),
		Status:    model.InstrumentStatusTrading,
	}))
}

func executionTestTriggerInstrumentID() model.InstrumentID {
	return model.MustInstrumentID("ETH-USDT-SPOT.BINANCE")
}

func countString(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func readExecutionLifecycle(t *testing.T, events <-chan model.ExecutionEvent) model.OrderLifecycleEvent {
	t.Helper()
	select {
	case event := <-events:
		require.NotNil(t, event.Lifecycle)
		return *event.Lifecycle
	default:
		t.Fatal("expected execution lifecycle event")
		return model.OrderLifecycleEvent{}
	}
}

func requireNoExecutionEvent(t *testing.T, events <-chan model.ExecutionEvent) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected execution event: %+v", event)
	default:
	}
}

var _ venue.ExecutionClient = (*executionFakeClient)(nil)
var _ venue.OrderModifier = (*executionFakeClient)(nil)
var _ venue.OrderQuerier = (*executionFakeClient)(nil)
var _ venue.FillReportGenerator = (*executionFakeClient)(nil)
var _ venue.PositionStatusReportGenerator = (*executionFakeClient)(nil)
