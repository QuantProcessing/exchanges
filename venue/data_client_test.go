package venue

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
)

func TestDataClientContract(t *testing.T) {
	var _ DataClient = (*stubDataClient)(nil)
}

type stubDataClient struct{}

func (s *stubDataClient) Venue() model.Venue { return model.VenueBinance }

func (s *stubDataClient) ClientID() string { return "stub-data" }

func (s *stubDataClient) Instruments() InstrumentProvider { return nil }

func (s *stubDataClient) Connect(context.Context) error { return nil }

func (s *stubDataClient) Disconnect(context.Context) error { return nil }

func (s *stubDataClient) Health() DataHealth {
	return DataHealth{Connected: true, LastEventTime: time.Now()}
}

func (s *stubDataClient) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{}, nil
}

func (s *stubDataClient) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, nil
}

func (s *stubDataClient) FetchTrades(context.Context, model.InstrumentID, TradeQuery) ([]model.Trade, error) {
	return nil, nil
}

func (s *stubDataClient) FetchBars(context.Context, model.InstrumentID, model.BarSpec, BarQuery) ([]model.Bar, error) {
	return nil, nil
}

func (s *stubDataClient) SubscribeTicker(context.Context, model.InstrumentID, TickerHandler) (Subscription, error) {
	return nil, nil
}

func (s *stubDataClient) SubscribeOrderBook(context.Context, model.InstrumentID, int, OrderBookHandler) (Subscription, error) {
	return nil, nil
}

func (s *stubDataClient) SubscribeTrades(context.Context, model.InstrumentID, TradeHandler) (Subscription, error) {
	return nil, nil
}

func (s *stubDataClient) SubscribeBars(context.Context, model.InstrumentID, model.BarSpec, BarHandler) (Subscription, error) {
	return nil, nil
}
