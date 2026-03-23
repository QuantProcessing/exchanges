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

type SpotAdapter struct {
	*exchanges.BaseAdapter
	client      *sdk.Client
	publicWS    *sdk.PublicWSClient
	privateWS   *sdk.PrivateWSClient
	private     spotPrivateProfile
	markets     *marketCache
	quote       exchanges.QuoteCurrency
	cancel      context.CancelFunc
	cancels     map[string]context.CancelFunc
	mu          sync.RWMutex
}

func NewSpotAdapter(ctx context.Context, opts Options) (*SpotAdapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}

	lifecycleCtx, cancel := context.WithCancel(ctx)
	adp, err := newSpotAdapterWithClient(lifecycleCtx, cancel, opts, quote, sdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase))
	if err != nil {
		cancel()
		return nil, err
	}
	return adp, nil
}

func newSpotAdapterWithClient(ctx context.Context, cancel context.CancelFunc, opts Options, quote exchanges.QuoteCurrency, client *sdk.Client) (*SpotAdapter, error) {
	base := exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypeSpot, opts.logger())
	base.SetOrderMode(exchanges.OrderModeREST)

	instruments, err := client.GetInstruments(ctx, categorySpot, "")
	if err != nil {
		return nil, err
	}
	if hasAnyCredentials(opts) && !hasFullCredentials(opts) {
		return nil, authError("bitget: api_key, secret_key, and passphrase must all be set together")
	}
	markets := buildMarketCache(instruments, quote)
	base.SetSymbolDetails(buildSymbolDetails(instruments, quote, exchanges.MarketTypeSpot))

	adp := &SpotAdapter{
		BaseAdapter: base,
		client:      client,
		publicWS:    sdk.NewPublicWSClient(),
		privateWS:   newPrivateWSClient(opts),
		markets:     markets,
		quote:       quote,
		cancel:      cancel,
		cancels:     make(map[string]context.CancelFunc),
	}
	adp.private = newSpotPrivateProfile(adp)
	return adp, nil
}

func (a *SpotAdapter) Close() error {
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

func (a *SpotAdapter) FormatSymbol(symbol string) string {
	upper := strings.ToUpper(symbol)
	a.mu.RLock()
	defer a.mu.RUnlock()
	if inst, ok := a.markets.spotByBase[upper]; ok {
		return inst.Symbol
	}
	return upper
}

func (a *SpotAdapter) ExtractSymbol(symbol string) string {
	upper := strings.ToUpper(symbol)
	a.mu.RLock()
	defer a.mu.RUnlock()
	if inst, ok := a.markets.bySymbol[upper]; ok && inst.BaseCoin != "" {
		return strings.ToUpper(inst.BaseCoin)
	}
	return upper
}

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	raw, err := a.client.GetTicker(ctx, categorySpot, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return toTicker(symbol, raw), nil
}

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	raw, err := a.client.GetOrderBook(ctx, categorySpot, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return toOrderBook(symbol, raw), nil
}

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	raw, err := a.client.GetRecentFills(ctx, categorySpot, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return mapTrades(symbol, raw), nil
}

func (a *SpotAdapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	rawInterval, err := klineIntervalString(interval)
	if err != nil {
		return nil, err
	}
	startTime, endTime, limit, err := klineTimeRange(interval, opts)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.GetCandles(ctx, categorySpot, a.FormatSymbol(symbol), rawInterval, "market", startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	return mapKlines(symbol, interval, raw), nil
}

func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	return a.private.PlaceOrder(ctx, params)
}

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	return a.private.CancelOrder(ctx, orderID, symbol)
}

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	return a.private.CancelAllOrders(ctx, symbol)
}

func (a *SpotAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	return a.private.FetchOrderByID(ctx, orderID, symbol)
}

func (a *SpotAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return a.private.FetchOrders(ctx, symbol)
}

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return a.private.FetchOpenOrders(ctx, symbol)
}

func (a *SpotAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	return a.private.FetchAccount(ctx)
}

func (a *SpotAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	return a.private.FetchBalance(ctx)
}

func (a *SpotAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	return a.GetSymbolDetail(strings.ToUpper(symbol))
}

func (a *SpotAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	return a.private.FetchFeeRate(ctx, symbol)
}

func (a *SpotAdapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	ob := a.BaseAdapter.GetLocalOrderBook(a.FormatSymbol(symbol), depth)
	if ob != nil {
		ob.Symbol = strings.ToUpper(symbol)
	}
	return ob
}

func (a *SpotAdapter) FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error) {
	return a.private.FetchSpotBalances(ctx)
}

func (a *SpotAdapter) TransferAsset(ctx context.Context, params *exchanges.TransferParams) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) WsOrderConnected(ctx context.Context) error {
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

func (a *SpotAdapter) WatchOrderBook(ctx context.Context, symbol string, cb exchanges.OrderBookCallback) error {
	formatted := a.FormatSymbol(symbol)
	if err := a.StopWatchOrderBook(context.Background(), symbol); err != nil {
		return err
	}

	ob := NewOrderBook(formatted)
	if snapshot, err := a.client.GetOrderBook(ctx, categorySpot, formatted, 50); err == nil {
		ob.LoadSnapshot(snapshot)
	}
	a.SetLocalOrderBook(formatted, ob)

	watchCtx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.cancels[formatted] = cancel
	a.mu.Unlock()

	err := a.publicWS.Subscribe(ctx, sdk.WSArg{
		InstType: "spot",
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

func (a *SpotAdapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	formatted := a.FormatSymbol(symbol)

	a.mu.Lock()
	if cancel, ok := a.cancels[formatted]; ok {
		cancel()
		delete(a.cancels, formatted)
	}
	a.mu.Unlock()

	a.RemoveLocalOrderBook(formatted)
	return a.publicWS.Unsubscribe(ctx, sdk.WSArg{
		InstType: "spot",
		Topic:    "books",
		Symbol:   formatted,
	})
}

func (a *SpotAdapter) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	return a.private.WatchOrders(ctx, cb)
}

func (a *SpotAdapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	return exchanges.ErrNotSupported
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

func (a *SpotAdapter) StopWatchOrders(ctx context.Context) error {
	return a.private.StopWatchOrders(ctx)
}

func (a *SpotAdapter) StopWatchPositions(ctx context.Context) error {
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
