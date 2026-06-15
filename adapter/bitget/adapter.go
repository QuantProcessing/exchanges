package bitget

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	bitgetsdk "github.com/QuantProcessing/exchanges/sdk/bitget"
	"github.com/QuantProcessing/exchanges/venue"
)

type Adapter struct {
	provider *productProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewSpotAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := bitgetsdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
	provider := newSpotProvider(client)
	return &Adapter{provider: provider, data: newDataClient("bitget-spot-data", provider, client), exec: newExecutionClient(opts.AccountID, provider, client, "SPOT", opts.APIKey, opts.SecretKey, opts.Passphrase)}, nil
}

func NewPerpAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := bitgetsdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
	provider := newPerpProvider(client)
	return &Adapter{provider: provider, data: newDataClient("bitget-perp-data", provider, client), exec: newExecutionClient(opts.AccountID, provider, client, "USDT-FUTURES", opts.APIKey, opts.SecretKey, opts.Passphrase)}, nil
}

func (a *Adapter) Venue() model.Venue                    { return Venue }
func (a *Adapter) Instruments() venue.InstrumentProvider { return a.provider }
func (a *Adapter) Data() venue.DataClient                { return a.data }
func (a *Adapter) Execution() venue.ExecutionClient      { return a.exec }

func (a *Adapter) Close(ctx context.Context) error {
	_ = a.data.Disconnect(ctx)
	return a.exec.Disconnect(ctx)
}

func (a *Adapter) Capabilities() venue.DeclaredCapabilities {
	return venue.DeclaredCapabilities{
		Venue:       Venue,
		Instruments: true,
		MarketData:  venue.MarketDataCapabilities{Snapshots: true, Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, Streams: true},
		Execution:   venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true, Resubscribe: true},
		Account:     venue.AccountCapabilities{Snapshot: true},
	}
}
