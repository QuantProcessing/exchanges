package exchanges_test

import (
	"fmt"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

type capabilityStubExchange struct {
	stubExchange
	caps exchanges.Capabilities
}

func (s capabilityStubExchange) Capabilities() exchanges.Capabilities {
	return s.caps
}

func TestGetCapabilitiesReturnsProviderCapabilities(t *testing.T) {
	t.Parallel()

	want := exchanges.Capabilities{
		PlaceOrder:          true,
		PlaceOrderWS:        true,
		WatchOrders:         true,
		WatchFills:          true,
		TradingAccountReady: true,
	}

	got := exchanges.GetCapabilities(capabilityStubExchange{caps: want})

	require.Equal(t, want, got)
}

func TestGetCapabilitiesWithoutProviderReturnsZeroValue(t *testing.T) {
	t.Parallel()

	got := exchanges.GetCapabilities(&stubExchange{})

	require.Equal(t, exchanges.Capabilities{}, got)
}

func TestBaseAdapterCapabilitiesRoundTrip(t *testing.T) {
	t.Parallel()

	base := exchanges.NewBaseAdapter("stub", exchanges.MarketTypePerp, exchanges.NopLogger)
	want := exchanges.Capabilities{
		FetchOpenOrders: true,
		ModifyOrder:     true,
		WatchPositions:  true,
	}

	base.SetCapabilities(want)

	require.Equal(t, want, base.Capabilities())
	require.Equal(t, want, exchanges.GetCapabilities(base))
}

func TestRegisterAndLookupCapabilities(t *testing.T) {
	name := uniqueCapabilityTestName("CAPS_TEST")
	want := exchanges.Capabilities{
		PlaceOrder:        true,
		WatchOrderBook:    true,
		WatchOrders:       true,
		FetchOpenOrders:   true,
		FetchOrderHistory: true,
	}

	exchanges.RegisterCapabilities(name, exchanges.MarketTypePerp, want)

	got, ok := exchanges.LookupCapabilities(name, exchanges.MarketTypePerp)
	require.True(t, ok)
	require.Equal(t, want, got)
}

func TestNewBaseAdapterLoadsRegisteredCapabilities(t *testing.T) {
	name := uniqueCapabilityTestName("CAPS_BASE_TEST")
	want := exchanges.Capabilities{
		PlaceOrder:          true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		TradingAccountReady: true,
	}

	exchanges.RegisterCapabilities(name, exchanges.MarketTypeSpot, want)

	base := exchanges.NewBaseAdapter(name, exchanges.MarketTypeSpot, exchanges.NopLogger)

	require.Equal(t, want, base.Capabilities())
}

func uniqueCapabilityTestName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
