package lighter

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestPerpPrivatePathsWithoutCredentialsReturnAuthSentinel(t *testing.T) {
	adp := &Adapter{}

	require.ErrorIs(t, adp.WsOrderConnected(context.Background()), exchanges.ErrAuthFailed)
	require.ErrorIs(t, adp.WsAccountConnected(context.Background()), exchanges.ErrAuthFailed)
	require.ErrorIs(t, adp.WatchFills(context.Background(), nil), exchanges.ErrAuthFailed)
	_, err := adp.FetchAccount(context.Background())
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	_, err = adp.FetchOpenOrders(context.Background(), "BTC")
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	_, err = adp.FetchFeeRate(context.Background(), "BTC")
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func TestSpotPrivatePathsWithoutCredentialsReturnAuthSentinel(t *testing.T) {
	adp := &SpotAdapter{}

	require.ErrorIs(t, adp.WsOrderConnected(context.Background()), exchanges.ErrAuthFailed)
	require.ErrorIs(t, adp.WsAccountConnected(context.Background()), exchanges.ErrAuthFailed)
	require.ErrorIs(t, adp.WatchFills(context.Background(), nil), exchanges.ErrAuthFailed)
	_, err := adp.FetchAccount(context.Background())
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	_, err = adp.FetchOpenOrders(context.Background(), "BTC")
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	_, err = adp.FetchFeeRate(context.Background(), "BTC")
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}
