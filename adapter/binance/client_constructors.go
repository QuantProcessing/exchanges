package binance

import (
	"context"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

const (
	defaultSpotDataClientID = "binance-spot-data"
	defaultPerpDataClientID = "binance-usdt-futures-data"
)

func NewSpotDataClient(ctx context.Context, opts Options) (venue.DataClient, error) {
	rest := newSpotRESTClient(opts, false)
	instruments := newInstrumentProvider(rest, nil)
	wsMarket := spot.NewWsMarketClient(ctx)
	marketData := newMarketDataClientWithID(defaultSpotDataClientID, instruments, rest, nil)
	marketData.spotStream = wsMarket
	return marketData, nil
}

func NewPerpDataClient(ctx context.Context, opts Options) (venue.DataClient, error) {
	rest := newPerpRESTClient(opts, false)
	instruments := newInstrumentProvider(nil, rest)
	wsMarket := perp.NewWsMarketClient(ctx)
	marketData := newMarketDataClientWithID(defaultPerpDataClientID, instruments, nil, rest)
	marketData.perpStream = wsMarket
	return marketData, nil
}

func NewSpotExecutionClient(ctx context.Context, opts Options) (venue.ExecutionClient, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}
	accountID := opts.AccountID
	if accountID == "" {
		accountID = defaultSpotAccountID
	}
	rest := newSpotRESTClient(opts, true)
	instruments := newInstrumentProvider(rest, nil)

	wsAPI := spot.NewWsAPIClient(ctx)
	if opts.BaseURLWS != "" {
		wsAPI.WithURL(opts.BaseURLWS)
	}
	wsAccount := spot.NewWsAccountClient(wsAPI, opts.APIKey, opts.SecretKey)

	exec := newSpotExecutionClient(accountID, instruments, rest, nil)
	stream := newSpotPrivateStream(
		accountID,
		spotSDKUserStream{account: wsAccount},
		spotSDKAPIStream{api: wsAPI},
		opts.OnResubscribe,
		exec.emitAccountState,
		exec.emitOrder,
		exec.emitFill,
	)
	stream.emitOrderEvent = exec.emitOrderEvent
	exec.stream = stream
	return exec, nil
}

func NewPerpExecutionClient(ctx context.Context, opts Options) (venue.ExecutionClient, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}
	accountID := opts.AccountID
	if accountID == "" {
		accountID = defaultPerpAccountID
	}
	rest := newPerpRESTClient(opts, true)
	instruments := newInstrumentProvider(nil, rest)

	wsAccount := perp.NewWsAccountClient(ctx, opts.APIKey, opts.SecretKey)
	if opts.BaseURLWSStream != "" {
		wsAccount.WithURL(opts.BaseURLWSStream)
	}

	exec := newPerpExecutionClient(accountID, instruments, rest, nil)
	stream := newPerpPrivateStream(
		accountID,
		perpSDKUserStream{account: wsAccount},
		opts.OnResubscribe,
		exec.emitOrder,
		exec.emitFill,
		exec.emitPosition,
	)
	stream.emitOrderEvent = exec.emitOrderEvent
	exec.stream = stream
	return exec, nil
}

func NewSpotInstrumentProvider(opts Options) (venue.InstrumentProvider, error) {
	return newInstrumentProvider(newSpotRESTClient(opts, false), nil), nil
}

func NewPerpInstrumentProvider(opts Options) (venue.InstrumentProvider, error) {
	return newInstrumentProvider(nil, newPerpRESTClient(opts, false)), nil
}

func newSpotRESTClient(opts Options, withCredentials bool) *spot.Client {
	rest := spot.NewClient()
	if withCredentials || opts.APIKey != "" || opts.SecretKey != "" {
		rest.WithCredentials(opts.APIKey, opts.SecretKey)
	}
	if opts.BaseURLHTTP != "" {
		rest.WithBaseURL(opts.BaseURLHTTP)
	}
	return rest
}

func newPerpRESTClient(opts Options, withCredentials bool) *perp.Client {
	rest := perp.NewClient()
	if withCredentials || opts.APIKey != "" || opts.SecretKey != "" {
		rest.WithCredentials(opts.APIKey, opts.SecretKey)
	}
	if opts.BaseURLHTTP != "" {
		rest.WithBaseURL(opts.BaseURLHTTP)
	}
	return rest
}

func accountIDOrDefault(id model.AccountID, fallback model.AccountID) model.AccountID {
	if id != "" {
		return id
	}
	return fallback
}
