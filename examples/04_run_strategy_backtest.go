package examples

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/backtest"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/shopspring/decimal"
)

type BacktestStrategyResult struct {
	EventsProcessed int
	Order           model.OrderStatusReport
	Position        model.PositionStatusReport
	Fills           []model.FillReport
	ExposureUSDT    decimal.Decimal
}

// RunThresholdStrategyBacktest replays two order-book events through the same
// strategy.Runtime surface used by live nodes. The strategy subscribes in
// OnStart, reacts in OnOrderBook, submits through OrderFactory, and reads final
// state from cache and portfolio.
func RunThresholdStrategyBacktest(ctx context.Context) (BacktestStrategyResult, error) {
	accountID := model.AccountID("backtest-account")
	instrumentID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	c := cache.New()
	if err := c.PutInstrument(backtestSpotInstrument(instrumentID)); err != nil {
		return BacktestStrategyResult{}, err
	}

	strat := &thresholdBacktestStrategy{
		accountID:    accountID,
		instrumentID: instrumentID,
		entryPrice:   decimal.RequireFromString("101.00"),
	}
	runner := backtest.NewRunner(backtest.Config{
		Cache: c,
		Strategies: []strategy.Strategy{
			strategy.NewTyped("threshold-entry", strat),
		},
		Events: []backtest.Event{
			backtestBookEvent(instrumentID, 1, "99.00", "102.00"),
			backtestBookEvent(instrumentID, 2, "100.00", "101.00"),
		},
	})

	result, err := runner.Run(ctx)
	if err != nil {
		return BacktestStrategyResult{}, err
	}
	order, ok := result.Cache.OrderByClientID(accountID, "backtest-account-1")
	if !ok {
		return BacktestStrategyResult{}, fmt.Errorf("submitted order not found")
	}
	position, ok := result.Cache.PositionByInstrument(accountID, instrumentID)
	if !ok {
		return BacktestStrategyResult{}, fmt.Errorf("position not found")
	}
	return BacktestStrategyResult{
		EventsProcessed: result.EventsProcessed,
		Order:           order,
		Position:        position,
		Fills:           result.Cache.FillsForOrder(accountID, order.OrderID),
		ExposureUSDT:    result.Portfolio.Exposure(accountID, "USDT"),
	}, nil
}

type thresholdBacktestStrategy struct {
	mu           sync.Mutex
	runtime      strategy.Runtime
	accountID    model.AccountID
	instrumentID model.InstrumentID
	entryPrice   decimal.Decimal
	submitted    bool
}

func (s *thresholdBacktestStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *thresholdBacktestStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if book.InstrumentID != s.instrumentID || len(book.Asks) == 0 || book.Asks[0].Price.GreaterThan(s.entryPrice) {
		return nil
	}
	s.mu.Lock()
	if s.submitted {
		s.mu.Unlock()
		return nil
	}
	s.submitted = true
	s.mu.Unlock()

	order := s.runtime.OrderFactory(s.accountID).Limit(
		book.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("0.01"),
		book.Asks[0].Price,
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func backtestSpotInstrument(instrumentID model.InstrumentID) model.Instrument {
	return model.Instrument{
		ID:        instrumentID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}
}

func backtestBookEvent(instrumentID model.InstrumentID, second int64, bid string, ask string) backtest.Event {
	at := time.Unix(second, 0)
	return backtest.Event{
		At:    at,
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{OrderBook: &model.OrderBook{
			InstrumentID: instrumentID,
			Bids: []model.OrderBookLevel{{
				Price: decimal.RequireFromString(bid),
				Size:  decimal.RequireFromString("2"),
			}},
			Asks: []model.OrderBookLevel{{
				Price: decimal.RequireFromString(ask),
				Size:  decimal.RequireFromString("1"),
			}},
			Timestamp: at,
		}},
	}
}
