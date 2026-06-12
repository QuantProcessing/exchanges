package bitget

import (
	"context"

	exchanges "github.com/QuantProcessing/exchanges"
)

func (a *Adapter) FetchTickerFor(ctx context.Context, market exchanges.MarketRef) (*exchanges.Ticker, error) {
	return a.FetchTicker(ctx, a.marketSymbol(market))
}

func (a *Adapter) FetchOrderBookFor(ctx context.Context, market exchanges.MarketRef, limit int) (*exchanges.OrderBook, error) {
	return a.FetchOrderBook(ctx, a.marketSymbol(market), limit)
}

func (a *Adapter) PlaceOrderFor(ctx context.Context, market exchanges.MarketRef, params *exchanges.OrderParams) (*exchanges.Order, error) {
	return a.PlaceOrder(ctx, a.paramsForMarket(market, params))
}

func (a *Adapter) FetchOpenOrdersFor(ctx context.Context, market exchanges.MarketRef) ([]exchanges.Order, error) {
	return a.FetchOpenOrders(ctx, a.marketSymbol(market))
}

func (a *Adapter) marketSymbol(market exchanges.MarketRef) string {
	return normalizeInstrumentMarket(market, a.quote, exchanges.MarketTypePerp).Symbol()
}

func (a *Adapter) paramsForMarket(market exchanges.MarketRef, params *exchanges.OrderParams) *exchanges.OrderParams {
	return paramsForMarket(market, a.quote, exchanges.MarketTypePerp, params)
}

func (a *SpotAdapter) FetchTickerFor(ctx context.Context, market exchanges.MarketRef) (*exchanges.Ticker, error) {
	return a.FetchTicker(ctx, a.marketSymbol(market))
}

func (a *SpotAdapter) FetchOrderBookFor(ctx context.Context, market exchanges.MarketRef, limit int) (*exchanges.OrderBook, error) {
	return a.FetchOrderBook(ctx, a.marketSymbol(market), limit)
}

func (a *SpotAdapter) PlaceOrderFor(ctx context.Context, market exchanges.MarketRef, params *exchanges.OrderParams) (*exchanges.Order, error) {
	return a.PlaceOrder(ctx, a.paramsForMarket(market, params))
}

func (a *SpotAdapter) FetchOpenOrdersFor(ctx context.Context, market exchanges.MarketRef) ([]exchanges.Order, error) {
	return a.FetchOpenOrders(ctx, a.marketSymbol(market))
}

func (a *SpotAdapter) marketSymbol(market exchanges.MarketRef) string {
	return normalizeInstrumentMarket(market, a.quote, exchanges.MarketTypeSpot).Symbol()
}

func (a *SpotAdapter) paramsForMarket(market exchanges.MarketRef, params *exchanges.OrderParams) *exchanges.OrderParams {
	return paramsForMarket(market, a.quote, exchanges.MarketTypeSpot, params)
}

func paramsForMarket(market exchanges.MarketRef, defaultQuote exchanges.QuoteCurrency, marketType exchanges.MarketType, params *exchanges.OrderParams) *exchanges.OrderParams {
	if params == nil {
		return nil
	}
	normalized := normalizeInstrumentMarket(market, defaultQuote, marketType)
	copied := *params
	copied.Symbol = normalized.Symbol()
	copied.Market = normalized
	return &copied
}

func normalizeInstrumentMarket(market exchanges.MarketRef, defaultQuote exchanges.QuoteCurrency, marketType exchanges.MarketType) exchanges.MarketRef {
	if market.Base == "" {
		market = exchanges.ParseMarketRef(market.VenueSymbol, defaultQuote, marketType)
	}
	if market.Quote == "" {
		market.Quote = defaultQuote
	}
	if market.Settle == "" {
		market.Settle = market.Quote
	}
	if market.Type == "" {
		market.Type = marketType
	}
	return market
}
