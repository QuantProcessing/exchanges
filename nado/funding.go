package nado

import (
	"context"

	exchanges "github.com/QuantProcessing/exchanges"
)

// GetFundingRate retrieves funding rate for a symbol
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	// Get product ID from symbol cache
	productId, err := a.getProductId(symbol)
	if err != nil {
		return nil, err
	}

	rate, err := a.httpClient.GetFundingRate(ctx, productId)
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
		UpdateTime:           rate.UpdateTime,
	}, nil
}

// GetAllFundingRates retrieves funding rates for all perp products
func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error) {
	rates, err := a.httpClient.GetAllFundingRates(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]exchanges.FundingRate, 0, len(rates))
	for _, rate := range rates {
		// Find symbol by productId
		symbol := a.getSymbol(rate.ProductID)
		if symbol == "" {
			symbol = rate.Symbol
		}

		fundingRate := parseDecimal(rate.FundingRate)

		result = append(result, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          fundingRate,
			FundingIntervalHours: rate.FundingIntervalHours,
			FundingTime:          rate.FundingTime,
			NextFundingTime:      rate.NextFundingTime,
			UpdateTime:           rate.UpdateTime,
		})
	}

	return result, nil
}
