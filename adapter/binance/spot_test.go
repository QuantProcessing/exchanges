package binance

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSpotClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	data := newSpotDataClient("binance-spot-data", provider, sdk)
	data.ws = &fakeSpotMarketStream{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newSpotExecutionClient("acct", provider, sdk),
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		Capabilities: (&SpotAdapter{}).Capabilities(),
	})
}

func TestSpotProviderLoadsExchangeInfo(t *testing.T) {
	provider := newSpotProvider(&fakeSpotSDK{})
	require.NoError(t, provider.LoadAll(context.Background()))

	inst, ok := provider.Get(model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"))
	require.True(t, ok)
	require.Equal(t, "BTCUSDT", inst.RawSymbol)
	require.Equal(t, model.InstrumentTypeSpot, inst.Type)
	require.Equal(t, decimal.RequireFromString("0.01"), inst.PriceTick)
	require.Equal(t, decimal.RequireFromString("0.0001"), inst.SizeTick)
}

func TestSpotDataClientMapsTickerAndBook(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newSpotDataClient("binance-spot-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")

	ticker, err := client.FetchTicker(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, ticker.InstrumentID)
	require.Equal(t, decimal.RequireFromString("100"), ticker.Last)

	book, err := client.FetchOrderBook(context.Background(), id, 5)
	require.NoError(t, err)
	require.Equal(t, id, book.InstrumentID)
	require.Equal(t, decimal.RequireFromString("99"), book.Bids[0].Price)
	require.Equal(t, decimal.RequireFromString("101"), book.Asks[0].Price)
}

func TestSpotDataClientRestTickerUsesVenueTimestamp(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newSpotDataClient("binance-spot-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")

	ticker, err := client.FetchTicker(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(1000), ticker.Timestamp)
}

func TestSpotDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newSpotDataClient("binance-spot-data", provider, sdk)
	ws := &fakeSpotMarketStream{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTicker,
	}))
	require.Equal(t, "BTCUSDT", ws.bookTickerSymbol)
	ws.bookTickerCB(&spot.BookTickerEvent{
		Symbol:       "BTCUSDT",
		BestBidPrice: "99",
		BestAskPrice: "101",
	})
	tickerEvent := <-client.Events()
	require.NotNil(t, tickerEvent.Ticker)
	require.Equal(t, id, tickerEvent.Ticker.InstrumentID)
	require.True(t, decimal.RequireFromString("99").Equal(tickerEvent.Ticker.Bid))
	require.True(t, decimal.RequireFromString("101").Equal(tickerEvent.Ticker.Ask))

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeQuoteTick,
	}))
	require.Equal(t, "BTCUSDT", ws.bookTickerSymbol)
	ws.bookTickerCB(&spot.BookTickerEvent{
		Symbol:       "BTCUSDT",
		BestBidPrice: "98",
		BestBidQty:   "1.5",
		BestAskPrice: "102",
		BestAskQty:   "2.5",
	})
	quoteEvent := requireNextQuoteEvent(t, client.Events())
	require.NotNil(t, quoteEvent.Quote)
	require.Equal(t, id, quoteEvent.Quote.InstrumentID)
	require.Equal(t, "98", quoteEvent.Quote.BidPrice.String())
	require.Equal(t, "2.5", quoteEvent.Quote.AskSize.String())

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTradeTick,
	}))
	require.Equal(t, "BTCUSDT", ws.tradeSymbol)
	ws.tradeCB(&spot.AggTradeEvent{
		Symbol:       "BTCUSDT",
		AggTradeID:   42,
		Price:        "100.5",
		Quantity:     "0.3",
		TradeTime:    2000,
		EventTime:    2100,
		IsBuyerMaker: false,
	})
	tradeEvent := <-client.Events()
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, id, tradeEvent.Trade.InstrumentID)
	require.Equal(t, "100.5", tradeEvent.Trade.Price.String())
	require.Equal(t, model.TradeID("42"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)

	barType := model.NewTimeBarType(id, time.Minute)
	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	}))
	require.Equal(t, "BTCUSDT", ws.klineSymbol)
	require.Equal(t, "1m", ws.klineInterval)
	kline := &spot.KlineEvent{Symbol: "BTCUSDT", EventTime: 61000}
	kline.Kline.StartTime = 60000
	kline.Kline.OpenPrice = "100"
	kline.Kline.HighPrice = "110"
	kline.Kline.LowPrice = "95"
	kline.Kline.ClosePrice = "105"
	kline.Kline.Volume = "12.5"
	require.NoError(t, ws.klineCB(kline))
	barEvent := <-client.Events()
	require.NotNil(t, barEvent.Bar)
	require.Equal(t, barType.Canonical(), barEvent.Bar.BarType)
	require.Equal(t, "105", barEvent.Bar.Close.String())
	require.Equal(t, "12.5", barEvent.Bar.Volume.String())

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        5,
	}))
	require.Equal(t, "BTCUSDT", ws.depthSymbol)
	require.Equal(t, 5, ws.depthLevels)
	ws.depthCB(&spot.DepthEvent{
		Symbol: "BTCUSDT",
		Bids:   [][]string{{"99", "1"}},
		Asks:   [][]string{{"101", "2"}},
	})
	bookEvent := <-client.Events()
	require.NotNil(t, bookEvent.OrderBook)
	require.Equal(t, id, bookEvent.OrderBook.InstrumentID)
	require.True(t, decimal.RequireFromString("99").Equal(bookEvent.OrderBook.Bids[0].Price))
	require.True(t, decimal.RequireFromString("2").Equal(bookEvent.OrderBook.Asks[0].Size))

	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        5,
	}))
	require.Equal(t, "BTCUSDT", ws.unsubDepthSymbol)
	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTradeTick,
	}))
	require.Equal(t, "BTCUSDT", ws.unsubTradeSymbol)
	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	}))
	require.Equal(t, "BTCUSDT", ws.unsubKlineSymbol)
}

func TestSpotExecutionClientMapsAccountAndOrders(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newSpotExecutionClient("acct", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")

	account, err := client.QueryAccount(context.Background())
	require.NoError(t, err)
	require.Equal(t, model.AccountID("acct"), account.AccountID)
	require.Equal(t, model.Venue("BINANCE"), account.Venue)

	report, err := client.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  id,
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.1"),
		Price:         decimal.RequireFromString("100"),
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderID("42"), report.OrderID)
	require.Equal(t, model.OrderStatusAccepted, report.Status)
	require.Equal(t, "BTCUSDT", sdk.placed.Symbol)
}

func TestSpotExecutionClientPrivateStreamMapsExecutionReports(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newSpotExecutionClient("acct", provider, sdk)
	stream := &fakeSpotAccountStream{}
	client.accountStream = stream

	require.NoError(t, client.Connect(context.Background()))
	require.True(t, stream.connected)
	stream.executionReportCB(&spot.ExecutionReportEvent{
		Symbol:                   "BTCUSDT",
		ClientOrderID:            "client-1",
		Side:                     "BUY",
		OrderType:                "LIMIT",
		OrderStatus:              "PARTIALLY_FILLED",
		OrderID:                  42,
		Quantity:                 "1",
		Price:                    "100",
		LastExecutedQuantity:     "0.4",
		CumulativeFilledQuantity: "0.4",
		LastExecutedPrice:        "100",
		CommissionAmount:         "0.01",
		CommissionAsset:          "USDT",
		TransactionTime:          1000,
		TradeID:                  99,
	})

	orderEvent := <-client.Events()
	require.NotNil(t, orderEvent.Order)
	require.Equal(t, model.OrderID("42"), orderEvent.Order.OrderID)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)
	fillEvent := <-client.Events()
	require.NotNil(t, fillEvent.Fill)
	require.Equal(t, model.TradeID("99"), fillEvent.Fill.TradeID)
	require.True(t, decimal.RequireFromString("0.4").Equal(fillEvent.Fill.Quantity))

	require.NoError(t, client.ResubscribeExecution(context.Background()))
	require.Equal(t, 2, stream.connects)
}

type fakeSpotSDK struct {
	placed spot.PlaceOrderParams
}

func (f *fakeSpotSDK) ExchangeInfo(context.Context) (*spot.ExchangeInfoResponse, error) {
	return &spot.ExchangeInfoResponse{Symbols: []spot.SymbolInfo{{
		Symbol:     "BTCUSDT",
		Status:     "TRADING",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		Filters: []map[string]interface{}{
			{"filterType": "PRICE_FILTER", "tickSize": "0.01"},
			{"filterType": "LOT_SIZE", "stepSize": "0.0001"},
		},
	}}}, nil
}

func (f *fakeSpotSDK) Ticker(context.Context, string) (*spot.TickerResponse, error) {
	return &spot.TickerResponse{Symbol: "BTCUSDT", LastPrice: "100", BidPrice: "99", AskPrice: "101", CloseTime: 1000}, nil
}

func (f *fakeSpotSDK) Depth(context.Context, string, int) (*spot.DepthResponse, error) {
	return &spot.DepthResponse{
		Bids: [][]string{{"99", "1"}},
		Asks: [][]string{{"101", "2"}},
	}, nil
}

func (f *fakeSpotSDK) GetAccount(context.Context) (*spot.AccountResponse, error) {
	return &spot.AccountResponse{Balances: []struct {
		Asset  string `json:"asset"`
		Free   string `json:"free"`
		Locked string `json:"locked"`
	}{{Asset: "USDT", Free: "10", Locked: "1"}}}, nil
}

func (f *fakeSpotSDK) PlaceOrder(_ context.Context, p spot.PlaceOrderParams) (*spot.OrderResponse, error) {
	f.placed = p
	return &spot.OrderResponse{
		Symbol:        p.Symbol,
		OrderID:       42,
		ClientOrderID: p.NewClientOrderID,
		Status:        "NEW",
		Side:          p.Side,
		Type:          p.Type,
		OrigQty:       p.Quantity,
		Price:         p.Price,
	}, nil
}

func (f *fakeSpotSDK) CancelOrder(context.Context, string, int64, string) (*spot.CancelOrderResponse, error) {
	return &spot.CancelOrderResponse{OrderID: 42, Status: "CANCELED"}, nil
}

func (f *fakeSpotSDK) GetOpenOrders(context.Context, string) ([]spot.OrderResponse, error) {
	return []spot.OrderResponse{{Symbol: "BTCUSDT", OrderID: 42, Status: "NEW"}}, nil
}

type fakeSpotMarketStream struct {
	connected        bool
	bookTickerSymbol string
	bookTickerCB     func(*spot.BookTickerEvent) error
	depthSymbol      string
	depthLevels      int
	depthCB          func(*spot.DepthEvent) error
	tradeSymbol      string
	tradeCB          func(*spot.AggTradeEvent) error
	klineSymbol      string
	klineInterval    string
	klineCB          func(*spot.KlineEvent) error
	unsubDepthSymbol string
	unsubTradeSymbol string
	unsubKlineSymbol string
}

func (f *fakeSpotMarketStream) Connect() error {
	f.connected = true
	return nil
}
func (f *fakeSpotMarketStream) Close()                  { f.connected = false }
func (f *fakeSpotMarketStream) IsConnected() bool       { return f.connected }
func (f *fakeSpotMarketStream) SetPostReconnect(func()) {}
func (f *fakeSpotMarketStream) SubscribeBookTicker(symbol string, cb func(*spot.BookTickerEvent) error) error {
	f.bookTickerSymbol = symbol
	f.bookTickerCB = cb
	return nil
}
func (f *fakeSpotMarketStream) SubscribeLimitOrderBook(symbol string, levels int, _ string, cb func(*spot.DepthEvent) error) error {
	f.depthSymbol = symbol
	f.depthLevels = levels
	f.depthCB = cb
	return nil
}
func (f *fakeSpotMarketStream) SubscribeAggTrade(symbol string, cb func(*spot.AggTradeEvent) error) error {
	f.tradeSymbol = symbol
	f.tradeCB = cb
	return nil
}
func (f *fakeSpotMarketStream) SubscribeKline(symbol string, interval string, cb func(*spot.KlineEvent) error) error {
	f.klineSymbol = symbol
	f.klineInterval = interval
	f.klineCB = cb
	return nil
}
func (f *fakeSpotMarketStream) UnsubscribeBookTicker(symbol string) error {
	f.bookTickerSymbol = symbol
	return nil
}
func (f *fakeSpotMarketStream) UnsubscribeLimitOrderBook(symbol string, _ int, _ string) error {
	f.unsubDepthSymbol = symbol
	return nil
}
func (f *fakeSpotMarketStream) UnsubscribeAggTrade(symbol string) error {
	f.unsubTradeSymbol = symbol
	return nil
}
func (f *fakeSpotMarketStream) UnsubscribeKline(symbol string, _ string) error {
	f.unsubKlineSymbol = symbol
	return nil
}

func requireNextQuoteEvent(t *testing.T, events <-chan model.MarketEvent) model.MarketEvent {
	t.Helper()
	for i := 0; i < 3; i++ {
		event := <-events
		if event.Quote != nil {
			return event
		}
	}
	require.Fail(t, "quote event not found")
	return model.MarketEvent{}
}

type fakeSpotAccountStream struct {
	connected         bool
	connects          int
	executionReportCB func(*spot.ExecutionReportEvent)
	accountPositionCB func(*spot.AccountPositionEvent)
}

func (f *fakeSpotAccountStream) Connect() error {
	f.connected = true
	f.connects++
	return nil
}
func (f *fakeSpotAccountStream) Close() {
	f.connected = false
}
func (f *fakeSpotAccountStream) SubscribeExecutionReport(cb func(*spot.ExecutionReportEvent)) {
	f.executionReportCB = cb
}
func (f *fakeSpotAccountStream) SubscribeAccountPosition(cb func(*spot.AccountPositionEvent)) {
	f.accountPositionCB = cb
}
