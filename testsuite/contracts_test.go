package testsuite

import (
	"context"
	"io"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestAdapterCapabilitySuiteAcceptsImplementedCapabilities(t *testing.T) {
	RunAdapterCapabilitySuite(t, AdapterCapabilityConfig{
		Adapter: fakeCapabilityAdapter{},
	})
}

func TestAdapterCapabilityReportRejectsClaimedGranularExecutionWithoutInterface(t *testing.T) {
	report := AdapterCapabilityReport(t, fakeClaimedModifyAdapter{})

	require.False(t, report.AllPassed(), "claimed modify without OrderModifier must fail: %#v", report)
	requireCaseFailed(t, report, "TC-A07", "modify capability requires venue.OrderModifier")
}

type fakeCapabilityAdapter struct{}

func (fakeCapabilityAdapter) Venue() model.Venue { return "FAKE" }
func (fakeCapabilityAdapter) Instruments() venue.InstrumentProvider {
	return fakeProvider{}
}
func (fakeCapabilityAdapter) Data() venue.DataClient {
	return fakeData{}
}
func (fakeCapabilityAdapter) Execution() venue.ExecutionClient {
	return fakeExecution{events: make(chan model.ExecutionEvent)}
}
func (fakeCapabilityAdapter) Capabilities() venue.DeclaredCapabilities {
	return venue.DeclaredCapabilities{
		Venue:       "FAKE",
		Instruments: true,
		MarketData:  venue.MarketDataCapabilities{Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, Streams: true},
		Execution: venue.ExecutionCapabilities{
			Submit:          true,
			Cancel:          true,
			Modify:          true,
			Query:           true,
			OrderReports:    true,
			FillReports:     true,
			PositionReports: true,
			PrivateStream:   true,
		},
		Account: venue.AccountCapabilities{Snapshot: true},
	}
}
func (fakeCapabilityAdapter) Close(context.Context) error { return nil }

type fakeClaimedModifyAdapter struct{}

func (fakeClaimedModifyAdapter) Venue() model.Venue                    { return "FAKE" }
func (fakeClaimedModifyAdapter) Instruments() venue.InstrumentProvider { return nil }
func (fakeClaimedModifyAdapter) Data() venue.DataClient                { return nil }
func (fakeClaimedModifyAdapter) Execution() venue.ExecutionClient {
	return fakeCoreExecution{events: make(chan model.ExecutionEvent)}
}
func (fakeClaimedModifyAdapter) Capabilities() venue.DeclaredCapabilities {
	return venue.DeclaredCapabilities{
		Venue:     "FAKE",
		Execution: venue.ExecutionCapabilities{Modify: true},
	}
}
func (fakeClaimedModifyAdapter) Close(context.Context) error { return nil }

type fakeProvider struct{}

func (fakeProvider) LoadAll(context.Context) error { return nil }
func (fakeProvider) Get(model.InstrumentID) (model.Instrument, bool) {
	return model.Instrument{}, false
}
func (fakeProvider) List() []model.Instrument { return nil }

type fakeData struct{}

func (fakeData) Venue() model.Venue                    { return "FAKE" }
func (fakeData) ClientID() string                      { return "fake-data" }
func (fakeData) Instruments() venue.InstrumentProvider { return fakeProvider{} }
func (fakeData) Connect(context.Context) error         { return nil }
func (fakeData) Disconnect(context.Context) error      { return nil }
func (fakeData) Health() venue.DataHealth              { return venue.DataHealth{Connected: true} }
func (fakeData) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{}, nil
}
func (fakeData) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, nil
}
func (fakeData) SubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (fakeData) UnsubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (fakeData) Events() <-chan model.MarketEvent {
	return make(chan model.MarketEvent)
}

type fakeExecution struct {
	events chan model.ExecutionEvent
}

func (fakeExecution) Venue() model.Venue               { return "FAKE" }
func (fakeExecution) AccountID() model.AccountID       { return "acct" }
func (fakeExecution) Connect(context.Context) error    { return nil }
func (fakeExecution) Disconnect(context.Context) error { return nil }
func (fakeExecution) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true}
}
func (fakeExecution) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{AccountID: "acct", Venue: "FAKE"}, nil
}
func (fakeExecution) SubmitOrder(context.Context, model.SubmitOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (fakeExecution) CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (fakeExecution) ModifyOrder(context.Context, model.ModifyOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (fakeExecution) QueryOrder(context.Context, model.QueryOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (fakeExecution) GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error) {
	return nil, nil
}
func (fakeExecution) GenerateFillReports(context.Context, model.InstrumentID) ([]model.FillReport, error) {
	return nil, nil
}
func (fakeExecution) GeneratePositionStatusReports(context.Context, model.InstrumentID) ([]model.PositionStatusReport, error) {
	return nil, nil
}
func (f fakeExecution) Events() <-chan model.ExecutionEvent { return f.events }
func (fakeExecution) ResubscribeExecution(context.Context) error {
	return nil
}

type fakeCoreExecution struct {
	events chan model.ExecutionEvent
}

func (fakeCoreExecution) Venue() model.Venue               { return "FAKE" }
func (fakeCoreExecution) AccountID() model.AccountID       { return "acct" }
func (fakeCoreExecution) Connect(context.Context) error    { return nil }
func (fakeCoreExecution) Disconnect(context.Context) error { return nil }
func (fakeCoreExecution) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true}
}
func (fakeCoreExecution) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{AccountID: "acct", Venue: "FAKE"}, nil
}
func (fakeCoreExecution) SubmitOrder(context.Context, model.SubmitOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (fakeCoreExecution) CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (fakeCoreExecution) GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error) {
	return nil, nil
}
func (f fakeCoreExecution) Events() <-chan model.ExecutionEvent { return f.events }

func TestDataTesterReportsNautilusStyleCaseResults(t *testing.T) {
	tester := NewDataTester(DataTesterConfig{
		Provider:     fakeProviderWithInstrument{},
		Data:         fakeDataWithEvents{events: make(chan model.MarketEvent, 1)},
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
	})

	report := tester.Run(context.Background(), t)

	require.Equal(t, "data", report.Suite)
	require.True(t, report.Passed(), "all cases should pass: %#v", report)
	requireCasePassed(t, report, "TC-D01", "Request instruments")
	requireCasePassed(t, report, "TC-D02", "Fetch ticker")
	requireCasePassed(t, report, "TC-D03", "Fetch order book")
	requireCasePassed(t, report, "TC-D11", "Subscribe ticker")
	requireCasePassed(t, report, "TC-D12", "Subscribe book depth")
	requireCasePassed(t, report, "TC-D13", "Subscribe trade ticks")
	requireCasePassed(t, report, "TC-D14", "Subscribe bars")
	requireCasePassed(t, report, "TC-D15", "Subscribe quote ticks")
}

func TestDataTesterRetriesTransientSubscribeEOF(t *testing.T) {
	data := &flakySubscribeData{fakeDataWithEvents: fakeDataWithEvents{events: make(chan model.MarketEvent, 1)}}
	tester := NewDataTester(DataTesterConfig{
		Provider:     fakeProviderWithInstrument{},
		Data:         data,
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
	})

	report := tester.Run(context.Background(), t)

	require.True(t, report.Passed(), "all cases should pass after retry: %#v", report)
	requireCasePassed(t, report, "TC-D12", "Subscribe book depth")
	require.GreaterOrEqual(t, data.subscribeCalls, 2)
	require.Equal(t, 1, data.disconnects)
	require.Equal(t, 1, data.connects)
}

func TestDataTesterAllPassedRejectsSkippedStreamCases(t *testing.T) {
	tester := NewDataTester(DataTesterConfig{
		Provider:     fakeProviderWithInstrument{},
		Data:         fakeNonStreamingData{},
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
	})

	report := tester.Run(context.Background(), t)

	require.True(t, report.Passed(), "optional contract pass still allows skipped cases")
	require.False(t, report.AllPassed(), "strict contract pass must reject skipped stream cases: %#v", report.Cases)
	requireCaseSkipped(t, report, "TC-D12", "data client does not implement StreamingDataClient")
}

func TestExecTesterReportsNautilusStyleCaseResults(t *testing.T) {
	events := make(chan model.ExecutionEvent, 2)
	exec := fakeExecutionWithLifecycle{events: events}
	tester := NewExecTester(ExecTesterConfig{
		Execution:    exec,
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
	})

	report := tester.Run(context.Background(), t)

	require.Equal(t, "execution", report.Suite)
	require.True(t, report.Passed(), "all cases should pass: %#v", report)
	requireCasePassed(t, report, "TC-E01", "Query account snapshot")
	requireCasePassed(t, report, "TC-E02", "Submit market order")
	requireCasePassed(t, report, "TC-E03", "Cancel order")
	requireCasePassed(t, report, "TC-E04", "Modify order")
	requireCasePassed(t, report, "TC-E05", "Query order")
	requireCasePassed(t, report, "TC-E80", "Generate order status reports")
	requireCasePassed(t, report, "TC-E81", "Generate fill reports")
	requireCasePassed(t, report, "TC-E82", "Generate position status reports")
	requireCasePassed(t, report, "TC-E84", "Resubscribe private stream")
}

func TestExecTesterModifyCaseCleansUpSubmittedLimitOrder(t *testing.T) {
	exec := &cleanupTrackingExecution{fakeExecutionWithLifecycle: fakeExecutionWithLifecycle{events: make(chan model.ExecutionEvent, 2)}}
	tester := NewExecTester(ExecTesterConfig{
		Execution:    exec,
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
	})

	report := tester.Run(context.Background(), t)

	require.True(t, report.Passed(), "all cases should pass: %#v", report)
	require.Contains(t, exec.canceledClientIDs, model.ClientOrderID("tc-e04-limit"))
}

func TestExecTesterAllPassedRejectsSkippedPrivateResubscribe(t *testing.T) {
	exec := fakeExecutionWithoutResubscribe{events: make(chan model.ExecutionEvent, 1)}
	tester := NewExecTester(ExecTesterConfig{
		Execution:    exec,
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
	})

	report := tester.Run(context.Background(), t)

	require.True(t, report.Passed(), "optional contract pass still allows skipped cases")
	require.False(t, report.AllPassed(), "strict contract pass must reject skipped resubscribe case: %#v", report.Cases)
	requireCaseSkipped(t, report, "TC-E84", "execution client does not implement ExecutionResubscriber")
}

func TestVenueContractSuiteChecksExpectedInstrumentMetadata(t *testing.T) {
	events := make(chan model.ExecutionEvent, 2)
	RunVenueContractSuite(t, VenueContractConfig{
		Provider:            fakeProviderWithInstrumentMetadata{},
		Data:                fakeDataWithEvents{events: make(chan model.MarketEvent, 1)},
		Execution:           fakeExecutionWithLifecycle{events: events},
		InstrumentID:        model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
		ExpectedMakerFee:    dec("0.0002"),
		ExpectedTakerFee:    dec("0.0005"),
		ExpectedMarginInit:  dec("0.10"),
		ExpectedMarginMaint: dec("0.05"),
	})
}

func requireCasePassed(t *testing.T, report ContractReport, id string, name string) {
	t.Helper()
	for _, result := range report.Cases {
		if result.ID == id {
			require.Equal(t, name, result.Name)
			require.Equal(t, CasePassed, result.Status, "case %s failed: %s", id, result.Error)
			return
		}
	}
	require.Failf(t, "missing case", "case %s not found in %#v", id, report.Cases)
}

func requireCaseSkipped(t *testing.T, report ContractReport, id string, reason string) {
	t.Helper()
	for _, result := range report.Cases {
		if result.ID == id {
			require.Equal(t, CaseSkipped, result.Status, "case %s status", id)
			require.Contains(t, result.Error, reason)
			return
		}
	}
	require.Failf(t, "missing case", "case %s not found in %#v", id, report.Cases)
}

func requireCaseFailed(t *testing.T, report ContractReport, id string, reason string) {
	t.Helper()
	for _, result := range report.Cases {
		if result.ID == id {
			require.Equal(t, CaseFailed, result.Status, "case %s status", id)
			require.Contains(t, result.Error, reason)
			return
		}
	}
	require.Failf(t, "missing case", "case %s not found in %#v", id, report.Cases)
}

type fakeProviderWithInstrument struct{}

func (fakeProviderWithInstrument) LoadAll(context.Context) error { return nil }
func (fakeProviderWithInstrument) Get(id model.InstrumentID) (model.Instrument, bool) {
	if id != model.MustInstrumentID("BTC-USDT-SPOT.FAKE") {
		return model.Instrument{}, false
	}
	return fakeInstrument(id), true
}
func (fakeProviderWithInstrument) List() []model.Instrument {
	return []model.Instrument{fakeInstrument(model.MustInstrumentID("BTC-USDT-SPOT.FAKE"))}
}

type fakeProviderWithInstrumentMetadata struct{}

func (fakeProviderWithInstrumentMetadata) LoadAll(context.Context) error { return nil }
func (fakeProviderWithInstrumentMetadata) Get(id model.InstrumentID) (model.Instrument, bool) {
	if id != model.MustInstrumentID("BTC-USDT-SPOT.FAKE") {
		return model.Instrument{}, false
	}
	inst := fakeInstrument(id)
	inst.MakerFee = dec("0.0002")
	inst.TakerFee = dec("0.0005")
	inst.MarginInit = dec("0.10")
	inst.MarginMaint = dec("0.05")
	return inst, true
}
func (fakeProviderWithInstrumentMetadata) List() []model.Instrument {
	inst, _ := fakeProviderWithInstrumentMetadata{}.Get(model.MustInstrumentID("BTC-USDT-SPOT.FAKE"))
	return []model.Instrument{inst}
}

type fakeDataWithEvents struct {
	events chan model.MarketEvent
}

func (fakeDataWithEvents) Venue() model.Venue                    { return "FAKE" }
func (fakeDataWithEvents) ClientID() string                      { return "fake-data-with-events" }
func (fakeDataWithEvents) Instruments() venue.InstrumentProvider { return fakeProviderWithInstrument{} }
func (fakeDataWithEvents) Connect(context.Context) error         { return nil }
func (fakeDataWithEvents) Disconnect(context.Context) error      { return nil }
func (fakeDataWithEvents) Health() venue.DataHealth {
	return venue.DataHealth{Connected: true, InstrumentReady: true}
}
func (fakeDataWithEvents) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
		Bid:          dec("100"),
		Ask:          dec("101"),
		Last:         dec("100.5"),
	}, nil
}
func (fakeDataWithEvents) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
		Bids: []model.OrderBookLevel{{
			Price: dec("100"),
			Size:  dec("1"),
		}},
		Asks: []model.OrderBookLevel{{
			Price: dec("101"),
			Size:  dec("1"),
		}},
	}, nil
}
func (fakeDataWithEvents) SubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (fakeDataWithEvents) UnsubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (f fakeDataWithEvents) Events() <-chan model.MarketEvent { return f.events }

type flakySubscribeData struct {
	fakeDataWithEvents
	subscribeCalls int
	connects       int
	disconnects    int
	failedOnce     bool
}

func (f *flakySubscribeData) Connect(context.Context) error {
	f.connects++
	return nil
}

func (f *flakySubscribeData) Disconnect(context.Context) error {
	f.disconnects++
	return nil
}

func (f *flakySubscribeData) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	f.subscribeCalls++
	if sub.Type == model.MarketDataTypeOrderBook && !f.failedOnce {
		f.failedOnce = true
		return io.EOF
	}
	return nil
}

type fakeNonStreamingData struct{}

func (fakeNonStreamingData) Venue() model.Venue { return "FAKE" }
func (fakeNonStreamingData) ClientID() string   { return "fake-non-streaming-data" }
func (fakeNonStreamingData) Instruments() venue.InstrumentProvider {
	return fakeProviderWithInstrument{}
}
func (fakeNonStreamingData) Connect(context.Context) error    { return nil }
func (fakeNonStreamingData) Disconnect(context.Context) error { return nil }
func (fakeNonStreamingData) Health() venue.DataHealth {
	return venue.DataHealth{Connected: true, InstrumentReady: true}
}
func (fakeNonStreamingData) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return fakeDataWithEvents{}.FetchTicker(context.Background(), model.MustInstrumentID("BTC-USDT-SPOT.FAKE"))
}
func (fakeNonStreamingData) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return fakeDataWithEvents{}.FetchOrderBook(context.Background(), model.MustInstrumentID("BTC-USDT-SPOT.FAKE"), 10)
}

type fakeExecutionWithLifecycle struct {
	events chan model.ExecutionEvent
}

type cleanupTrackingExecution struct {
	fakeExecutionWithLifecycle
	canceledClientIDs []model.ClientOrderID
}

func (e *cleanupTrackingExecution) CancelOrder(ctx context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	e.canceledClientIDs = append(e.canceledClientIDs, cancel.ClientOrderID)
	return e.fakeExecutionWithLifecycle.CancelOrder(ctx, cancel)
}

func (fakeExecutionWithLifecycle) Venue() model.Venue         { return "FAKE" }
func (fakeExecutionWithLifecycle) AccountID() model.AccountID { return "acct" }
func (fakeExecutionWithLifecycle) Connect(context.Context) error {
	return nil
}
func (fakeExecutionWithLifecycle) Disconnect(context.Context) error {
	return nil
}
func (fakeExecutionWithLifecycle) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true}
}
func (fakeExecutionWithLifecycle) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{AccountID: "acct", Venue: "FAKE"}, nil
}
func (fakeExecutionWithLifecycle) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{
		AccountID:      order.AccountID,
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
func (fakeExecutionWithLifecycle) CancelOrder(_ context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{
		AccountID:     cancel.AccountID,
		InstrumentID:  cancel.InstrumentID,
		OrderID:       cancel.OrderID,
		ClientOrderID: cancel.ClientOrderID,
		Status:        model.OrderStatusCanceled,
	}, nil
}
func (fakeExecutionWithLifecycle) ModifyOrder(_ context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{
		AccountID:      modify.AccountID,
		InstrumentID:   modify.InstrumentID,
		OrderID:        modify.OrderID,
		ClientOrderID:  modify.ClientOrderID,
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("0.01"),
		LeavesQuantity: decimal.RequireFromString("0.01"),
		Price:          modify.Price,
	}, nil
}
func (fakeExecutionWithLifecycle) QueryOrder(_ context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{
		AccountID:      query.AccountID,
		InstrumentID:   query.InstrumentID,
		OrderID:        query.OrderID,
		ClientOrderID:  query.ClientOrderID,
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeMarket,
		Quantity:       decimal.RequireFromString("0.01"),
		LeavesQuantity: decimal.RequireFromString("0.01"),
	}, nil
}
func (fakeExecutionWithLifecycle) GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error) {
	return []model.OrderStatusReport{{
		AccountID:    "acct",
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
		OrderID:      "order-1",
		Status:       model.OrderStatusAccepted,
	}}, nil
}
func (fakeExecutionWithLifecycle) GenerateFillReports(context.Context, model.InstrumentID) ([]model.FillReport, error) {
	return []model.FillReport{{
		AccountID:    "acct",
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
		OrderID:      "order-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("0.01"),
		Fee:          decimal.Zero,
		FeeCurrency:  "USDT",
	}}, nil
}
func (fakeExecutionWithLifecycle) GeneratePositionStatusReports(context.Context, model.InstrumentID) ([]model.PositionStatusReport, error) {
	return []model.PositionStatusReport{{
		AccountID:    "acct",
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.FAKE"),
		PositionID:   "pos-1",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("0.01"),
		EntryPrice:   decimal.RequireFromString("100"),
	}}, nil
}
func (f fakeExecutionWithLifecycle) Events() <-chan model.ExecutionEvent { return f.events }
func (fakeExecutionWithLifecycle) ResubscribeExecution(context.Context) error {
	return nil
}

type fakeExecutionWithoutResubscribe struct {
	events chan model.ExecutionEvent
}

func (f fakeExecutionWithoutResubscribe) Venue() model.Venue {
	return fakeExecutionWithLifecycle{}.Venue()
}
func (f fakeExecutionWithoutResubscribe) AccountID() model.AccountID {
	return fakeExecutionWithLifecycle{}.AccountID()
}
func (f fakeExecutionWithoutResubscribe) Connect(ctx context.Context) error {
	return fakeExecutionWithLifecycle{}.Connect(ctx)
}
func (f fakeExecutionWithoutResubscribe) Disconnect(ctx context.Context) error {
	return fakeExecutionWithLifecycle{}.Disconnect(ctx)
}
func (f fakeExecutionWithoutResubscribe) Health() venue.ExecutionHealth {
	return fakeExecutionWithLifecycle{}.Health()
}
func (f fakeExecutionWithoutResubscribe) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	return fakeExecutionWithLifecycle{}.QueryAccount(ctx)
}
func (f fakeExecutionWithoutResubscribe) SubmitOrder(ctx context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	return fakeExecutionWithLifecycle{}.SubmitOrder(ctx, order)
}
func (f fakeExecutionWithoutResubscribe) CancelOrder(ctx context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	return fakeExecutionWithLifecycle{}.CancelOrder(ctx, cancel)
}
func (f fakeExecutionWithoutResubscribe) ModifyOrder(ctx context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	return fakeExecutionWithLifecycle{}.ModifyOrder(ctx, modify)
}
func (f fakeExecutionWithoutResubscribe) QueryOrder(ctx context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	return fakeExecutionWithLifecycle{}.QueryOrder(ctx, query)
}
func (f fakeExecutionWithoutResubscribe) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	return fakeExecutionWithLifecycle{}.GenerateOrderStatusReports(ctx, id)
}
func (f fakeExecutionWithoutResubscribe) GenerateFillReports(ctx context.Context, id model.InstrumentID) ([]model.FillReport, error) {
	return fakeExecutionWithLifecycle{}.GenerateFillReports(ctx, id)
}
func (f fakeExecutionWithoutResubscribe) GeneratePositionStatusReports(ctx context.Context, id model.InstrumentID) ([]model.PositionStatusReport, error) {
	return fakeExecutionWithLifecycle{}.GeneratePositionStatusReports(ctx, id)
}
func (f fakeExecutionWithoutResubscribe) Events() <-chan model.ExecutionEvent { return f.events }

func fakeInstrument(id model.InstrumentID) model.Instrument {
	return model.Instrument{
		ID:        id,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: dec("0.01"),
		SizeTick:  dec("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}
}

func dec(value string) decimal.Decimal {
	return decimal.RequireFromString(value)
}
