package okx

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	okxsdk "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/QuantProcessing/exchanges/venue"
)

type Adapter struct {
	provider *productProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewSpotAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := okxsdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
	provider := newSpotProvider(client)
	return &Adapter{provider: provider, data: newDataClient("okx-spot-data", provider, client), exec: newExecutionClient(opts.AccountID, provider, client, "SPOT", "cash", opts.APIKey, opts.SecretKey, opts.Passphrase)}, nil
}

func NewSwapAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := okxsdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
	provider := newSwapProvider(client)
	return &Adapter{provider: provider, data: newDataClient("okx-swap-data", provider, client), exec: newExecutionClient(opts.AccountID, provider, client, "SWAP", "cross", opts.APIKey, opts.SecretKey, opts.Passphrase)}, nil
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
		MarketData:  venue.MarketDataCapabilities{Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, Streams: true},
		Execution:   venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true},
		Account:     venue.AccountCapabilities{Snapshot: true},
	}
}
