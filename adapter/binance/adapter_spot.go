package binance

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

const defaultSpotAccountID model.AccountID = "binance-spot-master"

type SpotAdapter struct {
	instruments venue.InstrumentProvider
	marketData  venue.DataClient
	execution   venue.ExecutionClient
}

var _ venue.Adapter = (*SpotAdapter)(nil)

func NewSpotAdapter(ctx context.Context, opts Options) (*SpotAdapter, error) {
	data, err := NewSpotDataClient(ctx, opts)
	if err != nil {
		return nil, err
	}
	exec, err := NewSpotExecutionClient(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &SpotAdapter{
		instruments: data.Instruments(),
		marketData:  data,
		execution:   exec,
	}, nil
}

func (a *SpotAdapter) Venue() model.Venue {
	return model.VenueBinance
}

func (a *SpotAdapter) Instruments() venue.InstrumentProvider {
	return a.instruments
}

func (a *SpotAdapter) MarketData() venue.MarketDataClient {
	return a.marketData
}

func (a *SpotAdapter) Execution() venue.ExecutionClient {
	return a.execution
}

func (a *SpotAdapter) Capabilities() venue.DeclaredCapabilities {
	return SpotCapabilities()
}

func (a *SpotAdapter) Close() error {
	if a.marketData != nil {
		_ = a.marketData.Disconnect(context.Background())
	}
	return a.execution.Disconnect(context.Background())
}

func SpotCapabilities() venue.DeclaredCapabilities {
	return venue.DeclaredCapabilities{
		Venue:        model.VenueBinance,
		AccountTypes: []model.AccountType{model.AccountTypeCash},
		InstrumentTypes: []model.InstrumentType{
			model.InstrumentTypeCurrencyPair,
		},
		MarketData: venue.MarketDataCapabilities{
			Ticker:          true,
			OrderBook:       true,
			StreamTicker:    true,
			StreamOrderBook: true,
			StreamTrades:    true,
			StreamBars:      true,
			PrivateData:     true,
		},
		Execution: venue.ExecutionCapabilities{
			Submit:       true,
			Cancel:       true,
			OrderReports: true,
			FillReports:  true,
		},
		AccountState: venue.AccountStateCapabilities{
			Snapshot: true,
			Balances: true,
		},
		Reconciliation: venue.ReconciliationCapabilities{
			Startup:   true,
			Reconnect: true,
		},
	}
}
