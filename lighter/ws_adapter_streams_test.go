package lighter

import (
	"context"
	"sync"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestPerpWatchOrderBookEmitsOnAppliedUpdateNotPolling(t *testing.T) {
	ws := newStubLighterMarketWS()
	adp := newTestPerpAdapterForStreams()

	var mu sync.Mutex
	var callbacks []*exchanges.OrderBook

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- adp.watchOrderBookWithWS(ctx, ws, "BTC", 5, func(ob *exchanges.OrderBook) {
			mu.Lock()
			defer mu.Unlock()
			callbacks = append(callbacks, ob)
		})
	}()

	require.Eventually(t, func() bool { return ws.subscribeCount(0) == 1 }, time.Second, 10*time.Millisecond)

	ws.emitOrderBook(0, `{
		"type":"subscribed/order_book",
		"order_book":{
			"nonce":10,
			"bids":[{"price":"100","size":"2"}],
			"asks":[{"price":"101","size":"3"}]
		}
	}`)

	require.NoError(t, <-done)

	ws.emitOrderBook(0, `{
		"type":"update/order_book",
		"order_book":{
			"begin_nonce":10,
			"nonce":11,
			"bids":[{"price":"100","size":"1"}]
		}
	}`)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(callbacks) == 2
	}, time.Second, 10*time.Millisecond)
}

func TestPerpWatchOrderBookGapTriggersResubscribe(t *testing.T) {
	ws := newStubLighterMarketWS()
	adp := newTestPerpAdapterForStreams()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- adp.watchOrderBookWithWS(ctx, ws, "BTC", 5, nil)
	}()

	require.Eventually(t, func() bool { return ws.subscribeCount(0) == 1 }, time.Second, 10*time.Millisecond)

	ws.emitOrderBook(0, `{
		"type":"subscribed/order_book",
		"order_book":{
			"nonce":10,
			"bids":[{"price":"100","size":"2"}],
			"asks":[{"price":"101","size":"3"}]
		}
	}`)

	require.NoError(t, <-done)

	ws.emitOrderBook(0, `{
		"type":"update/order_book",
		"order_book":{
			"begin_nonce":12,
			"nonce":13,
			"bids":[{"price":"100","size":"1"}]
		}
	}`)

	require.Eventually(t, func() bool {
		return ws.unsubscribeCount(0) == 1 && ws.subscribeCount(0) == 2
	}, time.Second, 10*time.Millisecond)
}

func TestPerpWatchTickerCombinesTickerAndMarketStats(t *testing.T) {
	ws := newStubLighterMarketWS()
	adp := newTestPerpAdapterForStreams()

	var mu sync.Mutex
	var got *exchanges.Ticker

	require.NoError(t, adp.watchTickerWithWS(context.Background(), ws, "BTC", func(tk *exchanges.Ticker) {
		mu.Lock()
		defer mu.Unlock()
		got = tk
	}))

	ws.emitTicker(0, `{
		"channel":"ticker:0",
		"type":"update/ticker",
		"timestamp":1700000000000,
		"ticker":{
			"b":{"price":"2000","size":"1"},
			"a":{"price":"2001","size":"2"}
		}
	}`)
	ws.emitMarketStats(0, `{
		"channel":"market_stats:0",
		"type":"update/market_stats",
		"timestamp":1700000000001,
		"market_stats":{
			"last_trade_price":"1999",
			"daily_base_token_volume":1000,
			"daily_price_high":2100,
			"daily_price_low":1900,
			"index_price":"1998",
			"mark_price":"2002"
		}
	}`)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got != nil &&
			got.Bid.String() == "2000" &&
			got.Ask.String() == "2001" &&
			got.LastPrice.String() == "1999" &&
			got.IndexPrice.String() == "1998" &&
			got.MarkPrice.String() == "2002"
	}, time.Second, 10*time.Millisecond)
}

func TestSpotWatchTickerUsesSpotMarketStats(t *testing.T) {
	ws := newStubLighterMarketWS()
	adp := newTestSpotAdapterForStreams()

	var mu sync.Mutex
	var got *exchanges.Ticker

	require.NoError(t, adp.watchTickerWithWS(context.Background(), ws, "ETH", func(tk *exchanges.Ticker) {
		mu.Lock()
		defer mu.Unlock()
		got = tk
	}))

	ws.emitSpotMarketStats(1, `{
		"channel":"spot_market_stats:1",
		"type":"update/spot_market_stats",
		"timestamp":1700000000002,
		"spot_market_stats":{
			"1":{
				"market_id":1,
				"mid_price":"2500.5",
				"last_trade_price":"2500",
				"daily_base_token_volume":12.5,
				"daily_quote_token_volume":31250,
				"daily_price_high":2600,
				"daily_price_low":2400,
				"daily_price_change":1.2
			}
		}
	}`)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got != nil &&
			got.LastPrice.String() == "2500" &&
			got.MidPrice.String() == "2500.5" &&
			got.Volume24h.String() == "12.5"
	}, time.Second, 10*time.Millisecond)
}

func TestSpotWatchTickerUsesSpecificSpotMarketStatsShape(t *testing.T) {
	ws := newStubLighterMarketWS()
	adp := newTestSpotAdapterForStreams()

	var mu sync.Mutex
	var got *exchanges.Ticker

	require.NoError(t, adp.watchTickerWithWS(context.Background(), ws, "ETH", func(tk *exchanges.Ticker) {
		mu.Lock()
		defer mu.Unlock()
		got = tk
	}))

	ws.emitSpotMarketStats(1, `{
		"channel":"spot_market_stats:1",
		"type":"update/spot_market_stats",
		"timestamp":1700000000002,
		"spot_market_stats":{
			"market_id":1,
			"mid_price":"2500.5",
			"last_trade_price":"2500",
			"daily_base_token_volume":12.5,
			"daily_quote_token_volume":31250,
			"daily_price_high":2600,
			"daily_price_low":2400,
			"daily_price_change":1.2
		}
	}`)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got != nil &&
			got.LastPrice.String() == "2500" &&
			got.MidPrice.String() == "2500.5" &&
			got.Volume24h.String() == "12.5"
	}, time.Second, 10*time.Millisecond)
}

func TestPerpWatchTradesEmitsEachTradeFromBatch(t *testing.T) {
	ws := newStubLighterMarketWS()
	adp := newTestPerpAdapterForStreams()

	var mu sync.Mutex
	var got []*exchanges.Trade

	require.NoError(t, adp.watchTradesWithWS(context.Background(), ws, "BTC", func(tr *exchanges.Trade) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, tr)
	}))

	ws.emitTrades(0, `{
		"channel":"trade:0",
		"type":"update/trade",
		"nonce":18,
		"trades":[
			{"trade_id":11,"trade_id_str":"11","price":"2000","size":"0.5","timestamp":1700000000123,"is_maker_ask":false}
		],
		"liquidation_trades":[
			{"trade_id":12,"trade_id_str":"12","price":"1995","size":"1.2","timestamp":1700000000456,"is_maker_ask":true}
		]
	}`)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) == 2 &&
			got[0].ID == "11" &&
			got[0].Timestamp == 1700000000123 &&
			got[0].Side == exchanges.TradeSideBuy &&
			got[1].ID == "12" &&
			got[1].Timestamp == 1700000000456 &&
			got[1].Side == exchanges.TradeSideSell
	}, time.Second, 10*time.Millisecond)
}

func TestSpotWatchTradesEmitsEachTradeFromBatch(t *testing.T) {
	ws := newStubLighterMarketWS()
	adp := newTestSpotAdapterForStreams()

	var mu sync.Mutex
	var got []*exchanges.Trade

	require.NoError(t, adp.watchTradesWithWS(context.Background(), ws, "ETH", func(tr *exchanges.Trade) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, tr)
	}))

	ws.emitTrades(1, `{
		"channel":"trade:1",
		"type":"update/trade",
		"trades":[
			{"trade_id":21,"trade_id_str":"21","price":"2500","size":"0.25","timestamp":1700000000789,"is_maker_ask":false}
		]
	}`)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) == 1 &&
			got[0].ID == "21" &&
			got[0].Timestamp == 1700000000789 &&
			got[0].Symbol == "ETH"
	}, time.Second, 10*time.Millisecond)
}

type stubLighterMarketWS struct {
	mu                  sync.Mutex
	orderBookHandlers   map[int]func([]byte)
	tickerHandlers      map[int]func([]byte)
	marketStatsHandlers map[int]func([]byte)
	spotStatsHandlers   map[int]func([]byte)
	tradeHandlers       map[int]func([]byte)
	orderBookSubCount   map[int]int
	orderBookUnsubCount map[int]int
}

func newStubLighterMarketWS() *stubLighterMarketWS {
	return &stubLighterMarketWS{
		orderBookHandlers:   make(map[int]func([]byte)),
		tickerHandlers:      make(map[int]func([]byte)),
		marketStatsHandlers: make(map[int]func([]byte)),
		spotStatsHandlers:   make(map[int]func([]byte)),
		tradeHandlers:       make(map[int]func([]byte)),
		orderBookSubCount:   make(map[int]int),
		orderBookUnsubCount: make(map[int]int),
	}
}

func (s *stubLighterMarketWS) SubscribeOrderBook(marketID int, cb func([]byte)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orderBookHandlers[marketID] = cb
	s.orderBookSubCount[marketID]++
	return nil
}

func (s *stubLighterMarketWS) UnsubscribeOrderBook(marketID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.orderBookHandlers, marketID)
	s.orderBookUnsubCount[marketID]++
	return nil
}

func (s *stubLighterMarketWS) SubscribeTicker(marketID int, cb func([]byte)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickerHandlers[marketID] = cb
	return nil
}

func (s *stubLighterMarketWS) SubscribeMarketStats(marketID int, cb func([]byte)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marketStatsHandlers[marketID] = cb
	return nil
}

func (s *stubLighterMarketWS) SubscribeSpotMarketStats(marketID int, cb func([]byte)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spotStatsHandlers[marketID] = cb
	return nil
}

func (s *stubLighterMarketWS) SubscribeTrades(marketID int, cb func([]byte)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tradeHandlers[marketID] = cb
	return nil
}

func (s *stubLighterMarketWS) emitOrderBook(marketID int, payload string) {
	s.mu.Lock()
	handler := s.orderBookHandlers[marketID]
	s.mu.Unlock()
	if handler != nil {
		handler([]byte(payload))
	}
}

func (s *stubLighterMarketWS) emitTicker(marketID int, payload string) {
	s.mu.Lock()
	handler := s.tickerHandlers[marketID]
	s.mu.Unlock()
	if handler != nil {
		handler([]byte(payload))
	}
}

func (s *stubLighterMarketWS) emitMarketStats(marketID int, payload string) {
	s.mu.Lock()
	handler := s.marketStatsHandlers[marketID]
	s.mu.Unlock()
	if handler != nil {
		handler([]byte(payload))
	}
}

func (s *stubLighterMarketWS) emitSpotMarketStats(marketID int, payload string) {
	s.mu.Lock()
	handler := s.spotStatsHandlers[marketID]
	s.mu.Unlock()
	if handler != nil {
		handler([]byte(payload))
	}
}

func (s *stubLighterMarketWS) emitTrades(marketID int, payload string) {
	s.mu.Lock()
	handler := s.tradeHandlers[marketID]
	s.mu.Unlock()
	if handler != nil {
		handler([]byte(payload))
	}
}

func (s *stubLighterMarketWS) subscribeCount(marketID int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.orderBookSubCount[marketID]
}

func (s *stubLighterMarketWS) unsubscribeCount(marketID int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.orderBookUnsubCount[marketID]
}

func newTestPerpAdapterForStreams() *Adapter {
	return &Adapter{
		BaseAdapter: exchanges.NewBaseAdapter("LIGHTER", exchanges.MarketTypePerp, exchanges.NopLogger),
		symbolToID:  map[string]int{"BTC": 0},
		cancels:     make(map[string]context.CancelFunc),
	}
}

func newTestSpotAdapterForStreams() *SpotAdapter {
	return &SpotAdapter{
		BaseAdapter: exchanges.NewBaseAdapter("LIGHTER", exchanges.MarketTypeSpot, exchanges.NopLogger),
		symbolToID:  map[string]int{"ETH": 1},
		cancels:     make(map[string]context.CancelFunc),
	}
}
