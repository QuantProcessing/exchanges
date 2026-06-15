package account

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTradingAccountStartsWithAccountOrderFillAndPositionReconciliation(t *testing.T) {
	ctx := context.Background()
	inst := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	client := newTradingAccountExecutionClient(inst)
	client.openOrders = []model.OrderStatusReport{{
		AccountID:      client.AccountID(),
		InstrumentID:   inst,
		OrderID:        "startup-order",
		ClientOrderID:  "startup-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
		Price:          decimal.RequireFromString("100"),
	}}
	client.startupFills = []model.FillReport{{
		AccountID:     client.AccountID(),
		InstrumentID:  inst,
		OrderID:       "startup-order",
		ClientOrderID: "startup-client",
		TradeID:       "startup-fill",
		Side:          model.OrderSideBuy,
		Price:         decimal.RequireFromString("100"),
		Quantity:      decimal.RequireFromString("0.25"),
		Timestamp:     time.Unix(1, 0),
	}}
	client.positions = []model.PositionStatusReport{{
		AccountID:    client.AccountID(),
		InstrumentID: inst,
		PositionID:   "startup-position",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("0.25"),
		EntryPrice:   decimal.RequireFromString("100"),
		Timestamp:    time.Unix(1, 0),
	}}

	acct, err := NewTradingAccount(client, TradingAccountConfig{
		Instruments: []model.InstrumentID{inst},
	})
	require.NoError(t, err)
	require.NoError(t, acct.Start(ctx))
	defer acct.Stop(ctx)

	account, ok := acct.Cache().Account(client.AccountID())
	require.True(t, ok)
	require.Equal(t, model.Venue("BINANCE"), account.Venue)
	order, ok := acct.Cache().Order(client.AccountID(), "startup-order")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusPartiallyFilled, order.Status)
	require.Equal(t, "0.25", order.FilledQuantity.String())
	fills := acct.Cache().FillsForOrder(client.AccountID(), "startup-order")
	require.Len(t, fills, 1)
	require.Equal(t, model.TradeID("startup-fill"), fills[0].TradeID)
	position, ok := acct.Cache().Position(client.AccountID(), "startup-position")
	require.True(t, ok)
	require.Equal(t, model.PositionSideLong, position.Side)

	health := acct.Health()
	require.True(t, health.Ready)
	require.True(t, health.AccountReady)
	require.True(t, health.OrderStreamReady)
	require.False(t, health.FillsUnsupported)
	require.False(t, health.PositionsUnsupported)
}

func TestTradingAccountSubmitOrderReturnsTrackerAndRoutesPrivateFills(t *testing.T) {
	ctx := context.Background()
	inst := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	client := newTradingAccountExecutionClient(inst)
	acct, err := NewTradingAccount(client, TradingAccountConfig{
		Instruments: []model.InstrumentID{inst},
		BufferSize:  2,
	})
	require.NoError(t, err)
	require.NoError(t, acct.Start(ctx))
	defer acct.Stop(ctx)

	tracker, err := acct.SubmitOrder(ctx, model.SubmitOrder{
		InstrumentID:  inst,
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
	})
	require.NoError(t, err)
	defer tracker.Close()

	require.Equal(t, client.AccountID(), client.lastSubmit.AccountID)
	require.Equal(t, model.OrderID("order-1"), (<-tracker.C()).OrderID)
	latest, ok := tracker.Latest()
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, latest.Status)

	fill := model.FillReport{
		AccountID:     client.AccountID(),
		InstrumentID:  inst,
		OrderID:       "order-1",
		ClientOrderID: "client-1",
		TradeID:       "trade-1",
		Side:          model.OrderSideBuy,
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("0.4"),
		Timestamp:     time.Unix(2, 0),
	}
	client.emit(model.ExecutionEvent{Fill: &fill})

	require.Equal(t, fill.TradeID, (<-tracker.Fills()).TradeID)
	require.Eventually(t, func() bool {
		got, ok := acct.Cache().Order(client.AccountID(), "order-1")
		return ok && got.FilledQuantity.Equal(decimal.RequireFromString("0.4"))
	}, time.Second, 10*time.Millisecond)

	health := acct.Health()
	require.Equal(t, int64(1), health.FillEvents)
}

func TestTradingAccountRecoversClosedPrivateStreamResubscribesAndRoutesNewEvents(t *testing.T) {
	ctx := context.Background()
	inst := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	client := newTradingAccountExecutionClient(inst)
	acct, err := NewTradingAccount(client, TradingAccountConfig{
		Instruments: []model.InstrumentID{inst},
		BufferSize:  4,
	})
	require.NoError(t, err)
	require.NoError(t, acct.Start(ctx))
	defer acct.Stop(ctx)

	tracker, err := acct.SubmitOrder(ctx, model.SubmitOrder{
		InstrumentID:  inst,
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
	})
	require.NoError(t, err)
	defer tracker.Close()
	require.Equal(t, model.OrderID("order-1"), (<-tracker.C()).OrderID)

	client.openOrders = []model.OrderStatusReport{{
		AccountID:      client.AccountID(),
		InstrumentID:   inst,
		OrderID:        "order-1",
		ClientOrderID:  "client-1",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeMarket,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
	}}
	client.closeAndReplaceEvents()

	require.Eventually(t, func() bool {
		health := acct.Health()
		return health.OrderStreamReady && health.Reconnects == 1 && client.resubscribeCount() == 1
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, model.OrderID("order-1"), (<-tracker.C()).OrderID)

	fill := model.FillReport{
		AccountID:     client.AccountID(),
		InstrumentID:  inst,
		OrderID:       "order-1",
		ClientOrderID: "client-1",
		TradeID:       "trade-after-reconnect",
		Side:          model.OrderSideBuy,
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("1"),
		Timestamp:     time.Unix(3, 0),
	}
	client.emit(model.ExecutionEvent{Fill: &fill})

	require.Equal(t, fill.TradeID, (<-tracker.Fills()).TradeID)
	require.Eventually(t, func() bool {
		got, ok := acct.Cache().Order(client.AccountID(), "order-1")
		return ok && got.Status == model.OrderStatusFilled
	}, time.Second, 10*time.Millisecond)
}

func TestTradingAccountCancelOrderAppliesReportAndNotifiesTracker(t *testing.T) {
	ctx := context.Background()
	inst := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	client := newTradingAccountExecutionClient(inst)
	acct, err := NewTradingAccount(client, TradingAccountConfig{Instruments: []model.InstrumentID{inst}})
	require.NoError(t, err)
	require.NoError(t, acct.Start(ctx))
	defer acct.Stop(ctx)

	tracker, err := acct.SubmitOrder(ctx, model.SubmitOrder{
		InstrumentID:  inst,
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("100"),
	})
	require.NoError(t, err)
	defer tracker.Close()
	require.Equal(t, model.OrderStatusAccepted, (<-tracker.C()).Status)

	report, err := acct.CancelOrder(ctx, model.CancelOrder{
		InstrumentID:  inst,
		ClientOrderID: "client-1",
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusCanceled, report.Status)
	require.Equal(t, model.OrderID("order-1"), (<-tracker.C()).OrderID)
	latest, ok := tracker.Latest()
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, latest.Status)
	cached, ok := acct.Cache().Order(client.AccountID(), "order-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, cached.Status)
}

func TestTradingAccountModifyOrderUsesOptionalModifierAndNotifiesTracker(t *testing.T) {
	ctx := context.Background()
	inst := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	client := newTradingAccountExecutionClient(inst)
	acct, err := NewTradingAccount(client, TradingAccountConfig{Instruments: []model.InstrumentID{inst}})
	require.NoError(t, err)
	require.NoError(t, acct.Start(ctx))
	defer acct.Stop(ctx)

	tracker, err := acct.SubmitOrder(ctx, model.SubmitOrder{
		InstrumentID:  inst,
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("100"),
	})
	require.NoError(t, err)
	defer tracker.Close()
	require.Equal(t, model.OrderStatusAccepted, (<-tracker.C()).Status)

	report, err := acct.ModifyOrder(ctx, model.ModifyOrder{
		InstrumentID:  inst,
		ClientOrderID: "client-1",
		Price:         decimal.RequireFromString("99"),
	})
	require.NoError(t, err)
	require.Equal(t, decimal.RequireFromString("99"), report.Price)
	require.Equal(t, decimal.RequireFromString("99"), (<-tracker.C()).Price)
	require.Equal(t, model.ClientOrderID("client-1"), client.lastModify.ClientOrderID)
}

func TestTradingAccountModifyOrderReturnsUnsupportedWithoutOptionalModifier(t *testing.T) {
	ctx := context.Background()
	inst := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	base := newTradingAccountExecutionClient(inst)
	client := executionClientWithoutOptionalCommands{base: base}
	acct, err := NewTradingAccount(client, TradingAccountConfig{Instruments: []model.InstrumentID{inst}})
	require.NoError(t, err)
	require.NoError(t, acct.Start(ctx))
	defer acct.Stop(ctx)

	_, err = acct.ModifyOrder(ctx, model.ModifyOrder{
		AccountID:     client.AccountID(),
		InstrumentID:  inst,
		ClientOrderID: "client-1",
		Price:         decimal.RequireFromString("99"),
	})
	require.ErrorIs(t, err, model.ErrNotSupported)
}

func TestTradingAccountQueryOrderFallsBackToOrderReportsAndRefreshesCache(t *testing.T) {
	ctx := context.Background()
	inst := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	base := newTradingAccountExecutionClient(inst)
	base.openOrders = []model.OrderStatusReport{{
		AccountID:      base.AccountID(),
		InstrumentID:   inst,
		OrderID:        "reported-order",
		ClientOrderID:  "reported-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideSell,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("2"),
		LeavesQuantity: decimal.RequireFromString("2"),
		Price:          decimal.RequireFromString("110"),
	}}
	client := executionClientWithoutOptionalCommands{base: base}
	acct, err := NewTradingAccount(client, TradingAccountConfig{Instruments: []model.InstrumentID{inst}})
	require.NoError(t, err)
	require.NoError(t, acct.Start(ctx))
	defer acct.Stop(ctx)

	report, err := acct.QueryOrder(ctx, model.QueryOrder{
		Metadata: model.CommandMetadata{
			CommandID: "query-command",
		},
		InstrumentID:  inst,
		ClientOrderID: "reported-client",
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderID("reported-order"), report.OrderID)
	require.Equal(t, model.CommandID("query-command"), report.Metadata.CommandID)
	cached, ok := acct.Cache().OrderByClientID(client.AccountID(), "reported-client")
	require.True(t, ok)
	require.Equal(t, report.OrderID, cached.OrderID)

	account, err := acct.QueryAccount(ctx)
	require.NoError(t, err)
	require.Equal(t, client.AccountID(), account.AccountID)
	require.Equal(t, int64(1), acct.Health().AccountEvents)
}

func TestTradingAccountPeriodicReconciliationRepairsMissingOrderAndPosition(t *testing.T) {
	ctx := context.Background()
	inst := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	client := newTradingAccountExecutionClient(inst)
	c := cache.New()
	require.NoError(t, c.PutOrder(model.OrderStatusReport{
		AccountID:      client.AccountID(),
		InstrumentID:   inst,
		OrderID:        "stale-order",
		ClientOrderID:  "stale-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
		Price:          decimal.RequireFromString("100"),
	}))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    client.AccountID(),
		InstrumentID: inst,
		PositionID:   "stale-position",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))

	acct, err := NewTradingAccount(client, TradingAccountConfig{
		Cache:             c,
		Instruments:       []model.InstrumentID{inst},
		ReconcileInterval: 10 * time.Millisecond,
	})
	require.NoError(t, err)
	require.NoError(t, acct.Start(ctx))
	defer acct.Stop(ctx)

	require.Eventually(t, func() bool {
		order, ok := acct.Cache().Order(client.AccountID(), "stale-order")
		return ok && order.Status == model.OrderStatusCanceled
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		position, ok := acct.Cache().Position(client.AccountID(), "stale-position")
		return ok && position.Side == model.PositionSideFlat && position.Quantity.IsZero()
	}, time.Second, 10*time.Millisecond)
	require.Positive(t, acct.Health().Reconciliations)
	require.False(t, acct.Health().LastReconcileTime.IsZero())
}

func TestTradingAccountReconciliationHonorsMissingOrderRepairDelay(t *testing.T) {
	ctx := context.Background()
	inst := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	client := newTradingAccountExecutionClient(inst)
	c := cache.New()
	recent := model.OrderStatusReport{
		AccountID:       client.AccountID(),
		InstrumentID:    inst,
		OrderID:         "recent-order",
		ClientOrderID:   "recent-client",
		Status:          model.OrderStatusAccepted,
		Side:            model.OrderSideBuy,
		Type:            model.OrderTypeLimit,
		Quantity:        decimal.RequireFromString("1"),
		LeavesQuantity:  decimal.RequireFromString("1"),
		Price:           decimal.RequireFromString("100"),
		LastUpdatedTime: time.Now().Add(-10 * time.Second),
	}
	stale := recent
	stale.OrderID = "stale-order"
	stale.ClientOrderID = "stale-client"
	stale.LastUpdatedTime = time.Now().Add(-2 * time.Hour)
	require.NoError(t, c.PutOrder(recent))
	require.NoError(t, c.PutOrder(stale))

	acct, err := NewTradingAccount(client, TradingAccountConfig{
		Cache:                   c,
		Instruments:             []model.InstrumentID{inst},
		MissingOrderRepairDelay: time.Hour,
	})
	require.NoError(t, err)

	require.NoError(t, acct.reconcileExecution(ctx, true))
	gotRecent, ok := acct.Cache().Order(client.AccountID(), "recent-order")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, gotRecent.Status)
	gotStale, ok := acct.Cache().Order(client.AccountID(), "stale-order")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, gotStale.Status)
}

func TestTradingAccountMarksFillsUnsupportedWhenClientCannotGenerateFills(t *testing.T) {
	ctx := context.Background()
	inst := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	base := newTradingAccountExecutionClient(inst)
	client := executionClientWithoutFillReports{base: base}

	acct, err := NewTradingAccount(client, TradingAccountConfig{
		Instruments: []model.InstrumentID{inst},
	})
	require.NoError(t, err)
	require.NoError(t, acct.Start(ctx))
	defer acct.Stop(ctx)

	health := acct.Health()
	require.True(t, health.Ready)
	require.True(t, health.FillsUnsupported)
}

type tradingAccountExecutionClient struct {
	mu           sync.Mutex
	inst         model.InstrumentID
	events       chan model.ExecutionEvent
	openOrders   []model.OrderStatusReport
	startupFills []model.FillReport
	positions    []model.PositionStatusReport
	lastSubmit   model.SubmitOrder
	lastCancel   model.CancelOrder
	lastModify   model.ModifyOrder
	connected    bool
	connects     int
	resubscribes int
}

func newTradingAccountExecutionClient(inst model.InstrumentID) *tradingAccountExecutionClient {
	return &tradingAccountExecutionClient{
		inst:   inst,
		events: make(chan model.ExecutionEvent, 8),
	}
}

func (c *tradingAccountExecutionClient) Venue() model.Venue         { return "BINANCE" }
func (c *tradingAccountExecutionClient) AccountID() model.AccountID { return "acct" }
func (c *tradingAccountExecutionClient) Connect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true
	c.connects++
	return nil
}
func (c *tradingAccountExecutionClient) Disconnect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}
func (c *tradingAccountExecutionClient) Health() venue.ExecutionHealth {
	c.mu.Lock()
	defer c.mu.Unlock()
	return venue.ExecutionHealth{Connected: c.connected, AccountReady: c.connected}
}
func (c *tradingAccountExecutionClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{AccountID: c.AccountID(), Venue: c.Venue(), Type: model.AccountTypeMargin}, nil
}
func (c *tradingAccountExecutionClient) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSubmit = order
	return model.OrderStatusReport{
		AccountID:      c.AccountID(),
		InstrumentID:   order.InstrumentID,
		OrderID:        "order-1",
		ClientOrderID:  order.ClientOrderID,
		Status:         model.OrderStatusAccepted,
		Side:           order.Side,
		Type:           order.Type,
		Quantity:       order.Quantity,
		LeavesQuantity: order.Quantity,
	}, nil
}
func (c *tradingAccountExecutionClient) CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cancel := model.CancelOrder{}
	if c.lastSubmit.AccountID != "" {
		cancel.AccountID = c.lastSubmit.AccountID
		cancel.InstrumentID = c.lastSubmit.InstrumentID
		cancel.OrderID = "order-1"
		cancel.ClientOrderID = c.lastSubmit.ClientOrderID
	}
	c.lastCancel = cancel
	return model.OrderStatusReport{
		AccountID:      c.AccountID(),
		InstrumentID:   cancel.InstrumentID,
		OrderID:        cancel.OrderID,
		ClientOrderID:  cancel.ClientOrderID,
		Status:         model.OrderStatusCanceled,
		Side:           c.lastSubmit.Side,
		Type:           c.lastSubmit.Type,
		Quantity:       c.lastSubmit.Quantity,
		FilledQuantity: decimal.Zero,
		LeavesQuantity: c.lastSubmit.Quantity,
		Price:          c.lastSubmit.Price,
	}, nil
}
func (c *tradingAccountExecutionClient) GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]model.OrderStatusReport(nil), c.openOrders...), nil
}
func (c *tradingAccountExecutionClient) ModifyOrder(_ context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if modify.AccountID == "" {
		modify.AccountID = c.AccountID()
	}
	if modify.OrderID == "" {
		modify.OrderID = "order-1"
	}
	if modify.ClientOrderID == "" {
		modify.ClientOrderID = c.lastSubmit.ClientOrderID
	}
	c.lastModify = modify
	return model.OrderStatusReport{
		AccountID:      c.AccountID(),
		InstrumentID:   modify.InstrumentID,
		OrderID:        modify.OrderID,
		ClientOrderID:  modify.ClientOrderID,
		Status:         model.OrderStatusAccepted,
		Side:           c.lastSubmit.Side,
		Type:           c.lastSubmit.Type,
		Quantity:       c.lastSubmit.Quantity,
		LeavesQuantity: c.lastSubmit.Quantity,
		Price:          modify.Price,
	}, nil
}
func (c *tradingAccountExecutionClient) QueryOrder(_ context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, report := range c.openOrders {
		if query.OrderID != "" && report.OrderID == query.OrderID {
			return report, nil
		}
		if query.ClientOrderID != "" && report.ClientOrderID == query.ClientOrderID {
			return report, nil
		}
		if query.VenueOrderID != "" && report.VenueOrderID == query.VenueOrderID {
			return report, nil
		}
	}
	return model.OrderStatusReport{}, model.ErrInvalidOrder
}
func (c *tradingAccountExecutionClient) GenerateFillReports(context.Context, model.InstrumentID) ([]model.FillReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]model.FillReport(nil), c.startupFills...), nil
}
func (c *tradingAccountExecutionClient) GeneratePositionStatusReports(context.Context, model.InstrumentID) ([]model.PositionStatusReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]model.PositionStatusReport(nil), c.positions...), nil
}
func (c *tradingAccountExecutionClient) ResubscribeExecution(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resubscribes++
	return nil
}
func (c *tradingAccountExecutionClient) Events() <-chan model.ExecutionEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.events
}
func (c *tradingAccountExecutionClient) emit(event model.ExecutionEvent) {
	c.mu.Lock()
	events := c.events
	c.mu.Unlock()
	events <- event
}
func (c *tradingAccountExecutionClient) closeAndReplaceEvents() {
	c.mu.Lock()
	old := c.events
	c.events = make(chan model.ExecutionEvent, 8)
	c.mu.Unlock()
	close(old)
}
func (c *tradingAccountExecutionClient) resubscribeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resubscribes
}

type executionClientWithoutFillReports struct {
	base *tradingAccountExecutionClient
}

func (c executionClientWithoutFillReports) Venue() model.Venue { return c.base.Venue() }
func (c executionClientWithoutFillReports) AccountID() model.AccountID {
	return c.base.AccountID()
}
func (c executionClientWithoutFillReports) Connect(ctx context.Context) error {
	return c.base.Connect(ctx)
}
func (c executionClientWithoutFillReports) Disconnect(ctx context.Context) error {
	return c.base.Disconnect(ctx)
}
func (c executionClientWithoutFillReports) Health() venue.ExecutionHealth {
	return c.base.Health()
}
func (c executionClientWithoutFillReports) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	return c.base.QueryAccount(ctx)
}
func (c executionClientWithoutFillReports) SubmitOrder(ctx context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	return c.base.SubmitOrder(ctx, order)
}
func (c executionClientWithoutFillReports) CancelOrder(ctx context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	return c.base.CancelOrder(ctx, cancel)
}
func (c executionClientWithoutFillReports) GenerateOrderStatusReports(ctx context.Context, instrumentID model.InstrumentID) ([]model.OrderStatusReport, error) {
	return c.base.GenerateOrderStatusReports(ctx, instrumentID)
}
func (c executionClientWithoutFillReports) Events() <-chan model.ExecutionEvent {
	return c.base.Events()
}

type executionClientWithoutOptionalCommands struct {
	base *tradingAccountExecutionClient
}

func (c executionClientWithoutOptionalCommands) Venue() model.Venue { return c.base.Venue() }
func (c executionClientWithoutOptionalCommands) AccountID() model.AccountID {
	return c.base.AccountID()
}
func (c executionClientWithoutOptionalCommands) Connect(ctx context.Context) error {
	return c.base.Connect(ctx)
}
func (c executionClientWithoutOptionalCommands) Disconnect(ctx context.Context) error {
	return c.base.Disconnect(ctx)
}
func (c executionClientWithoutOptionalCommands) Health() venue.ExecutionHealth {
	return c.base.Health()
}
func (c executionClientWithoutOptionalCommands) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	return c.base.QueryAccount(ctx)
}
func (c executionClientWithoutOptionalCommands) SubmitOrder(ctx context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	return c.base.SubmitOrder(ctx, order)
}
func (c executionClientWithoutOptionalCommands) CancelOrder(ctx context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	return c.base.CancelOrder(ctx, cancel)
}
func (c executionClientWithoutOptionalCommands) GenerateOrderStatusReports(ctx context.Context, instrumentID model.InstrumentID) ([]model.OrderStatusReport, error) {
	return c.base.GenerateOrderStatusReports(ctx, instrumentID)
}
func (c executionClientWithoutOptionalCommands) Events() <-chan model.ExecutionEvent {
	return c.base.Events()
}
