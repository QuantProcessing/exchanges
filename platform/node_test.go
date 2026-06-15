package platform

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/execution"
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestNodeStartLoadsInstrumentsConnectsClientsAndPublishesStartupReports(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()

	require.NoError(t, node.Start(context.Background()))
	require.Equal(t, []string{"load_all"}, data.provider.calls)
	require.Equal(t, []string{"data_connect"}, data.calls)
	require.Equal(t, []string{
		"exec_connect",
		"query_account",
		"reports:BTC-USDT-SPOT.BINANCE",
		"fills:BTC-USDT-SPOT.BINANCE",
		"positions:BTC-USDT-SPOT.BINANCE",
	}, exec.Calls())
	require.True(t, node.Health().Ready)
	require.Len(t, node.Cache().Instruments(), 1)

	var sawAccount bool
	var sawOrder bool
	for i := 0; i < 2; i++ {
		env := <-events.C()
		event := env.Message.(model.ExecutionEvent)
		sawAccount = sawAccount || event.Account != nil
		sawOrder = sawOrder || event.Order != nil
	}
	require.True(t, sawAccount)
	require.True(t, sawOrder)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeHealthTracksKernelLifecycleState(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.Equal(t, kernel.ComponentStateInitialized, node.Health().State)
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))

	require.NoError(t, node.Start(context.Background()))
	require.True(t, node.Health().Ready)
	require.Equal(t, kernel.ComponentStateRunning, node.Health().State)

	require.NoError(t, node.Stop(context.Background()))
	require.False(t, node.Health().Ready)
	require.Equal(t, kernel.ComponentStateStopped, node.Health().State)
}

func TestNodeDrivesConfiguredRiskEngineLifecycle(t *testing.T) {
	engine := risk.NewEngine(cache.New(), risk.Config{})
	node := NewNode(Config{Risk: engine})
	require.Equal(t, kernel.ComponentStateInitialized, node.Health().Risk.State)

	require.NoError(t, node.Start(context.Background()))
	require.Equal(t, kernel.ComponentStateRunning, node.Health().Risk.State)

	require.NoError(t, node.Stop(context.Background()))
	require.Equal(t, kernel.ComponentStateStopped, node.Health().Risk.State)
}

func TestNodeHealthFaultsWhenStartupFails(t *testing.T) {
	exec := newFakeExecutionClient()
	exec.connectErr = fmt.Errorf("connect failed")
	node := NewNode(Config{})
	require.NoError(t, node.AddExecutionClient(exec))

	require.ErrorContains(t, node.Start(context.Background()), "connect failed")
	health := node.Health()
	require.False(t, health.Ready)
	require.Equal(t, kernel.ComponentStateFaulted, health.State)
	require.EqualError(t, health.LastError, "connect failed")
}

func TestNodeSetTimerPublishesTimerEventsAndCanCancel(t *testing.T) {
	node := NewNode(Config{})
	events := node.Bus().Subscribe(strategy.TopicTimer, 4)
	defer events.Close()

	require.NoError(t, node.SetTimer(context.Background(), "heartbeat", 10*time.Millisecond))
	select {
	case env := <-events.C():
		event := env.Message.(strategy.TimerEvent)
		require.Equal(t, "heartbeat", event.Name)
		require.False(t, event.Timestamp.IsZero())
	case <-time.After(time.Second):
		t.Fatal("timer event not published")
	}
	require.NoError(t, node.CancelTimer(context.Background(), "heartbeat"))

	node.mu.RLock()
	_, ok := node.timers["heartbeat"]
	node.mu.RUnlock()
	require.False(t, ok)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeSubmitOrderAppliesPublishesAndTracksReport(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()

	submit := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-submit-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
		Metadata: model.CommandMetadata{
			TraderID:      "trader-1",
			StrategyID:    "strategy-1",
			CommandID:     "command-1",
			CorrelationID: "corr-1",
			Params:        map[string]string{"route": "platform"},
		},
	}

	report, err := node.SubmitOrder(context.Background(), submit)
	require.NoError(t, err)
	require.Equal(t, model.OrderID("submitted-1"), report.OrderID)
	require.Equal(t, submit.Metadata.CommandID, report.Metadata.CommandID)
	require.Equal(t, []string{
		"exec_connect",
		"query_account",
		"reports:BTC-USDT-SPOT.BINANCE",
		"fills:BTC-USDT-SPOT.BINANCE",
		"positions:BTC-USDT-SPOT.BINANCE",
		"submit:client-submit-1",
	}, exec.Calls())

	got, ok := node.Cache().OrderByClientID("acct", "client-submit-1")
	require.True(t, ok)
	require.Equal(t, report, got)
	require.Equal(t, "platform", got.Metadata.Params["route"])

	orderEvent := readOrderReport(t, events)
	require.Equal(t, model.OrderID("submitted-1"), orderEvent.OrderID)
	require.Equal(t, submit.Metadata.CorrelationID, orderEvent.Metadata.CorrelationID)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeSubmitOrderDelegatesThroughExecutionEngine(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	defer node.Stop(context.Background())

	submit := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-engine-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
	}

	report, err := node.SubmitOrder(context.Background(), submit)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusAccepted, report.Status)
	require.NotNil(t, node.ExecutionEngine())
	cachedCommand, ok := node.ExecutionEngine().Manager().SubmitCommand("client-engine-1")
	require.True(t, ok, "platform submit must pass through execution.Engine manager")
	require.Equal(t, submit.ClientOrderID, cachedCommand.ClientOrderID)
	require.Equal(t, int64(1), node.ExecutionEngine().Health().Submits)
	require.Contains(t, exec.Calls(), "submit:client-engine-1")
}

func TestNodeSubmitOrderPublishesSubmittedAndAcceptedLifecycle(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()

	submit := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-submit-lifecycle",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
	}
	report, err := node.SubmitOrder(context.Background(), submit)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusAccepted, report.Status)

	submitted := readOrderLifecycle(t, events)
	require.Equal(t, model.OrderEventSubmitted, submitted.Kind)
	require.Equal(t, model.OrderStatusInitialized, submitted.PreviousStatus)
	require.Equal(t, model.ClientOrderID("client-submit-lifecycle"), submitted.ClientOrderID)

	accepted := readOrderLifecycle(t, events)
	require.Equal(t, model.OrderEventAccepted, accepted.Kind)
	require.Equal(t, model.OrderStatusSubmitted, accepted.PreviousStatus)
	require.NotNil(t, accepted.Report)
	require.Equal(t, report.OrderID, accepted.Report.OrderID)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeSubmitOrderPublishesRejectedLifecycleWhenVenueRejects(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	exec.submitErr = fmt.Errorf("venue rejected submit")
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()

	submit := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-submit-rejected",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
	}
	_, err := node.SubmitOrder(context.Background(), submit)
	require.ErrorContains(t, err, "venue rejected submit")

	submitted := readOrderLifecycle(t, events)
	require.Equal(t, model.OrderEventSubmitted, submitted.Kind)
	rejected := readOrderLifecycle(t, events)
	require.Equal(t, model.OrderEventRejected, rejected.Kind)
	require.Equal(t, model.OrderStatusSubmitted, rejected.PreviousStatus)
	require.Contains(t, rejected.Reason, "venue rejected submit")
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeQueryOrderReturnsCachedOrderByClientID(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	submitted := submitRestingOrder(t, node, instID, "client-query-1")
	report, err := node.QueryOrder(context.Background(), model.QueryOrder{
		Metadata: model.CommandMetadata{
			CommandID: "query-command",
		},
		AccountID:     "acct",
		InstrumentID:  instID,
		ClientOrderID: "client-query-1",
	})
	require.NoError(t, err)
	require.Equal(t, submitted.OrderID, report.OrderID)
	require.Equal(t, model.OrderStatusAccepted, report.Status)
	require.Equal(t, model.CommandID("query-command"), report.Metadata.CommandID)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeQueryAccountRefreshesCachesAndPublishesSnapshot(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()

	snapshot, err := node.QueryAccount(context.Background(), model.QueryAccount{AccountID: "acct"})
	require.NoError(t, err)
	require.Equal(t, model.AccountID("acct"), snapshot.AccountID)
	require.GreaterOrEqual(t, countCalls(exec.Calls(), "query_account"), 2)

	cached, ok := node.Cache().Account("acct")
	require.True(t, ok)
	require.Equal(t, snapshot, cached)
	published := readAccountSnapshot(t, events)
	require.Equal(t, snapshot, published)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeSubmitOrderListHoldsChildrenUntilParentFilledAndCancelsOcoSibling(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	instrumentID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	list := node.OrderFactory("acct").Bracket(model.BracketOrderRequest{
		InstrumentID: instrumentID,
		Side:         model.OrderSideBuy,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
		TakeProfit:   decimal.RequireFromString("110"),
		StopLoss:     decimal.RequireFromString("95"),
	})
	list.Metadata = model.CommandMetadata{CommandID: "list-command"}
	reports, err := node.SubmitOrderList(context.Background(), list)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.CommandID("list-command"), reports[0].Metadata.CommandID)
	require.Contains(t, exec.Calls(), "submit:acct-1")
	require.NotContains(t, exec.Calls(), "submit:acct-2")
	require.NotContains(t, exec.Calls(), "submit:acct-3")

	entry, ok := node.Cache().OrderByClientID("acct", "acct-1")
	require.True(t, ok)
	require.Equal(t, model.CommandID("list-command"), entry.Metadata.CommandID)
	_, ok = node.Cache().OrderByClientID("acct", "acct-2")
	require.False(t, ok)
	_, ok = node.Cache().OrderByClientID("acct", "acct-3")
	require.False(t, ok)

	require.NoError(t, node.applyAndPublish(context.Background(), node.reconcilerFor("acct"), model.ExecutionEvent{Fill: &model.FillReport{
		AccountID:     "acct",
		InstrumentID:  instrumentID,
		OrderID:       entry.OrderID,
		ClientOrderID: entry.ClientOrderID,
		TradeID:       "entry-fill",
		Side:          model.OrderSideBuy,
		Price:         decimal.RequireFromString("100"),
		Quantity:      decimal.RequireFromString("1"),
		Timestamp:     time.Unix(100, 0),
	}}))
	require.Contains(t, exec.Calls(), "submit:acct-2")
	require.Contains(t, exec.Calls(), "submit:acct-3")
	stopLoss, ok := node.Cache().OrderByClientID("acct", "acct-2")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, stopLoss.Status)
	require.Equal(t, model.CommandID("list-command"), stopLoss.Metadata.CommandID)
	takeProfit, ok := node.Cache().OrderByClientID("acct", "acct-3")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, takeProfit.Status)
	require.Equal(t, model.CommandID("list-command"), takeProfit.Metadata.CommandID)

	require.NoError(t, node.applyAndPublish(context.Background(), node.reconcilerFor("acct"), model.ExecutionEvent{Fill: &model.FillReport{
		AccountID:     "acct",
		InstrumentID:  instrumentID,
		OrderID:       takeProfit.OrderID,
		ClientOrderID: takeProfit.ClientOrderID,
		TradeID:       "take-profit-fill",
		Side:          model.OrderSideSell,
		Price:         decimal.RequireFromString("110"),
		Quantity:      decimal.RequireFromString("1"),
		Timestamp:     time.Unix(101, 0),
	}}))
	stopLoss, ok = node.Cache().OrderByClientID("acct", "acct-2")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, stopLoss.Status)
	require.Equal(t, model.CommandID("list-command"), stopLoss.Metadata.CommandID)
	require.Contains(t, exec.Calls(), "cancel:acct-2")
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeModifyOrderPublishesPendingUpdateAndUpdated(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()

	submit := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-modify-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
	}
	accepted, err := node.SubmitOrder(context.Background(), submit)
	require.NoError(t, err)

	report, err := node.ModifyOrder(context.Background(), model.ModifyOrder{
		Metadata: model.CommandMetadata{
			CommandID: "modify-command",
		},
		AccountID:     "acct",
		InstrumentID:  submit.InstrumentID,
		OrderID:       accepted.OrderID,
		ClientOrderID: submit.ClientOrderID,
		Price:         decimal.RequireFromString("101"),
	})
	require.NoError(t, err)
	require.Equal(t, "101", report.Price.String())
	require.Equal(t, model.CommandID("modify-command"), report.Metadata.CommandID)
	require.Contains(t, exec.Calls(), "modify:client-modify-1")

	pending := readOrderLifecycleKind(t, events, model.OrderEventPendingUpdate)
	require.Equal(t, model.OrderEventPendingUpdate, pending.Kind)
	require.Equal(t, model.CommandID("modify-command"), pending.Metadata.CommandID)
	updated := readOrderLifecycleKind(t, events, model.OrderEventUpdated)
	require.Equal(t, model.OrderEventUpdated, updated.Kind)
	require.Equal(t, model.CommandID("modify-command"), updated.Metadata.CommandID)

	got, ok := node.Cache().OrderByClientID("acct", "client-modify-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, got.Status)
	require.Equal(t, "101", got.Price.String())
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeCancelModifyQueryDelegateThroughExecutionEngine(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	defer node.Stop(context.Background())

	submit := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-engine-cmq",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
	}
	report, err := node.SubmitOrder(context.Background(), submit)
	require.NoError(t, err)

	_, err = node.ModifyOrder(context.Background(), model.ModifyOrder{
		AccountID:     "acct",
		InstrumentID:  submit.InstrumentID,
		ClientOrderID: submit.ClientOrderID,
		OrderID:       report.OrderID,
		Quantity:      decimal.RequireFromString("0.4"),
		Price:         decimal.RequireFromString("99"),
	})
	require.NoError(t, err)
	_, err = node.QueryOrder(context.Background(), model.QueryOrder{
		AccountID:     "acct",
		InstrumentID:  submit.InstrumentID,
		ClientOrderID: "client-engine-query",
	})
	require.NoError(t, err)
	_, err = node.CancelOrder(context.Background(), model.CancelOrder{
		AccountID:     "acct",
		InstrumentID:  submit.InstrumentID,
		ClientOrderID: submit.ClientOrderID,
		OrderID:       report.OrderID,
	})
	require.NoError(t, err)

	health := node.ExecutionEngine().Health()
	require.Equal(t, int64(1), health.Submits)
	require.Equal(t, int64(1), health.Modifies)
	require.Equal(t, int64(1), health.Queries)
	require.Equal(t, int64(1), health.Cancels)
	require.Contains(t, exec.Calls(), "modify:client-engine-cmq")
	require.Contains(t, exec.Calls(), "cancel:client-engine-cmq")
}

func TestNodeModifyOrderPublishesRejectedWhenVenueRejects(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	exec.modifyErr = fmt.Errorf("venue rejected modify")
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()

	submit := model.SubmitOrder{
		Metadata: model.CommandMetadata{
			CommandID: "submit-command",
		},
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-modify-rejected",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
	}
	_, err := node.SubmitOrder(context.Background(), submit)
	require.NoError(t, err)

	_, err = node.ModifyOrder(context.Background(), model.ModifyOrder{
		AccountID:     "acct",
		InstrumentID:  submit.InstrumentID,
		ClientOrderID: submit.ClientOrderID,
		Price:         decimal.RequireFromString("101"),
	})
	require.ErrorContains(t, err, "venue rejected modify")

	pending := readOrderLifecycleKind(t, events, model.OrderEventPendingUpdate)
	require.Equal(t, model.OrderEventPendingUpdate, pending.Kind)
	rejected := readOrderLifecycleKind(t, events, model.OrderEventModifyRejected)
	require.Equal(t, model.OrderEventModifyRejected, rejected.Kind)
	require.Contains(t, rejected.Reason, "venue rejected modify")

	got, ok := node.Cache().OrderByClientID("acct", "client-modify-rejected")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, got.Status)
	require.Equal(t, "100", got.Price.String())
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeModifyOrderChecksRiskBeforeVenueModification(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	c := cache.New()
	node := NewNode(Config{
		Cache: c,
		Risk:  risk.NewEngine(c, risk.Config{}),
	})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()

	submit := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-modify-risk",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
	}
	_, err := node.SubmitOrder(context.Background(), submit)
	require.NoError(t, err)

	_, err = node.ModifyOrder(context.Background(), model.ModifyOrder{
		AccountID:     "acct",
		InstrumentID:  submit.InstrumentID,
		ClientOrderID: submit.ClientOrderID,
		Price:         decimal.RequireFromString("100.001"),
	})
	require.ErrorIs(t, err, model.ErrInvalidOrder)
	require.NotContains(t, exec.Calls(), "modify:client-modify-risk")

	require.Equal(t, model.OrderEventPendingUpdate, readOrderLifecycleKind(t, events, model.OrderEventPendingUpdate).Kind)
	rejected := readOrderLifecycleKind(t, events, model.OrderEventModifyRejected)
	require.Equal(t, model.OrderEventModifyRejected, rejected.Kind)

	got, ok := node.Cache().OrderByClientID("acct", "client-modify-risk")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, got.Status)
	require.Equal(t, "100", got.Price.String())
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeCancelOrderPublishesPendingCancelAndCanceled(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()

	submit := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-cancel-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
	}
	accepted, err := node.SubmitOrder(context.Background(), submit)
	require.NoError(t, err)

	report, err := node.CancelOrder(context.Background(), model.CancelOrder{
		Metadata: model.CommandMetadata{
			CommandID: "cancel-command",
		},
		AccountID:     "acct",
		InstrumentID:  submit.InstrumentID,
		OrderID:       accepted.OrderID,
		ClientOrderID: submit.ClientOrderID,
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusCanceled, report.Status)
	require.Equal(t, model.CommandID("cancel-command"), report.Metadata.CommandID)

	pending := readOrderLifecycleKind(t, events, model.OrderEventPendingCancel)
	require.Equal(t, model.OrderEventPendingCancel, pending.Kind)
	require.Equal(t, model.CommandID("cancel-command"), pending.Metadata.CommandID)
	canceled := readOrderLifecycleKind(t, events, model.OrderEventCanceled)
	require.Equal(t, model.OrderEventCanceled, canceled.Kind)
	require.Equal(t, model.CommandID("cancel-command"), canceled.Metadata.CommandID)

	got, ok := node.Cache().OrderByClientID("acct", "client-cancel-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, got.Status)
	require.Contains(t, exec.Calls(), "cancel:client-cancel-1")
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeCancelOrderPublishesRejectedWhenVenueRejects(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	exec.cancelErr = fmt.Errorf("venue rejected cancel")
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()

	submit := model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-cancel-rejected",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100"),
	}
	_, err := node.SubmitOrder(context.Background(), submit)
	require.NoError(t, err)

	_, err = node.CancelOrder(context.Background(), model.CancelOrder{
		AccountID:     "acct",
		InstrumentID:  submit.InstrumentID,
		ClientOrderID: submit.ClientOrderID,
	})
	require.ErrorContains(t, err, "venue rejected cancel")

	pending := readOrderLifecycleKind(t, events, model.OrderEventPendingCancel)
	require.Equal(t, model.OrderEventPendingCancel, pending.Kind)
	rejected := readOrderLifecycleKind(t, events, model.OrderEventCancelRejected)
	require.Equal(t, model.OrderEventCancelRejected, rejected.Kind)
	require.Contains(t, rejected.Reason, "venue rejected cancel")

	got, ok := node.Cache().OrderByClientID("acct", "client-cancel-rejected")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, got.Status)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeBatchCancelOrdersCancelsEachOrder(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	first := submitRestingOrder(t, node, instID, "batch-cancel-1")
	second := submitRestingOrder(t, node, instID, "batch-cancel-2")

	reports, err := node.BatchCancelOrders(context.Background(), model.BatchCancelOrders{
		AccountID:    "acct",
		InstrumentID: instID,
		Cancels: []model.CancelOrder{
			{AccountID: "acct", InstrumentID: instID, OrderID: first.OrderID, ClientOrderID: first.ClientOrderID},
			{AccountID: "acct", InstrumentID: instID, OrderID: second.OrderID, ClientOrderID: second.ClientOrderID},
		},
	})
	require.NoError(t, err)
	require.Len(t, reports, 2)
	require.Equal(t, model.OrderStatusCanceled, reports[0].Status)
	require.Equal(t, model.OrderStatusCanceled, reports[1].Status)
	require.Contains(t, exec.Calls(), "cancel:batch-cancel-1")
	require.Contains(t, exec.Calls(), "cancel:batch-cancel-2")
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeCancelAllOrdersCancelsOpenOrdersForInstrument(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	submitRestingOrder(t, node, instID, "cancel-all-1")
	submitRestingOrder(t, node, instID, "cancel-all-2")

	reports, err := node.CancelAllOrders(context.Background(), model.CancelAllOrders{
		Metadata: model.CommandMetadata{
			CommandID: "cancel-all-command",
		},
		AccountID:    "acct",
		InstrumentID: instID,
	})
	require.NoError(t, err)
	require.Len(t, reports, 3)
	for _, report := range reports {
		require.Equal(t, model.OrderStatusCanceled, report.Status)
		require.Equal(t, model.CommandID("cancel-all-command"), report.Metadata.CommandID)
	}
	require.Empty(t, node.Cache().OpenOrders("acct"))
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeCancelAllOrdersFiltersByOrderSide(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	submitRestingOrderWithSide(t, node, instID, "cancel-all-buy", model.OrderSideBuy)
	submitRestingOrderWithSide(t, node, instID, "cancel-all-sell", model.OrderSideSell)

	reports, err := node.CancelAllOrders(context.Background(), model.CancelAllOrders{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderSide:    model.OrderSideBuy,
	})
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.ClientOrderID("cancel-all-buy"), reports[0].ClientOrderID)

	buy, ok := node.Cache().OrderByClientID("acct", "cancel-all-buy")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, buy.Status)

	sell, ok := node.Cache().OrderByClientID("acct", "cancel-all-sell")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, sell.Status)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeChecksRiskBeforeVenueSubmission(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	c := cache.New()
	node := NewNode(Config{
		Cache: c,
		Risk:  risk.NewEngine(c, risk.Config{MaxOrderNotional: decimal.RequireFromString("1000")}),
	})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	_, err := node.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-risk-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("100.001"),
	})
	require.ErrorIs(t, err, model.ErrInvalidOrder)
	require.NotContains(t, exec.Calls(), "submit:client-risk-1")
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodePublishesOrderDeniedWhenRiskRejectsSubmit(t *testing.T) {
	c := cache.New()
	inst := model.Instrument{
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
	require.NoError(t, c.PutInstrument(inst))
	node := NewNode(Config{
		Cache: c,
		Risk:  risk.NewEngine(c, risk.Config{MaxOrderNotional: decimal.RequireFromString("100")}),
	})
	exec := newFakeExecutionClient()
	require.NoError(t, node.AddExecutionClient(exec))
	events := node.Bus().Subscribe(TopicExecution, 4)
	defer events.Close()

	_, err := node.SubmitOrder(context.Background(), model.SubmitOrder{
		Metadata: model.CommandMetadata{
			TraderID:      "trader-denied",
			StrategyID:    "strategy-denied",
			CommandID:     "command-denied",
			CorrelationID: "correlation-denied",
		},
		AccountID:     "acct",
		InstrumentID:  inst.ID,
		ClientOrderID: "client-denied-risk",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("2"),
		Price:         decimal.RequireFromString("100"),
	})
	require.ErrorIs(t, err, risk.ErrRiskRejected)
	require.NotContains(t, exec.Calls(), "submit:client-denied-risk")

	lifecycle := readOrderLifecycle(t, events)
	require.Equal(t, model.OrderEventDenied, lifecycle.Kind)
	require.Equal(t, model.OrderStatusDenied, lifecycle.Status)
	require.Equal(t, model.ClientOrderID("client-denied-risk"), lifecycle.ClientOrderID)
	require.Equal(t, model.CommandID("command-denied"), lifecycle.Metadata.CommandID)
	require.Equal(t, model.CorrelationID("correlation-denied"), lifecycle.Metadata.CorrelationID)
	require.Equal(t, model.StrategyID("strategy-denied"), lifecycle.Metadata.StrategyID)
	require.Contains(t, lifecycle.Reason, "max order notional")
}

func TestNodeForwardsPrivateFillAndPositionEvents(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	fill := model.FillReport{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderID:      "order-1",
		TradeID:      "trade-private-1",
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("0.1"),
		Timestamp:    time.Unix(100, 0),
	}
	position := model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "BTC-USDT-SPOT.BINANCE",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("0.1"),
		EntryPrice:   decimal.RequireFromString("100"),
		Timestamp:    time.Unix(100, 0),
	}
	exec.events <- model.ExecutionEvent{Fill: &fill}
	exec.events <- model.ExecutionEvent{Position: &position}

	var gotPosition model.PositionStatusReport
	require.Eventually(t, func() bool {
		position, ok := node.Cache().PositionByInstrument("acct", instID)
		if ok {
			gotPosition = position
		}
		return len(node.Cache().FillsForOrder("acct", "order-1")) == 1 && ok
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, position, gotPosition)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodePublishesDerivedPositionLifecycleEvents(t *testing.T) {
	node := NewNode(Config{})
	events := node.Bus().Subscribe(TopicExecution, 8)
	defer events.Close()
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")

	position := model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "pos-1",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
		Timestamp:    time.Unix(100, 0),
	}
	require.NoError(t, node.applyAndPublish(context.Background(), node.reconcilerFor("acct"), model.ExecutionEvent{Position: &position}))
	require.Equal(t, model.PositionEventOpened, readPositionLifecycle(t, events).Kind)

	position.Quantity = decimal.RequireFromString("1.5")
	position.Timestamp = time.Unix(101, 0)
	require.NoError(t, node.applyAndPublish(context.Background(), node.reconcilerFor("acct"), model.ExecutionEvent{Position: &position}))
	require.Equal(t, model.PositionEventChanged, readPositionLifecycle(t, events).Kind)

	position.Side = model.PositionSideFlat
	position.Quantity = decimal.Zero
	position.Timestamp = time.Unix(102, 0)
	require.NoError(t, node.applyAndPublish(context.Background(), node.reconcilerFor("acct"), model.ExecutionEvent{Position: &position}))
	require.Equal(t, model.PositionEventClosed, readPositionLifecycle(t, events).Kind)
}

func TestNodeUpdatesPortfolioFromPrivateFills(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	c := cache.New()
	p := portfolio.New(c)
	node := NewNode(Config{Cache: c, Portfolio: p})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	fill := model.FillReport{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderID:      "order-1",
		TradeID:      "portfolio-fill-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("0.2"),
		Timestamp:    time.Unix(101, 0),
	}
	exec.events <- model.ExecutionEvent{Fill: &fill}

	require.Eventually(t, func() bool {
		position, ok := node.Cache().PositionByInstrument("acct", instID)
		return ok && position.Side == model.PositionSideLong && position.Quantity.Equal(decimal.RequireFromString("0.2"))
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeCreatesDefaultPortfolioWithSharedCache(t *testing.T) {
	node := NewNode(Config{})
	require.NotNil(t, node.Portfolio())

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	require.NoError(t, node.Portfolio().ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderID:      "default-portfolio-order",
		TradeID:      "default-portfolio-trade",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("0.2"),
		Timestamp:    time.Unix(101, 0),
	}))

	position, ok := node.Cache().PositionByInstrument("acct", instID)
	require.True(t, ok)
	require.True(t, decimal.RequireFromString("0.2").Equal(position.Quantity))
}

func TestNodeUpdatesPortfolioMarksFromMarketData(t *testing.T) {
	c := cache.New()
	p := portfolio.New(c)
	node := NewNode(Config{Cache: c, Portfolio: p})
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	require.NoError(t, p.ApplyFill(model.FillReport{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderID:      "order-1",
		TradeID:      "portfolio-market-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("1"),
		Timestamp:    time.Unix(100, 0),
	}))

	require.NoError(t, node.applyMarketAndPublish(context.Background(), model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: instID,
		BidPrice:     decimal.RequireFromString("120"),
		AskPrice:     decimal.RequireFromString("121"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
		Timestamp:    time.Unix(101, 0),
	}}))

	require.Equal(t, "20", p.UnrealizedPnL("acct", instID).String())
}

func readOrderLifecycle(t *testing.T, sub bus.Subscription) model.OrderLifecycleEvent {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case env := <-sub.C():
			event := env.Message.(model.ExecutionEvent)
			if event.Lifecycle != nil {
				return *event.Lifecycle
			}
		case <-deadline:
			require.FailNow(t, "timed out waiting for order lifecycle event")
		}
	}
}

func readOrderLifecycleKind(t *testing.T, sub bus.Subscription, kind model.OrderEventKind) model.OrderLifecycleEvent {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case env := <-sub.C():
			event := env.Message.(model.ExecutionEvent)
			if event.Lifecycle != nil && event.Lifecycle.Kind == kind {
				return *event.Lifecycle
			}
		case <-deadline:
			require.FailNow(t, "timed out waiting for order lifecycle event "+string(kind))
		}
	}
}

func readOrderReport(t *testing.T, sub bus.Subscription) model.OrderStatusReport {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case env := <-sub.C():
			event := env.Message.(model.ExecutionEvent)
			if event.Order != nil {
				return *event.Order
			}
		case <-deadline:
			require.FailNow(t, "timed out waiting for order report")
		}
	}
}

func readAccountSnapshot(t *testing.T, sub bus.Subscription) model.AccountSnapshot {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case env := <-sub.C():
			event := env.Message.(model.ExecutionEvent)
			if event.Account != nil {
				return *event.Account
			}
		case <-deadline:
			require.FailNow(t, "timed out waiting for account snapshot")
		}
	}
}

func readPositionLifecycle(t *testing.T, sub bus.Subscription) model.PositionLifecycleEvent {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case env := <-sub.C():
			event := env.Message.(model.ExecutionEvent)
			if event.PositionLifecycle != nil {
				return *event.PositionLifecycle
			}
		case <-deadline:
			require.FailNow(t, "timed out waiting for position lifecycle event")
		}
	}
}

func submitRestingOrder(t *testing.T, node *Node, instID model.InstrumentID, clientOrderID model.ClientOrderID) model.OrderStatusReport {
	return submitRestingOrderWithSide(t, node, instID, clientOrderID, model.OrderSideBuy)
}

func submitRestingOrderWithSide(t *testing.T, node *Node, instID model.InstrumentID, clientOrderID model.ClientOrderID, side model.OrderSide) model.OrderStatusReport {
	t.Helper()
	price := decimal.RequireFromString("100")
	if side == model.OrderSideSell {
		price = decimal.RequireFromString("101")
	}
	report, err := node.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  instID,
		ClientOrderID: clientOrderID,
		Side:          side,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         price,
	})
	require.NoError(t, err)
	return report
}

func TestNodeSubscribesMarketDataForwardsEventsAndCachesLatest(t *testing.T) {
	data := newFakeDataClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicMarketData, 8)
	defer events.Close()

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	sub := model.SubscribeMarketData{InstrumentID: instID, Type: model.MarketDataTypeTicker}
	require.NoError(t, node.SubscribeMarketData(context.Background(), sub))
	require.Contains(t, data.calls, "subscribe:ticker:BTC-USDT-SPOT.BINANCE")

	ticker := model.Ticker{
		InstrumentID: instID,
		Bid:          decimal.RequireFromString("100"),
		Ask:          decimal.RequireFromString("101"),
		Last:         decimal.RequireFromString("100.5"),
		Timestamp:    time.Unix(102, 0),
	}
	data.marketEvents <- model.MarketEvent{Ticker: &ticker}

	env := <-events.C()
	event := env.Message.(model.MarketEvent)
	require.NotNil(t, event.Ticker)
	require.Equal(t, ticker, *event.Ticker)
	gotTicker, ok := node.Cache().Ticker(instID)
	require.True(t, ok)
	require.Equal(t, ticker, gotTicker)

	require.NoError(t, node.UnsubscribeMarketData(context.Background(), sub))
	require.Contains(t, data.calls, "unsubscribe:ticker:BTC-USDT-SPOT.BINANCE")
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeRequestDataFetchesTickerAndCachesLatest(t *testing.T) {
	data := newFakeDataClient()
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	data.ticker = model.Ticker{
		InstrumentID: instID,
		Bid:          decimal.RequireFromString("100"),
		Ask:          decimal.RequireFromString("101"),
		Last:         decimal.RequireFromString("100.5"),
		Timestamp:    time.Unix(200, 0),
	}
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.Start(context.Background()))
	defer node.Stop(context.Background())

	response, err := node.RequestData(context.Background(), model.DataRequest{
		RequestID:    "platform-request-1",
		InstrumentID: instID,
		Type:         model.MarketDataTypeTicker,
	})
	require.NoError(t, err)
	require.True(t, response.IsFinal)
	require.Len(t, response.Events, 1)
	require.Equal(t, data.ticker, *response.Events[0].Ticker)
	require.Contains(t, data.Calls(), "fetch_ticker:BTC-USDT-SPOT.BINANCE")
	cached, ok := node.Cache().Ticker(instID)
	require.True(t, ok)
	require.Equal(t, data.ticker, cached)
}

func TestNodeDelegatesMarketDataLifecycleToSharedDataEngine(t *testing.T) {
	data := newFakeDataClient()
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	data.ticker = model.Ticker{
		InstrumentID: instID,
		Bid:          decimal.RequireFromString("100"),
		Ask:          decimal.RequireFromString("101"),
		Last:         decimal.RequireFromString("100.5"),
		Timestamp:    time.Unix(200, 0),
	}
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	sub := model.SubscribeMarketData{InstrumentID: instID, Type: model.MarketDataTypeTicker}
	require.NoError(t, node.SubscribeMarketData(context.Background(), sub))
	require.Equal(t, 1, node.DataEngine().Health().Subscriptions)

	require.NoError(t, node.Start(context.Background()))
	defer node.Stop(context.Background())
	require.True(t, node.DataEngine().Health().Running)
	require.Contains(t, data.Calls(), "subscribe:ticker:BTC-USDT-SPOT.BINANCE")

	response, err := node.RequestData(context.Background(), model.DataRequest{
		Metadata:     model.CommandMetadata{CommandID: "platform-data-engine-request"},
		RequestID:    "platform-data-engine-1",
		InstrumentID: instID,
		Type:         model.MarketDataTypeTicker,
	})
	require.NoError(t, err)
	require.Equal(t, model.CorrelationID("platform-data-engine-1"), response.Metadata.CorrelationID)
	require.Equal(t, model.CommandID("platform-data-engine-request"), response.Metadata.CommandID)
	require.Equal(t, int64(1), node.DataEngine().Health().Requests)

	health := node.Health()
	require.True(t, health.DataEngine.Running)
	require.Equal(t, 1, health.DataEngine.Clients)
	require.Equal(t, 1, health.DataEngine.Subscriptions)
}

func TestNodeRecoversMarketDataStreamAndResubscribesActiveSubscriptions(t *testing.T) {
	data := newFakeDataClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicMarketData, 4)
	defer events.Close()

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	sub := model.SubscribeMarketData{InstrumentID: instID, Type: model.MarketDataTypeTicker}
	require.NoError(t, node.SubscribeMarketData(context.Background(), sub))
	data.marketEvents <- model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: instID,
		Bid:          decimal.RequireFromString("100"),
		Ask:          decimal.RequireFromString("101"),
		Last:         decimal.RequireFromString("100.5"),
		Timestamp:    time.Unix(102, 0),
	}}
	require.NotNil(t, (<-events.C()).Message.(model.MarketEvent).Ticker)

	data.breakStream()

	require.Eventually(t, func() bool {
		calls := data.Calls()
		return countCalls(calls, "data_connect") >= 2 &&
			countCalls(calls, "subscribe:ticker:BTC-USDT-SPOT.BINANCE") >= 2
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeSubscribesTradeTicksForwardsEventsAndCachesLatest(t *testing.T) {
	data := newFakeDataClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicMarketData, 8)
	defer events.Close()

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	require.NoError(t, node.SubscribeTradeTicks(context.Background(), instID))
	require.Contains(t, data.calls, "subscribe:trade_tick:BTC-USDT-SPOT.BINANCE")

	trade := model.TradeTick{
		InstrumentID:  instID,
		Price:         decimal.RequireFromString("100.25"),
		Size:          decimal.RequireFromString("0.2"),
		AggressorSide: model.AggressorSideBuyer,
		TradeID:       "venue-trade-1",
		Timestamp:     time.Unix(103, 0),
	}
	data.marketEvents <- model.MarketEvent{Trade: &trade}

	env := <-events.C()
	event := env.Message.(model.MarketEvent)
	require.NotNil(t, event.Trade)
	require.Equal(t, trade, *event.Trade)
	gotTrade, ok := node.Cache().TradeTick(instID)
	require.True(t, ok)
	require.Equal(t, trade, gotTrade)

	require.NoError(t, node.UnsubscribeTradeTicks(context.Background(), instID))
	require.Contains(t, data.calls, "unsubscribe:trade_tick:BTC-USDT-SPOT.BINANCE")
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeSubscribesQuoteTicksForwardsEventsAndCachesLatest(t *testing.T) {
	data := newFakeDataClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicMarketData, 8)
	defer events.Close()

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	require.NoError(t, node.SubscribeQuoteTicks(context.Background(), instID))
	require.Contains(t, data.calls, "subscribe:quote_tick:BTC-USDT-SPOT.BINANCE")

	quote := model.QuoteTick{
		InstrumentID: instID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1.5"),
		AskSize:      decimal.RequireFromString("2.5"),
		Timestamp:    time.Unix(104, 0),
	}
	data.marketEvents <- model.MarketEvent{Quote: &quote}

	env := <-events.C()
	event := env.Message.(model.MarketEvent)
	require.NotNil(t, event.Quote)
	require.Equal(t, quote, *event.Quote)
	gotQuote, ok := node.Cache().QuoteTick(instID)
	require.True(t, ok)
	require.Equal(t, quote, gotQuote)

	require.NoError(t, node.UnsubscribeQuoteTicks(context.Background(), instID))
	require.Contains(t, data.calls, "unsubscribe:quote_tick:BTC-USDT-SPOT.BINANCE")
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeFeedsDataEngineMarketEventsIntoExecutionEmulator(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	exec.recoveryOrders = []model.OrderStatusReport{}
	c := cache.New()
	node := NewNode(Config{
		Cache:           c,
		ExecutionEngine: execution.NewEngine(execution.EngineConfig{Cache: c, Emulator: execution.NewEmulator(execution.EmulatorConfig{})}),
	})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))
	defer node.Stop(context.Background())
	events := node.Bus().Subscribe(TopicExecution, 16)
	defer events.Close()

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	report, err := node.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:        "acct",
		InstrumentID:     instID,
		ClientOrderID:    "client-node-emulated-stop",
		Side:             model.OrderSideBuy,
		Type:             model.OrderTypeStopMarket,
		TimeInForce:      model.TimeInForceGTC,
		Quantity:         decimal.RequireFromString("1"),
		TriggerPrice:     decimal.RequireFromString("101"),
		EmulationTrigger: model.TriggerTypeBidAsk,
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusEmulated, report.Status)
	require.NotContains(t, exec.Calls(), "submit:client-node-emulated-stop")
	require.Eventually(t, func() bool {
		return containsCall(data.Calls(), "subscribe:quote_tick:BTC-USDT-SPOT.BINANCE")
	}, time.Second, 10*time.Millisecond)

	submitted := readOrderLifecycleKind(t, events, model.OrderEventSubmitted)
	require.Equal(t, model.OrderStatusSubmitted, submitted.Status)
	emulated := readOrderLifecycleKind(t, events, model.OrderEventEmulated)
	require.Equal(t, model.OrderStatusEmulated, emulated.Status)
	require.Equal(t, model.OrderStatusInitialized, emulated.PreviousStatus)

	data.marketEvents <- model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: instID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("100.5"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
		Timestamp:    time.Unix(104, 0),
	}}
	require.Never(t, func() bool {
		return containsCall(exec.Calls(), "submit:client-node-emulated-stop")
	}, 50*time.Millisecond, 10*time.Millisecond)

	data.marketEvents <- model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: instID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
		Timestamp:    time.Unix(105, 0),
	}}

	triggered := readOrderLifecycleKind(t, events, model.OrderEventTriggered)
	require.Equal(t, model.OrderStatusTriggered, triggered.Status)
	require.Equal(t, model.OrderStatusEmulated, triggered.PreviousStatus)
	released := readOrderLifecycleKind(t, events, model.OrderEventReleased)
	require.Equal(t, model.OrderStatusReleased, released.Status)
	require.Equal(t, model.OrderStatusTriggered, released.PreviousStatus)
	accepted := readOrderLifecycleKind(t, events, model.OrderEventAccepted)
	require.Equal(t, model.OrderStatusAccepted, accepted.Status)
	require.Equal(t, model.OrderStatusReleased, accepted.PreviousStatus)
	require.Eventually(t, func() bool {
		return containsCall(exec.Calls(), "submit:client-node-emulated-stop")
	}, time.Second, 10*time.Millisecond)
	cached, ok := node.Cache().OrderByClientID("acct", "client-node-emulated-stop")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, cached.Status)
}

func TestNodeEmulationSubscriptionsUseTriggerInstrument(t *testing.T) {
	orderInstrument := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	triggerInstrument := model.MustInstrumentID("ETH-USDT-SPOT.BINANCE")

	subs := emulationTriggerSubscriptions(model.SubmitOrder{
		InstrumentID:        orderInstrument,
		TriggerInstrumentID: triggerInstrument,
		EmulationTrigger:    model.TriggerTypeBidAsk,
	})

	require.Equal(t, []model.SubscribeMarketData{
		{InstrumentID: triggerInstrument, Type: model.MarketDataTypeOrderBook, Depth: 1},
		{InstrumentID: triggerInstrument, Type: model.MarketDataTypeQuoteTick},
	}, subs)
}

func TestNodeEmulationSubscriptionsSkipOrderBookForSyntheticTrigger(t *testing.T) {
	orderInstrument := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	triggerInstrument := model.MustInstrumentID("BTC-ETH-SPREAD.SYNTH")

	subs := emulationTriggerSubscriptions(model.SubmitOrder{
		InstrumentID:        orderInstrument,
		TriggerInstrumentID: triggerInstrument,
		EmulationTrigger:    model.TriggerTypeBidAsk,
	})

	require.Equal(t, []model.SubscribeMarketData{
		{InstrumentID: triggerInstrument, Type: model.MarketDataTypeQuoteTick},
	}, subs)
}

func TestNodeSubscribesBarsForwardsEventsAndCachesLatest(t *testing.T) {
	data := newFakeDataClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.Start(context.Background()))
	events := node.Bus().Subscribe(TopicMarketData, 8)
	defer events.Close()

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	barType := model.NewTimeBarType(instID, time.Minute)
	require.NoError(t, node.SubscribeBars(context.Background(), barType))
	require.Contains(t, data.calls, "subscribe:bar:BTC-USDT-SPOT.BINANCE:"+barType.String())

	bar := model.Bar{
		BarType:   barType,
		Open:      decimal.RequireFromString("100"),
		High:      decimal.RequireFromString("102"),
		Low:       decimal.RequireFromString("99"),
		Close:     decimal.RequireFromString("101"),
		Volume:    decimal.RequireFromString("12.5"),
		Timestamp: time.Unix(104, 0),
	}
	data.marketEvents <- model.MarketEvent{Bar: &bar}

	env := <-events.C()
	event := env.Message.(model.MarketEvent)
	require.NotNil(t, event.Bar)
	require.Equal(t, bar, *event.Bar)
	gotBar, ok := node.Cache().Bar(barType)
	require.True(t, ok)
	require.Equal(t, bar, gotBar)

	require.NoError(t, node.UnsubscribeBars(context.Background(), barType))
	require.Contains(t, data.calls, "unsubscribe:bar:BTC-USDT-SPOT.BINANCE:"+barType.String())
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeRecordsPrivateEventReconcileErrors(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	invalid := model.FillReport{
		AccountID:    "acct",
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderID:      "order-1",
		TradeID:      "bad-trade",
		Quantity:     decimal.RequireFromString("0.1"),
	}
	exec.events <- model.ExecutionEvent{Fill: &invalid}

	require.Eventually(t, func() bool {
		return node.Health().LastError != nil
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeRecoversPrivateStreamResubscribesAndReconcilesReports(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	exec.recoveryFills = []model.FillReport{{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderID:      "order-1",
		TradeID:      "recovery-trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("0.1"),
		Timestamp:    time.Unix(103, 0),
	}}
	exec.recoveryPositions = []model.PositionStatusReport{{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "BTC-USDT-SPOT.BINANCE",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("0.1"),
		EntryPrice:   decimal.RequireFromString("100"),
		Timestamp:    time.Unix(103, 0),
	}}
	exec.breakStream()

	require.Eventually(t, func() bool {
		fillCount := len(node.Cache().FillsForOrder("acct", "order-1"))
		position, ok := node.Cache().PositionByInstrument("acct", instID)
		calls := exec.Calls()
		return countCalls(calls, "exec_connect") >= 2 &&
			containsCall(calls, "resubscribe_execution") &&
			containsCall(calls, "fills:BTC-USDT-SPOT.BINANCE") &&
			containsCall(calls, "positions:BTC-USDT-SPOT.BINANCE") &&
			fillCount == 1 &&
			ok &&
			position.Quantity.Equal(decimal.RequireFromString("0.1"))
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeRetriesMarketDataStreamRecoveryUntilPolicySucceeds(t *testing.T) {
	data := newFakeDataClient()
	node := NewNode(Config{ReconnectPolicy: RetryPolicy{MaxAttempts: 3}})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.Start(context.Background()))
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	require.NoError(t, node.SubscribeTicker(context.Background(), instID))

	data.connectErrs = []error{errors.New("temporary data connect 1"), errors.New("temporary data connect 2")}
	data.breakStream()

	require.Eventually(t, func() bool {
		calls := data.Calls()
		return countCalls(calls, "data_connect") >= 4 &&
			countCalls(calls, "subscribe:ticker:BTC-USDT-SPOT.BINANCE") >= 2 &&
			node.Health().LastError == nil
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeRetriesExecutionStreamRecoveryUntilPolicySucceeds(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{ReconnectPolicy: RetryPolicy{MaxAttempts: 3}})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	exec.connectErrs = []error{errors.New("temporary exec connect 1"), errors.New("temporary exec connect 2")}
	exec.breakStream()

	require.Eventually(t, func() bool {
		calls := exec.Calls()
		return countCalls(calls, "exec_connect") >= 4 &&
			containsCall(calls, "resubscribe_execution") &&
			containsCall(calls, "query_account") &&
			node.Health().LastError == nil
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeRecoveryCancelsLocalOpenOrdersMissingFromVenueSnapshot(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	order, ok := node.Cache().Order("acct", "order-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, order.Status)

	exec.recoveryOrders = []model.OrderStatusReport{}
	exec.breakStream()

	require.Eventually(t, func() bool {
		order, ok := node.Cache().Order("acct", "order-1")
		return ok && order.Status == model.OrderStatusCanceled
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, node.Stop(context.Background()))
}

func TestNodeRecoveryFlattensLocalPositionsMissingFromVenueSnapshot(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakeExecutionClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	require.NoError(t, node.Start(context.Background()))

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	position := model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "pos-1",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}
	require.NoError(t, node.applyAndPublish(context.Background(), node.reconcilerFor("acct"), model.ExecutionEvent{Position: &position}))

	exec.recoveryPositions = []model.PositionStatusReport{}
	exec.breakStream()

	require.Eventually(t, func() bool {
		position, ok := node.Cache().PositionByInstrument("acct", instID)
		return ok && position.Side == model.PositionSideFlat && position.Quantity.IsZero()
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, node.Stop(context.Background()))
}

type fakeDataClient struct {
	mu                sync.Mutex
	provider          *fakeProvider
	marketEvents      chan model.MarketEvent
	replacementEvents chan model.MarketEvent
	connectErrs       []error
	ticker            model.Ticker
	book              model.OrderBook
	calls             []string
}

func newFakeDataClient() *fakeDataClient {
	return &fakeDataClient{provider: newFakeProvider(), marketEvents: make(chan model.MarketEvent, 8)}
}

func (f *fakeDataClient) Venue() model.Venue                    { return "BINANCE" }
func (f *fakeDataClient) ClientID() string                      { return "data" }
func (f *fakeDataClient) Instruments() venue.InstrumentProvider { return f.provider }
func (f *fakeDataClient) Connect(context.Context) error {
	f.recordCall("data_connect")
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.connectErrs) > 0 {
		err := f.connectErrs[0]
		f.connectErrs = f.connectErrs[1:]
		return err
	}
	if f.replacementEvents != nil {
		f.marketEvents = f.replacementEvents
		f.replacementEvents = nil
	}
	return nil
}
func (f *fakeDataClient) Disconnect(context.Context) error { return nil }
func (f *fakeDataClient) Health() venue.DataHealth         { return venue.DataHealth{Connected: true} }
func (f *fakeDataClient) FetchTicker(_ context.Context, instrumentID model.InstrumentID) (model.Ticker, error) {
	f.recordCall("fetch_ticker:" + instrumentID.String())
	return f.ticker, nil
}
func (f *fakeDataClient) FetchOrderBook(_ context.Context, instrumentID model.InstrumentID, depth int) (model.OrderBook, error) {
	f.recordCall(fmt.Sprintf("fetch_book:%s:%d", instrumentID, depth))
	return f.book, nil
}
func (f *fakeDataClient) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	call := "subscribe:" + string(sub.Type) + ":" + sub.InstrumentID.String()
	if sub.Type == model.MarketDataTypeBar {
		call += ":" + sub.BarType.String()
	}
	f.recordCall(call)
	return nil
}
func (f *fakeDataClient) UnsubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	call := "unsubscribe:" + string(sub.Type) + ":" + sub.InstrumentID.String()
	if sub.Type == model.MarketDataTypeBar {
		call += ":" + sub.BarType.String()
	}
	f.recordCall(call)
	return nil
}
func (f *fakeDataClient) Events() <-chan model.MarketEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.marketEvents
}
func (f *fakeDataClient) recordCall(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}
func (f *fakeDataClient) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}
func (f *fakeDataClient) breakStream() {
	f.mu.Lock()
	old := f.marketEvents
	f.replacementEvents = make(chan model.MarketEvent, 8)
	f.mu.Unlock()
	close(old)
}

type fakeProvider struct {
	calls []string
	insts []model.Instrument
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{insts: []model.Instrument{{
		ID:        model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}}}
}

func (f *fakeProvider) LoadAll(context.Context) error {
	f.calls = append(f.calls, "load_all")
	return nil
}
func (f *fakeProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	for _, inst := range f.insts {
		if inst.ID == id {
			return inst, true
		}
	}
	return model.Instrument{}, false
}
func (f *fakeProvider) List() []model.Instrument { return append([]model.Instrument(nil), f.insts...) }

type fakeExecutionClient struct {
	mu                sync.Mutex
	events            chan model.ExecutionEvent
	replacementEvents chan model.ExecutionEvent
	recoveryOrders    []model.OrderStatusReport
	recoveryFills     []model.FillReport
	recoveryPositions []model.PositionStatusReport
	connectErrs       []error
	connectErr        error
	submitErr         error
	modifyErr         error
	cancelErr         error
	nextSubmitted     int
	calls             []string
}

func newFakeExecutionClient() *fakeExecutionClient {
	return &fakeExecutionClient{events: make(chan model.ExecutionEvent, 4)}
}

func (f *fakeExecutionClient) recordCall(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakeExecutionClient) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeExecutionClient) nextSubmittedOrderID() model.OrderID {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextSubmitted++
	return model.OrderID(fmt.Sprintf("submitted-%d", f.nextSubmitted))
}

func (f *fakeExecutionClient) Venue() model.Venue         { return "BINANCE" }
func (f *fakeExecutionClient) AccountID() model.AccountID { return "acct" }
func (f *fakeExecutionClient) Connect(context.Context) error {
	f.recordCall("exec_connect")
	f.mu.Lock()
	if len(f.connectErrs) > 0 {
		err := f.connectErrs[0]
		f.connectErrs = f.connectErrs[1:]
		f.mu.Unlock()
		return err
	}
	f.mu.Unlock()
	if f.connectErr != nil {
		return f.connectErr
	}
	if f.replacementEvents != nil {
		f.events = f.replacementEvents
		f.replacementEvents = nil
	}
	return nil
}
func (f *fakeExecutionClient) Disconnect(context.Context) error { return nil }
func (f *fakeExecutionClient) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true}
}
func (f *fakeExecutionClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	f.recordCall("query_account")
	return model.AccountSnapshot{AccountID: "acct", Venue: "BINANCE"}, nil
}
func (f *fakeExecutionClient) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	f.recordCall("submit:" + string(order.ClientOrderID))
	if f.submitErr != nil {
		return model.OrderStatusReport{}, f.submitErr
	}
	orderID := f.nextSubmittedOrderID()
	return model.OrderStatusReport{
		AccountID:      order.AccountID,
		InstrumentID:   order.InstrumentID,
		OrderID:        orderID,
		VenueOrderID:   model.VenueOrderID(fmt.Sprintf("venue-%s", orderID)),
		ClientOrderID:  order.ClientOrderID,
		Status:         model.OrderStatusAccepted,
		Side:           order.Side,
		Type:           order.Type,
		Quantity:       order.Quantity,
		FilledQuantity: decimal.Zero,
		LeavesQuantity: order.Quantity,
		Price:          order.Price,
	}, nil
}
func (f *fakeExecutionClient) CancelOrder(_ context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	f.recordCall("cancel:" + string(cancel.ClientOrderID))
	if f.cancelErr != nil {
		return model.OrderStatusReport{}, f.cancelErr
	}
	return model.OrderStatusReport{
		AccountID:     cancel.AccountID,
		InstrumentID:  cancel.InstrumentID,
		OrderID:       cancel.OrderID,
		ClientOrderID: cancel.ClientOrderID,
		Status:        model.OrderStatusCanceled,
	}, nil
}
func (f *fakeExecutionClient) ModifyOrder(_ context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	f.recordCall("modify:" + string(modify.ClientOrderID))
	if f.modifyErr != nil {
		return model.OrderStatusReport{}, f.modifyErr
	}
	return model.OrderStatusReport{
		AccountID:     modify.AccountID,
		InstrumentID:  modify.InstrumentID,
		OrderID:       modify.OrderID,
		ClientOrderID: modify.ClientOrderID,
		Status:        model.OrderStatusAccepted,
		Quantity:      modify.Quantity,
		Price:         modify.Price,
		TriggerPrice:  modify.TriggerPrice,
	}, nil
}
func (f *fakeExecutionClient) QueryOrder(_ context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	f.recordCall("query:" + string(query.ClientOrderID))
	orderID := query.OrderID
	if orderID == "" {
		orderID = model.OrderID("query-" + string(query.ClientOrderID))
	}
	return model.OrderStatusReport{
		AccountID:      query.AccountID,
		InstrumentID:   query.InstrumentID,
		OrderID:        orderID,
		ClientOrderID:  query.ClientOrderID,
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("1"),
		FilledQuantity: decimal.Zero,
		LeavesQuantity: decimal.RequireFromString("1"),
		Price:          decimal.RequireFromString("100"),
	}, nil
}
func (f *fakeExecutionClient) GenerateOrderStatusReports(_ context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	f.recordCall("reports:" + id.String())
	if f.recoveryOrders != nil {
		return append([]model.OrderStatusReport(nil), f.recoveryOrders...), nil
	}
	return []model.OrderStatusReport{{
		AccountID:    "acct",
		InstrumentID: id,
		OrderID:      "order-1",
		Status:       model.OrderStatusAccepted,
	}}, nil
}
func (f *fakeExecutionClient) ResubscribeExecution(context.Context) error {
	f.recordCall("resubscribe_execution")
	return nil
}
func (f *fakeExecutionClient) GenerateFillReports(_ context.Context, id model.InstrumentID) ([]model.FillReport, error) {
	f.recordCall("fills:" + id.String())
	return append([]model.FillReport(nil), f.recoveryFills...), nil
}
func (f *fakeExecutionClient) GeneratePositionStatusReports(_ context.Context, id model.InstrumentID) ([]model.PositionStatusReport, error) {
	f.recordCall("positions:" + id.String())
	return append([]model.PositionStatusReport(nil), f.recoveryPositions...), nil
}
func (f *fakeExecutionClient) Events() <-chan model.ExecutionEvent { return f.events }

func (f *fakeExecutionClient) breakStream() {
	f.replacementEvents = make(chan model.ExecutionEvent, 8)
	close(f.events)
}

func containsCall(calls []string, want string) bool {
	return countCalls(calls, want) > 0
}

func countCalls(calls []string, want string) int {
	var count int
	for _, call := range calls {
		if call == want {
			count++
		}
	}
	return count
}

var _ = bus.Envelope{}
