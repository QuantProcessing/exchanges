package binance

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	perpsdk "github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

type SpotAdapter struct {
	provider *spotProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

var _ venue.Adapter = (*SpotAdapter)(nil)

func NewSpotAdapter(_ context.Context, opts Options) (*SpotAdapter, error) {
	client := spot.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	provider := newSpotProvider(client)
	return &SpotAdapter{
		provider: provider,
		data:     newSpotDataClient("binance-spot-data", provider, client),
		exec:     newSpotExecutionClient(opts.AccountID, provider, client, opts.APIKey, opts.SecretKey),
	}, nil
}

func (a *SpotAdapter) Venue() model.Venue                    { return Venue }
func (a *SpotAdapter) Instruments() venue.InstrumentProvider { return a.provider }
func (a *SpotAdapter) Data() venue.DataClient                { return a.data }
func (a *SpotAdapter) Execution() venue.ExecutionClient      { return a.exec }
func (a *SpotAdapter) Close(ctx context.Context) error {
	_ = a.data.Disconnect(ctx)
	return a.exec.Disconnect(ctx)
}
func (a *SpotAdapter) Capabilities() venue.DeclaredCapabilities {
	return venue.DeclaredCapabilities{
		Venue:       Venue,
		Instruments: true,
		MarketData:  venue.MarketDataCapabilities{Snapshots: true, Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, Streams: true},
		Execution:   venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true, Resubscribe: true},
		Account:     venue.AccountCapabilities{Snapshot: true},
	}
}

type PerpAdapter struct {
	provider *perpProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

var _ venue.Adapter = (*PerpAdapter)(nil)

func NewPerpAdapter(_ context.Context, opts Options) (*PerpAdapter, error) {
	client := perpsdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	provider := newPerpProvider(client)
	return &PerpAdapter{
		provider: provider,
		data:     newPerpDataClient("binance-perp-data", provider, client),
		exec:     newPerpExecutionClient(opts.AccountID, provider, client, opts.APIKey, opts.SecretKey),
	}, nil
}

func (a *PerpAdapter) Venue() model.Venue                    { return Venue }
func (a *PerpAdapter) Instruments() venue.InstrumentProvider { return a.provider }
func (a *PerpAdapter) Data() venue.DataClient                { return a.data }
func (a *PerpAdapter) Execution() venue.ExecutionClient      { return a.exec }
func (a *PerpAdapter) Close(ctx context.Context) error {
	_ = a.data.Disconnect(ctx)
	return a.exec.Disconnect(ctx)
}
func (a *PerpAdapter) Capabilities() venue.DeclaredCapabilities {
	return venue.DeclaredCapabilities{
		Venue:       Venue,
		Instruments: true,
		MarketData:  venue.MarketDataCapabilities{Snapshots: true, Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, FundingRates: true, Streams: true},
		Execution:   venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true, Resubscribe: true},
		Account:     venue.AccountCapabilities{Snapshot: true},
	}
}
