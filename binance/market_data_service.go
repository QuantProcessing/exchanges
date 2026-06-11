package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/binance/sdk/perp"
	"github.com/shopspring/decimal"
)

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (_ *exchanges.Ticker, retErr error) {
	formattedSymbol := a.FormatSymbol(symbol)
	t, err := a.client.Ticker(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}

	depth, err := a.client.Depth(ctx, formattedSymbol, 5)
	if err != nil {
		return nil, err
	}

	ticker := &exchanges.Ticker{
		Symbol:             symbol,
		LastPrice:          parseDecimal(t.LastPrice),
		High24h:            parseDecimal(t.HighPrice),
		Low24h:             parseDecimal(t.LowPrice),
		Volume24h:          parseDecimal(t.Volume),
		QuoteVol:           parseDecimal(t.QuoteVolume),
		OpenPrice:          parseDecimal(t.OpenPrice),
		PriceChange:        parseDecimal(t.PriceChange),
		PriceChangePercent: parseDecimal(t.PriceChangePercent),
		WeightedAvgPrice:   parseDecimal(t.WeightedAvgPrice),
		TradeCount:         t.Count,
		Timestamp:          t.CloseTime,
	}

	if len(depth.Bids) > 0 {
		ticker.Bid = parseDecimal(depth.Bids[0][0])
	}
	if len(depth.Asks) > 0 {
		ticker.Ask = parseDecimal(depth.Asks[0][0])
	}

	return ticker, nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (_ *exchanges.OrderBook, retErr error) {
	formattedSymbol := a.FormatSymbol(symbol)
	res, err := a.client.Depth(ctx, formattedSymbol, limit)
	if err != nil {
		return nil, err
	}

	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: res.T,
		Bids:      make([]exchanges.Level, 0, len(res.Bids)),
		Asks:      make([]exchanges.Level, 0, len(res.Asks)),
	}

	for _, item := range res.Bids {
		ob.Bids = append(ob.Bids, exchanges.Level{Price: parseDecimal(item[0]), Quantity: parseDecimal(item[1])})
	}
	for _, item := range res.Asks {
		ob.Asks = append(ob.Asks, exchanges.Level{Price: parseDecimal(item[0]), Quantity: parseDecimal(item[1])})
	}

	return ob, nil
}

func (a *Adapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) (_ []exchanges.Kline, retErr error) {
	var start, end *time.Time
	var limit int
	if opts != nil {
		start = opts.Start
		end = opts.End
		limit = opts.Limit
	}
	formattedSymbol := a.FormatSymbol(symbol)
	var startTime, endTime int64
	if start != nil {
		startTime = start.UnixMilli()
	}
	if end != nil {
		endTime = end.UnixMilli()
	}
	res, err := a.client.Klines(ctx, formattedSymbol, string(interval), limit, startTime, endTime)
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, 0, len(res))
	for _, item := range res {
		row := item
		if len(row) < 8 {
			continue
		}

		k := exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: parseInt64(row[0]),
			Open:      parseDecimalInterface(row[1]),
			High:      parseDecimalInterface(row[2]),
			Low:       parseDecimalInterface(row[3]),
			Close:     parseDecimalInterface(row[4]),
			Volume:    parseDecimalInterface(row[5]),
			QuoteVol:  parseDecimalInterface(row[7]),
		}
		klines = append(klines, k)
	}

	return klines, nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) (_ []exchanges.Trade, retErr error) {
	formattedSymbol := a.FormatSymbol(symbol)
	res, err := a.client.GetAggTrades(ctx, formattedSymbol, limit)
	if err != nil {
		return nil, err
	}

	trades := make([]exchanges.Trade, 0, len(res))
	for _, r := range res {
		side := exchanges.TradeSideBuy
		if r.IsBuyerMaker {
			side = exchanges.TradeSideSell
		}

		trades = append(trades, exchanges.Trade{
			ID:        fmt.Sprintf("%d", r.ID),
			Symbol:    symbol,
			Price:     parseDecimal(r.Price),
			Quantity:  parseDecimal(r.Quantity),
			Side:      side,
			Timestamp: r.Timestamp,
		})
	}
	return trades, nil
}

func (a *Adapter) FetchHistoricalTrades(ctx context.Context, symbol string, opts *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	formattedSymbol := a.FormatSymbol(symbol)

	q := perp.AggTradesQuery{Symbol: formattedSymbol, Limit: 500}
	if opts != nil {
		if opts.Limit > 0 {
			q.Limit = opts.Limit
		}
		if opts.FromID != "" {
			id, err := strconv.ParseInt(opts.FromID, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid FromID: %w", err)
			}
			q.FromID = &id
		} else {
			if opts.Start != nil {
				q.StartTime = opts.Start.UnixMilli()
			}
			if opts.End != nil {
				q.EndTime = opts.End.UnixMilli()
			}
		}
	}

	raw, err := a.client.GetAggTradesPaged(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Trade, 0, len(raw))
	for _, r := range raw {
		side := exchanges.TradeSideBuy
		if r.IsBuyerMaker {
			side = exchanges.TradeSideSell
		}
		out = append(out, exchanges.Trade{
			ID:        fmt.Sprintf("%d", r.ID),
			Symbol:    symbol,
			Price:     parseDecimal(r.Price),
			Quantity:  parseDecimal(r.Quantity),
			Side:      side,
			Timestamp: r.Timestamp,
		})
	}
	return out, nil
}

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	formattedSymbol := strings.ToUpper(a.FormatSymbol(symbol))
	bookTicker, err := a.client.BookTicker(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}

	ticker, err := a.client.Ticker(ctx, formattedSymbol)
	res := &exchanges.Ticker{
		Symbol:    symbol,
		Bid:       parseDecimal(bookTicker.BidPrice),
		Ask:       parseDecimal(bookTicker.AskPrice),
		Timestamp: time.Now().UnixMilli(),
	}

	if err == nil && ticker != nil {
		res.LastPrice = parseDecimal(ticker.LastPrice)
		res.High24h = parseDecimal(ticker.HighPrice)
		res.Low24h = parseDecimal(ticker.LowPrice)
		res.Volume24h = parseDecimal(ticker.Volume)
		res.QuoteVol = parseDecimal(ticker.QuoteVolume)
		res.Timestamp = ticker.CloseTime
	} else {
		res.LastPrice = res.Bid.Add(res.Ask).Div(decimal.NewFromInt(2))
	}

	return res, nil
}

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	formattedSymbol := strings.ToUpper(a.FormatSymbol(symbol))
	res, err := a.client.Depth(ctx, formattedSymbol, limit)
	if err != nil {
		return nil, err
	}

	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
		Bids:      make([]exchanges.Level, 0, len(res.Bids)),
		Asks:      make([]exchanges.Level, 0, len(res.Asks)),
	}

	for _, item := range res.Bids {
		if len(item) >= 2 {
			ob.Bids = append(ob.Bids, exchanges.Level{Price: parseDecimal(item[0]), Quantity: parseDecimal(item[1])})
		}
	}
	for _, item := range res.Asks {
		if len(item) >= 2 {
			ob.Asks = append(ob.Asks, exchanges.Level{Price: parseDecimal(item[0]), Quantity: parseDecimal(item[1])})
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
	formattedSymbol := strings.ToUpper(a.FormatSymbol(symbol))
	var startTime, endTime int64
	if start != nil {
		startTime = start.UnixMilli()
	}
	if end != nil {
		endTime = end.UnixMilli()
	}

	res, err := a.client.Klines(ctx, formattedSymbol, string(interval), limit, startTime, endTime)
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, 0, len(res))
	for _, row := range res {
		rowSlice := []interface{}(row)
		if len(rowSlice) < 6 {
			continue
		}

		k := exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: parseInt64(rowSlice[0]),
			Open:      parseDecimalInterface(rowSlice[1]),
			High:      parseDecimalInterface(rowSlice[2]),
			Low:       parseDecimalInterface(rowSlice[3]),
			Close:     parseDecimalInterface(rowSlice[4]),
			Volume:    parseDecimalInterface(rowSlice[5]),
		}
		if len(rowSlice) > 7 {
			k.QuoteVol = parseDecimalInterface(rowSlice[7])
		}
		klines = append(klines, k)
	}
	return klines, nil
}

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	resp, err := a.client.MyTrades(ctx, strings.ToUpper(a.FormatSymbol(symbol)), limit, 0, 0, 0)
	if err != nil {
		return nil, err
	}

	trades := make([]exchanges.Trade, 0, len(resp))
	for _, t := range resp {
		side := exchanges.TradeSideBuy
		if !t.IsBuyer {
			side = exchanges.TradeSideSell
		}

		trades = append(trades, exchanges.Trade{
			ID:        fmt.Sprintf("%d", t.ID),
			Symbol:    a.ExtractSymbol(t.Symbol),
			Price:     parseDecimal(t.Price),
			Quantity:  parseDecimal(t.Qty),
			Side:      side,
			Timestamp: t.Time,
		})
	}

	return trades, nil
}
