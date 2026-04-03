package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

type lighterPerpTickerWS interface {
	SubscribeTicker(marketID int, cb func([]byte)) error
	SubscribeMarketStats(marketID int, cb func([]byte)) error
}

type lighterSpotTickerWS interface {
	SubscribeSpotMarketStats(marketID int, cb func([]byte)) error
}

type lighterTickerState struct {
	mu         sync.Mutex
	bid        decimal.Decimal
	ask        decimal.Decimal
	last       decimal.Decimal
	indexPrice decimal.Decimal
	markPrice  decimal.Decimal
	midPrice   decimal.Decimal
	volume24h  decimal.Decimal
	high24h    decimal.Decimal
	low24h     decimal.Decimal
}

func (a *Adapter) watchTickerWithWS(ctx context.Context, ws lighterPerpTickerWS, symbol string, callback exchanges.TickerCallback) error {
	_ = ctx
	formattedSymbol := a.FormatSymbol(symbol)

	a.metaMu.RLock()
	mid, ok := a.symbolToID[formattedSymbol]
	a.metaMu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}

	state := &lighterTickerState{}

	emit := func(ts int64) {
		if callback == nil {
			return
		}
		state.mu.Lock()
		ticker := &exchanges.Ticker{
			Symbol:     symbol,
			Bid:        state.bid,
			Ask:        state.ask,
			LastPrice:  state.last,
			IndexPrice: state.indexPrice,
			MarkPrice:  state.markPrice,
			Volume24h:  state.volume24h,
			High24h:    state.high24h,
			Low24h:     state.low24h,
			Timestamp:  ts,
		}
		if state.bid.IsPositive() && state.ask.IsPositive() {
			ticker.MidPrice = state.bid.Add(state.ask).Div(decimal.NewFromInt(2))
		}
		state.mu.Unlock()
		callback(ticker)
	}

	if err := ws.SubscribeTicker(mid, func(data []byte) {
		var evt struct {
			Timestamp int64 `json:"timestamp"`
			Ticker    struct {
				A struct {
					Price string `json:"price"`
				} `json:"a"`
				B struct {
					Price string `json:"price"`
				} `json:"b"`
			} `json:"ticker"`
		}
		if err := json.Unmarshal(data, &evt); err != nil {
			return
		}
		state.mu.Lock()
		state.bid = parseLighterFloat(evt.Ticker.B.Price)
		state.ask = parseLighterFloat(evt.Ticker.A.Price)
		state.mu.Unlock()
		emit(evt.Timestamp)
	}); err != nil {
		return err
	}

	return ws.SubscribeMarketStats(mid, func(data []byte) {
		var evt struct {
			Timestamp   int64 `json:"timestamp"`
			MarketStats struct {
				LastTradePrice       string  `json:"last_trade_price"`
				IndexPrice           string  `json:"index_price"`
				MarkPrice            string  `json:"mark_price"`
				DailyBaseTokenVolume float64 `json:"daily_base_token_volume"`
				DailyPriceHigh       float64 `json:"daily_price_high"`
				DailyPriceLow        float64 `json:"daily_price_low"`
			} `json:"market_stats"`
		}
		if err := json.Unmarshal(data, &evt); err != nil {
			return
		}
		state.mu.Lock()
		state.last = parseLighterFloat(evt.MarketStats.LastTradePrice)
		state.indexPrice = parseLighterFloat(evt.MarketStats.IndexPrice)
		state.markPrice = parseLighterFloat(evt.MarketStats.MarkPrice)
		state.volume24h = decimal.NewFromFloat(evt.MarketStats.DailyBaseTokenVolume)
		state.high24h = decimal.NewFromFloat(evt.MarketStats.DailyPriceHigh)
		state.low24h = decimal.NewFromFloat(evt.MarketStats.DailyPriceLow)
		state.mu.Unlock()
		emit(evt.Timestamp)
	})
}

func (a *SpotAdapter) watchTickerWithWS(ctx context.Context, ws lighterSpotTickerWS, symbol string, callback exchanges.TickerCallback) error {
	_ = ctx
	formattedSymbol := a.FormatSymbol(symbol)

	a.metaMu.RLock()
	mid, ok := a.symbolToID[formattedSymbol]
	a.metaMu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}

	return ws.SubscribeSpotMarketStats(mid, func(data []byte) {
		var evt struct {
			Timestamp       int64           `json:"timestamp"`
			SpotMarketStats json.RawMessage `json:"spot_market_stats"`
		}
		if err := json.Unmarshal(data, &evt); err != nil {
			return
		}

		stats, ok := decodeSpotMarketStats(evt.SpotMarketStats, mid)
		if !ok {
			return
		}

		if callback != nil {
			callback(&exchanges.Ticker{
				Symbol:    symbol,
				LastPrice: parseLighterFloat(stats.LastTradePrice),
				MidPrice:  parseLighterFloat(stats.MidPrice),
				Volume24h: decimal.NewFromFloat(stats.DailyBaseTokenVolume),
				QuoteVol:  decimal.NewFromFloat(stats.DailyQuoteTokenVolume),
				High24h:   decimal.NewFromFloat(stats.DailyPriceHigh),
				Low24h:    decimal.NewFromFloat(stats.DailyPriceLow),
				Timestamp: evt.Timestamp,
			})
		}
	})
}

type lighterSpotMarketStats struct {
	MarketID              int     `json:"market_id"`
	MidPrice              string  `json:"mid_price"`
	LastTradePrice        string  `json:"last_trade_price"`
	DailyBaseTokenVolume  float64 `json:"daily_base_token_volume"`
	DailyQuoteTokenVolume float64 `json:"daily_quote_token_volume"`
	DailyPriceLow         float64 `json:"daily_price_low"`
	DailyPriceHigh        float64 `json:"daily_price_high"`
	DailyPriceChange      float64 `json:"daily_price_change"`
}

func decodeSpotMarketStats(raw json.RawMessage, marketID int) (lighterSpotMarketStats, bool) {
	if len(raw) == 0 {
		return lighterSpotMarketStats{}, false
	}

	if raw[0] == '{' {
		var byMarket map[string]lighterSpotMarketStats
		if err := json.Unmarshal(raw, &byMarket); err == nil {
			if stats, ok := byMarket[fmt.Sprintf("%d", marketID)]; ok {
				return stats, true
			}
		}
	}

	var direct lighterSpotMarketStats
	if err := json.Unmarshal(raw, &direct); err != nil {
		return lighterSpotMarketStats{}, false
	}
	if direct.MarketID != 0 && direct.MarketID != marketID {
		return lighterSpotMarketStats{}, false
	}
	return direct, true
}
