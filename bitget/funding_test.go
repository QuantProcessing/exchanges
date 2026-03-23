package bitget

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestBitgetFundingMethodsRemainExplicitlyUnsupported(t *testing.T) {
	adp := &Adapter{}

	_, err := adp.FetchFundingRate(context.Background(), "BTC")
	require.ErrorIs(t, err, exchanges.ErrNotSupported)

	_, err = adp.FetchAllFundingRates(context.Background())
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}
