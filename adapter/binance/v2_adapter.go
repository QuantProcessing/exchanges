package binance

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

type V2Options struct {
	Options
	AccountID model.AccountID
}

type V2Adapter struct {
	instruments *v2InstrumentProvider
	marketData  *v2MarketDataClient
	execution   *v2ExecutionClient
}

var _ venue.Adapter = (*V2Adapter)(nil)

func NewV2Adapter(_ context.Context, opts V2Options) (*V2Adapter, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}
	accountID := opts.AccountID
	if accountID == "" {
		accountID = "binance"
	}
	spotClient := spot.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	perpClient := perp.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	instruments := newV2InstrumentProvider(spotClient, perpClient)
	return &V2Adapter{
		instruments: instruments,
		marketData:  newV2MarketDataClient(instruments, spotClient, perpClient),
		execution:   newV2ExecutionClient(accountID, instruments, spotClient, perpClient),
	}, nil
}

func (a *V2Adapter) Venue() model.Venue {
	return model.VenueBinance
}

func (a *V2Adapter) Instruments() venue.InstrumentProvider {
	return a.instruments
}

func (a *V2Adapter) MarketData() venue.MarketDataClient {
	return a.marketData
}

func (a *V2Adapter) Execution() venue.ExecutionClient {
	return a.execution
}

func (a *V2Adapter) Capabilities() venue.DeclaredCapabilities {
	return V2DeclaredCapabilities()
}

func (a *V2Adapter) Close() error {
	return nil
}

func V2DeclaredCapabilities() venue.DeclaredCapabilities {
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
