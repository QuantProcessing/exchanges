package binance

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

type VenueOptions struct {
	Options
	AccountID model.AccountID
}

type VenueAdapter struct {
	instruments *instrumentProvider
	marketData  *marketDataClient
	execution   *executionClient
}

var _ venue.Adapter = (*VenueAdapter)(nil)

func NewVenueAdapter(_ context.Context, opts VenueOptions) (*VenueAdapter, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}
	accountID := opts.AccountID
	if accountID == "" {
		accountID = "binance"
	}
	spotClient := spot.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	perpClient := perp.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	instruments := newInstrumentProvider(spotClient, perpClient)
	return &VenueAdapter{
		instruments: instruments,
		marketData:  newMarketDataClient(instruments, spotClient, perpClient),
		execution:   newExecutionClient(accountID, instruments, spotClient, perpClient),
	}, nil
}

func (a *VenueAdapter) Venue() model.Venue {
	return model.VenueBinance
}

func (a *VenueAdapter) Instruments() venue.InstrumentProvider {
	return a.instruments
}

func (a *VenueAdapter) MarketData() venue.MarketDataClient {
	return a.marketData
}

func (a *VenueAdapter) Execution() venue.ExecutionClient {
	return a.execution
}

func (a *VenueAdapter) Capabilities() venue.DeclaredCapabilities {
	return DeclaredCapabilities()
}

func (a *VenueAdapter) Close() error {
	return nil
}

func DeclaredCapabilities() venue.DeclaredCapabilities {
	return venue.DeclaredCapabilities{
		Venue:        model.VenueBinance,
		AccountTypes: []model.AccountType{model.AccountTypeCash, model.AccountTypeMargin},
		InstrumentTypes: []model.InstrumentType{
			model.InstrumentTypeCurrencyPair,
			model.InstrumentTypeCryptoPerp,
		},
		MarketData: venue.MarketDataCapabilities{
			Ticker:    true,
			OrderBook: true,
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
			Startup: true,
		},
	}
}
