package backpack

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
)

type backpackPrivateOrderStreamState struct {
	mu         sync.Mutex
	subscribed bool
	orderCB    exchanges.OrderUpdateCallback
	fillCB     exchanges.FillCallback
}

func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, depth int, cb exchanges.OrderBookCallback) error {
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
		emitOrderBookUpdate(cb, ob, symbol, event.EngineTimestamp, depth)
	}); err != nil {
		cancel()
		a.RemoveLocalOrderBook(formatted)
		return err
	}

	if err := waitForInitialOrderBookSnapshot(ctx, a.client, formatted, ob); err != nil {
		_ = a.marketWS.Unsubscribe(context.Background(), "depth."+formatted)
		cancel()
		a.RemoveLocalOrderBook(formatted)
		return err
	}

	a.MarkMarketConnected()
	return a.BaseAdapter.WaitOrderBookReady(ctx, formatted)
}

func (a *Adapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
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

func (a *Adapter) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	return a.watchPrivateOrderStream(ctx, cb, nil)
}

func (a *Adapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	err := a.accountWS.Subscribe(ctx, "account.positionUpdate", true, func(payload json.RawMessage) {
		event, err := decodePositionUpdate(payload)
		if err != nil || !strings.HasSuffix(strings.ToUpper(event.Symbol), "_PERP") {
			return
		}
		if cb != nil {
			cb(mapPositionUpdate(*event))
		}
	})
	if err == nil {
		a.MarkAccountConnected()
	}
	return err
}

func (a *Adapter) StopWatchOrders(ctx context.Context) error {
	return a.stopPrivateOrderStream(ctx, true, false)
}

func (a *Adapter) WatchFills(ctx context.Context, cb exchanges.FillCallback) error {
	return a.watchPrivateOrderStream(ctx, nil, cb)
}

func (a *Adapter) StopWatchFills(ctx context.Context) error {
	return a.stopPrivateOrderStream(ctx, false, true)
}

func (a *Adapter) StopWatchPositions(ctx context.Context) error {
	return a.accountWS.Unsubscribe(ctx, "account.positionUpdate")
}

func (a *SpotAdapter) WatchOrderBook(ctx context.Context, symbol string, depth int, cb exchanges.OrderBookCallback) error {
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
		emitOrderBookUpdate(cb, ob, symbol, event.EngineTimestamp, depth)
	}); err != nil {
		cancel()
		a.RemoveLocalOrderBook(formatted)
		return err
	}

	if err := waitForInitialOrderBookSnapshot(ctx, a.client, formatted, ob); err != nil {
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
	return a.watchPrivateOrderStream(ctx, cb, nil)
}

func (a *SpotAdapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	_ = ctx
	_ = cb
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchOrders(ctx context.Context) error {
	return a.stopPrivateOrderStream(ctx, true, false)
}

func (a *SpotAdapter) WatchFills(ctx context.Context, cb exchanges.FillCallback) error {
	return a.watchPrivateOrderStream(ctx, nil, cb)
}

func (a *SpotAdapter) StopWatchFills(ctx context.Context) error {
	return a.stopPrivateOrderStream(ctx, false, true)
}

func (a *SpotAdapter) StopWatchPositions(ctx context.Context) error {
	_ = ctx
	return exchanges.ErrNotSupported
}

func (a *Adapter) watchPrivateOrderStream(ctx context.Context, orderCB exchanges.OrderUpdateCallback, fillCB exchanges.FillCallback) error {
	if a.accountWS == nil {
		return exchanges.NewExchangeError("BACKPACK", "", "private websocket not configured", exchanges.ErrAuthFailed)
	}

	needSubscribe := false
	a.privateOrderStream.mu.Lock()
	if orderCB != nil {
		a.privateOrderStream.orderCB = orderCB
	}
	if fillCB != nil {
		a.privateOrderStream.fillCB = fillCB
	}
	if !a.privateOrderStream.subscribed {
		a.privateOrderStream.subscribed = true
		needSubscribe = true
	}
	a.privateOrderStream.mu.Unlock()

	if !needSubscribe {
		return nil
	}

	err := a.accountWS.Subscribe(ctx, "account.orderUpdate", true, func(payload json.RawMessage) {
		a.dispatchPrivateOrderUpdate(payload, true)
	})
	if err != nil {
		a.privateOrderStream.mu.Lock()
		a.privateOrderStream.subscribed = false
		if orderCB != nil {
			a.privateOrderStream.orderCB = nil
		}
		if fillCB != nil {
			a.privateOrderStream.fillCB = nil
		}
		a.privateOrderStream.mu.Unlock()
		return err
	}

	a.MarkOrderConnected()
	return nil
}

func (a *Adapter) stopPrivateOrderStream(ctx context.Context, clearOrders, clearFills bool) error {
	if a.accountWS == nil {
		return nil
	}

	shouldUnsubscribe := false
	a.privateOrderStream.mu.Lock()
	if clearOrders {
		a.privateOrderStream.orderCB = nil
	}
	if clearFills {
		a.privateOrderStream.fillCB = nil
	}
	if a.privateOrderStream.subscribed && a.privateOrderStream.orderCB == nil && a.privateOrderStream.fillCB == nil {
		a.privateOrderStream.subscribed = false
		shouldUnsubscribe = true
	}
	a.privateOrderStream.mu.Unlock()

	if !shouldUnsubscribe {
		return nil
	}
	return a.accountWS.Unsubscribe(ctx, "account.orderUpdate")
}

func (a *Adapter) dispatchPrivateOrderUpdate(payload json.RawMessage, wantPerp bool) {
	event, err := decodeOrderUpdate(payload)
	if err != nil {
		return
	}
	if strings.HasSuffix(strings.ToUpper(event.Symbol), "_PERP") != wantPerp {
		return
	}

	a.privateOrderStream.mu.Lock()
	orderCB := a.privateOrderStream.orderCB
	fillCB := a.privateOrderStream.fillCB
	a.privateOrderStream.mu.Unlock()

	if orderCB != nil {
		orderCB(mapOrderUpdate(*event))
	}
	if fillCB != nil {
		if fill := mapOrderFill(*event); fill != nil {
			fillCB(fill)
		}
	}
}

func (a *SpotAdapter) watchPrivateOrderStream(ctx context.Context, orderCB exchanges.OrderUpdateCallback, fillCB exchanges.FillCallback) error {
	if a.accountWS == nil {
		return exchanges.NewExchangeError("BACKPACK", "", "private websocket not configured", exchanges.ErrAuthFailed)
	}

	needSubscribe := false
	a.privateOrderStream.mu.Lock()
	if orderCB != nil {
		a.privateOrderStream.orderCB = orderCB
	}
	if fillCB != nil {
		a.privateOrderStream.fillCB = fillCB
	}
	if !a.privateOrderStream.subscribed {
		a.privateOrderStream.subscribed = true
		needSubscribe = true
	}
	a.privateOrderStream.mu.Unlock()

	if !needSubscribe {
		return nil
	}

	err := a.accountWS.Subscribe(ctx, "account.orderUpdate", true, func(payload json.RawMessage) {
		a.dispatchPrivateOrderUpdate(payload, false)
	})
	if err != nil {
		a.privateOrderStream.mu.Lock()
		a.privateOrderStream.subscribed = false
		if orderCB != nil {
			a.privateOrderStream.orderCB = nil
		}
		if fillCB != nil {
			a.privateOrderStream.fillCB = nil
		}
		a.privateOrderStream.mu.Unlock()
		return err
	}

	a.MarkOrderConnected()
	return nil
}

func (a *SpotAdapter) stopPrivateOrderStream(ctx context.Context, clearOrders, clearFills bool) error {
	if a.accountWS == nil {
		return nil
	}

	shouldUnsubscribe := false
	a.privateOrderStream.mu.Lock()
	if clearOrders {
		a.privateOrderStream.orderCB = nil
	}
	if clearFills {
		a.privateOrderStream.fillCB = nil
	}
	if a.privateOrderStream.subscribed && a.privateOrderStream.orderCB == nil && a.privateOrderStream.fillCB == nil {
		a.privateOrderStream.subscribed = false
		shouldUnsubscribe = true
	}
	a.privateOrderStream.mu.Unlock()

	if !shouldUnsubscribe {
		return nil
	}
	return a.accountWS.Unsubscribe(ctx, "account.orderUpdate")
}

func (a *SpotAdapter) dispatchPrivateOrderUpdate(payload json.RawMessage, wantPerp bool) {
	event, err := decodeOrderUpdate(payload)
	if err != nil {
		return
	}
	if strings.HasSuffix(strings.ToUpper(event.Symbol), "_PERP") != wantPerp {
		return
	}

	a.privateOrderStream.mu.Lock()
	orderCB := a.privateOrderStream.orderCB
	fillCB := a.privateOrderStream.fillCB
	a.privateOrderStream.mu.Unlock()

	if orderCB != nil {
		orderCB(mapOrderUpdate(*event))
	}
	if fillCB != nil {
		if fill := mapOrderFill(*event); fill != nil {
			fillCB(fill)
		}
	}
}
