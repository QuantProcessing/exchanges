package exchanges

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

func (a *TradingAccount) Start(ctx context.Context) (err error) {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	a.runMu.RLock()
	alreadyStarted := a.started
	a.runMu.RUnlock()
	if alreadyStarted {
		return nil
	}

	runCtx, runCancel := context.WithCancel(ctx)
	runGen := a.beginRun(runCancel)

	defer func() {
		if err == nil {
			return
		}
		if cancel := a.failRunStart(runGen); cancel != nil {
			cancel()
		} else {
			runCancel()
		}
		a.resetSnapshotState()
	}()

	a.logger.Infow("trading_account: fetching initial account state")
	acc, err := a.adp.FetchAccount(runCtx)
	if err != nil {
		return fmt.Errorf("trading_account: failed to get initial state: %w", err)
	}
	a.applyAccountSnapshot(runGen, acc)
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

	if err := streamable.WatchOrders(runCtx, func(order *Order) {
		a.applyOrderUpdate(runGen, order)
	}); err != nil {
		return fmt.Errorf("trading_account: WatchOrders failed: %w", err)
	}

	if watchErr := streamable.WatchPositions(runCtx, func(position *Position) {
		a.applyPositionUpdate(runGen, position)
	}); watchErr != nil {
		if !errors.Is(watchErr, ErrNotSupported) {
			return fmt.Errorf("trading_account: WatchPositions failed: %w", watchErr)
		}
		a.logger.Warnw("trading_account: WatchPositions failed (may not be supported)", "error", watchErr)
	}

	if runErr := runCtx.Err(); runErr != nil {
		return runErr
	}
	if !a.finishRunStart(runGen) {
		return context.Canceled
	}

	go a.periodicRefresh(runCtx, time.Minute, runGen)

	return nil
}

func (a *TradingAccount) Close() {
	runCancel := a.closeRun()
	if runCancel != nil {
		runCancel()
	}

	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	a.resetSnapshotState()
	a.orderBus.Close()
	a.positionBus.Close()
	a.flows.CloseAll()
}

func (a *TradingAccount) periodicRefresh(ctx context.Context, interval time.Duration, runGen uint64) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			acc, err := a.adp.FetchAccount(ctx)
			if err != nil {
				if ctx.Err() != nil || !a.isActiveRun(runGen) {
					return
				}
				a.logger.Errorw("trading_account: periodic refresh failed", "error", err)
				continue
			}
			a.applyAccountSnapshot(runGen, acc)
			if !a.isActiveRun(runGen) {
				return
			}
			balance := decimal.Zero
			if acc != nil {
				balance = acc.TotalBalance
			}
			a.logger.Debugw("trading_account: periodic refresh completed", "balance", balance)
		}
	}
}

func (a *TradingAccount) beginRun(runCancel context.CancelFunc) uint64 {
	a.runMu.Lock()
	defer a.runMu.Unlock()

	a.runGen++
	a.runCancel = runCancel
	a.started = false
	a.starting = true
	return a.runGen
}

func (a *TradingAccount) failRunStart(runGen uint64) context.CancelFunc {
	a.runMu.Lock()
	defer a.runMu.Unlock()

	if a.runGen != runGen {
		return nil
	}
	runCancel := a.runCancel
	a.started = false
	a.starting = false
	a.runCancel = nil
	return runCancel
}

func (a *TradingAccount) finishRunStart(runGen uint64) bool {
	a.runMu.Lock()
	defer a.runMu.Unlock()

	if a.runGen != runGen || a.runCancel == nil || !a.starting {
		return false
	}
	a.starting = false
	a.started = true
	return true
}

func (a *TradingAccount) currentRunCancel() (context.CancelFunc, bool) {
	a.runMu.RLock()
	defer a.runMu.RUnlock()

	return a.runCancel, a.started || a.starting || a.runCancel != nil
}

func (a *TradingAccount) closeRun() context.CancelFunc {
	a.runMu.Lock()
	defer a.runMu.Unlock()

	runCancel := a.runCancel
	a.runGen++
	a.started = false
	a.starting = false
	a.runCancel = nil
	return runCancel
}

func (a *TradingAccount) isActiveRun(runGen uint64) bool {
	a.runMu.RLock()
	defer a.runMu.RUnlock()

	return a.runGen == runGen && a.runCancel != nil && (a.started || a.starting)
}
