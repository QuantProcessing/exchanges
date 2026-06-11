package okx

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/shopspring/decimal"
)

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	instId := a.FormatSymbol(symbol)

	res, err := a.client.GetTicker(ctx, instId)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no ticker")
	}

	t := res[0]
	last := parseString(t.Last)
	open := parseString(t.Open24h)
	priceChange := decimal.Zero
	priceChangePct := decimal.Zero
	if open.IsPositive() {
		priceChange = last.Sub(open)
		priceChangePct = priceChange.Div(open).Mul(decimal.NewFromInt(100))
	}
	return &exchanges.Ticker{
		Symbol:             symbol,
		LastPrice:          last,
		Bid:                parseString(t.BidPx),
		Ask:                parseString(t.AskPx),
		High24h:            parseString(t.High24h),
		Low24h:             parseString(t.Low24h),
		Volume24h:          parseString(t.Vol24h),
		OpenPrice:          open,
		PriceChange:        priceChange,
		PriceChangePercent: priceChangePct,
		Timestamp:          parseTime(t.Ts),
	}, nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	instId := a.FormatSymbol(symbol)

	sz := 400
	if limit > 0 && limit < 400 {
		sz = limit
	}

	res, err := a.client.GetOrderBook(ctx, instId, &sz)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no orderbook")
	}

	book := res[0]
	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: parseTime(book.Ts),
		Bids:      make([]exchanges.Level, 0, len(book.Bids)),
		Asks:      make([]exchanges.Level, 0, len(book.Asks)),
	}
	for _, b := range book.Bids {
		if len(b) >= 2 {
			ob.Bids = append(ob.Bids, exchanges.Level{Price: parseString(b[0]), Quantity: parseString(b[1])})
		}
	}
	for _, as := range book.Asks {
		if len(as) >= 2 {
			ob.Asks = append(ob.Asks, exchanges.Level{Price: parseString(as[0]), Quantity: parseString(as[1])})
		}
	}
	return ob, nil
}

func (a *Adapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	var start, end *time.Time
	var limit int
	if opts != nil {
		start = opts.Start
		end = opts.End
		limit = opts.Limit
	}
	_ = start
	_ = end
	_ = limit
	instId := a.FormatSymbol(symbol)

	bar := "1m"
	switch interval {
	case exchanges.Interval1m:
		bar = "1m"
	case exchanges.Interval5m:
		bar = "5m"
	case exchanges.Interval15m:
		bar = "15m"
	case exchanges.Interval1h:
		bar = "1H"
	case exchanges.Interval4h:
		bar = "4H"
	case exchanges.Interval1d:
		bar = "1D"
	}

	var after *string
	if end != nil {
		s := fmt.Sprintf("%d", end.UnixMilli())
		after = &s
	}

	res, err := a.client.GetCandles(ctx, instId, &bar, after, nil, &limit)
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, len(res))
	for i, k := range res {
		idx := len(res) - 1 - i
		klines[idx] = exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: parseTime(k[0]),
			Open:      parseString(k[1]),
			High:      parseString(k[2]),
			Low:       parseString(k[3]),
			Close:     parseString(k[4]),
			Volume:    parseString(k[5]),
		}
	}
	return klines, nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	instId := a.FormatSymbol(symbol)

	params := url.Values{}
	params.Add("instId", instId)
	if limit > 0 {
		params.Add("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/v5/market/trades?" + params.Encode()

	type OkxTrade struct {
		TradeId string `json:"tradeId"`
		Px      string `json:"px"`
		Sz      string `json:"sz"`
		Side    string `json:"side"`
		Ts      string `json:"ts"`
	}
	trades, err := okx.Request[OkxTrade](a.client, ctx, okx.MethodGet, path, nil, false)
	if err != nil {
		return nil, err
	}

	res := make([]exchanges.Trade, len(trades))
	for i, t := range trades {
		side := exchanges.TradeSideBuy
		if t.Side == "sell" {
			side = exchanges.TradeSideSell
		}
		res[i] = exchanges.Trade{
			ID:        t.TradeId,
			Symbol:    symbol,
			Price:     parseString(t.Px),
			Quantity:  parseString(t.Sz),
			Side:      side,
			Timestamp: parseTime(t.Ts),
		}
	}
	return res, nil
}

// FetchHistoricalTrades returns paginated historical public trades.
// Uses OKX's /api/v5/market/history-trades endpoint in tradeId-cursor mode
// (type=1) when FromID is set; otherwise timestamp-cursor mode (type=2).
func (a *Adapter) FetchHistoricalTrades(ctx context.Context, symbol string, opts *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	instId := a.FormatSymbol(symbol)

	typ := 1
	var before, after string
	limit := 100
	if opts != nil {
		if opts.Limit > 0 {
			limit = opts.Limit
		}
		if opts.FromID != "" {
			typ = 1
			after = opts.FromID // older than this tradeId
		} else if opts.Start != nil || opts.End != nil {
			typ = 2
			if opts.End != nil {
				before = strconv.FormatInt(opts.End.UnixMilli(), 10)
			}
			if opts.Start != nil {
				after = strconv.FormatInt(opts.Start.UnixMilli(), 10)
			}
		}
	}

	raw, err := a.client.GetHistoryTrades(ctx, instId, typ, before, after, limit)
	if err != nil {
		return nil, err
	}

	out := make([]exchanges.Trade, 0, len(raw))
	for _, r := range raw {
		side := exchanges.TradeSideBuy
		if r.Side == "sell" {
			side = exchanges.TradeSideSell
		}
		out = append(out, exchanges.Trade{
			ID:        r.TradeId,
			Symbol:    symbol,
			Price:     parseString(r.Px),
			Quantity:  parseString(r.Sz),
			Side:      side,
			Timestamp: parseTime(r.Ts),
		})
	}
	return out, nil
}

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	instId := a.FormatSymbol(symbol)

	res, err := a.client.GetTicker(ctx, instId)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no ticker")
	}

	t := res[0]
	return &exchanges.Ticker{
		Symbol:    symbol,
		LastPrice: parseString(t.Last),
		Bid:       parseString(t.BidPx),
		Ask:       parseString(t.AskPx),
		High24h:   parseString(t.High24h),
		Low24h:    parseString(t.Low24h),
		Volume24h: parseString(t.Vol24h),
		Timestamp: parseTime(t.Ts),
	}, nil
}

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	instId := a.FormatSymbol(symbol)

	sz := 400
	if limit > 0 && limit < 400 {
		sz = limit
	}

	res, err := a.client.GetOrderBook(ctx, instId, &sz)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no orderbook")
	}

	book := res[0]
	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: parseTime(book.Ts),
		Bids:      make([]exchanges.Level, 0, len(book.Bids)),
		Asks:      make([]exchanges.Level, 0, len(book.Asks)),
	}
	for _, b := range book.Bids {
		if len(b) >= 2 {
			ob.Bids = append(ob.Bids, exchanges.Level{Price: parseString(b[0]), Quantity: parseString(b[1])})
		}
	}
	for _, as := range book.Asks {
		if len(as) >= 2 {
			ob.Asks = append(ob.Asks, exchanges.Level{Price: parseString(as[0]), Quantity: parseString(as[1])})
		}
	}
	return ob, nil
}

func (a *SpotAdapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	var start, end *time.Time
	var limit int
	if opts != nil {
		start = opts.Start
		end = opts.End
		limit = opts.Limit
	}
	_ = start
	_ = end
	_ = limit
	instId := a.FormatSymbol(symbol)

	bar := "1m"
	switch interval {
	case exchanges.Interval1m:
		bar = "1m"
	case exchanges.Interval5m:
		bar = "5m"
	case exchanges.Interval15m:
		bar = "15m"
	case exchanges.Interval1h:
		bar = "1H"
	case exchanges.Interval4h:
		bar = "4H"
	case exchanges.Interval1d:
		bar = "1D"
	}

	var after *string
	if end != nil {
		s := fmt.Sprintf("%d", end.UnixMilli())
		after = &s
	}

	res, err := a.client.GetCandles(ctx, instId, &bar, after, nil, &limit)
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, len(res))
	for i, k := range res {
		idx := len(res) - 1 - i
		klines[idx] = exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: parseTime(k[0]),
			Open:      parseString(k[1]),
			High:      parseString(k[2]),
			Low:       parseString(k[3]),
			Close:     parseString(k[4]),
			Volume:    parseString(k[5]),
		}
	}
	return klines, nil
}

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	instId := a.FormatSymbol(symbol)

	params := url.Values{}
	params.Add("instId", instId)
	if limit > 0 {
		params.Add("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/v5/market/trades?" + params.Encode()

	type OkxTrade struct {
		TradeId string `json:"tradeId"`
		Px      string `json:"px"`
		Sz      string `json:"sz"`
		Side    string `json:"side"`
		Ts      string `json:"ts"`
	}
	trades, err := okx.Request[OkxTrade](a.client, ctx, okx.MethodGet, path, nil, false)
	if err != nil {
		return nil, err
	}

	res := make([]exchanges.Trade, len(trades))
	for i, t := range trades {
		side := exchanges.TradeSideBuy
		if t.Side == "sell" {
			side = exchanges.TradeSideSell
		}
		res[i] = exchanges.Trade{
			ID:        t.TradeId,
			Symbol:    symbol,
			Price:     parseString(t.Px),
			Quantity:  parseString(t.Sz),
			Side:      side,
			Timestamp: parseTime(t.Ts),
		}
	}
	return res, nil
}
