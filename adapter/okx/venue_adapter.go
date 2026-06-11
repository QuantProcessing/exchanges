package okx

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	sdkokx "github.com/QuantProcessing/exchanges/sdk/okx"
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
	if _, err := opts.quoteCurrency(); err != nil {
		return nil, err
	}
	accountID := opts.AccountID
	if accountID == "" {
		accountID = "okx"
	}
	client := sdkokx.NewClient()
	if opts.hasFullCredentials() {
		client.WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
	}
	instruments := newInstrumentProvider(client)
	return &VenueAdapter{
		instruments: instruments,
		marketData:  newMarketDataClient(instruments, client),
		execution:   newExecutionClient(accountID, instruments, client),
	}, nil
}

func (a *VenueAdapter) Venue() model.Venue {
	return model.VenueOKX
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
		Venue:        model.VenueOKX,
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
			Modify:       true,
			CancelAll:    true,
			OrderReports: true,
			FillReports:  false,
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
