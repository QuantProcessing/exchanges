package venue

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestDefaultRegistryOpensRegisteredAdapter(t *testing.T) {
	testVenue := model.Venue("TEST-DEFAULT-REGISTRY")
	Register(testVenue, func(ctx context.Context, cfg map[string]string) (Adapter, error) {
		require.Equal(t, "value", cfg["key"])
		return stubAdapter{venue: testVenue}, nil
	})

	got, err := Open(context.Background(), testVenue, map[string]string{"key": "value"})
	require.NoError(t, err)
	require.Equal(t, testVenue, got.Venue())
}

type stubAdapter struct {
	venue model.Venue
}

func (s stubAdapter) Venue() model.Venue                 { return s.venue }
func (s stubAdapter) Instruments() InstrumentProvider    { return nil }
func (s stubAdapter) MarketData() MarketDataClient       { return nil }
func (s stubAdapter) Execution() ExecutionClient         { return nil }
func (s stubAdapter) Capabilities() DeclaredCapabilities { return DeclaredCapabilities{Venue: s.venue} }
func (s stubAdapter) Close() error                       { return nil }
