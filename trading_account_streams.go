package exchanges

import (
	"context"
	"fmt"
	"time"

	"errors"
	"github.com/shopspring/decimal"
)

func (a *TradingAccount) Start(ctx context.Context) (err error) {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	if a.started {
		return nil
	}

	runCtx, runCancel := context.WithCancel(ctx)

	defer func() {
		if err == nil {
			return
		}
		runCancel()
		a.resetSnapshotState()
	}()

	a.logger.Infow("trading_account: fetching initial account state")
	acc, err := a.adp.FetchAccount(runCtx)
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

	if err := streamable.WatchOrders(runCtx, a.applyOrderUpdate); err != nil {
		return fmt.Errorf("trading_account: WatchOrders failed: %w", err)
	}

	if watchErr := streamable.WatchPositions(runCtx, a.applyPositionUpdate); watchErr != nil {
		if !errors.Is(watchErr, ErrNotSupported) {
			return fmt.Errorf("trading_account: WatchPositions failed: %w", watchErr)
		}
		a.logger.Warnw("trading_account: WatchPositions failed (may not be supported)", "error", watchErr)
	}

	a.runCancel = runCancel
	a.started = true

	go a.periodicRefresh(runCtx, time.Minute)

	return nil
}

func (a *TradingAccount) Close() {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	if !a.started {
		return
	}
	a.started = false
	runCancel := a.runCancel
	a.runCancel = nil

	if runCancel != nil {
		runCancel()
	}
	a.orderBus.Close()
	a.positionBus.Close()
	a.flows.CloseAll()
}

func (a *TradingAccount) periodicRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
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
