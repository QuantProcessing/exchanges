package nado

import (
	"context"
	"fmt"
	"math/big"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
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

// FetchOpenInterest returns current open interest for a perp symbol.
// OI data is embedded in the GetContracts response.
func (a *Adapter) FetchOpenInterest(ctx context.Context, symbol string) (*exchanges.OpenInterest, error) {
	tickerID := a.FormatSymbol(symbol)
	contracts, err := a.httpClient.GetContracts(ctx, nil)
	if err != nil {
		return nil, err
	}
	c, ok := contracts[tickerID]
	if !ok {
		return nil, exchanges.ErrSymbolNotFound
	}
	return &exchanges.OpenInterest{
		Symbol:      symbol,
		OIContracts: decimal.NewFromFloat(c.OpenInterest),
		OINotional:  decimal.NewFromFloat(c.OpenInterestUsd),
		Timestamp:   time.Now().UnixMilli(),
	}, nil
}

// FetchFundingRateHistory retrieves historical funding rates for a symbol from
// the archive indexer.
func (a *Adapter) FetchFundingRateHistory(ctx context.Context, symbol string, opts *exchanges.FundingRateHistoryOpts) ([]exchanges.FundingRate, error) {
	productID, err := a.getProductId(symbol)
	if err != nil {
		return nil, err
	}

	var startMs, endMs int64
	var limit int
	if opts != nil {
		if opts.Start != nil {
			startMs = opts.Start.UnixMilli()
		}
		if opts.End != nil {
			endMs = opts.End.UnixMilli()
		}
		limit = opts.Limit
	}

	raw, err := a.httpClient.GetFundingRateHistory(ctx, productID, startMs, endMs, limit)
	if err != nil {
		return nil, err
	}

	// The archive stores funding_rate_x18 as a 24-hour rate scaled by 10^18.
	// Divide by 24 to get the per-hour rate, consistent with FetchFundingRate.
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	out := make([]exchanges.FundingRate, 0, len(raw))
	for _, r := range raw {
		rateX18 := new(big.Int)
		if _, ok := rateX18.SetString(r.FundingRateX18, 10); !ok {
			return nil, fmt.Errorf("FetchFundingRateHistory: invalid funding_rate_x18: %s", r.FundingRateX18)
		}
		rate24hBig := new(big.Rat).SetFrac(rateX18, divisor)
		rateHourlyBig := new(big.Rat).Quo(rate24hBig, big.NewRat(24, 1))
		hourlyFloat, _ := rateHourlyBig.Float64()

		out = append(out, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          decimal.NewFromFloat(hourlyFloat),
			FundingIntervalHours: 1,
			FundingTime:          r.Timestamp,
			UpdateTime:           time.Now().Unix(),
		})
	}
	return out, nil
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
