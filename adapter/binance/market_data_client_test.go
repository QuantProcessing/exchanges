package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestMarketDataClientFetchTickerRejectsUnloadedInstrument(t *testing.T) {
	provider := newInstrumentProviderForTest(nil)
	client := newMarketDataClient(provider, nil, nil)

	_, err := client.FetchTicker(context.Background(), model.MustInstrumentID("BTC-USDT-PERP.BINANCE"))
	require.ErrorIs(t, err, model.ErrInstrumentNotLoaded)
}

func TestMarketDataClientFetchTickerRoutesSpot(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintSpot, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	spotClient := &fakeSpotMarketData{}
	client := newMarketDataClient(provider, spotClient, nil)

	got, err := client.FetchTicker(context.Background(), model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"))
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", spotClient.bookTickerSymbol)
	require.Equal(t, model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"), got.InstrumentID)
	require.Equal(t, "100.1", got.Bid.String())
	require.Equal(t, "100.2", got.Ask.String())
	require.Equal(t, "100.15", got.Last.String())
}

func TestMarketDataClientFetchOrderBookRoutesPerp(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	perpClient := &fakePerpMarketData{}
	client := newMarketDataClient(provider, nil, perpClient)

	got, err := client.FetchOrderBook(context.Background(), model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), 5)
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", perpClient.depthSymbol)
	require.Equal(t, 5, perpClient.depthLimit)
	require.Len(t, got.Bids, 1)
	require.Equal(t, "99.9", got.Bids[0].Price.String())
	require.Len(t, got.Asks, 1)
	require.Equal(t, "100.3", got.Asks[0].Price.String())
}

func TestMarketDataClientSubscribeTickerRoutesSpotStream(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintSpot, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	stream := &fakeSpotMarketStream{}
	client := newMarketDataClient(provider, &fakeSpotMarketData{}, nil)
	client.spotStream = stream

	var got model.Ticker
	sub, err := client.SubscribeTicker(context.Background(), model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"), func(tick model.Ticker) {
		got = tick
	})
	require.NoError(t, err)
	require.True(t, stream.connected)
	require.Equal(t, "btcusdt", stream.bookTickerSymbol)

	require.NoError(t, stream.bookTickerHandler(&spot.BookTickerEvent{
		Symbol:       "BTCUSDT",
		BestBidPrice: "100.1",
		BestAskPrice: "100.2",
	}))
	require.Equal(t, model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"), got.InstrumentID)
	require.Equal(t, "100.1", got.Bid.String())
	require.Equal(t, "100.2", got.Ask.String())

	require.NoError(t, sub.Close())
	require.Equal(t, "btcusdt", stream.unsubBookTickerSymbol)
}

func TestMarketDataClientSubscribeOrderBookRoutesSpotPartialDepth(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "ETHUSDT", Product: venue.ProductHintSpot, Base: model.ETH, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	stream := &fakeSpotMarketStream{}
	client := newMarketDataClient(provider, &fakeSpotMarketData{}, nil)
	client.spotStream = stream

	var got model.OrderBook
	sub, err := client.SubscribeOrderBook(context.Background(), model.MustInstrumentID("ETH-USDT-SPOT.BINANCE"), 10, func(book model.OrderBook) {
		got = book
	})
	require.NoError(t, err)
	require.True(t, stream.connected)
	require.Equal(t, "ethusdt", stream.limitOrderBookSymbol)
	require.Equal(t, 10, stream.limitOrderBookLevels)
	require.Equal(t, "100ms", stream.limitOrderBookInterval)

	require.NoError(t, stream.limitOrderBookHandler(&spot.DepthEvent{
		Symbol: "ETHUSDT",
		Bids:   [][]string{{"2000.1", "1.5"}},
		Asks:   [][]string{{"2000.2", "2.5"}},
	}))
	require.Equal(t, model.MustInstrumentID("ETH-USDT-SPOT.BINANCE"), got.InstrumentID)
	require.Len(t, got.Bids, 1)
	require.Equal(t, "2000.1", got.Bids[0].Price.String())
	require.Len(t, got.Asks, 1)
	require.Equal(t, "2000.2", got.Asks[0].Price.String())

	require.NoError(t, sub.Close())
	require.Equal(t, "ethusdt", stream.unsubLimitOrderBookSymbol)
	require.Equal(t, 10, stream.unsubLimitOrderBookLevels)
	require.Equal(t, "100ms", stream.unsubLimitOrderBookInterval)
}

func TestMarketDataClientSubscribeOrderBookSpotDiffDepthBuffersThenReplaysSnapshot(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "ETHUSDT", Product: venue.ProductHintSpot, Base: model.ETH, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	stream := &fakeSpotMarketStream{
		incrementOrderBookOnSubscribe: &spot.WsDepthEvent{
			FirstUpdateID: 101,
			FinalUpdateID: 101,
			Bids:          [][]string{{"2000.1", "2.5"}},
			Asks:          [][]string{{"2000.2", "0"}},
		},
	}
	spotClient := &fakeSpotMarketData{
		depthResponse: &spot.DepthResponse{
			LastUpdateID: 100,
			Bids:         [][]string{{"2000.1", "1.5"}},
			Asks:         [][]string{{"2000.2", "2.5"}},
		},
	}
	client := newMarketDataClient(provider, spotClient, nil)
	client.spotStream = stream

	var got []model.OrderBook
	sub, err := client.SubscribeOrderBook(context.Background(), model.MustInstrumentID("ETH-USDT-SPOT.BINANCE"), 0, func(book model.OrderBook) {
		got = append(got, book)
	})
	require.NoError(t, err)
	require.True(t, stream.connected)
	require.Equal(t, "ethusdt", stream.incrementOrderBookSymbol)
	require.Equal(t, "100ms", stream.incrementOrderBookInterval)
	require.Equal(t, "ETHUSDT", spotClient.depthSymbol)
	require.Equal(t, 1000, spotClient.depthLimit)
	require.Len(t, got, 2)
	require.Equal(t, "1.5", got[0].Bids[0].Size.String())
	require.Len(t, got[0].Asks, 1)
	require.Equal(t, "2.5", got[1].Bids[0].Size.String())
	require.Empty(t, got[1].Asks)

	require.NoError(t, sub.Close())
	require.Equal(t, "ethusdt", stream.unsubIncrementOrderBookSymbol)
	require.Equal(t, "100ms", stream.unsubIncrementOrderBookInterval)
}

func TestMarketDataClientSubscribeOrderBookSpotDiffDepthRebuildsAfterReconnect(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "ETHUSDT", Product: venue.ProductHintSpot, Base: model.ETH, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	stream := &fakeSpotMarketStream{}
	spotClient := &fakeSpotMarketData{
		depthResponse: &spot.DepthResponse{
			LastUpdateID: 100,
			Bids:         [][]string{{"2000.1", "1.5"}},
			Asks:         [][]string{{"2000.2", "2.5"}},
		},
	}
	client := newMarketDataClient(provider, spotClient, nil)
	client.spotStream = stream

	var got []model.OrderBook
	sub, err := client.SubscribeOrderBook(context.Background(), model.MustInstrumentID("ETH-USDT-SPOT.BINANCE"), 0, func(book model.OrderBook) {
		got = append(got, book)
	})
	require.NoError(t, err)
	require.NotNil(t, stream.postReconnect)
	require.Len(t, got, 1)
	require.Equal(t, "1.5", got[0].Bids[0].Size.String())

	spotClient.depthResponse = &spot.DepthResponse{
		LastUpdateID: 200,
		Bids:         [][]string{{"2000.1", "3.0"}},
		Asks:         [][]string{{"2000.2", "4.0"}},
	}
	stream.postReconnect()
	require.Len(t, got, 2)
	require.Equal(t, "3", got[1].Bids[0].Size.String())
	require.Equal(t, "4", got[1].Asks[0].Size.String())

	require.NoError(t, sub.Close())
	stream.postReconnect()
	require.Len(t, got, 2)
}

func TestMarketDataClientSubscribeOrderBookPerpDiffDepthAppliesUpdates(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	stream := &fakePerpMarketStream{}
	perpClient := &fakePerpMarketData{
		depthResponse: &perp.DepthResponse{
			LastUpdateID: 200,
			Bids:         [][]string{{"100.1", "1.0"}},
			Asks:         [][]string{{"100.2", "1.0"}},
		},
	}
	client := newMarketDataClient(provider, nil, perpClient)
	client.perpStream = stream

	var got []model.OrderBook
	_, err := client.SubscribeOrderBook(context.Background(), model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), 100, func(book model.OrderBook) {
		got = append(got, book)
	})
	require.NoError(t, err)
	require.Equal(t, "btcusdt", stream.incrementOrderBookSymbol)
	require.Equal(t, "250ms", stream.incrementOrderBookInterval)
	require.Equal(t, "BTCUSDT", perpClient.depthSymbol)
	require.Equal(t, 100, perpClient.depthLimit)
	require.Len(t, got, 1)

	require.NoError(t, stream.incrementOrderBookHandler(&perp.WsDepthEvent{
		FirstUpdateID:     201,
		FinalUpdateID:     201,
		FinalUpdateIDLast: 200,
		TransactionTime:   1700000000000,
		Bids:              [][]interface{}{{"100.1", "1.7"}},
		Asks:              [][]interface{}{{"100.2", "0"}},
	}))
	require.Len(t, got, 2)
	require.Equal(t, "1.7", got[1].Bids[0].Size.String())
	require.Empty(t, got[1].Asks)
}

func TestMarketDataClientSubscribeTradesRoutesPerpAggTrades(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	stream := &fakePerpMarketStream{}
	client := newMarketDataClient(provider, nil, &fakePerpMarketData{})
	client.perpStream = stream

	var got model.Trade
	sub, err := client.SubscribeTrades(context.Background(), model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), func(trade model.Trade) {
		got = trade
	})
	require.NoError(t, err)
	require.True(t, stream.connected)
	require.Equal(t, "btcusdt", stream.aggTradeSymbol)

	require.NoError(t, stream.aggTradeHandler(&perp.WsAggTradeEvent{
		Symbol:       "BTCUSDT",
		AggTradeID:   123,
		Price:        "100.1",
		Quantity:     "0.7",
		TradeTime:    1700000000000,
		IsBuyerMaker: true,
	}))
	require.Equal(t, model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), got.InstrumentID)
	require.Equal(t, model.TradeID("123"), got.TradeID)
	require.Equal(t, model.OrderSideSell, got.Side)
	require.Equal(t, "100.1", got.Price.String())
	require.Equal(t, "0.7", got.Size.String())

	require.NoError(t, sub.Close())
	require.Equal(t, "btcusdt", stream.unsubAggTradeSymbol)
}

func TestMarketDataClientSubscribeBarsEmitsClosedSpotKlinesOnly(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "ETHUSDT", Product: venue.ProductHintSpot, Base: model.ETH, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	stream := &fakeSpotMarketStream{}
	client := newMarketDataClient(provider, &fakeSpotMarketData{}, nil)
	client.spotStream = stream

	var got []model.Bar
	spec := model.BarSpec{Interval: "1m"}
	sub, err := client.SubscribeBars(context.Background(), model.MustInstrumentID("ETH-USDT-SPOT.BINANCE"), spec, func(bar model.Bar) {
		got = append(got, bar)
	})
	require.NoError(t, err)
	require.True(t, stream.connected)
	require.Equal(t, "ethusdt", stream.klineSymbol)
	require.Equal(t, "1m", stream.klineInterval)

	require.NoError(t, stream.klineHandler(&spot.KlineEvent{Symbol: "ETHUSDT"}))
	require.Empty(t, got)

	closed := &spot.KlineEvent{Symbol: "ETHUSDT"}
	closed.Kline.IsClosed = true
	closed.Kline.OpenPrice = "10"
	closed.Kline.HighPrice = "12"
	closed.Kline.LowPrice = "9"
	closed.Kline.ClosePrice = "11"
	closed.Kline.Volume = "42"
	closed.Kline.CloseTime = 1700000000000
	require.NoError(t, stream.klineHandler(closed))
	require.Len(t, got, 1)
	require.Equal(t, "11", got[0].Close.String())
	require.Equal(t, "42", got[0].Volume.String())

	require.NoError(t, sub.Close())
	require.Equal(t, "ethusdt", stream.unsubKlineSymbol)
	require.Equal(t, "1m", stream.unsubKlineInterval)
}

type fakeSpotMarketData struct {
	bookTickerSymbol string
	tickerSymbol     string
	depthSymbol      string
	depthLimit       int
	depthResponse    *spot.DepthResponse
}

func (f *fakeSpotMarketData) BookTicker(ctx context.Context, symbol string) (*spot.BookTickerResponse, error) {
	f.bookTickerSymbol = symbol
	return &spot.BookTickerResponse{
		Symbol:   symbol,
		BidPrice: "100.1",
		BidQty:   "1",
		AskPrice: "100.2",
		AskQty:   "2",
	}, nil
}

func (f *fakeSpotMarketData) Ticker(ctx context.Context, symbol string) (*spot.TickerResponse, error) {
	f.tickerSymbol = symbol
	return &spot.TickerResponse{Symbol: symbol, LastPrice: "100.15"}, nil
}

func (f *fakeSpotMarketData) Depth(ctx context.Context, symbol string, limit int) (*spot.DepthResponse, error) {
	f.depthSymbol = symbol
	f.depthLimit = limit
	if f.depthResponse != nil {
		return f.depthResponse, nil
	}
	return &spot.DepthResponse{
		Bids: [][]string{{"99.9", "1"}},
		Asks: [][]string{{"100.3", "2"}},
	}, nil
}

type fakePerpMarketData struct {
	tickerSymbol  string
	depthSymbol   string
	depthLimit    int
	depthResponse *perp.DepthResponse
}

func (f *fakePerpMarketData) Ticker(ctx context.Context, symbol string) (*perp.TickerResponse, error) {
	f.tickerSymbol = symbol
	return &perp.TickerResponse{Symbol: symbol, LastPrice: "100.15"}, nil
}

func (f *fakePerpMarketData) Depth(ctx context.Context, symbol string, limit int) (*perp.DepthResponse, error) {
	f.depthSymbol = symbol
	f.depthLimit = limit
	if f.depthResponse != nil {
		return f.depthResponse, nil
	}
	return &perp.DepthResponse{
		Bids: [][]string{{"99.9", "1"}},
		Asks: [][]string{{"100.3", "2"}},
	}, nil
}

type fakeSpotMarketStream struct {
	connected bool
	closed    bool

	bookTickerSymbol      string
	unsubBookTickerSymbol string
	bookTickerHandler     func(*spot.BookTickerEvent) error

	limitOrderBookSymbol            string
	limitOrderBookLevels            int
	limitOrderBookInterval          string
	unsubLimitOrderBookSymbol       string
	unsubLimitOrderBookLevels       int
	unsubLimitOrderBookInterval     string
	limitOrderBookHandler           func(*spot.DepthEvent) error
	incrementOrderBookSymbol        string
	incrementOrderBookInterval      string
	unsubIncrementOrderBookSymbol   string
	unsubIncrementOrderBookInterval string
	incrementOrderBookHandler       func(*spot.WsDepthEvent) error
	incrementOrderBookOnSubscribe   *spot.WsDepthEvent
	tradeSymbol                     string
	unsubTradeSymbol                string
	tradeHandler                    func(*spot.TradeEvent) error
	klineSymbol                     string
	klineInterval                   string
	unsubKlineSymbol                string
	unsubKlineInterval              string
	klineHandler                    func(*spot.KlineEvent) error
	postReconnect                   func()
}

func (f *fakeSpotMarketStream) Connect() error {
	f.connected = true
	return nil
}

func (f *fakeSpotMarketStream) Close() { f.closed = true }

func (f *fakeSpotMarketStream) SetPostReconnect(h func()) {
	f.postReconnect = h
}

func (f *fakeSpotMarketStream) SubscribeBookTicker(symbol string, h func(*spot.BookTickerEvent) error) error {
	f.bookTickerSymbol = symbol
	f.bookTickerHandler = h
	return nil
}

func (f *fakeSpotMarketStream) UnsubscribeBookTicker(symbol string) error {
	f.unsubBookTickerSymbol = symbol
	return nil
}

func (f *fakeSpotMarketStream) SubscribeLimitOrderBook(symbol string, levels int, interval string, h func(*spot.DepthEvent) error) error {
	f.limitOrderBookSymbol = symbol
	f.limitOrderBookLevels = levels
	f.limitOrderBookInterval = interval
	f.limitOrderBookHandler = h
	return nil
}

func (f *fakeSpotMarketStream) UnsubscribeLimitOrderBook(symbol string, levels int, interval string) error {
	f.unsubLimitOrderBookSymbol = symbol
	f.unsubLimitOrderBookLevels = levels
	f.unsubLimitOrderBookInterval = interval
	return nil
}

func (f *fakeSpotMarketStream) SubscribeIncrementOrderBook(symbol string, interval string, h func(*spot.WsDepthEvent) error) error {
	f.incrementOrderBookSymbol = symbol
	f.incrementOrderBookInterval = interval
	f.incrementOrderBookHandler = h
	if f.incrementOrderBookOnSubscribe != nil {
		return h(f.incrementOrderBookOnSubscribe)
	}
	return nil
}

func (f *fakeSpotMarketStream) UnsubscribeIncrementOrderBook(symbol string, interval string) error {
	f.unsubIncrementOrderBookSymbol = symbol
	f.unsubIncrementOrderBookInterval = interval
	return nil
}

func (f *fakeSpotMarketStream) SubscribeTrade(symbol string, h func(*spot.TradeEvent) error) error {
	f.tradeSymbol = symbol
	f.tradeHandler = h
	return nil
}

func (f *fakeSpotMarketStream) UnsubscribeTrade(symbol string) error {
	f.unsubTradeSymbol = symbol
	return nil
}

func (f *fakeSpotMarketStream) SubscribeKline(symbol string, interval string, h func(*spot.KlineEvent) error) error {
	f.klineSymbol = symbol
	f.klineInterval = interval
	f.klineHandler = h
	return nil
}

func (f *fakeSpotMarketStream) UnsubscribeKline(symbol string, interval string) error {
	f.unsubKlineSymbol = symbol
	f.unsubKlineInterval = interval
	return nil
}

type fakePerpMarketStream struct {
	connected bool
	closed    bool

	bookTickerSymbol      string
	unsubBookTickerSymbol string
	bookTickerHandler     func(*perp.WsBookTickerEvent) error

	limitOrderBookSymbol            string
	limitOrderBookLevels            int
	limitOrderBookInterval          string
	unsubLimitOrderBookSymbol       string
	unsubLimitOrderBookLevels       int
	unsubLimitOrderBookInterval     string
	limitOrderBookHandler           func(*perp.WsDepthEvent) error
	incrementOrderBookSymbol        string
	incrementOrderBookInterval      string
	unsubIncrementOrderBookSymbol   string
	unsubIncrementOrderBookInterval string
	incrementOrderBookHandler       func(*perp.WsDepthEvent) error
	aggTradeSymbol                  string
	unsubAggTradeSymbol             string
	aggTradeHandler                 func(*perp.WsAggTradeEvent) error
	klineSymbol                     string
	klineInterval                   string
	unsubKlineSymbol                string
	unsubKlineInterval              string
	klineHandler                    func(*perp.WsKlineEvent) error
	postReconnect                   func()
}

func (f *fakePerpMarketStream) Connect() error {
	f.connected = true
	return nil
}

func (f *fakePerpMarketStream) Close() { f.closed = true }

func (f *fakePerpMarketStream) SetPostReconnect(h func()) {
	f.postReconnect = h
}

func (f *fakePerpMarketStream) SubscribeBookTicker(symbol string, h func(*perp.WsBookTickerEvent) error) error {
	f.bookTickerSymbol = symbol
	f.bookTickerHandler = h
	return nil
}

func (f *fakePerpMarketStream) UnsubscribeBookTicker(symbol string) error {
	f.unsubBookTickerSymbol = symbol
	return nil
}

func (f *fakePerpMarketStream) SubscribeLimitOrderBook(symbol string, levels int, interval string, h func(*perp.WsDepthEvent) error) error {
	f.limitOrderBookSymbol = symbol
	f.limitOrderBookLevels = levels
	f.limitOrderBookInterval = interval
	f.limitOrderBookHandler = h
	return nil
}

func (f *fakePerpMarketStream) UnsubscribeLimitOrderBook(symbol string, levels int, interval string) error {
	f.unsubLimitOrderBookSymbol = symbol
	f.unsubLimitOrderBookLevels = levels
	f.unsubLimitOrderBookInterval = interval
	return nil
}

func (f *fakePerpMarketStream) SubscribeIncrementOrderBook(symbol string, interval string, h func(*perp.WsDepthEvent) error) error {
	f.incrementOrderBookSymbol = symbol
	f.incrementOrderBookInterval = interval
	f.incrementOrderBookHandler = h
	return nil
}

func (f *fakePerpMarketStream) UnsubscribeIncrementOrderBook(symbol string, interval string) error {
	f.unsubIncrementOrderBookSymbol = symbol
	f.unsubIncrementOrderBookInterval = interval
	return nil
}

func (f *fakePerpMarketStream) SubscribeAggTrade(symbol string, h func(*perp.WsAggTradeEvent) error) error {
	f.aggTradeSymbol = symbol
	f.aggTradeHandler = h
	return nil
}

func (f *fakePerpMarketStream) UnsubscribeAggTrade(symbol string) error {
	f.unsubAggTradeSymbol = symbol
	return nil
}

func (f *fakePerpMarketStream) SubscribeKline(symbol string, interval string, h func(*perp.WsKlineEvent) error) error {
	f.klineSymbol = symbol
	f.klineInterval = interval
	f.klineHandler = h
	return nil
}

func (f *fakePerpMarketStream) UnsubscribeKline(symbol string, interval string) error {
	f.unsubKlineSymbol = symbol
	f.unsubKlineInterval = interval
	return nil
}
