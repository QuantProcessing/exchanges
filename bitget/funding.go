package bitget

import (
	"context"

	exchanges "github.com/QuantProcessing/exchanges"
)

// Funding remains explicitly unsupported until Bitget's controlled hybrid adapter
// grows a documented perp funding implementation.
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error) {
	return nil, exchanges.ErrNotSupported
}
