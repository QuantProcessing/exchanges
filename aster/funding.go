package aster

import (
	"context"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

// FetchFundingRate retrieves funding rate for a symbol
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	formattedSymbol := a.FormatSymbol(symbol)
	rate, err := a.client.GetFundingRate(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}

	return &exchanges.FundingRate{
		Symbol:               symbol,
		FundingRate:          parseDecimal(rate.LastFundingRate),
		FundingIntervalHours: rate.FundingIntervalHours,
		FundingTime:          rate.FundingTime,
		NextFundingTime:      rate.NextFundingTime,
		UpdateTime:           time.Now().Unix(),
	}, nil
}

// FetchAllFundingRates retrieves funding rates for all perpetual symbols
func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error) {
	rates, err := a.client.GetAllFundingRates(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]exchanges.FundingRate, 0, len(rates))
	for _, rate := range rates {
		symbol := a.ExtractSymbol(rate.Symbol)

		result = append(result, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          parseDecimal(rate.LastFundingRate),
			FundingIntervalHours: rate.FundingIntervalHours,
			FundingTime:          rate.FundingTime,
			NextFundingTime:      rate.NextFundingTime,
			UpdateTime:           time.Now().Unix(),
		})
	}

	return result, nil
}
