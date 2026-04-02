package okx

import (
	"context"
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/okx/sdk"
)

type okxPrivateOrderStreamState struct {
	mu         sync.Mutex
	subscribed bool
	orderCB    exchanges.OrderUpdateCallback
	fillCB     exchanges.FillCallback
}

func (a *Adapter) watchPrivateOrders(ctx context.Context, orderCB exchanges.OrderUpdateCallback, fillCB exchanges.FillCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
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

	err := a.wsPrivate.SubscribeOrders("SWAP", nil, func(order *okx.Order) {
		a.dispatchPrivateOrder(order)
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

func (a *Adapter) stopPrivateOrders(ctx context.Context, clearOrders, clearFills bool) error {
	args := okx.WsSubscribeArgs{Channel: "orders", InstType: "SWAP"}
	return stopOKXPrivateOrders(ctx, a.wsPrivate, args, &a.privateOrderStream, clearOrders, clearFills)
}

func (a *Adapter) dispatchPrivateOrder(order *okx.Order) {
	a.privateOrderStream.mu.Lock()
	orderCB := a.privateOrderStream.orderCB
	fillCB := a.privateOrderStream.fillCB
	a.privateOrderStream.mu.Unlock()

	if orderCB != nil {
		orderCB(a.mapOrderStream(order))
	}
	if fillCB != nil {
		if fill := a.mapOrderFill(order); fill != nil {
			fillCB(fill)
		}
	}
}

func (a *SpotAdapter) watchPrivateOrders(ctx context.Context, orderCB exchanges.OrderUpdateCallback, fillCB exchanges.FillCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
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

	err := a.wsPrivate.SubscribeOrders("SPOT", nil, func(order *okx.Order) {
		a.dispatchPrivateOrder(order)
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

func (a *SpotAdapter) stopPrivateOrders(ctx context.Context, clearOrders, clearFills bool) error {
	args := okx.WsSubscribeArgs{Channel: "orders", InstType: "SPOT"}
	return stopOKXPrivateOrders(ctx, a.wsPrivate, args, &a.privateOrderStream, clearOrders, clearFills)
}

func (a *SpotAdapter) dispatchPrivateOrder(order *okx.Order) {
	a.privateOrderStream.mu.Lock()
	orderCB := a.privateOrderStream.orderCB
	fillCB := a.privateOrderStream.fillCB
	a.privateOrderStream.mu.Unlock()

	if orderCB != nil {
		orderCB(a.mapOrderStream(order))
	}
	if fillCB != nil {
		if fill := a.mapOrderFill(order); fill != nil {
			fillCB(fill)
		}
	}
}

func stopOKXPrivateOrders(ctx context.Context, ws *okx.WSClient, args okx.WsSubscribeArgs, state *okxPrivateOrderStreamState, clearOrders, clearFills bool) error {
	if ws == nil {
		return nil
	}

	shouldUnsubscribe := false
	state.mu.Lock()
	if clearOrders {
		state.orderCB = nil
	}
	if clearFills {
		state.fillCB = nil
	}
	if state.subscribed && state.orderCB == nil && state.fillCB == nil {
		state.subscribed = false
		shouldUnsubscribe = true
	}
	state.mu.Unlock()

	if !shouldUnsubscribe {
		return nil
	}
	return ws.Unsubscribe(args)
}
