package okx

import (
	"context"
	"strconv"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

// GetFundingRate retrieves funding rate for a symbol
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	instId := a.FormatSymbol(symbol)
	rate, err := a.client.GetFundingRate(ctx, instId)
	if err != nil {
		return nil, err
	}

	fundingRate := parseDecimal(rate.FundingRate)
	fundingTime, _ := strconv.ParseInt(rate.FundingTime, 10, 64)
	nextFundingTime, _ := strconv.ParseInt(rate.NextFundingTime, 10, 64)

	return &exchanges.FundingRate{
		Symbol:               symbol,
		FundingRate:          fundingRate,
		FundingIntervalHours: rate.FundingIntervalHours,
		FundingTime:          fundingTime,
		NextFundingTime:      nextFundingTime,
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
		symbol := a.ExtractSymbol(rate.Symbol)
		fundingRate := parseDecimal(rate.FundingRate)
		fundingTime, _ := strconv.ParseInt(rate.FundingTime, 10, 64)
		nextFundingTime, _ := strconv.ParseInt(rate.NextFundingTime, 10, 64)

		result = append(result, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          fundingRate,
			FundingIntervalHours: rate.FundingIntervalHours,
			FundingTime:          fundingTime,
			NextFundingTime:      nextFundingTime,
			UpdateTime:           time.Now().Unix(),
		})
	}

	return result, nil
}
