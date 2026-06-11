package bitget

import (
	"context"
	"fmt"
	"strconv"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

// Funding remains explicitly unsupported until Bitget's controlled hybrid adapter
// grows a documented perp funding implementation.
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error) {
	return nil, exchanges.ErrNotSupported
}

// FetchOpenInterest retrieves current open interest for a perp symbol.
func (a *Adapter) FetchOpenInterest(ctx context.Context, symbol string) (*exchanges.OpenInterest, error) {
	sym := a.FormatSymbol(symbol)
	res, err := a.client.GetOpenInterest(ctx, sym, a.perpCategory)
	if err != nil {
		return nil, err
	}
	if len(res.List) == 0 {
		return nil, fmt.Errorf("bitget open interest not found for %s", sym)
	}
	entry := res.List[0]
	ts, _ := strconv.ParseInt(res.TS, 10, 64)
	return &exchanges.OpenInterest{
		Symbol:      symbol,
		OIContracts: parseDecimal(entry.Size),
		Timestamp:   ts,
	}, nil
}

// FetchFundingRateHistory returns historical funding rates, normalized to per-hour
// assuming an 8h funding interval (standard for USDT-FUTURES).
// TODO: read the per-symbol interval from instrument metadata instead of hardcoding.
func (a *Adapter) FetchFundingRateHistory(ctx context.Context, symbol string, opts *exchanges.FundingRateHistoryOpts) ([]exchanges.FundingRate, error) {
	sym := a.FormatSymbol(symbol)

	pageSize := 100
	if opts != nil && opts.Limit > 0 && opts.Limit <= 100 {
		pageSize = opts.Limit
	}
	// Bitget history-fund-rate uses pageSize + pageNo; Start/End are not supported on v2.
	raw, err := a.client.GetHistoryFundRate(ctx, sym, a.perpCategory, pageSize, 1)
	if err != nil {
		return nil, err
	}

	const intervalHours int64 = 8
	out := make([]exchanges.FundingRate, 0, len(raw))
	for _, r := range raw {
		rate := parseDecimal(r.FundingRate)
		rate = rate.Div(decimal.NewFromInt(intervalHours))
		ts, _ := strconv.ParseInt(r.FundingTime, 10, 64)
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
