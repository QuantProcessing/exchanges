package bitget

import (
	"context"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/shopspring/decimal"
)

// privateProfile isolates Bitget account-model-specific private behavior.
// This split is a Bitget exception, not the repository default adapter pattern.
type privateProfile interface {
	PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error)
	PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error
	CancelOrder(ctx context.Context, orderID, symbol string) error
	CancelOrderWS(ctx context.Context, orderID, symbol string) error
	CancelAllOrders(ctx context.Context, symbol string) error
	FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error)
	FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error)
	FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error)
	FetchAccount(ctx context.Context) (*exchanges.Account, error)
	FetchBalance(ctx context.Context) (decimal.Decimal, error)
	FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error)
	WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error
	WatchFills(ctx context.Context, cb exchanges.FillCallback) error
	StopWatchOrders(ctx context.Context) error
	StopWatchFills(ctx context.Context) error
}

type perpPrivateProfile interface {
	privateProfile
	FetchPositions(ctx context.Context) ([]exchanges.Position, error)
	SetLeverage(ctx context.Context, symbol string, leverage int) error
	ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error)
	ModifyOrderWS(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) error
	WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error
	StopWatchPositions(ctx context.Context) error
}

type spotPrivateProfile interface {
	privateProfile
	FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error)
}

func newPrivateWSClient(opts Options) *sdk.PrivateWSClient {
	return sdk.NewPrivateWSClient().
		WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase).
		WithClassicMode()
}

func newPerpPrivateProfile(adp *Adapter) perpPrivateProfile {
	// Classic mode is the only Bitget private profile wired today.
	return &classicPerpProfile{adp: adp}
}

func newSpotPrivateProfile(adp *SpotAdapter) spotPrivateProfile {
	// Classic mode is the only Bitget private profile wired today.
	return &classicSpotProfile{adp: adp}
}
