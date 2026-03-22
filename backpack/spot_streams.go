package backpack

import (
	"context"
	"encoding/json"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
)

func (a *SpotAdapter) WatchOrderBook(ctx context.Context, symbol string, cb exchanges.OrderBookCallback) error {
	formatted := a.FormatSymbol(symbol)
	if err := a.StopWatchOrderBook(context.Background(), symbol); err != nil {
		return err
	}

	ob := NewOrderBook(formatted)
	a.SetLocalOrderBook(formatted, ob)

	watchCtx, cancel := context.WithCancel(context.Background())
	a.cancelMu.Lock()
	a.cancels[formatted] = cancel
	a.cancelMu.Unlock()

	if err := a.marketWS.Subscribe(ctx, "depth."+formatted, false, func(payload json.RawMessage) {
		select {
		case <-watchCtx.Done():
			return
		default:
		}

		event, err := decodeDepthEvent(payload)
		if err != nil {
			return
		}
		if err := ob.ProcessUpdate(event); err != nil {
			_ = refreshOrderBookSnapshot(a.client, formatted, ob)
		}
		emitOrderBookUpdate(cb, ob, symbol, event.EngineTimestamp)
	}); err != nil {
		cancel()
		a.RemoveLocalOrderBook(formatted)
		return err
	}

	if err := refreshOrderBookSnapshot(a.client, formatted, ob); err != nil {
		_ = a.marketWS.Unsubscribe(context.Background(), "depth."+formatted)
		cancel()
		a.RemoveLocalOrderBook(formatted)
		return err
	}

	a.MarkMarketConnected()
	return a.BaseAdapter.WaitOrderBookReady(ctx, formatted)
}

func (a *SpotAdapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	formatted := a.FormatSymbol(symbol)

	a.cancelMu.Lock()
	if cancel, ok := a.cancels[formatted]; ok {
		cancel()
		delete(a.cancels, formatted)
	}
	a.cancelMu.Unlock()

	a.RemoveLocalOrderBook(formatted)
	return a.marketWS.Unsubscribe(ctx, "depth."+formatted)
}

func (a *SpotAdapter) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	err := a.accountWS.Subscribe(ctx, "account.orderUpdate", true, func(payload json.RawMessage) {
		event, err := decodeOrderUpdate(payload)
		if err != nil || strings.HasSuffix(strings.ToUpper(event.Symbol), "_PERP") {
			return
		}
		if cb != nil {
			cb(mapOrderUpdate(*event))
		}
	})
	if err == nil {
		a.MarkOrderConnected()
	}
	return err
}

func (a *SpotAdapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	_ = ctx
	_ = cb
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchOrders(ctx context.Context) error {
	return a.accountWS.Unsubscribe(ctx, "account.orderUpdate")
}

func (a *SpotAdapter) StopWatchPositions(ctx context.Context) error {
	_ = ctx
	return exchanges.ErrNotSupported
}
