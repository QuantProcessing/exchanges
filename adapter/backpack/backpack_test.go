package backpack

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	backpacksdk "github.com/QuantProcessing/exchanges/sdk/backpack"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	data := newDataClient("backpack-perp-data", provider, sdk)
	data.ws = &fakeWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newExecutionClient("perp-acct", provider, sdk),
		InstrumentID: model.MustInstrumentID("BTC-USDC-PERP.BACKPACK"),
		Capabilities: (&Adapter{}).Capabilities(),
	})
}

func TestSubmitMapsOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk)

	report, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDC-PERP.BACKPACK"),
		ClientOrderID: "42",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderID("placed-1"), report.OrderID)
	require.Equal(t, "BTC_USDC_PERP", sdk.created.Symbol)
	require.Equal(t, "Ask", sdk.created.Side)
	require.Equal(t, "Limit", sdk.created.OrderType)
	require.Equal(t, "IOC", sdk.created.TimeInForce)
	require.Equal(t, "0.5", sdk.created.Quantity)
	require.Equal(t, "10", sdk.created.Price)
	require.Equal(t, uint32(42), sdk.created.ClientID)
}

func TestDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("backpack-perp-data", provider, sdk)
	ws := &fakeWS{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDC-PERP.BACKPACK")

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTicker,
	}))
	require.Equal(t, "bookTicker.BTC_USDC_PERP", ws.lastStream)
	require.False(t, ws.lastPrivate)
	ws.emit("bookTicker.BTC_USDC_PERP", map[string]any{
		"e": "bookTicker",
		"E": 1000000,
		"s": "BTC_USDC_PERP",
		"b": "9",
		"a": "11",
		"T": 1000000,
	})
	require.NoError(t, client.Health().LastError)
	tickerEvent := <-client.Events()
	require.NotNil(t, tickerEvent.Ticker)
	require.Equal(t, id, tickerEvent.Ticker.InstrumentID)
	require.True(t, decimal.RequireFromString("9").Equal(tickerEvent.Ticker.Bid))
	require.True(t, decimal.RequireFromString("11").Equal(tickerEvent.Ticker.Ask))

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        100,
	}))
	require.Equal(t, "depth.BTC_USDC_PERP", ws.lastStream)
	ws.emit("depth.BTC_USDC_PERP", backpacksdk.DepthEvent{
		EventType:       "depth",
		EventTime:       1000000,
		Symbol:          "BTC_USDC_PERP",
		Bids:            [][]string{{"9", "1"}},
		Asks:            [][]string{{"11", "2"}},
		EngineTimestamp: 1000000,
	})
	bookEvent := <-client.Events()
	require.NotNil(t, bookEvent.OrderBook)
	require.Equal(t, id, bookEvent.OrderBook.InstrumentID)
	require.True(t, decimal.RequireFromString("11").Equal(bookEvent.OrderBook.Asks[0].Price))
	require.True(t, decimal.RequireFromString("2").Equal(bookEvent.OrderBook.Asks[0].Size))

	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        100,
	}))
	require.Equal(t, "depth.BTC_USDC_PERP", ws.unsubStream)
}

func TestDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("backpack-perp-data", provider, sdk)
	ws := &fakeWS{}
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	id := model.MustInstrumentID("BTC-USDC-PERP.BACKPACK")
	bookSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeOrderBook, Depth: 100}
	quoteSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), bookSub))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 1, ws.streamSubscribeCount["depth.BTC_USDC_PERP"])

	ws.emit("depth.BTC_USDC_PERP", backpacksdk.DepthEvent{
		EventType:       "depth",
		EventTime:       1000000,
		Symbol:          "BTC_USDC_PERP",
		Bids:            [][]string{{"100", "1.2"}},
		Asks:            [][]string{{"101", "0.7"}},
		EngineTimestamp: 1000000,
	})
	firstEvent := <-streaming.Events()
	secondEvent := <-streaming.Events()
	if firstEvent.Quote == nil {
		firstEvent, secondEvent = secondEvent, firstEvent
	}
	require.NotNil(t, firstEvent.Quote)
	require.Equal(t, id, firstEvent.Quote.InstrumentID)
	require.Equal(t, "100", firstEvent.Quote.BidPrice.String())
	require.Equal(t, "1.2", firstEvent.Quote.BidSize.String())
	require.Equal(t, "101", firstEvent.Quote.AskPrice.String())
	require.Equal(t, "0.7", firstEvent.Quote.AskSize.String())
	require.NotNil(t, secondEvent.OrderBook)
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, "", ws.unsubStream)
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), bookSub))
	require.Equal(t, "depth.BTC_USDC_PERP", ws.unsubStream)

	tradeSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTradeTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, "trade.BTC_USDC_PERP", ws.lastStream)
	ws.emit("trade.BTC_USDC_PERP", backpacksdk.Trade{
		ID:           55,
		Price:        "100.25",
		Quantity:     "0.25",
		Timestamp:    1710000000000,
		IsBuyerMaker: false,
	})
	tradeEvent := <-streaming.Events()
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, id, tradeEvent.Trade.InstrumentID)
	require.Equal(t, model.TradeID("55"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)
	require.Equal(t, "100.25", tradeEvent.Trade.Price.String())
	require.Equal(t, "0.25", tradeEvent.Trade.Size.String())
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, "trade.BTC_USDC_PERP", ws.unsubStream)

	barType := model.NewTimeBarType(id, time.Minute)
	barSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeBar, BarType: barType}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), barSub))
	require.Equal(t, "kline.1m.BTC_USDC_PERP", ws.lastStream)
	ws.emit("kline.1m.BTC_USDC_PERP", backpacksdk.Kline{
		Start:  "1710000000000",
		End:    "1710000059999",
		Open:   "100",
		High:   "102",
		Low:    "99",
		Close:  "101",
		Volume: "5",
	})
	barEvent := <-streaming.Events()
	require.NotNil(t, barEvent.Bar)
	require.Equal(t, barType.String(), barEvent.Bar.BarType.String())
	require.Equal(t, "100", barEvent.Bar.Open.String())
	require.Equal(t, "102", barEvent.Bar.High.String())
	require.Equal(t, "99", barEvent.Bar.Low.String())
	require.Equal(t, "101", barEvent.Bar.Close.String())
	require.Equal(t, "5", barEvent.Bar.Volume.String())
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), barSub))
	require.Equal(t, "kline.1m.BTC_USDC_PERP", ws.unsubStream)
}

func TestExecutionClientPrivateStreamMapsOrdersFillsAndPositions(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("perp-acct", provider, sdk)
	ws := &fakeWS{}
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.True(t, ws.connected)
	require.True(t, ws.privateByStream["account.orderUpdate"])
	require.True(t, ws.privateByStream["account.positionUpdate"])

	ws.emit("account.orderUpdate", backpacksdk.OrderUpdateEvent{
		EventType:        "orderFill",
		EventTime:        1000000,
		Symbol:           "BTC_USDC_PERP",
		ClientID:         "42",
		Side:             "Bid",
		OrderType:        "Limit",
		TimeInForce:      "GTC",
		Quantity:         "1",
		Price:            "10",
		OrderState:       "PartiallyFilled",
		OrderID:          "order-1",
		TradeID:          "9001",
		FillQuantity:     "0.4",
		ExecutedQuantity: "0.4",
		FillPrice:        "10",
		Fee:              "0.01",
		FeeSymbol:        "USDC",
		EngineTimestamp:  1000000,
	})
	orderEvent := <-exec.Events()
	require.NotNil(t, orderEvent.Order)
	require.Equal(t, model.OrderID("order-1"), orderEvent.Order.OrderID)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)
	fillEvent := <-exec.Events()
	require.NotNil(t, fillEvent.Fill)
	require.Equal(t, model.TradeID("9001"), fillEvent.Fill.TradeID)
	require.True(t, decimal.RequireFromString("0.4").Equal(fillEvent.Fill.Quantity))

	ws.emit("account.positionUpdate", backpacksdk.PositionUpdateEvent{
		EventType:       "positionAdjusted",
		EventTime:       1000000,
		Symbol:          "BTC_USDC_PERP",
		EntryPrice:      "10",
		NetQuantity:     "-0.2",
		PositionID:      "pos-1",
		EngineTimestamp: 1000000,
	})
	positionEvent := <-exec.Events()
	require.NotNil(t, positionEvent.Position)
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)
	require.True(t, decimal.RequireFromString("0.2").Equal(positionEvent.Position.Quantity))

	require.NoError(t, exec.ResubscribeExecution(context.Background()))
	require.Equal(t, 4, ws.subscribeCount)
}

type fakeSDK struct {
	created backpacksdk.CreateOrderRequest
}

func (f *fakeSDK) GetMarkets(context.Context) ([]backpacksdk.Market, error) {
	return []backpacksdk.Market{{
		Symbol:      "BTC_USDC_PERP",
		BaseSymbol:  "BTC",
		QuoteSymbol: "USDC",
		MarketType:  "PERP",
		Visible:     true,
		Filters: backpacksdk.MarketFilters{
			Price:    backpacksdk.PriceFilter{TickSize: "0.1"},
			Quantity: backpacksdk.QuantityFilter{StepSize: "0.001"},
		},
	}}, nil
}

func (f *fakeSDK) GetTicker(context.Context, string) (*backpacksdk.Ticker, error) {
	return &backpacksdk.Ticker{Symbol: "BTC_USDC_PERP", LastPrice: "10"}, nil
}

func (f *fakeSDK) GetOrderBook(context.Context, string, int) (*backpacksdk.Depth, error) {
	return &backpacksdk.Depth{Bids: [][]string{{"9", "1"}}, Asks: [][]string{{"11", "1"}}}, nil
}

func (f *fakeSDK) GetBalances(context.Context) (map[string]backpacksdk.CapitalBalance, error) {
	return map[string]backpacksdk.CapitalBalance{"USDC": {Available: "90", Locked: "10"}}, nil
}

func (f *fakeSDK) GetOpenOrders(context.Context, string, string) ([]backpacksdk.Order, error) {
	return []backpacksdk.Order{{
		ID:               "open-1",
		ClientID:         7,
		Symbol:           "BTC_USDC_PERP",
		Side:             "Bid",
		OrderType:        "Limit",
		Status:           "New",
		Quantity:         "1",
		ExecutedQuantity: "0",
		Price:            "10",
		TimeInForce:      "GTC",
	}}, nil
}

func (f *fakeSDK) GetOpenPositions(context.Context, string) ([]backpacksdk.Position, error) {
	return []backpacksdk.Position{{
		Symbol:      "BTC_USDC_PERP",
		PositionID:  "pos-1",
		NetQuantity: "0.1",
		EntryPrice:  "10",
	}}, nil
}

func (f *fakeSDK) PlaceOrder(_ context.Context, req backpacksdk.CreateOrderRequest) (*backpacksdk.Order, error) {
	f.created = req
	return &backpacksdk.Order{ID: "placed-1", ClientID: req.ClientID, Symbol: req.Symbol, Side: req.Side, OrderType: req.OrderType, Status: "New", Quantity: req.Quantity, Price: req.Price}, nil
}

func (f *fakeSDK) CancelOrder(context.Context, backpacksdk.CancelOrderRequest) (*backpacksdk.Order, error) {
	return &backpacksdk.Order{ID: "canceled-1", Status: "Cancelled"}, nil
}

type fakeWS struct {
	connected            bool
	subscribeCount       int
	lastStream           string
	lastPrivate          bool
	unsubStream          string
	handlers             map[string]func(json.RawMessage)
	privateByStream      map[string]bool
	streamSubscribeCount map[string]int
}

func (f *fakeWS) Subscribe(_ context.Context, stream string, private bool, handler func(json.RawMessage)) error {
	f.connected = true
	f.subscribeCount++
	f.lastStream = stream
	f.lastPrivate = private
	if f.handlers == nil {
		f.handlers = make(map[string]func(json.RawMessage))
	}
	if f.privateByStream == nil {
		f.privateByStream = make(map[string]bool)
	}
	if f.streamSubscribeCount == nil {
		f.streamSubscribeCount = make(map[string]int)
	}
	f.handlers[stream] = handler
	f.privateByStream[stream] = private
	f.streamSubscribeCount[stream]++
	return nil
}

func (f *fakeWS) Unsubscribe(_ context.Context, stream string) error {
	f.unsubStream = stream
	return nil
}

func (f *fakeWS) Close() error {
	f.connected = false
	return nil
}

func (f *fakeWS) emit(stream string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	f.handlers[stream](data)
}
