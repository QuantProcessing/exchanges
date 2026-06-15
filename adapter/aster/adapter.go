package aster

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	asterperp "github.com/QuantProcessing/exchanges/sdk/aster/perp"
	asterspot "github.com/QuantProcessing/exchanges/sdk/aster/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

type SpotAdapter struct {
	provider *spotProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewSpotAdapter(_ context.Context, opts Options) (*SpotAdapter, error) {
	client := asterspot.NewClient(opts.APIKey, opts.SecretKey)
	provider := newSpotProvider(client)
	data := newSpotDataClient("aster-spot-data", provider, client)
	exec := newSpotExecutionClient(opts.AccountID, provider, client)
	if opts.APIKey != "" && opts.SecretKey != "" {
		exec.privateWS = asterspot.NewWsAccountClient(context.Background(), opts.APIKey, opts.SecretKey)
	}
	return &SpotAdapter{provider: provider, data: data, exec: exec}, nil
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
	return venue.DeclaredCapabilities{Venue: Venue, Instruments: true, MarketData: venue.MarketDataCapabilities{Snapshots: true, Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, Streams: true}, Execution: venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true, Resubscribe: true}, Account: venue.AccountCapabilities{Snapshot: true}}
}

type PerpAdapter struct {
	provider *perpProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewPerpAdapter(_ context.Context, opts Options) (*PerpAdapter, error) {
	client := asterperp.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	provider := newPerpProvider(client)
	data := newPerpDataClient("aster-perp-data", provider, client)
	exec := newPerpExecutionClient(opts.AccountID, provider, client)
	if opts.APIKey != "" && opts.SecretKey != "" {
		exec.privateWS = asterperp.NewWsAccountClient(context.Background(), opts.APIKey, opts.SecretKey)
	}
	return &PerpAdapter{provider: provider, data: data, exec: exec}, nil
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
	return venue.DeclaredCapabilities{Venue: Venue, Instruments: true, MarketData: venue.MarketDataCapabilities{Snapshots: true, Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, Streams: true}, Execution: venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true, Resubscribe: true}, Account: venue.AccountCapabilities{Snapshot: true}}
}
