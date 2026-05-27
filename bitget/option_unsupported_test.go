package bitget

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestBitgetDoesNotRegisterOptionSupport(t *testing.T) {
	t.Parallel()

	_, ok := exchanges.LookupCapabilities(exchangeName, exchanges.MarketTypeOption)
	require.False(t, ok)

	ctor, err := exchanges.LookupConstructor(exchangeName)
	require.NoError(t, err)
	_, err = ctor(context.Background(), exchanges.MarketTypeOption, nil)
	require.Error(t, err)
}
