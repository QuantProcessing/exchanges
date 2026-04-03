package grvt

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestPrivatePathsWithoutCredentialsReturnAuthSentinel(t *testing.T) {
	adp := &Adapter{}

	require.ErrorIs(t, adp.WsAccountConnected(context.Background()), exchanges.ErrAuthFailed)
	require.ErrorIs(t, adp.WsOrderConnected(context.Background()), exchanges.ErrAuthFailed)
	_, err := adp.FetchAccount(context.Background())
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	_, err = adp.FetchOpenOrders(context.Background(), "BTC")
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	_, err = adp.FetchFeeRate(context.Background(), "BTC")
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func TestPrimaryWritePathsWithoutCredentialsReturnAuthSentinel(t *testing.T) {
	adp := &Adapter{BaseAdapter: exchanges.NewBaseAdapter("GRVT", exchanges.MarketTypePerp, exchanges.NopLogger)}

	require.ErrorIs(t, adp.CancelOrder(context.Background(), "1", "BTC"), exchanges.ErrAuthFailed)
	require.ErrorIs(t, adp.CancelAllOrders(context.Background(), "BTC"), exchanges.ErrAuthFailed)
}
