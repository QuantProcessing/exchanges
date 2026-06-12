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

// FetchOpenInterest retrieves current open interest by extracting the field
// from the same metaAndAssetCtxs response that powers FetchFundingRate.
func (a *Adapter) FetchOpenInterest(ctx context.Context, symbol string) (*exchanges.OpenInterest, error) {
	meta, err := a.client.GetMetaAndAssetCtxs(ctx)
	if err != nil {
		return nil, err
	}
	formatted := a.FormatSymbol(symbol)
	for i, u := range meta.Meta.Universe {
		if u.Name == formatted && i < len(meta.AssetCtxs) {
			return &exchanges.OpenInterest{
				Symbol:      symbol,
				OIContracts: parseDecimal(meta.AssetCtxs[i].OpenInterest),
				Timestamp:   time.Now().UnixMilli(),
			}, nil
		}
	}
	return nil, exchanges.ErrSymbolNotFound
}

// FetchFundingRateHistory retrieves historical funding rates for a symbol.
// HL API requires startTime; defaults to 7 days ago if unset.
// endTime is optional (0 = omit, HL treats as now).
// Limit is honored via client-side truncation (HL has no limit param).
func (a *Adapter) FetchFundingRateHistory(ctx context.Context, symbol string, opts *exchanges.FundingRateHistoryOpts) ([]exchanges.FundingRate, error) {
	startMs := time.Now().Add(-7 * 24 * time.Hour).UnixMilli()
	var endMs int64
	if opts != nil {
		if opts.Start != nil {
			startMs = opts.Start.UnixMilli()
		}
		if opts.End != nil {
			endMs = opts.End.UnixMilli()
		}
	}

	coin := a.FormatSymbol(symbol)
	raw, err := a.client.GetFundingRateHistory(ctx, coin, startMs, endMs)
	if err != nil {
		return nil, err
	}

	// Honor opts.Limit by truncating (HL API doesn't accept a limit param).
	if opts != nil && opts.Limit > 0 && len(raw) > opts.Limit {
		raw = raw[:opts.Limit]
	}

	out := make([]exchanges.FundingRate, 0, len(raw))
	for _, r := range raw {
		out = append(out, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          parseDecimal(r.FundingRate),
			FundingIntervalHours: 1,
			FundingTime:          r.Time,
			UpdateTime:           time.Now().Unix(),
		})
	}
	return out, nil
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
