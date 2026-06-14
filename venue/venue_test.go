package venue

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestRegistryOpensRegisteredVenue(t *testing.T) {
	reg := NewRegistry()
	want := &fakeAdapter{venue: "BINANCE"}
	reg.Register("BINANCE", func(context.Context, map[string]string) (Adapter, error) {
		return want, nil
	})

	got, err := reg.Open(context.Background(), "BINANCE", nil)
	require.NoError(t, err)
	require.Same(t, want, got)

	_, err = reg.Open(context.Background(), "MISSING", nil)
	require.ErrorIs(t, err, ErrUnknownVenue)
}

type fakeAdapter struct {
	venue model.Venue
}

func (f *fakeAdapter) Venue() model.Venue              { return f.venue }
func (f *fakeAdapter) Instruments() InstrumentProvider { return nil }
func (f *fakeAdapter) Data() DataClient                { return nil }
func (f *fakeAdapter) Execution() ExecutionClient      { return nil }
func (f *fakeAdapter) Capabilities() DeclaredCapabilities {
	return DeclaredCapabilities{Venue: f.venue}
}
func (f *fakeAdapter) Close(context.Context) error { return nil }
