package account

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTradingAccountStartReconcilesBeforeConnect(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	exec := newFakeExecution()
	exec.accountState = testAccountState(t)
	exec.orderReports = []model.OrderStatusReport{{
		AccountID:    exec.accountID,
		InstrumentID: instID,
		OrderID:      "100",
		ClientID:     "cli-1",
		Status:       model.OrderStatusAccepted,
		Side:         model.OrderSideBuy,
		Type:         model.OrderTypeLimit,
		Quantity:     decimal.RequireFromString("0.25"),
		EventTime:    time.Now(),
	}}
	exec.fillReports = []model.FillReport{{
		AccountID:    exec.accountID,
		InstrumentID: instID,
		OrderID:      "100",
		ClientID:     "cli-1",
		TradeID:      "tr-1",
		Side:         model.OrderSideBuy,
		Quantity:     decimal.RequireFromString("0.10"),
		Price:        decimal.RequireFromString("65000"),
		EventTime:    time.Now(),
	}}
	exec.positionReports = []model.PositionStatusReport{{
		AccountID:    exec.accountID,
		InstrumentID: instID,
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("0.10"),
		AvgPrice:     decimal.RequireFromString("65000"),
		EventTime:    time.Now(),
	}}

	acct, err := NewTradingAccount(exec, TradingAccountConfig{Instruments: []model.InstrumentID{instID}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, acct.Stop(context.Background())) })

	require.NoError(t, acct.Start(context.Background()))
	require.Equal(t, []string{
		"query_account",
		"order_reports:BTC-USDT-PERP.BINANCE",
		"fill_reports:BTC-USDT-PERP.BINANCE",
		"position_reports:BTC-USDT-PERP.BINANCE",
		"connect",
	}, exec.callsSnapshot())
	require.True(t, acct.Ready())

	state, ok := acct.AccountState()
	require.True(t, ok)
	require.Len(t, state.Balances, 1)
	require.True(t, state.Balances[0].Free.Amount.Equal(decimal.RequireFromString("8")))

	flow, ok := acct.FlowByClientID("cli-1")
	require.True(t, ok)
	latest, ok := flow.Latest()
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, latest.Status)
	require.Len(t, flow.FillsSnapshot(), 1)

	positions := acct.PositionsSnapshot()
	require.Len(t, positions, 1)
	require.Equal(t, model.PositionSideLong, positions[0].Side)

	health := acct.Health()
	require.True(t, health.Started)
	require.True(t, health.SnapshotLoaded)
	require.Equal(t, StreamStatusReady, health.Streams[StreamOrders].Status)
	require.Equal(t, StreamStatusReady, health.Streams[StreamBalances].Status)
}

func TestTradingAccountStartTreatsUnsupportedReportsAsOptional(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	exec := newFakeExecution()
	exec.accountState = testAccountState(t)
	exec.fillErr = model.ErrNotSupported
	exec.positionErr = model.ErrNotSupported

	acct, err := NewTradingAccount(exec, TradingAccountConfig{Instruments: []model.InstrumentID{instID}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, acct.Stop(context.Background())) })

	require.NoError(t, acct.Start(context.Background()))
	require.True(t, acct.Ready())

	health := acct.Health()
	require.Equal(t, StreamStatusUnsupported, health.Streams[StreamFills].Status)
	require.Equal(t, StreamStatusUnsupported, health.Streams[StreamPositions].Status)
	require.Equal(t, StreamStatusReady, health.Streams[StreamOrders].Status)
}

func TestTradingAccountAppliesLiveExecutionEvents(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	exec := newFakeExecution()
	exec.accountState = testAccountState(t)

	acct, err := NewTradingAccount(exec, TradingAccountConfig{Instruments: []model.InstrumentID{instID}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, acct.Stop(context.Background())) })
	require.NoError(t, acct.Start(context.Background()))

	exec.emit(model.ExecutionEvent{Order: &model.OrderStatusReport{
		AccountID:    exec.accountID,
		InstrumentID: instID,
		OrderID:      "101",
		ClientID:     "cli-live",
		Status:       model.OrderStatusAccepted,
		Side:         model.OrderSideSell,
		Type:         model.OrderTypeLimit,
		Quantity:     decimal.RequireFromString("0.50"),
		EventTime:    time.Now(),
	}})
	exec.emit(model.ExecutionEvent{Fill: &model.FillReport{
		AccountID:    exec.accountID,
		InstrumentID: instID,
		OrderID:      "101",
		ClientID:     "cli-live",
		TradeID:      "tr-live",
		Side:         model.OrderSideSell,
		Quantity:     decimal.RequireFromString("0.20"),
		Price:        decimal.RequireFromString("66000"),
		EventTime:    time.Now(),
	}})

	require.Eventually(t, func() bool {
		flow, ok := acct.FlowByOrderID("101")
		return ok && len(flow.FillsSnapshot()) == 1
	}, time.Second, 10*time.Millisecond)

	health := acct.Health()
	require.GreaterOrEqual(t, health.Streams[StreamOrders].Events, uint64(1))
	require.GreaterOrEqual(t, health.Streams[StreamFills].Events, uint64(1))
}

func testAccountState(t *testing.T) *model.AccountState {
	t.Helper()
	total := model.Money{Amount: decimal.RequireFromString("10"), Currency: model.USDT}
	free := model.Money{Amount: decimal.RequireFromString("8"), Currency: model.USDT}
	bal, err := model.BalanceFromTotalAndFree(total, free)
	require.NoError(t, err)
	return &model.AccountState{
		AccountID: "binance-main",
		Venue:     model.VenueBinance,
		Type:      model.AccountTypeMargin,
		Reported:  true,
		Balances:  []model.AccountBalance{bal},
		EventTime: time.Now(),
	}
}

type fakeExecution struct {
	mu              sync.Mutex
	accountID       model.AccountID
	venueID         model.Venue
	events          chan model.ExecutionEvent
	connected       bool
	calls           []string
	accountState    *model.AccountState
	orderReports    []model.OrderStatusReport
	fillReports     []model.FillReport
	positionReports []model.PositionStatusReport
	fillErr         error
	positionErr     error
}

var _ venue.ExecutionClient = (*fakeExecution)(nil)

func newFakeExecution() *fakeExecution {
	return &fakeExecution{
		accountID: "binance-main",
		venueID:   model.VenueBinance,
		events:    make(chan model.ExecutionEvent, 64),
	}
}

func (f *fakeExecution) AccountID() model.AccountID { return f.accountID }

func (f *fakeExecution) Venue() model.Venue { return f.venueID }

func (f *fakeExecution) Connect(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = true
	f.calls = append(f.calls, "connect")
	return nil
}

func (f *fakeExecution) Disconnect(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = false
	return nil
}

func (f *fakeExecution) Health() venue.ExecutionHealth {
	f.mu.Lock()
	defer f.mu.Unlock()
	return venue.ExecutionHealth{Connected: f.connected, AccountReady: f.connected}
}

func (f *fakeExecution) SubmitOrder(context.Context, model.SubmitOrder) error { return nil }

func (f *fakeExecution) ModifyOrder(context.Context, model.ModifyOrder) error {
	return model.ErrNotSupported
}

func (f *fakeExecution) CancelOrder(context.Context, model.CancelOrder) error { return nil }

func (f *fakeExecution) CancelAllOrders(context.Context, model.CancelAllOrders) error {
	return nil
}

func (f *fakeExecution) QueryAccount(context.Context) error {
	f.record("query_account")
	if f.accountState != nil {
		f.emit(model.ExecutionEvent{AccountState: f.accountState})
	}
	return nil
}

func (f *fakeExecution) GenerateOrderStatusReports(_ context.Context, q venue.OrderStatusQuery) ([]model.OrderStatusReport, error) {
	f.record("order_reports:" + q.InstrumentID.String())
	return f.orderReports, nil
}

func (f *fakeExecution) GenerateFillReports(_ context.Context, q venue.FillQuery) ([]model.FillReport, error) {
	f.record("fill_reports:" + q.InstrumentID.String())
	if f.fillErr != nil {
		return nil, f.fillErr
	}
	return f.fillReports, nil
}

func (f *fakeExecution) GeneratePositionStatusReports(_ context.Context, q venue.PositionQuery) ([]model.PositionStatusReport, error) {
	f.record("position_reports:" + q.InstrumentID.String())
	if f.positionErr != nil {
		return nil, f.positionErr
	}
	return f.positionReports, nil
}

func (f *fakeExecution) Events() <-chan model.ExecutionEvent { return f.events }

func (f *fakeExecution) emit(ev model.ExecutionEvent) {
	f.events <- ev
}

func (f *fakeExecution) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakeExecution) callsSnapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func TestTradingAccountRejectsNilExecutionClient(t *testing.T) {
	_, err := NewTradingAccount(nil, TradingAccountConfig{})
	require.True(t, errors.Is(err, model.ErrInvalidAccountState))
}
