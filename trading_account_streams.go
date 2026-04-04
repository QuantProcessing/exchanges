package exchanges

import (
	"context"
	"fmt"
	"time"

	"errors"
	"github.com/shopspring/decimal"
)

func (a *TradingAccount) Start(ctx context.Context) (err error) {
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return nil
	}
	a.started = true
	a.mu.Unlock()

	defer func() {
		if err == nil {
			return
		}
		a.mu.Lock()
		a.started = false
		a.mu.Unlock()
		a.resetSnapshotState()
	}()

	a.logger.Infow("trading_account: fetching initial account state")
	acc, err := a.adp.FetchAccount(ctx)
	if err != nil {
		return fmt.Errorf("trading_account: failed to get initial state: %w", err)
	}
	a.applyAccountSnapshot(acc)
	orderCount := 0
	positionCount := 0
	if acc != nil {
		orderCount = len(acc.Orders)
		positionCount = len(acc.Positions)
	}
	a.logger.Infow("trading_account: initial state loaded",
		"orders", orderCount, "positions", positionCount)

	streamable, ok := a.adp.(Streamable)
	if !ok {
		return fmt.Errorf("trading_account: adapter %s does not implement Streamable", a.adp.GetExchange())
	}

	if err := streamable.WatchOrders(ctx, a.applyOrderUpdate); err != nil {
		return fmt.Errorf("trading_account: WatchOrders failed: %w", err)
	}

	if watchErr := streamable.WatchPositions(ctx, a.applyPositionUpdate); watchErr != nil {
		if !errors.Is(watchErr, ErrNotSupported) {
			return fmt.Errorf("trading_account: WatchPositions failed: %w", watchErr)
		}
		a.logger.Warnw("trading_account: WatchPositions failed (may not be supported)", "error", watchErr)
	}

	go a.periodicRefresh(ctx, time.Minute)

	return nil
}

func (a *TradingAccount) Close() {
	a.mu.Lock()
	if !a.started {
		a.mu.Unlock()
		return
	}
	a.started = false
	done := a.done
	a.done = make(chan struct{})
	a.mu.Unlock()

	close(done)
	a.orderBus.Close()
	a.positionBus.Close()
	a.flows.CloseAll()
}

func (a *TradingAccount) periodicRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	a.mu.RLock()
	done := a.done
	a.mu.RUnlock()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			acc, err := a.adp.FetchAccount(ctx)
			if err != nil {
				a.logger.Errorw("trading_account: periodic refresh failed", "error", err)
				continue
			}
			a.applyAccountSnapshot(acc)
			balance := decimal.Zero
			if acc != nil {
				balance = acc.TotalBalance
			}
			a.logger.Debugw("trading_account: periodic refresh completed", "balance", balance)
		}
	}
}
