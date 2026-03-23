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

type Adapter struct {
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

func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}
	lifecycleCtx, cancel := context.WithCancel(ctx)
	adp, err := newPerpAdapterWithClient(lifecycleCtx, cancel, opts, quote, sdk.NewClient().WithCredentials(opts.APIKey, opts.PrivateKey))
	if err != nil {
		cancel()
		return nil, err
	}
	return adp, nil
}

func newPerpAdapterWithClient(ctx context.Context, cancel context.CancelFunc, opts Options, quote exchanges.QuoteCurrency, client adapterRESTClient) (*Adapter, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}
	markets, err := client.GetMarkets(ctx)
	if err != nil {
		return nil, err
	}
	cache, err := buildMarketCache(markets, quote)
	if err != nil {
		return nil, err
	}
	base := exchanges.NewBaseAdapter("BACKPACK", exchanges.MarketTypePerp, opts.logger())
	// Backpack places and cancels orders over REST in this adapter pass.
	base.SetOrderMode(exchanges.OrderModeREST)
	base.SetSymbolDetails(buildSymbolDetails(markets, quote, exchanges.MarketTypePerp))
	return &Adapter{
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

func (a *Adapter) Close() error {
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

func (a *Adapter) FormatSymbol(symbol string) string {
	if a.markets == nil {
		return strings.ToUpper(symbol)
	}
	if market, ok := a.markets.perpByBase[strings.ToUpper(symbol)]; ok {
		return market.Symbol
	}
	return strings.ToUpper(symbol)
}

func (a *Adapter) ExtractSymbol(symbol string) string {
	upper := strings.ToUpper(symbol)
	if market, ok := a.markets.bySymbol[upper]; ok {
		return strings.ToUpper(market.BaseSymbol)
	}
	return strings.TrimSuffix(strings.TrimSuffix(upper, "_PERP"), "_"+string(a.quote))
}

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	raw, err := a.client.GetTicker(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return toTicker(symbol, raw), nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	raw, err := a.client.GetOrderBook(ctx, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return toOrderBook(strings.ToUpper(symbol), raw), nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
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

func (a *Adapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
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

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}
	if err := a.BaseAdapter.ValidateOrder(params); err != nil {
		return nil, err
	}
	market, ok := a.markets.perpByBase[strings.ToUpper(params.Symbol)]
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

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	_, err := a.client.CancelOrder(ctx, toCancelOrderRequest(orderID, a.FormatSymbol(symbol)))
	return err
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
	return a.client.CancelOpenOrders(ctx, a.FormatSymbol(symbol), "PERP")
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	raw, err := a.client.GetOpenOrders(ctx, "PERP", a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	orders := make([]exchanges.Order, 0, len(raw))
	for _, order := range raw {
		orders = append(orders, *mapOrder(order))
	}
	return orders, nil
}

func (a *Adapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	balances, err := a.client.GetBalances(ctx)
	if err != nil {
		return nil, err
	}
	orders, err := a.client.GetOpenOrders(ctx, "PERP", "")
	if err != nil {
		return nil, err
	}
	positions, err := a.client.GetOpenPositions(ctx, "")
	if err != nil {
		return nil, err
	}

	account := &exchanges.Account{
		Positions: make([]exchanges.Position, 0, len(positions)),
		Orders:    make([]exchanges.Order, 0, len(orders)),
	}
	if balance, ok := balances[string(a.quote)]; ok {
		account.TotalBalance = parseDecimal(balance.Available).Add(parseDecimal(balance.Locked)).Add(parseDecimal(balance.Staked))
		account.AvailableBalance = parseDecimal(balance.Available)
	}
	for _, order := range orders {
		account.Orders = append(account.Orders, *mapOrder(order))
	}
	for _, position := range positions {
		account.Positions = append(account.Positions, mapPosition(position))
		account.UnrealizedPnL = account.UnrealizedPnL.Add(parseDecimal(position.PnlUnrealized))
		account.RealizedPnL = account.RealizedPnL.Add(parseDecimal(position.PnlRealized))
	}
	return account, nil
}

func (a *Adapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
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

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	return a.GetSymbolDetail(strings.ToUpper(symbol))
}

func (a *Adapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	account, err := a.client.GetAccount(ctx)
	if err != nil {
		return nil, err
	}
	return &exchanges.FeeRate{
		Maker: parseDecimal(account.FuturesMakerFee),
		Taker: parseDecimal(account.FuturesTakerFee),
	}, nil
}

func (a *Adapter) WatchTicker(ctx context.Context, symbol string, cb exchanges.TickerCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchTrades(ctx context.Context, symbol string, cb exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, cb exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchTicker(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchTrades(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	raw, err := a.client.GetOpenPositions(ctx, "")
	if err != nil {
		return nil, err
	}
	positions := make([]exchanges.Position, 0, len(raw))
	for _, position := range raw {
		positions = append(positions, mapPosition(position))
	}
	return positions, nil
}

func (a *Adapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	rates, err := a.client.GetFundingRates(ctx)
	if err != nil {
		return nil, err
	}
	target := a.FormatSymbol(symbol)
	for _, rate := range rates {
		if rate.Symbol == target {
			return &exchanges.FundingRate{
				Symbol:          strings.ToUpper(symbol),
				FundingRate:     parseDecimal(rate.FundingRate),
				NextFundingTime: microsToMillis(rate.NextFundingTimestamp),
			}, nil
		}
	}
	return nil, exchanges.ErrSymbolNotFound
}

func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error) {
	raw, err := a.client.GetFundingRates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.FundingRate, 0, len(raw))
	for _, rate := range raw {
		out = append(out, exchanges.FundingRate{
			Symbol:          a.ExtractSymbol(rate.Symbol),
			FundingRate:     parseDecimal(rate.FundingRate),
			NextFundingTime: microsToMillis(rate.NextFundingTimestamp),
		})
	}
	return out, nil
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func toOrderBook(symbol string, raw *sdk.Depth) *exchanges.OrderBook {
	bids := make([]exchanges.Level, 0, len(raw.Bids))
	for _, level := range raw.Bids {
		if len(level) < 2 {
			continue
		}
		bids = append(bids, exchanges.Level{Price: parseDecimal(level[0]), Quantity: parseDecimal(level[1])})
	}
	asks := make([]exchanges.Level, 0, len(raw.Asks))
	for _, level := range raw.Asks {
		if len(level) < 2 {
			continue
		}
		asks = append(asks, exchanges.Level{Price: parseDecimal(level[0]), Quantity: parseDecimal(level[1])})
	}
	return &exchanges.OrderBook{
		Symbol:    symbol,
		Bids:      bids,
		Asks:      asks,
		Timestamp: microsToMillis(raw.Timestamp),
	}
}

func (a *Adapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	ob := a.BaseAdapter.GetLocalOrderBook(a.FormatSymbol(symbol), depth)
	if ob == nil {
		return nil
	}
	ob.Symbol = strings.ToUpper(symbol)
	return ob
}
