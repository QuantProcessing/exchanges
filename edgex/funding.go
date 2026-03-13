
package edgex

import (
	"context"
	"fmt"
	"strconv"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

// GetFundingRate retrieves funding rate for a symbol
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	formattedSymbol := a.FormatSymbol(symbol)
	contract, ok := a.symbolToContract[formattedSymbol]
	if !ok {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}

	rate, err := a.client.GetFundingRate(ctx, contract.ContractId)
	if err != nil {
		return nil, err
	}

	fundingRate := parseDecimal(rate.FundingRate)
	fundingTime, _ := strconv.ParseInt(rate.FundingTime, 10, 64)
	nextFundingTime, _ := strconv.ParseInt(rate.NextFundingTime, 10, 64)

	return &exchanges.FundingRate{
		Symbol:               symbol,
		FundingRate:          fundingRate,
		FundingIntervalHours: 4, // EdgeX uses 4-hour funding intervals
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
		// Find symbol by contractId
		symbol := rate.ContractId
		if name, ok := a.contractToSymbol[rate.ContractId]; ok {
			symbol = a.ExtractSymbol(name)
		}

		fundingRate := parseDecimal(rate.FundingRate)
		fundingTime, _ := strconv.ParseInt(rate.FundingTime, 10, 64)
		nextFundingTime, _ := strconv.ParseInt(rate.NextFundingTime, 10, 64)

		result = append(result, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          fundingRate,
			FundingIntervalHours: 4, // EdgeX uses 4-hour intervals
			FundingTime:          fundingTime,
			NextFundingTime:      nextFundingTime,
			UpdateTime:           time.Now().Unix(),
		})
	}

	return result, nil
}
