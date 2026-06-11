package lighter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

type lighterOrderBookWS interface {
	SubscribeOrderBook(marketID int, cb func([]byte)) error
	UnsubscribeOrderBook(marketID int) error
}

func startLighterOrderBookWatch(
	ctx context.Context,
	base *exchanges.BaseAdapter,
	cancelMu *sync.Mutex,
	cancels map[string]context.CancelFunc,
	ws lighterOrderBookWS,
	formattedSymbol string,
	mid int,
	depth int,
	cb exchanges.OrderBookCallback,
) error {
	cancelMu.Lock()
	if cancel, ok := cancels[formattedSymbol]; ok {
		cancel()
	}
	subCtx, cancel := context.WithCancel(context.Background())
	cancels[formattedSymbol] = cancel
	cancelMu.Unlock()

	ob := NewOrderBook(formattedSymbol)
	base.SetLocalOrderBook(formattedSymbol, ob)

	var resubscribing atomic.Bool
	var handler func([]byte)

	resubscribe := func() {
		if !resubscribing.CompareAndSwap(false, true) {
			return
		}

		go func() {
			defer resubscribing.Store(false)

			select {
			case <-subCtx.Done():
				return
			default:
			}

			if err := ws.UnsubscribeOrderBook(mid); err != nil {
				base.Logger.Debugw("lighter orderbook unsubscribe during resync failed", "symbol", formattedSymbol, "error", err)
			}
			if err := ws.SubscribeOrderBook(mid, handler); err != nil {
				base.Logger.Errorw("lighter orderbook resubscribe failed", "symbol", formattedSymbol, "error", err)
			}
		}()
	}

	handler = func(data []byte) {
		select {
		case <-subCtx.Done():
			return
		default:
		}

		err := ob.ProcessUpdate(data)
		if err != nil {
			if errors.Is(err, ErrOrderBookResyncRequired) {
				resubscribe()
			}
			return
		}

		if cb != nil && ob.IsReady() {
			cb(ob.ToAdapterOrderBook(depth))
		}
	}

	if err := ws.SubscribeOrderBook(mid, handler); err != nil {
		cancel()
		cancelMu.Lock()
		delete(cancels, formattedSymbol)
		cancelMu.Unlock()
		base.RemoveLocalOrderBook(formattedSymbol)
		return err
	}

	if !ob.WaitReady(ctx, 10*time.Second) {
		return fmt.Errorf("orderbook %s not ready within timeout", formattedSymbol)
	}

	return nil
}
