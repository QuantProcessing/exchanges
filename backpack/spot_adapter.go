package backpack

import (
	"context"
	"fmt"
	"strings"
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/shopspring/decimal"
)

type SpotAdapter struct {
	*exchanges.BaseAdapter
	client    adapterRESTClient
	marketWS  *sdk.WSClient
	accountWS *sdk.WSClient
	markets   *marketCache
	quote     exchanges.QuoteCurrency
	cancel    context.CancelFunc
	cancelMu  sync.Mutex
	cancels   map[string]context.CancelFunc
}

func NewSpotAdapter(ctx context.Context, opts Options) (*SpotAdapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}
	lifecycleCtx, cancel := context.WithCancel(ctx)
	adp, err := newSpotAdapterWithClient(lifecycleCtx, cancel, opts, quote, sdk.NewClient().WithCredentials(opts.APIKey, opts.PrivateKey))
	if err != nil {
		cancel()
		return nil, err
	}
	return adp, nil
}

func newSpotAdapterWithClient(ctx context.Context, cancel context.CancelFunc, opts Options, quote exchanges.QuoteCurrency, client adapterRESTClient) (*SpotAdapter, error) {
	markets, err := client.GetMarkets(ctx)
	if err != nil {
		return nil, err
	}
	cache, err := buildMarketCache(markets, quote)
	if err != nil {
		return nil, err
	}
	base := exchanges.NewBaseAdapter("BACKPACK", exchanges.MarketTypeSpot, opts.logger())
	// Backpack places and cancels orders over REST in this adapter pass.
	base.SetOrderMode(exchanges.OrderModeREST)
	base.SetSymbolDetails(buildSymbolDetails(markets, quote, exchanges.MarketTypeSpot))
	return &SpotAdapter{
		BaseAdapter: base,
		client:      client,
		marketWS:    sdk.NewWSClient(),
		accountWS:   sdk.NewWSClient().WithCredentials(opts.APIKey, opts.PrivateKey),
		markets:     cache,
		quote:       quote,
		cancel:      cancel,
		cancels:     make(map[string]context.CancelFunc),
	}, nil
}

func (a *SpotAdapter) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.marketWS != nil {
		_ = a.marketWS.Close()
	}
	if a.accountWS != nil {
		_ = a.accountWS.Close()
	}
	return nil
}

func (a *SpotAdapter) FormatSymbol(symbol string) string {
	if a.markets == nil {
		return strings.ToUpper(symbol)
	}
	if market, ok := a.markets.spotByBase[strings.ToUpper(symbol)]; ok {
		return market.Symbol
	}
	return strings.ToUpper(symbol)
}

func (a *SpotAdapter) ExtractSymbol(symbol string) string {
	upper := strings.ToUpper(symbol)
	if market, ok := a.markets.bySymbol[upper]; ok {
		return strings.ToUpper(market.BaseSymbol)
	}
	return strings.TrimSuffix(upper, "_"+string(a.quote))
}

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	raw, err := a.client.GetTicker(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return toTicker(symbol, raw), nil
}

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	raw, err := a.client.GetOrderBook(ctx, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return toOrderBook(strings.ToUpper(symbol), raw), nil
}

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	raw, err := a.client.GetTrades(ctx, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	trades := make([]exchanges.Trade, 0, len(raw))
	for _, trade := range raw {
		side := exchanges.TradeSideBuy
		if trade.IsBuyerMaker {
			side = exchanges.TradeSideSell
		}
		trades = append(trades, exchanges.Trade{
			ID:        fmt.Sprintf("%d", trade.ID),
			Symbol:    strings.ToUpper(symbol),
			Price:     parseDecimal(trade.Price),
			Quantity:  parseDecimal(trade.Quantity),
			Side:      side,
			Timestamp: trade.Timestamp,
		})
	}
	return trades, nil
}

func (a *SpotAdapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	rawInterval, err := klineIntervalString(interval)
	if err != nil {
		return nil, err
	}
	start, end, err := klineTimeRange(interval, opts)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.GetKlines(ctx, a.FormatSymbol(symbol), rawInterval, start, end, "Last")
	if err != nil {
		return nil, err
	}
	return mapKlines(symbol, interval, raw), nil
}

func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}
	if err := a.BaseAdapter.ValidateOrder(params); err != nil {
		return nil, err
	}
	market, ok := a.markets.spotByBase[strings.ToUpper(params.Symbol)]
	if !ok {
		return nil, exchanges.ErrSymbolNotFound
	}
	req, err := toCreateOrderRequest(market, params)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.PlaceOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	order := mapOrder(*raw)
	if order.ClientOrderID == "0" && params.ClientID != "" {
		order.ClientOrderID = params.ClientID
	}
	return order, nil
}

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	_, err := a.client.CancelOrder(ctx, toCancelOrderRequest(orderID, a.FormatSymbol(symbol)))
	return err
}

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	return a.client.CancelOpenOrders(ctx, a.FormatSymbol(symbol), "SPOT")
}

func (a *SpotAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	raw, err := a.client.GetOpenOrders(ctx, "SPOT", a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	orders := make([]exchanges.Order, 0, len(raw))
	for _, order := range raw {
		orders = append(orders, *mapOrder(order))
	}
	return orders, nil
}

func (a *SpotAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	balances, err := a.client.GetBalances(ctx)
	if err != nil {
		return nil, err
	}
	orders, err := a.client.GetOpenOrders(ctx, "SPOT", "")
	if err != nil {
		return nil, err
	}
	account := &exchanges.Account{
		Orders: make([]exchanges.Order, 0, len(orders)),
	}
	if balance, ok := balances[string(a.quote)]; ok {
		account.TotalBalance = parseDecimal(balance.Available).Add(parseDecimal(balance.Locked)).Add(parseDecimal(balance.Staked))
		account.AvailableBalance = parseDecimal(balance.Available)
	}
	for _, order := range orders {
		account.Orders = append(account.Orders, *mapOrder(order))
	}
	return account, nil
}

func (a *SpotAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	balances, err := a.client.GetBalances(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	balance, ok := balances[string(a.quote)]
	if !ok {
		return decimal.Zero, exchanges.ErrSymbolNotFound
	}
	return parseDecimal(balance.Available), nil
}

func (a *SpotAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	return a.GetSymbolDetail(strings.ToUpper(symbol))
}

func (a *SpotAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	account, err := a.client.GetAccount(ctx)
	if err != nil {
		return nil, err
	}
	return &exchanges.FeeRate{
		Maker: parseDecimal(account.SpotMakerFee),
		Taker: parseDecimal(account.SpotTakerFee),
	}, nil
}

func (a *SpotAdapter) WatchTicker(ctx context.Context, symbol string, cb exchanges.TickerCallback) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) WatchTrades(ctx context.Context, symbol string, cb exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, cb exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchTicker(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchTrades(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error) {
	raw, err := a.client.GetBalances(ctx)
	if err != nil {
		return nil, err
	}
	return mapSpotBalances(raw), nil
}

func (a *SpotAdapter) TransferAsset(ctx context.Context, params *exchanges.TransferParams) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	ob := a.BaseAdapter.GetLocalOrderBook(a.FormatSymbol(symbol), depth)
	if ob == nil {
		return nil
	}
	ob.Symbol = strings.ToUpper(symbol)
	return ob
}
