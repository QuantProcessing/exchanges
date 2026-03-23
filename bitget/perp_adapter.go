package bitget

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/shopspring/decimal"
)

type Adapter struct {
	*exchanges.BaseAdapter
	client       *sdk.Client
	publicWS     *sdk.PublicWSClient
	privateWS    *sdk.PrivateWSClient
	private      perpPrivateProfile
	markets      *marketCache
	quote        exchanges.QuoteCurrency
	perpCategory string
	cancel       context.CancelFunc
	cancels      map[string]context.CancelFunc
	mu           sync.RWMutex
}

func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}

	lifecycleCtx, cancel := context.WithCancel(ctx)
	adp, err := newPerpAdapterWithClient(lifecycleCtx, cancel, opts, quote, sdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase))
	if err != nil {
		cancel()
		return nil, err
	}
	return adp, nil
}

func newPerpAdapterWithClient(ctx context.Context, cancel context.CancelFunc, opts Options, quote exchanges.QuoteCurrency, client *sdk.Client) (*Adapter, error) {
	base := exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypePerp, opts.logger())
	// Bitget defaults order transport to REST; only classic place/cancel paths switch on OrderModeWS.
	base.SetOrderMode(exchanges.OrderModeREST)

	instruments, err := client.GetInstruments(ctx, quoteToPerpCategory(quote), "")
	if err != nil {
		return nil, err
	}
	if hasAnyCredentials(opts) && !hasFullCredentials(opts) {
		return nil, authError("bitget: api_key, secret_key, and passphrase must all be set together")
	}
	markets := buildMarketCache(instruments, quote)
	base.SetSymbolDetails(buildSymbolDetails(instruments, quote, exchanges.MarketTypePerp))

	adp := &Adapter{
		BaseAdapter:  base,
		client:       client,
		publicWS:     sdk.NewPublicWSClient(),
		privateWS:    newPrivateWSClient(opts),
		markets:      markets,
		quote:        quote,
		perpCategory: quoteToPerpCategory(quote),
		cancel:       cancel,
		cancels:      make(map[string]context.CancelFunc),
	}
	adp.private = newPerpPrivateProfile(adp)
	return adp, nil
}

func (a *Adapter) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.publicWS != nil {
		_ = a.publicWS.Close()
	}
	if a.privateWS != nil {
		_ = a.privateWS.Close()
	}
	return nil
}

func (a *Adapter) FormatSymbol(symbol string) string {
	upper := strings.ToUpper(symbol)
	a.mu.RLock()
	defer a.mu.RUnlock()
	if inst, ok := a.markets.perpByBase[upper]; ok {
		return inst.Symbol
	}
	return upper
}

func (a *Adapter) ExtractSymbol(symbol string) string {
	upper := strings.ToUpper(symbol)
	a.mu.RLock()
	defer a.mu.RUnlock()
	if inst, ok := a.markets.bySymbol[upper]; ok && inst.BaseCoin != "" {
		return strings.ToUpper(inst.BaseCoin)
	}
	return upper
}

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	raw, err := a.client.GetTicker(ctx, a.perpCategory, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return toTicker(symbol, raw), nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	raw, err := a.client.GetOrderBook(ctx, a.perpCategory, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return toOrderBook(symbol, raw), nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	raw, err := a.client.GetRecentFills(ctx, a.perpCategory, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return mapTrades(symbol, raw), nil
}

func (a *Adapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	rawInterval, err := klineIntervalString(interval)
	if err != nil {
		return nil, err
	}
	startTime, endTime, limit, err := klineTimeRange(interval, opts)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.GetCandles(ctx, a.perpCategory, a.FormatSymbol(symbol), rawInterval, "market", startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	return mapKlines(symbol, interval, raw), nil
}

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	return a.private.PlaceOrder(ctx, params)
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	return a.private.CancelOrder(ctx, orderID, symbol)
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
	return a.private.CancelAllOrders(ctx, symbol)
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	return a.private.FetchOrderByID(ctx, orderID, symbol)
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return a.private.FetchOrders(ctx, symbol)
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return a.private.FetchOpenOrders(ctx, symbol)
}

func (a *Adapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	return a.private.FetchAccount(ctx)
}

func (a *Adapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	return a.private.FetchBalance(ctx)
}

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	return a.GetSymbolDetail(strings.ToUpper(symbol))
}

func (a *Adapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	return a.private.FetchFeeRate(ctx, symbol)
}

func (a *Adapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	ob := a.BaseAdapter.GetLocalOrderBook(a.FormatSymbol(symbol), depth)
	if ob != nil {
		ob.Symbol = strings.ToUpper(symbol)
	}
	return ob
}

func (a *Adapter) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	return a.private.FetchPositions(ctx)
}

func (a *Adapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	return a.private.SetLeverage(ctx, symbol, leverage)
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return a.private.ModifyOrder(ctx, orderID, symbol, params)
}

func (a *Adapter) WsOrderConnected(ctx context.Context) error {
	if err := requirePrivateAccess(a.client); err != nil {
		return err
	}
	if a.privateWS == nil {
		return fmt.Errorf("bitget: private ws client unavailable")
	}
	if err := a.privateWS.Connect(ctx); err != nil {
		return err
	}
	a.MarkOrderConnected()
	return nil
}

func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, cb exchanges.OrderBookCallback) error {
	formatted := a.FormatSymbol(symbol)
	if err := a.StopWatchOrderBook(context.Background(), symbol); err != nil {
		return err
	}

	ob := NewOrderBook(formatted)
	if snapshot, err := a.client.GetOrderBook(ctx, a.perpCategory, formatted, 50); err == nil {
		ob.LoadSnapshot(snapshot)
	}
	a.SetLocalOrderBook(formatted, ob)

	watchCtx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.cancels[formatted] = cancel
	a.mu.Unlock()

	err := a.publicWS.Subscribe(ctx, sdk.WSArg{
		InstType: strings.ToLower(a.perpCategory),
		Topic:    "books",
		Symbol:   formatted,
	}, func(payload json.RawMessage) {
		select {
		case <-watchCtx.Done():
			return
		default:
		}

		msg, err := sdk.DecodeOrderBookMessage(payload)
		if err != nil || len(msg.Data) == 0 {
			return
		}
		if err := ob.ProcessUpdate(msg.Action, &msg.Data[0]); err != nil {
			return
		}
		if cb != nil {
			cb(a.GetLocalOrderBook(formatted, 50))
		}
	})
	if err != nil {
		cancel()
		a.RemoveLocalOrderBook(formatted)
		return err
	}

	a.MarkMarketConnected()
	return a.BaseAdapter.WaitOrderBookReady(ctx, formatted)
}

func (a *Adapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	formatted := a.FormatSymbol(symbol)

	a.mu.Lock()
	if cancel, ok := a.cancels[formatted]; ok {
		cancel()
		delete(a.cancels, formatted)
	}
	a.mu.Unlock()

	a.RemoveLocalOrderBook(formatted)
	return a.publicWS.Unsubscribe(ctx, sdk.WSArg{
		InstType: strings.ToLower(a.perpCategory),
		Topic:    "books",
		Symbol:   formatted,
	})
}

func (a *Adapter) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	return a.private.WatchOrders(ctx, cb)
}

func (a *Adapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	return a.private.WatchPositions(ctx, cb)
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

func (a *Adapter) StopWatchOrders(ctx context.Context) error {
	return a.private.StopWatchOrders(ctx)
}

func (a *Adapter) StopWatchPositions(ctx context.Context) error {
	return a.private.StopWatchPositions(ctx)
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
