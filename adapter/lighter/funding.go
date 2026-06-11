package lighter

import (
	"context"
	"fmt"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

// GetFundingRate retrieves funding rate for a symbol
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	formattedSymbol := a.FormatSymbol(symbol)

	a.metaMu.RLock()
	marketId, ok := a.symbolToID[formattedSymbol]
	a.metaMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("symbol to market ID mapping not found: %s", symbol)
	}

	rate, err := a.client.GetFundingRate(ctx, marketId)
	if err != nil {
		return nil, err
	}

	fundingRate := parseString(rate.FundingRate)

	return &exchanges.FundingRate{
		Symbol:               symbol,
		FundingRate:          fundingRate,
		FundingIntervalHours: rate.FundingIntervalHours,
		FundingTime:          rate.FundingTime,
		NextFundingTime:      rate.NextFundingTime,
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
	a.metaMu.RLock()
	defer a.metaMu.RUnlock()

	for _, rate := range rates {
		// CRITICAL: Filter out data from other exchanges
		// Lighter API returns data for binance, bybit, hyperliquid, lighter
		// We only want lighter's own data
		if rate.Exchange != "lighter" {
			continue
		}

		// Find symbol by marketId
		symbol := rate.Symbol
		if mapped, ok := a.idToSymbol[int(rate.MarketId)]; ok {
			symbol = mapped
		}

		fundingRate := parseString(rate.FundingRate)

		result = append(result, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          fundingRate,
			FundingIntervalHours: rate.FundingIntervalHours,
			FundingTime:          rate.FundingTime,
			NextFundingTime:      rate.NextFundingTime,
			UpdateTime:           time.Now().Unix(),
		})
	}

	return result, nil
}
