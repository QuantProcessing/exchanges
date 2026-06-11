package bybit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

// FetchOpenInterest requests a single most-recent OI point via the 5min series.
// Bybit's API returns a time series; we ask for limit=1 to get the latest value.
func (a *Adapter) FetchOpenInterest(ctx context.Context, symbol string) (*exchanges.OpenInterest, error) {
	sym := a.FormatSymbol(symbol)
	res, err := a.client.GetOpenInterest(ctx, categoryLinear, sym, "5min", 0, 0, 1, "")
	if err != nil {
		return nil, err
	}
	if len(res.List) == 0 {
		return nil, fmt.Errorf("bybit: empty open-interest response for %s", symbol)
	}
	first := res.List[0]
	ts, _ := strconv.ParseInt(first.Timestamp, 10, 64)
	return &exchanges.OpenInterest{
		Symbol:      symbol,
		OIContracts: parseDecimal(first.OpenInterest),
		Timestamp:   ts,
	}, nil
}

// FetchFundingRateHistory returns historical funding rates, normalized to per-hour
// assuming an 8h funding interval (standard for USDT linear perps).
// TODO: read the per-symbol interval from instrument metadata instead of hardcoding.
func (a *Adapter) FetchFundingRateHistory(ctx context.Context, symbol string, opts *exchanges.FundingRateHistoryOpts) ([]exchanges.FundingRate, error) {
	sym := a.FormatSymbol(symbol)

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

	raw, err := a.client.GetFundingHistory(ctx, categoryLinear, sym, startMs, endMs, limit)
	if err != nil {
		return nil, err
	}

	const intervalHours int64 = 8
	out := make([]exchanges.FundingRate, 0, len(raw))
	for _, r := range raw {
		rate := parseDecimal(r.FundingRate)
		rate = rate.Div(decimal.NewFromInt(intervalHours))
		ts, _ := strconv.ParseInt(r.FundingRateTimestamp, 10, 64)
		out = append(out, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          rate,
			FundingIntervalHours: intervalHours,
			FundingTime:          ts,
			UpdateTime:           time.Now().Unix(),
		})
	}
	return out, nil
}
