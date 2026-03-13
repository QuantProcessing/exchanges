package hyperliquid

import (
	"context"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

// GetFundingRate retrieves funding rate for a symbol
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	// Hyperliquid uses coin name directly
	rate, err := a.client.GetFundingRate(ctx, symbol)
	if err != nil {
		return nil, err
	}

	fundingRate := parseDecimal(rate.FundingRate)

	return &exchanges.FundingRate{
		Symbol:               symbol,
		FundingRate:          fundingRate,
		FundingIntervalHours: rate.FundingIntervalHours,
		FundingTime:          rate.FundingTime,
		NextFundingTime:      rate.NextFundingTime,
		UpdateTime:           time.Now().Unix(),
	}, nil
}

// GetAllRundingRates retrieves funding rates for all perpetual symbols
func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error) {
	ratesMap, err := a.client.GetAllFundingRates(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]exchanges.FundingRate, 0, len(ratesMap))
	for coin, rateStr := range ratesMap {
		fundingRate := parseDecimal(rateStr)

		result = append(result, exchanges.FundingRate{
			Symbol:               coin,
			FundingRate:          fundingRate,
			FundingIntervalHours: 1, // Hyperliquid uses 1-hour intervals
			FundingTime:          time.Now().Truncate(time.Hour).UnixMilli(),
			NextFundingTime:      time.Now().Truncate(time.Hour).Add(time.Hour).UnixMilli(),
			UpdateTime:           time.Now().Unix(),
		})
	}

	return result, nil
}
