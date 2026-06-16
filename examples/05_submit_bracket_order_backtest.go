package examples

import (
	"context"
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/backtest"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/shopspring/decimal"
)

type BracketBacktestResult struct {
	Entry      model.OrderStatusReport
	StopLoss   model.OrderStatusReport
	TakeProfit model.OrderStatusReport
	Fills      []model.FillReport
}

// RunBracketOrderBacktest shows the advanced order-list path: one parent entry
// order releases two reduce-only children, and the take-profit fill cancels the
// stop-loss sibling.
func RunBracketOrderBacktest(ctx context.Context) (BracketBacktestResult, error) {
	accountID := model.AccountID("bracket-account")
	instrumentID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	c := cache.New()
	if err := c.PutInstrument(backtestSpotInstrument(instrumentID)); err != nil {
		return BracketBacktestResult{}, err
	}

	strat := &bracketBacktestStrategy{
		accountID:    accountID,
		instrumentID: instrumentID,
	}
	runner := backtest.NewRunner(backtest.Config{
		Cache: c,
		Strategies: []strategy.Strategy{
			strategy.NewTyped("bracket-entry", strat),
		},
		Events: []backtest.Event{
			backtestBookEvent(instrumentID, 10, "100.00", "101.00"),
			backtestBookEvent(instrumentID, 11, "110.00", "111.00"),
		},
	})
	result, err := runner.Run(ctx)
	if err != nil {
		return BracketBacktestResult{}, err
	}

	entry, ok := result.Cache.OrderByClientID(accountID, "bracket-account-1")
	if !ok {
		return BracketBacktestResult{}, fmt.Errorf("entry order not found")
	}
	stopLoss, ok := result.Cache.OrderByClientID(accountID, "bracket-account-2")
	if !ok {
		return BracketBacktestResult{}, fmt.Errorf("stop-loss order not found")
	}
	takeProfit, ok := result.Cache.OrderByClientID(accountID, "bracket-account-3")
	if !ok {
		return BracketBacktestResult{}, fmt.Errorf("take-profit order not found")
	}
	return BracketBacktestResult{
		Entry:      entry,
		StopLoss:   stopLoss,
		TakeProfit: takeProfit,
		Fills:      append(result.Cache.FillsForOrder(accountID, entry.OrderID), result.Cache.FillsForOrder(accountID, takeProfit.OrderID)...),
	}, nil
}

type bracketBacktestStrategy struct {
	mu           sync.Mutex
	runtime      strategy.Runtime
	accountID    model.AccountID
	instrumentID model.InstrumentID
	submitted    bool
}

func (s *bracketBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *bracketBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if book.InstrumentID != s.instrumentID || len(book.Asks) == 0 || book.Asks[0].Price.GreaterThan(decimal.RequireFromString("101.00")) {
		return nil
	}
	s.mu.Lock()
	if s.submitted {
		s.mu.Unlock()
		return nil
	}
	s.submitted = true
	s.mu.Unlock()

	list := s.runtime.OrderFactory(s.accountID).Bracket(model.BracketOrderRequest{
		InstrumentID: s.instrumentID,
		Side:         model.OrderSideBuy,
		Quantity:     decimal.RequireFromString("0.01"),
		EntryPrice:   decimal.RequireFromString("101.00"),
		TakeProfit:   decimal.RequireFromString("110.00"),
		StopLoss:     decimal.RequireFromString("99.00"),
	})
	_, err := s.runtime.SubmitOrderList(ctx, list)
	return err
}
