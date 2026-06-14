package hyperliquid

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	hlsdk "github.com/QuantProcessing/exchanges/sdk/hyperliquid"
	hlperp "github.com/QuantProcessing/exchanges/sdk/hyperliquid/perp"
	hlspot "github.com/QuantProcessing/exchanges/sdk/hyperliquid/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

type SpotAdapter struct {
	provider *spotProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewSpotAdapter(_ context.Context, opts Options) (*SpotAdapter, error) {
	vault := opts.Vault
	base := hlsdk.NewClient().WithCredentials(opts.PrivateKey, &vault).WithAccount(opts.AccountAddress)
	client := hlspot.NewClient(base)
	provider := newSpotProvider(client)
	exec := newSpotExecutionClient(opts.AccountID, opts.AccountAddress, provider, client)
	if opts.AccountAddress != "" {
		exec.privateWS = hlspot.NewWebsocketClient(hlsdk.NewWebsocketClient(context.Background()))
	}
	return &SpotAdapter{provider: provider, data: newSpotDataClient("hyperliquid-spot-data", provider, client), exec: exec}, nil
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
	return venue.DeclaredCapabilities{Venue: Venue, Instruments: true, MarketData: venue.MarketDataCapabilities{Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, Streams: true}, Execution: venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true}, Account: venue.AccountCapabilities{Snapshot: true}}
}

type PerpAdapter struct {
	provider *perpProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewPerpAdapter(_ context.Context, opts Options) (*PerpAdapter, error) {
	vault := opts.Vault
	base := hlsdk.NewClient().WithCredentials(opts.PrivateKey, &vault).WithAccount(opts.AccountAddress)
	client := hlperp.NewClient(base)
	provider := newPerpProvider(client)
	data := newPerpDataClient("hyperliquid-perp-data", provider, client)
	exec := newPerpExecutionClient(opts.AccountID, opts.AccountAddress, provider, client)
	if opts.AccountAddress != "" {
		ws := hlperp.NewWebsocketClient(hlsdk.NewWebsocketClient(context.Background()))
		if opts.PrivateKey != "" {
			ws = ws.WithCredentials(opts.PrivateKey, opts.AccountAddress)
		}
		exec.privateWS = ws
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
	return venue.DeclaredCapabilities{Venue: Venue, Instruments: true, MarketData: venue.MarketDataCapabilities{Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, Streams: true}, Execution: venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true}, Account: venue.AccountCapabilities{Snapshot: true}}
}
