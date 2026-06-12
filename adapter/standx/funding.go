package standx

import (
	"context"
	"fmt"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

// GetFundingRate returns the specific funding rate for a symbol
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	rates, err := a.client.QueryFundingRates(ctx, symbol, 0, 0)
	if err != nil {
		return nil, err
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("no funding rate data")
	}
	latest := rates[0]
	t, err := time.Parse(time.RFC3339Nano, latest.Time)
	if err != nil {
		return nil, err
	}
	return &exchanges.FundingRate{
		Symbol:      symbol,
		FundingRate: parseDecimal(latest.FundingRate),
		FundingTime: t.UnixMilli(),
	}, nil
}

// GetAllFundingRates returns all funding rates
func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error) {
	return nil, exchanges.ErrNotSupported
}
