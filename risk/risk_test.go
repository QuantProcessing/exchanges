package risk

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestEngineHealthTracksKernelLifecycleState(t *testing.T) {
	engine := NewEngine(nil, Config{})
	health := engine.Health()
	require.Equal(t, "risk.engine", health.ID)
	require.Equal(t, kernel.ComponentStateInitialized, health.State)

	require.NoError(t, engine.Start(context.Background()))
	require.Equal(t, kernel.ComponentStateRunning, engine.Health().State)

	require.NoError(t, engine.Stop(context.Background()))
	require.Equal(t, kernel.ComponentStateStopped, engine.Health().State)
}

func TestEngineExecuteProcessesOrdersThroughCommandQueue(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{QueueSize: 2})
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	results, err := engine.Execute(context.Background(), validRiskOrder(inst.ID, "async-client-1"))
	require.NoError(t, err)
	select {
	case result := <-results:
		require.True(t, result.Accepted())
		require.NoError(t, result.Error)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for risk decision")
	}

	health := engine.Health()
	require.Equal(t, 2, health.CommandQueueCapacity)
	require.Zero(t, health.CommandQueueDepth)
	require.Equal(t, int64(1), health.ProcessedCommands)
	require.Zero(t, health.RejectedCommands)
}

func TestEngineExecuteRejectsWhenCommandQueueFull(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{QueueSize: 1})

	_, err := engine.Execute(context.Background(), validRiskOrder(inst.ID, "queued-client-1"))
	require.NoError(t, err)
	_, err = engine.Execute(context.Background(), validRiskOrder(inst.ID, "queued-client-2"))
	require.ErrorIs(t, err, ErrRiskQueueFull)

	health := engine.Health()
	require.Equal(t, 1, health.CommandQueueCapacity)
	require.Equal(t, 1, health.CommandQueueDepth)
	require.Equal(t, int64(1), health.DroppedCommands)
}

func TestEngineExecuteReturnsDeniedLifecycleEventWithCommandMetadata(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{
		MaxOrderNotional: decimal.RequireFromString("100"),
	})
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	order := validRiskOrder(inst.ID, "async-denied-client")
	order.Metadata = model.CommandMetadata{
		TraderID:      "trader-001",
		StrategyID:    "strategy-001",
		CommandID:     "command-001",
		CorrelationID: "correlation-001",
	}
	order.Quantity = decimal.RequireFromString("2")
	results, err := engine.Execute(context.Background(), order)
	require.NoError(t, err)

	select {
	case result := <-results:
		require.False(t, result.Accepted())
		require.ErrorIs(t, result.Error, ErrRiskRejected)
		require.NotNil(t, result.Event)
		require.NotNil(t, result.Event.Lifecycle)
		lifecycle := result.Event.Lifecycle
		require.Equal(t, model.OrderEventDenied, lifecycle.Kind)
		require.Equal(t, model.OrderStatusDenied, lifecycle.Status)
		require.Equal(t, order.Metadata, lifecycle.Metadata)
		require.Equal(t, order.ClientOrderID, lifecycle.ClientOrderID)
		require.Contains(t, lifecycle.Reason, "max order notional")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for risk decision")
	}
}

func TestEngineProcessExecutionEventsThroughEventQueue(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{QueueSize: 4})
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	position := model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}
	require.NoError(t, engine.Process(context.Background(), model.ExecutionEvent{Position: &position}))

	require.Eventually(t, func() bool {
		_, ok := c.PositionByInstrument("acct", inst.ID)
		return ok && engine.Health().ProcessedEvents == 1
	}, time.Second, 10*time.Millisecond)
	require.Zero(t, engine.Health().EventQueueDepth)
}

func TestEngineRejectsOrdersThatViolateInstrumentPrecisionAndNotional(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{
		MaxOrderNotional: decimal.RequireFromString("1000"),
	})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		Quantity:      decimal.RequireFromString("0.125"),
		Price:         decimal.RequireFromString("100.01"),
	}
	require.NoError(t, engine.Check(order))

	order.Price = decimal.RequireFromString("100.001")
	require.ErrorIs(t, engine.Check(order), model.ErrInvalidOrder)

	order.Price = decimal.RequireFromString("100.01")
	order.Quantity = decimal.RequireFromString("0.0005")
	require.ErrorIs(t, engine.Check(order), model.ErrInvalidOrder)

	order.Quantity = decimal.RequireFromString("20")
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineRejectsReduceOnlyOrdersThatIncreaseExposure(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{})

	buy := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-2",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		Quantity:      decimal.RequireFromString("0.5"),
		ReduceOnly:    true,
	}
	require.ErrorIs(t, engine.Check(buy), ErrRiskRejected)

	sell := buy
	sell.ClientOrderID = "client-3"
	sell.Side = model.OrderSideSell
	require.NoError(t, engine.Check(sell))

	sell.ClientOrderID = "client-4"
	sell.Quantity = decimal.RequireFromString("2")
	require.ErrorIs(t, engine.Check(sell), ErrRiskRejected)
}

func TestEngineTradingStateHaltedAndReducing(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-trading-state",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("0.5"),
	}

	halted := NewEngine(c, Config{TradingState: TradingStateHalted})
	require.ErrorIs(t, halted.Check(order), ErrRiskRejected)

	reducing := NewEngine(c, Config{TradingState: TradingStateReducing})
	require.ErrorIs(t, reducing.Check(order), ErrRiskRejected)

	order.Side = model.OrderSideSell
	require.NoError(t, reducing.Check(order))

	order.Quantity = decimal.RequireFromString("2")
	require.ErrorIs(t, reducing.Check(order), ErrRiskRejected)
}

func TestEngineRuntimeKillSwitchTransition(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{})
	order := validRiskOrder(inst.ID, "runtime-kill-client")
	require.NoError(t, engine.Check(order))

	require.NoError(t, engine.EngageKillSwitch("operator halt"))
	health := engine.Health()
	require.Equal(t, TradingStateHalted, health.TradingState)
	require.Equal(t, "operator halt", health.TradingStateReason)

	err := engine.Check(order)
	require.ErrorIs(t, err, ErrRiskRejected)
	require.ErrorContains(t, err, "operator halt")

	require.NoError(t, engine.ResumeTrading())
	health = engine.Health()
	require.Equal(t, TradingStateActive, health.TradingState)
	require.Empty(t, health.TradingStateReason)
	require.NoError(t, engine.Check(order))
}

func TestEngineRuntimeReducingOnlyTransition(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{})
	require.NoError(t, engine.SetReducingOnly("drawdown limit"))

	buy := validRiskOrder(inst.ID, "runtime-reducing-buy")
	err := engine.Check(buy)
	require.ErrorIs(t, err, ErrRiskRejected)
	require.ErrorContains(t, err, "drawdown limit")

	sell := buy
	sell.ClientOrderID = "runtime-reducing-sell"
	sell.Side = model.OrderSideSell
	require.NoError(t, engine.Check(sell))
	require.Equal(t, TradingStateReducing, engine.Health().TradingState)
}

func TestEngineRejectsDuplicateClientOrderID(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutOrder(model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   inst.ID,
		OrderID:        "existing-order",
		ClientOrderID:  "duplicate-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
		Price:          decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{})

	order := validRiskOrder(inst.ID, "duplicate-client")
	err := engine.Check(order)
	require.ErrorIs(t, err, ErrRiskRejected)
	require.ErrorContains(t, err, "duplicate client order ID")

	order.AccountID = "other-acct"
	require.NoError(t, engine.Check(order))
}

func TestEngineCheckExistingOrderAllowsCurrentClientOrderID(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutOrder(model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   inst.ID,
		OrderID:        "existing-order",
		ClientOrderID:  "modify-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
		Price:          decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{})

	require.NoError(t, engine.CheckExistingOrder(validRiskOrder(inst.ID, "modify-client")))
}

func TestEngineRejectsWhenCommandRateLimitExceeded(t *testing.T) {
	clock := kernel.NewTestClock(time.Unix(10, 0))
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{
		Clock:                clock,
		MaxCommandsPerWindow: 2,
		CommandRateWindow:    time.Minute,
	})

	require.NoError(t, engine.Check(validRiskOrder(inst.ID, "rate-client-1")))
	require.NoError(t, engine.Check(validRiskOrder(inst.ID, "rate-client-2")))

	err := engine.Check(validRiskOrder(inst.ID, "rate-client-3"))
	require.ErrorIs(t, err, ErrRiskRejected)
	require.ErrorContains(t, err, "command rate limit exceeded")
	require.Equal(t, int64(1), engine.Health().ThrottledCommands)

	clock.Advance(time.Minute)
	require.NoError(t, engine.Check(validRiskOrder(inst.ID, "rate-client-4")))
}

func TestEngineRejectsWhenOpenOrderLimitExceeded(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutOrder(model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   inst.ID,
		OrderID:        "existing-open",
		ClientOrderID:  "existing-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
		Price:          decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{MaxOpenOrders: 1})

	err := engine.Check(validRiskOrder(inst.ID, "new-open-client"))
	require.ErrorIs(t, err, ErrRiskRejected)
	require.ErrorContains(t, err, "max open orders exceeded")

	require.NoError(t, engine.CheckExistingOrder(validRiskOrder(inst.ID, "existing-client")))
}

func TestEngineRejectsMarketOrdersExceedingNotionalUsingCachedMarketPrice(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: inst.ID,
		Last:         decimal.RequireFromString("101"),
	}}))
	engine := NewEngine(c, Config{
		MaxOrderNotional: decimal.RequireFromString("1000"),
	})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-market-risk",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("10"),
	}

	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineRejectsMarketOrdersExceedingNotionalUsingQuoteTick(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: inst.ID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("2"),
		AskSize:      decimal.RequireFromString("2"),
	}}))
	engine := NewEngine(c, Config{
		MaxOrderNotional: decimal.RequireFromString("1000"),
	})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-quote-risk",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("10"),
	}

	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineRejectsOrderNotionalWhenPriceUnavailable(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{MaxOrderNotional: decimal.RequireFromString("1000")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-missing-price",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("1"),
	}

	err := engine.Check(order)
	require.ErrorIs(t, err, ErrRiskRejected)
	require.ErrorContains(t, err, "cannot estimate order notional")
}

func TestEngineRejectsOrdersExceedingProjectedPositionNotional(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("9"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{MaxPositionNotional: decimal.RequireFromString("1000")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-position-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("2"),
		Price:         decimal.RequireFromString("100"),
	}
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)

	order.Side = model.OrderSideSell
	require.NoError(t, engine.Check(order))
}

func TestEngineRejectsOrdersExceedingProjectedPositionNotionalIncludingOpenOrders(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutOrder(model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   inst.ID,
		OrderID:        "open-buy",
		ClientOrderID:  "open-buy-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("9"),
		LeavesQuantity: decimal.RequireFromString("9"),
		Price:          decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{MaxPositionNotional: decimal.RequireFromString("1000")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-position-open-order-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("2"),
		Price:         decimal.RequireFromString("100"),
	}
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineRejectsOrdersExceedingProjectedAccountExposure(t *testing.T) {
	c := cache.New()
	btc := riskInstrument()
	eth := riskInstrument()
	eth.ID = model.MustInstrumentID("ETH-USDT-PERP.BINANCE")
	eth.RawSymbol = "ETHUSDT"
	eth.Base = "ETH"
	require.NoError(t, c.PutInstrument(btc))
	require.NoError(t, c.PutInstrument(eth))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: btc.ID,
		PositionID:   model.PositionID(btc.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("4"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: eth.ID,
		PositionID:   model.PositionID(eth.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("200"),
	}))
	engine := NewEngine(c, Config{MaxAccountExposure: decimal.RequireFromString("700")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  btc.ID,
		ClientOrderID: "client-account-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("2"),
		Price:         decimal.RequireFromString("100"),
	}
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)

	order.Quantity = decimal.RequireFromString("1")
	require.NoError(t, engine.Check(order))
}

func TestEngineAppliesAccountStrategyAndInstrumentScopedOrderLimits(t *testing.T) {
	c := cache.New()
	btc := riskInstrument()
	eth := riskInstrument()
	eth.ID = model.MustInstrumentID("ETH-USDT-PERP.BINANCE")
	eth.RawSymbol = "ETHUSDT"
	eth.Base = "ETH"
	require.NoError(t, c.PutInstrument(btc))
	require.NoError(t, c.PutInstrument(eth))
	engine := NewEngine(c, Config{
		MaxOrderNotional: decimal.RequireFromString("1000"),
		AccountLimits: map[model.AccountID]Limits{
			"acct-tight": {MaxOrderNotional: decimal.RequireFromString("150")},
		},
		StrategyLimits: map[model.StrategyID]Limits{
			"mean-reversion": {MaxOrderNotional: decimal.RequireFromString("120")},
		},
		InstrumentLimits: map[model.InstrumentID]Limits{
			eth.ID: {MaxOrderNotional: decimal.RequireFromString("90")},
		},
	})

	accountScoped := validRiskOrder(btc.ID, "account-scoped-limit")
	accountScoped.AccountID = "acct-tight"
	accountScoped.Quantity = decimal.RequireFromString("2")
	require.ErrorIs(t, engine.Check(accountScoped), ErrRiskRejected)
	require.ErrorContains(t, engine.Check(accountScoped), "account acct-tight max order notional exceeded")

	strategyScoped := validRiskOrder(btc.ID, "strategy-scoped-limit")
	strategyScoped.Metadata.StrategyID = "mean-reversion"
	strategyScoped.Quantity = decimal.RequireFromString("2")
	require.ErrorIs(t, engine.Check(strategyScoped), ErrRiskRejected)
	require.ErrorContains(t, engine.Check(strategyScoped), "strategy mean-reversion max order notional exceeded")

	instrumentScoped := validRiskOrder(eth.ID, "instrument-scoped-limit")
	require.ErrorIs(t, engine.Check(instrumentScoped), ErrRiskRejected)
	require.ErrorContains(t, engine.Check(instrumentScoped), "instrument ETH-USDT-PERP.BINANCE max order notional exceeded")

	unscoped := validRiskOrder(btc.ID, "unscoped-limit")
	unscoped.Quantity = decimal.RequireFromString("2")
	require.NoError(t, engine.Check(unscoped))
}

func TestEngineAppliesScopedPositionAndExposureLimits(t *testing.T) {
	c := cache.New()
	btc := riskInstrument()
	eth := riskInstrument()
	eth.ID = model.MustInstrumentID("ETH-USDT-PERP.BINANCE")
	eth.RawSymbol = "ETHUSDT"
	eth.Base = "ETH"
	require.NoError(t, c.PutInstrument(btc))
	require.NoError(t, c.PutInstrument(eth))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: btc.ID,
		PositionID:   model.PositionID(btc.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct-exposure",
		InstrumentID: eth.ID,
		PositionID:   model.PositionID(eth.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{
		InstrumentLimits: map[model.InstrumentID]Limits{
			btc.ID: {MaxPositionNotional: decimal.RequireFromString("150")},
		},
		AccountLimits: map[model.AccountID]Limits{
			"acct-exposure": {MaxAccountExposure: decimal.RequireFromString("150")},
		},
	})

	positionScoped := validRiskOrder(btc.ID, "scoped-position-limit")
	require.ErrorIs(t, engine.Check(positionScoped), ErrRiskRejected)
	require.ErrorContains(t, engine.Check(positionScoped), "instrument BTC-USDT-PERP.BINANCE max position notional exceeded")

	exposureScoped := validRiskOrder(btc.ID, "scoped-exposure-limit")
	exposureScoped.AccountID = "acct-exposure"
	require.ErrorIs(t, engine.Check(exposureScoped), ErrRiskRejected)
	require.ErrorContains(t, engine.Check(exposureScoped), "account acct-exposure max account exposure exceeded")
}

func TestEngineUsesQuoteTickMarksForProjectedAccountExposure(t *testing.T) {
	c := cache.New()
	btc := riskInstrument()
	eth := riskInstrument()
	eth.ID = model.MustInstrumentID("ETH-USDT-PERP.BINANCE")
	eth.RawSymbol = "ETHUSDT"
	eth.Base = "ETH"
	require.NoError(t, c.PutInstrument(btc))
	require.NoError(t, c.PutInstrument(eth))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: eth.ID,
		PositionID:   model.PositionID(eth.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: eth.ID,
		BidPrice:     decimal.RequireFromString("300"),
		AskPrice:     decimal.RequireFromString("301"),
		BidSize:      decimal.RequireFromString("2"),
		AskSize:      decimal.RequireFromString("2"),
	}}))
	engine := NewEngine(c, Config{MaxAccountExposure: decimal.RequireFromString("350")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  btc.ID,
		ClientOrderID: "client-account-quote-mark",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("100"),
	}

	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineConvertsProjectedAccountExposureToAccountBaseCurrency(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	inst.ID = model.MustInstrumentID("BTC-EUR-PERP.BINANCE")
	inst.RawSymbol = "BTCEUR"
	inst.Quote = "EUR"
	inst.Settle = "EUR"
	xrate := model.Instrument{
		ID:        model.MustInstrumentID("EUR-USD-SPOT.BINANCE"),
		RawSymbol: "EURUSD",
		Type:      model.InstrumentTypeSpot,
		Base:      "EUR",
		Quote:     "USD",
		PriceTick: decimal.RequireFromString("0.0001"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutInstrument(xrate))
	c.PutAccount(model.AccountSnapshot{
		AccountID:    "acct",
		Venue:        "BINANCE",
		Type:         model.AccountTypeMargin,
		BaseCurrency: "USD",
	})
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("9"),
		EntryPrice:   decimal.RequireFromString("10"),
	}))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: xrate.ID,
		BidPrice:     decimal.RequireFromString("1.20"),
		AskPrice:     decimal.RequireFromString("1.20"),
		BidSize:      decimal.RequireFromString("1000"),
		AskSize:      decimal.RequireFromString("1000"),
	}}))
	engine := NewEngine(c, Config{MaxAccountExposure: decimal.RequireFromString("115")})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-base-exposure-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("10"),
	}

	err := engine.Check(order)
	require.ErrorIs(t, err, ErrRiskRejected)
	require.ErrorContains(t, err, "max account exposure exceeded")
}

func TestEngineRejectsOrdersExceedingAvailableInitialMargin(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	inst.MarginInit = decimal.RequireFromString("0.10")
	inst.MarginMaint = decimal.RequireFromString("0.05")
	require.NoError(t, c.PutInstrument(inst))
	c.PutAccount(model.AccountSnapshot{
		AccountID: "acct",
		Venue:     "BINANCE",
		Type:      model.AccountTypeMargin,
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "90",
			Locked:   "10",
			Total:    "100",
		}},
		Margins: []model.MarginBalance{{
			Currency:    "USDT",
			Initial:     "20",
			Maintenance: "10",
		}},
	})
	engine := NewEngine(c, Config{})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-margin-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("1000"),
	}
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)

	order.Quantity = decimal.RequireFromString("0.5")
	require.NoError(t, engine.Check(order))
}

func TestEngineRejectsOrdersExceedingAvailableInitialMarginIncludingOpenOrders(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	inst.MarginInit = decimal.RequireFromString("0.10")
	require.NoError(t, c.PutInstrument(inst))
	c.PutAccount(model.AccountSnapshot{
		AccountID: "acct",
		Venue:     "BINANCE",
		Type:      model.AccountTypeMargin,
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "100",
			Total:    "100",
		}},
	})
	require.NoError(t, c.PutOrder(model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   inst.ID,
		OrderID:        "open-margin-buy",
		ClientOrderID:  "open-margin-buy-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("9"),
		LeavesQuantity: decimal.RequireFromString("9"),
		Price:          decimal.RequireFromString("100"),
	}))
	engine := NewEngine(c, Config{})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-margin-open-order-limit",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("2"),
		Price:         decimal.RequireFromString("100"),
	}
	require.ErrorIs(t, engine.Check(order), ErrRiskRejected)
}

func TestEngineRejectsInvalidTimeInForce(t *testing.T) {
	c := cache.New()
	inst := riskInstrument()
	require.NoError(t, c.PutInstrument(inst))
	engine := NewEngine(c, Config{})

	order := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-bad-tif",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForce("until-bored"),
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("100"),
	}

	require.ErrorIs(t, engine.Check(order), model.ErrInvalidOrder)
}

func riskInstrument() model.Instrument {
	return model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypePerp,
		Base:      "BTC",
		Quote:     "USDT",
		Settle:    "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}
}

func validRiskOrder(instrumentID model.InstrumentID, clientID model.ClientOrderID) model.SubmitOrder {
	return model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  instrumentID,
		ClientOrderID: clientID,
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("100"),
	}
}
