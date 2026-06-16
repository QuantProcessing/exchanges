package testsuite

import (
	"context"
	"fmt"
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
	if caps.MarketData.FundingRates {
		ids = append(ids, "TC-D16")
	}
	if caps.MarketData.Streams && caps.MarketData.FundingRateStream {
		ids = append(ids, "TC-D17")
	}
	return ids
}

func requiredExecutionCaseIDs(caps venue.DeclaredCapabilities, requirePrivateStream bool) []string {
	if caps.Venue == "" && caps.Execution == (venue.ExecutionCapabilities{}) && caps.Account == (venue.AccountCapabilities{}) {
		return []string{"TC-E01", "TC-E02", "TC-E03", "TC-E04", "TC-E05", "TC-E80", "TC-E81", "TC-E82", "TC-E84"}
	}
	ids := make([]string, 0, 9)
	if caps.Account.Snapshot {
		ids = append(ids, "TC-E01")
	}
	if caps.Execution.Submit {
		ids = append(ids, "TC-E02")
	}
	if caps.Execution.Cancel {
		ids = append(ids, "TC-E03")
	}
	if caps.Execution.Modify {
		ids = append(ids, "TC-E04")
	}
	if caps.Execution.Query {
		ids = append(ids, "TC-E05")
	}
	if caps.Execution.OrderReports {
		ids = append(ids, "TC-E80")
	}
	if caps.Execution.FillReports {
		ids = append(ids, "TC-E81")
	}
	if caps.Execution.PositionReports {
		ids = append(ids, "TC-E82")
	}
	if caps.Execution.Resubscribe && requirePrivateStream {
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
	report := AdapterCapabilityReport(t, cfg.Adapter)
	require.True(t, report.AllPassed(), "adapter capability contract failed: %#v", report.Cases)
}

func AdapterCapabilityReport(t *testing.T, adapter venue.Adapter) ContractReport {
	t.Helper()
	return runContractCases(t, "adapter", []contractCase{
		{id: "TC-A01", name: "Venue capability identity", run: func() error {
			if adapter == nil {
				return fmt.Errorf("adapter is nil")
			}
			caps := adapter.Capabilities()
			if caps.Venue != adapter.Venue() {
				return fmt.Errorf("venue mismatch: adapter=%s caps=%s", adapter.Venue(), caps.Venue)
			}
			return nil
		}},
		{id: "TC-A02", name: "Instrument provider capability", run: func() error {
			if adapter == nil {
				return fmt.Errorf("adapter is nil")
			}
			caps := adapter.Capabilities()
			if caps.Instruments && adapter.Instruments() == nil {
				return fmt.Errorf("instrument capability requires an instrument provider")
			}
			return nil
		}},
		{id: "TC-A03", name: "Market data capability", run: func() error {
			if adapter == nil {
				return fmt.Errorf("adapter is nil")
			}
			caps := adapter.Capabilities()
			if !declaresMarketData(caps.MarketData) {
				return nil
			}
			data := adapter.Data()
			if data == nil {
				return fmt.Errorf("market-data capability requires a data client")
			}
			if data.Venue() != adapter.Venue() {
				return fmt.Errorf("market-data venue mismatch: %s", data.Venue())
			}
			if caps.MarketData.FundingRates {
				if _, ok := data.(venue.FundingRateProvider); !ok {
					return fmt.Errorf("funding-rate snapshot capability requires venue.FundingRateProvider")
				}
			}
			return nil
		}},
		{id: "TC-A04", name: "Streaming market data capability", run: func() error {
			if adapter == nil {
				return fmt.Errorf("adapter is nil")
			}
			caps := adapter.Capabilities()
			if !declaresStreamingMarketData(caps.MarketData) {
				return nil
			}
			data := adapter.Data()
			if data == nil {
				return fmt.Errorf("stream capability requires a data client")
			}
			if _, ok := data.(venue.StreamingDataClient); !ok {
				return fmt.Errorf("stream capability requires venue.StreamingDataClient")
			}
			return nil
		}},
		{id: "TC-A05", name: "Execution capability", run: func() error {
			if adapter == nil {
				return fmt.Errorf("adapter is nil")
			}
			caps := adapter.Capabilities()
			if !declaresExecution(caps.Execution, caps.Account) {
				return nil
			}
			exec := adapter.Execution()
			if exec == nil {
				return fmt.Errorf("execution or account capability requires an execution client")
			}
			if exec.Venue() != adapter.Venue() {
				return fmt.Errorf("execution venue mismatch: %s", exec.Venue())
			}
			return nil
		}},
		{id: "TC-A06", name: "Private execution stream capability", run: func() error {
			if adapter == nil {
				return fmt.Errorf("adapter is nil")
			}
			caps := adapter.Capabilities()
			if !caps.Execution.PrivateStream {
				return nil
			}
			exec := adapter.Execution()
			if exec == nil {
				return fmt.Errorf("private-stream capability requires an execution client")
			}
			if exec.Events() == nil {
				return fmt.Errorf("private-stream capability requires execution events")
			}
			return nil
		}},
		{id: "TC-A07", name: "Granular execution capability interfaces", run: func() error {
			if adapter == nil {
				return fmt.Errorf("adapter is nil")
			}
			caps := adapter.Capabilities()
			if !declaresGranularExecution(caps.Execution) {
				return nil
			}
			exec := adapter.Execution()
			if exec == nil {
				return fmt.Errorf("granular execution capability requires an execution client")
			}
			if caps.Execution.Modify {
				if _, ok := exec.(venue.OrderModifier); !ok {
					return fmt.Errorf("modify capability requires venue.OrderModifier")
				}
			}
			if caps.Execution.Query {
				if _, ok := exec.(venue.OrderQuerier); !ok {
					return fmt.Errorf("query capability requires venue.OrderQuerier")
				}
			}
			if caps.Execution.FillReports {
				if _, ok := exec.(venue.FillReportGenerator); !ok {
					return fmt.Errorf("fill-report capability requires venue.FillReportGenerator")
				}
			}
			if caps.Execution.PositionReports {
				if _, ok := exec.(venue.PositionStatusReportGenerator); !ok {
					return fmt.Errorf("position-report capability requires venue.PositionStatusReportGenerator")
				}
			}
			if caps.Execution.Resubscribe {
				if _, ok := exec.(venue.ExecutionResubscriber); !ok {
					return fmt.Errorf("resubscribe capability requires venue.ExecutionResubscriber")
				}
			}
			if caps.Execution.MassStatus {
				if _, ok := exec.(venue.ExecutionMassStatusGenerator); !ok {
					return fmt.Errorf("mass-status capability requires venue.ExecutionMassStatusGenerator")
				}
			}
			if caps.Execution.OrderLists {
				if _, ok := exec.(venue.OrderListSubmitter); !ok {
					return fmt.Errorf("order-list capability requires venue.OrderListSubmitter")
				}
			}
			return nil
		}},
	})
}

func declaresMarketData(caps venue.MarketDataCapabilities) bool {
	return caps.Snapshots || caps.Ticker || caps.OrderBook || caps.TickerStream || caps.OrderBookStream ||
		caps.TradeTicks || caps.QuoteTicks || caps.Bars || caps.FundingRates || caps.FundingRateStream || caps.Streams
}

func declaresStreamingMarketData(caps venue.MarketDataCapabilities) bool {
	return caps.Streams || caps.TickerStream || caps.OrderBookStream ||
		caps.TradeTicks || caps.QuoteTicks || caps.Bars || caps.FundingRateStream
}

func declaresExecution(caps venue.ExecutionCapabilities, account venue.AccountCapabilities) bool {
	return caps.Submit || caps.Cancel || caps.Modify || caps.Query || caps.OrderReports ||
		caps.FillReports || caps.PositionReports || caps.PrivateStream || caps.Resubscribe ||
		caps.MassStatus || caps.OrderLists || account.Snapshot
}

func declaresGranularExecution(caps venue.ExecutionCapabilities) bool {
	return caps.Modify || caps.Query || caps.FillReports || caps.PositionReports ||
		caps.Resubscribe || caps.MassStatus || caps.OrderLists
}
