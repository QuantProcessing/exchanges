package lighter

import (
	"context"
	"encoding/json"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
	sdklighter "github.com/QuantProcessing/exchanges/lighter/sdk"
)

type lighterTradeWS interface {
	SubscribeTrades(marketID int, cb func([]byte)) error
}

func (a *Adapter) watchTradesWithWS(ctx context.Context, ws lighterTradeWS, symbol string, callback exchanges.TradeCallback) error {
	_ = ctx
	formattedSymbol := a.FormatSymbol(symbol)

	a.metaMu.RLock()
	mid, ok := a.symbolToID[formattedSymbol]
	a.metaMu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}

	return subscribeLighterTrades(ws, mid, symbol, callback)
}

func (a *SpotAdapter) watchTradesWithWS(ctx context.Context, ws lighterTradeWS, symbol string, callback exchanges.TradeCallback) error {
	_ = ctx
	formattedSymbol := a.FormatSymbol(symbol)

	a.metaMu.RLock()
	mid, ok := a.symbolToID[formattedSymbol]
	a.metaMu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}

	return subscribeLighterTrades(ws, mid, symbol, callback)
}

func subscribeLighterTrades(ws lighterTradeWS, marketID int, symbol string, callback exchanges.TradeCallback) error {
	return ws.SubscribeTrades(marketID, func(data []byte) {
		if callback == nil {
			return
		}

		var event sdklighter.WsTradeEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return
		}

		for _, trade := range event.Trades {
			callback(mapLighterTradeToTrade(symbol, trade))
		}
		for _, trade := range event.LiquidationTrades {
			callback(mapLighterTradeToTrade(symbol, trade))
		}
	})
}

func mapLighterTradeToTrade(symbol string, trade sdklighter.Trade) *exchanges.Trade {
	side := exchanges.TradeSideSell
	if !trade.IsMakerAsk {
		side = exchanges.TradeSideBuy
	}

	id := trade.TradeIdStr
	if id == "" {
		id = fmt.Sprintf("%d", trade.TradeId)
	}

	return &exchanges.Trade{
		ID:        id,
		Symbol:    symbol,
		Price:     parseLighterFloat(trade.Price),
		Quantity:  parseLighterFloat(trade.Size),
		Side:      side,
		Timestamp: trade.Timestamp,
	}
}
