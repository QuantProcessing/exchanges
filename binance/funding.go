package binance

import (
	"context"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

// GetFundingRate retrieves funding rate for a symbol
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	formattedSymbol := a.FormatSymbol(symbol)
	rate, err := a.client.GetFundingRate(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}

	fundingRate := parseDecimal(rate.LastFundingRate)

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
	for _, rate := range rates {
		symbol := a.ExtractSymbol(rate.Symbol)
		fundingRate := parseDecimal(rate.LastFundingRate)

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

// FetchFundingRateHistory retrieves historical funding rates for a symbol.
// Binance's fundingRate endpoint emits per-period rates; divide by the symbol's
// funding interval hours to get per-hour values (same convention as FetchFundingRate).
func (a *Adapter) FetchFundingRateHistory(ctx context.Context, symbol string, opts *exchanges.FundingRateHistoryOpts) ([]exchanges.FundingRate, error) {
	formattedSymbol := a.FormatSymbol(symbol)

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

	raw, err := a.client.GetFundingRateHistory(ctx, formattedSymbol, startMs, endMs, limit)
	if err != nil {
		return nil, err
	}

	intervalHours, err := a.client.GetFundingIntervalHours(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}
	if intervalHours <= 0 {
		intervalHours = 8
	}

	out := make([]exchanges.FundingRate, 0, len(raw))
	for _, r := range raw {
		rate := parseDecimal(r.FundingRate)
		if intervalHours != 1 {
			rate = rate.Div(decimal.NewFromInt(intervalHours))
		}
		out = append(out, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          rate,
			FundingIntervalHours: intervalHours,
			FundingTime:          r.FundingTime,
			UpdateTime:           time.Now().Unix(),
		})
	}
	return out, nil
}

// FetchOpenInterest retrieves current open interest for a perp symbol.
// Binance returns OI only in contracts (base asset); notional is left zero —
// callers needing notional can multiply by FetchTicker().MarkPrice.
func (a *Adapter) FetchOpenInterest(ctx context.Context, symbol string) (*exchanges.OpenInterest, error) {
	formattedSymbol := a.FormatSymbol(symbol)
	res, err := a.client.GetOpenInterest(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}
	return &exchanges.OpenInterest{
		Symbol:      symbol,
		OIContracts: parseDecimal(res.OpenInterest),
		Timestamp:   res.Time,
	}, nil
}
