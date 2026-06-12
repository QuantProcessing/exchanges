package binance

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

const defaultPerpAccountID model.AccountID = "binance-usdt-futures-master"

type PerpAdapter struct {
	instruments venue.InstrumentProvider
	marketData  venue.DataClient
	execution   venue.ExecutionClient
}

type Adapter = PerpAdapter

var _ venue.Adapter = (*PerpAdapter)(nil)

func NewAdapter(ctx context.Context, opts Options) (*PerpAdapter, error) {
	return NewPerpAdapter(ctx, opts)
}

func NewPerpAdapter(ctx context.Context, opts Options) (*PerpAdapter, error) {
	data, err := NewPerpDataClient(ctx, opts)
	if err != nil {
		return nil, err
	}
	exec, err := NewPerpExecutionClient(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &PerpAdapter{
		instruments: data.Instruments(),
		marketData:  data,
		execution:   exec,
	}, nil
}

func (a *PerpAdapter) Venue() model.Venue {
	return model.VenueBinance
}

func (a *PerpAdapter) Instruments() venue.InstrumentProvider {
	return a.instruments
}

func (a *PerpAdapter) MarketData() venue.MarketDataClient {
	return a.marketData
}

func (a *PerpAdapter) Execution() venue.ExecutionClient {
	return a.execution
}

func (a *PerpAdapter) Capabilities() venue.DeclaredCapabilities {
	return PerpCapabilities()
}

func (a *PerpAdapter) Close() error {
	if a.marketData != nil {
		_ = a.marketData.Disconnect(context.Background())
	}
	return a.execution.Disconnect(context.Background())
}

func PerpCapabilities() venue.DeclaredCapabilities {
	return venue.DeclaredCapabilities{
		Venue:        model.VenueBinance,
		AccountTypes: []model.AccountType{model.AccountTypeMargin},
		InstrumentTypes: []model.InstrumentType{
			model.InstrumentTypeCryptoPerp,
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
			CancelAll:    true,
			OrderReports: true,
			FillReports:  true,
		},
		AccountState: venue.AccountStateCapabilities{
			Snapshot:  true,
			Balances:  true,
			Margins:   true,
			Positions: true,
		},
		Reconciliation: venue.ReconciliationCapabilities{
			Startup:   true,
			Reconnect: true,
		},
	}
}

func DeclaredCapabilities() venue.DeclaredCapabilities {
	return PerpCapabilities()
}
