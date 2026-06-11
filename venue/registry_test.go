package venue

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestRegistryRegisterAndOpen(t *testing.T) {
	reg := NewRegistry()
	reg.Register(model.VenueBinance, func(ctx context.Context, cfg map[string]string) (Adapter, error) {
		return fakeAdapter{venue: model.VenueBinance}, nil
	})
	got, err := reg.Open(context.Background(), model.VenueBinance, nil)
	require.NoError(t, err)
	require.Equal(t, model.VenueBinance, got.Venue())
}

func TestRegistryUnknownVenue(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Open(context.Background(), model.Venue("MISSING"), nil)
	require.ErrorIs(t, err, ErrUnknownVenue)
}

type fakeAdapter struct{ venue model.Venue }

func (f fakeAdapter) Venue() model.Venue                 { return f.venue }
func (f fakeAdapter) Instruments() InstrumentProvider    { return nil }
func (f fakeAdapter) MarketData() MarketDataClient       { return nil }
func (f fakeAdapter) Execution() ExecutionClient         { return nil }
func (f fakeAdapter) Capabilities() DeclaredCapabilities { return DeclaredCapabilities{Venue: f.venue} }
func (f fakeAdapter) Certified() []CertifiedCapabilities { return nil }
func (f fakeAdapter) Close() error                       { return nil }
