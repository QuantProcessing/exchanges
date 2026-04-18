package okx

import (
	"context"
	"strconv"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
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

// fundingIntervalHours returns the funding interval (hours) for an instId.
// Implementation uses the GetFundingRate call's returned FundingIntervalHours.
func (a *Adapter) fundingIntervalHours(ctx context.Context, instId string) (int64, error) {
	meta, err := a.client.GetFundingRate(ctx, instId)
	if err != nil {
		return 0, err
	}
	return meta.FundingIntervalHours, nil
}

// FetchFundingRateHistory retrieves historical funding rates for a symbol.
// OKX emits per-period rates; convert to per-hour using the instrument's
// funding interval (same rule used by FetchFundingRate).
func (a *Adapter) FetchFundingRateHistory(ctx context.Context, symbol string, opts *exchanges.FundingRateHistoryOpts) ([]exchanges.FundingRate, error) {
	instId := a.FormatSymbol(symbol)

	var before, after int64
	var limit int
	if opts != nil {
		if opts.End != nil {
			before = opts.End.UnixMilli() // newer than
		}
		if opts.Start != nil {
			after = opts.Start.UnixMilli() // older than
		}
		limit = opts.Limit
	}

	raw, err := a.client.GetFundingRateHistory(ctx, instId, before, after, limit)
	if err != nil {
		return nil, err
	}

	intervalHours, err := a.fundingIntervalHours(ctx, instId)
	if err != nil {
		return nil, err
	}
	if intervalHours <= 0 {
		intervalHours = 8
	}

	out := make([]exchanges.FundingRate, 0, len(raw))
	for _, r := range raw {
		rate := parseString(r.FundingRate)
		if intervalHours != 1 {
			rate = rate.Div(decimal.NewFromInt(intervalHours))
		}
		ft, _ := strconv.ParseInt(r.FundingTime, 10, 64)
		out = append(out, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          rate,
			FundingIntervalHours: intervalHours,
			FundingTime:          ft,
			UpdateTime:           time.Now().Unix(),
		})
	}
	return out, nil
}

// FetchOpenInterest retrieves current open interest for a perp symbol.
func (a *Adapter) FetchOpenInterest(ctx context.Context, symbol string) (*exchanges.OpenInterest, error) {
	instId := a.FormatSymbol(symbol)
	res, err := a.client.GetOpenInterest(ctx, instId)
	if err != nil {
		return nil, err
	}
	ts, _ := strconv.ParseInt(res.Ts, 10, 64)
	return &exchanges.OpenInterest{
		Symbol:      symbol,
		OIContracts: parseString(res.OI),
		OINotional:  parseString(res.OIUsd),
		Timestamp:   ts,
	}, nil
}
