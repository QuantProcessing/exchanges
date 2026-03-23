package bitget

import (
	"context"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/shopspring/decimal"
)

const (
	accountModeAuto    = "auto"
	accountModeUTA     = "uta"
	accountModeClassic = "classic"
)

type privateProfile interface {
	PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error)
	CancelOrder(ctx context.Context, orderID, symbol string) error
	CancelAllOrders(ctx context.Context, symbol string) error
	FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error)
	FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error)
	FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error)
	FetchAccount(ctx context.Context) (*exchanges.Account, error)
	FetchBalance(ctx context.Context) (decimal.Decimal, error)
	FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error)
	WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error
	StopWatchOrders(ctx context.Context) error
}

type perpPrivateProfile interface {
	privateProfile
	FetchPositions(ctx context.Context) ([]exchanges.Position, error)
	SetLeverage(ctx context.Context, symbol string, leverage int) error
	ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error)
	WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error
	StopWatchPositions(ctx context.Context) error
}

type spotPrivateProfile interface {
	privateProfile
	FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error)
}

func newPrivateWSClient(opts Options, mode string) *sdk.PrivateWSClient {
	client := sdk.NewPrivateWSClient().WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
	if mode == accountModeClassic {
		client = client.WithClassicMode()
	}
	return client
}

func newPerpPrivateProfile(adp *Adapter, mode string) perpPrivateProfile {
	switch mode {
	case accountModeClassic:
		return &classicPerpProfile{adp: adp}
	default:
		return &utaPerpProfile{adp: adp}
	}
}

func newSpotPrivateProfile(adp *SpotAdapter, mode string) spotPrivateProfile {
	switch mode {
	case accountModeClassic:
		return &classicSpotProfile{adp: adp}
	default:
		return &utaSpotProfile{adp: adp}
	}
}
