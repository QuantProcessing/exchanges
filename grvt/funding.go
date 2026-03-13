
package grvt

import (
	"context"
	"strconv"
	"time"
	exchanges "github.com/QuantProcessing/exchanges"
)

// GetFundingRate retrieves funding rate for a symbol
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	instrument := a.FormatSymbol(symbol)
	rate, err := a.client.GetFundingRate(ctx, instrument)
	if err != nil {
		return nil, err
	}

	fundingRate := parseDecimal(rate.FundingRate)
	nextFundingTime, _ := strconv.ParseInt(rate.NextFundingTime, 10, 64)
	fundingTime, _ := strconv.ParseInt(rate.FundingTime, 10, 64)

	return &exchanges.FundingRate{
		Symbol:               symbol,
		FundingRate:          fundingRate,
		FundingIntervalHours: rate.FundingIntervalHours,
		FundingTime:          fundingTime / 1000000, // Convert from nanoseconds to milliseconds
		NextFundingTime:      nextFundingTime / 1000000,
		UpdateTime:           time.Now().Unix(),
	}, nil
}

// GetAllFundingRates retrieves funding rates for all perpetual symbols
func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error) {
	rates, err := a.client.GetAllFundingRates(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]exchanges.FundingRate, 0, len(rates))
	for _, rate := range rates {
		symbol := a.ExtractSymbol(rate.Instrument)
		fundingRate := parseDecimal(rate.FundingRate)
		nextFundingTime, _ := strconv.ParseInt(rate.NextFundingTime, 10, 64)
		fundingTime, _ := strconv.ParseInt(rate.FundingTime, 10, 64)

		result = append(result, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          fundingRate,
			FundingIntervalHours: rate.FundingIntervalHours,
			FundingTime:          fundingTime / 1000000,
			NextFundingTime:      nextFundingTime / 1000000,
			UpdateTime:           time.Now().Unix(),
		})
	}

	return result, nil
}
