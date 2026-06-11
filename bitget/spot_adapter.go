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
	client    *sdk.Client
	publicWS  *sdk.PublicWSClient
	privateWS *sdk.PrivateWSClient
	private   spotPrivateProfile
	markets   *marketCache
	quote     exchanges.QuoteCurrency
	cancel    context.CancelFunc
	cancels   map[string]context.CancelFunc
	mu        sync.RWMutex
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
	if err := ensureSupportedAccountMode(opts); err != nil {
		return nil, err
	}
	if hasAnyCredentials(opts) && !hasFullCredentials(opts) {
		return nil, authError("bitget: api_key, secret_key, and passphrase must all be set together")
	}

	base := exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypeSpot, opts.logger())

	instruments, err := client.GetInstruments(ctx, categorySpot, "")
	if err != nil {
		return nil, err
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
	adp.private = newSpotPrivateProfile(adp, opts)
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
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.markets.FormatSymbol(symbol, a.quote, exchanges.MarketTypeSpot)
}

func (a *SpotAdapter) ExtractSymbol(symbol string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.markets.ExtractSymbol(symbol, a.quote, exchanges.MarketTypeSpot)
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

func (a *SpotAdapter) WatchOrderBook(ctx context.Context, symbol string, depth int, cb exchanges.OrderBookCallback) error {
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
			cb(a.GetLocalOrderBook(formatted, depth))
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

func (a *SpotAdapter) WatchFills(ctx context.Context, cb exchanges.FillCallback) error {
	return a.private.WatchFills(ctx, cb)
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

func (a *SpotAdapter) StopWatchFills(ctx context.Context) error {
	return a.private.StopWatchFills(ctx)
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
