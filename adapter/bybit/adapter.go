package bybit

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	bybitsdk "github.com/QuantProcessing/exchanges/sdk/bybit"
	"github.com/QuantProcessing/exchanges/venue"
)

type Adapter struct {
	provider *productProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewSpotAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := bybitsdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	provider := newSpotProvider(client)
	return &Adapter{provider: provider, data: newDataClient("bybit-spot-data", provider, client), exec: newExecutionClient(opts.AccountID, provider, client, "spot", opts.APIKey, opts.SecretKey)}, nil
}

func NewLinearAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := bybitsdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	provider := newLinearProvider(client)
	return &Adapter{provider: provider, data: newDataClient("bybit-linear-data", provider, client), exec: newExecutionClient(opts.AccountID, provider, client, "linear", opts.APIKey, opts.SecretKey)}, nil
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
