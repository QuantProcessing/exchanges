package testsuite

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type VenueContractConfig struct {
	Provider             venue.InstrumentProvider
	Data                 venue.DataClient
	Execution            venue.ExecutionClient
	InstrumentID         model.InstrumentID
	Capabilities         venue.DeclaredCapabilities
	RequirePrivateStream bool

	ExpectedMakerFee    decimal.Decimal
	ExpectedTakerFee    decimal.Decimal
	ExpectedMarginInit  decimal.Decimal
	ExpectedMarginMaint decimal.Decimal
}

func RunVenueContractSuite(t *testing.T, cfg VenueContractConfig) {
	t.Helper()
	require.NotNil(t, cfg.Provider)
	require.NotNil(t, cfg.Data)
	require.NotNil(t, cfg.Execution)
	require.NoError(t, cfg.InstrumentID.Validate())

	ctx := context.Background()
	dataReport := NewDataTester(DataTesterConfig{
		Provider:     cfg.Provider,
		Data:         cfg.Data,
		InstrumentID: cfg.InstrumentID,
	}).Run(ctx, t)
	require.True(t, dataReport.RequiredPassed(requiredDataCaseIDs(cfg.Capabilities)...), "data contract failed: %#v", dataReport.Cases)
	requireExpectedInstrumentMetadata(t, cfg)
	require.NoError(t, cfg.Data.Disconnect(ctx))

	require.NoError(t, cfg.Execution.Connect(ctx))
	require.True(t, cfg.Execution.Health().Connected)
	execReport := NewExecTester(ExecTesterConfig{
		Execution:    cfg.Execution,
		InstrumentID: cfg.InstrumentID,
	}).Run(ctx, t)
	require.True(t, execReport.RequiredPassed(requiredExecutionCaseIDs(cfg.Capabilities, cfg.RequirePrivateStream)...), "execution contract failed: %#v", execReport.Cases)
	require.NoError(t, cfg.Execution.Disconnect(ctx))
}

func requiredDataCaseIDs(caps venue.DeclaredCapabilities) []string {
	if caps.Venue == "" && !caps.Instruments && caps.MarketData == (venue.MarketDataCapabilities{}) {
		return []string{"TC-D01", "TC-D02", "TC-D03", "TC-D11", "TC-D12", "TC-D13", "TC-D14", "TC-D15"}
	}
	ids := []string{"TC-D01"}
	if caps.MarketData.Ticker {
		ids = append(ids, "TC-D02")
	}
	if caps.MarketData.OrderBook {
		ids = append(ids, "TC-D03")
	}
	if caps.MarketData.Streams && caps.MarketData.TickerStream {
		ids = append(ids, "TC-D11")
	}
	if caps.MarketData.Streams && caps.MarketData.OrderBookStream {
		ids = append(ids, "TC-D12")
	}
	if caps.MarketData.Streams && caps.MarketData.TradeTicks {
		ids = append(ids, "TC-D13")
	}
	if caps.MarketData.Streams && caps.MarketData.Bars {
		ids = append(ids, "TC-D14")
	}
	if caps.MarketData.Streams && caps.MarketData.QuoteTicks {
		ids = append(ids, "TC-D15")
	}
	return ids
}

func requiredExecutionCaseIDs(caps venue.DeclaredCapabilities, requirePrivateStream bool) []string {
	if caps.Venue == "" && caps.Execution == (venue.ExecutionCapabilities{}) && caps.Account == (venue.AccountCapabilities{}) {
		return []string{"TC-E01", "TC-E02", "TC-E03", "TC-E80", "TC-E84"}
	}
	ids := make([]string, 0, 5)
	if caps.Account.Snapshot {
		ids = append(ids, "TC-E01")
	}
	if caps.Execution.Submit {
		ids = append(ids, "TC-E02")
	}
	if caps.Execution.Cancel {
		ids = append(ids, "TC-E03")
	}
	if caps.Execution.OrderReports {
		ids = append(ids, "TC-E80")
	}
	if caps.Execution.PrivateStream && requirePrivateStream {
		ids = append(ids, "TC-E84")
	}
	return ids
}

func requireExpectedInstrumentMetadata(t *testing.T, cfg VenueContractConfig) {
	t.Helper()
	if cfg.ExpectedMakerFee.IsZero() &&
		cfg.ExpectedTakerFee.IsZero() &&
		cfg.ExpectedMarginInit.IsZero() &&
		cfg.ExpectedMarginMaint.IsZero() {
		return
	}
	inst, ok := cfg.Provider.Get(cfg.InstrumentID)
	require.True(t, ok, "expected instrument metadata for %s", cfg.InstrumentID)
	if cfg.ExpectedMakerFee.IsPositive() {
		require.Equal(t, cfg.ExpectedMakerFee.String(), inst.MakerFee.String(), "maker fee")
	}
	if cfg.ExpectedTakerFee.IsPositive() {
		require.Equal(t, cfg.ExpectedTakerFee.String(), inst.TakerFee.String(), "taker fee")
	}
	if cfg.ExpectedMarginInit.IsPositive() {
		require.Equal(t, cfg.ExpectedMarginInit.String(), inst.MarginInit.String(), "initial margin")
	}
	if cfg.ExpectedMarginMaint.IsPositive() {
		require.Equal(t, cfg.ExpectedMarginMaint.String(), inst.MarginMaint.String(), "maintenance margin")
	}
}

type AdapterCapabilityConfig struct {
	Adapter venue.Adapter
}

func RunAdapterCapabilitySuite(t *testing.T, cfg AdapterCapabilityConfig) {
	t.Helper()
	require.NotNil(t, cfg.Adapter)

	caps := cfg.Adapter.Capabilities()
	require.Equal(t, cfg.Adapter.Venue(), caps.Venue)

	if caps.Instruments {
		require.NotNil(t, cfg.Adapter.Instruments(), "instrument capability requires an instrument provider")
	}
	if caps.MarketData.Ticker || caps.MarketData.OrderBook || caps.MarketData.TickerStream || caps.MarketData.OrderBookStream || caps.MarketData.TradeTicks || caps.MarketData.QuoteTicks || caps.MarketData.Bars || caps.MarketData.Streams {
		require.NotNil(t, cfg.Adapter.Data(), "market-data capability requires a data client")
		require.Equal(t, cfg.Adapter.Venue(), cfg.Adapter.Data().Venue())
	}
	if caps.MarketData.Streams || caps.MarketData.TickerStream || caps.MarketData.OrderBookStream || caps.MarketData.TradeTicks || caps.MarketData.QuoteTicks || caps.MarketData.Bars {
		require.Implements(t, (*venue.StreamingDataClient)(nil), cfg.Adapter.Data(), "stream capability requires venue.StreamingDataClient")
	}
	if caps.Execution.Submit || caps.Execution.Cancel || caps.Execution.OrderReports || caps.Execution.PrivateStream || caps.Account.Snapshot {
		require.NotNil(t, cfg.Adapter.Execution(), "execution or account capability requires an execution client")
		require.Equal(t, cfg.Adapter.Venue(), cfg.Adapter.Execution().Venue())
	}
	if caps.Execution.PrivateStream {
		require.NotNil(t, cfg.Adapter.Execution().Events(), "private-stream capability requires execution events")
		require.Implements(t, (*venue.ExecutionResubscriber)(nil), cfg.Adapter.Execution(), "private-stream capability requires explicit resubscribe support")
	}
}
