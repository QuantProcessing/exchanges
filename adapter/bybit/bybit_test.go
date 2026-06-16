package bybit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	bybitsdk "github.com/QuantProcessing/exchanges/sdk/bybit"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSpotClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSpotProvider(sdk)
	data := newDataClient("bybit-spot-data", provider, sdk)
	data.ws = &fakePublicWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newExecutionClient("spot-acct", provider, sdk, "spot"),
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BYBIT"),
		Capabilities: (&Adapter{}).Capabilities(),
	})
}

func TestLinearClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newLinearProvider(sdk)
	data := newDataClient("bybit-linear-data", provider, sdk)
	data.ws = &fakePublicWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newExecutionClient("linear-acct", provider, sdk, "linear"),
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.BYBIT"),
		Capabilities: (&Adapter{}).Capabilities(),
	})
}

func TestSpotSubmitMapsOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk, "spot")

	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BYBIT"),
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, "spot", sdk.placed.Category)
	require.Equal(t, "BTCUSDT", sdk.placed.Symbol)
	require.Equal(t, "Buy", sdk.placed.Side)
	require.Equal(t, "Limit", sdk.placed.OrderType)
	require.Equal(t, "GTC", sdk.placed.TimeInForce)
}

func TestLinearSubmitMapsOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newLinearProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk, "linear")

	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-PERP.BYBIT"),
		ClientOrderID: "client-2",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("2"),
	})
	require.NoError(t, err)
	require.Equal(t, "linear", sdk.placed.Category)
	require.Equal(t, "BTCUSDT", sdk.placed.Symbol)
	require.Equal(t, "Sell", sdk.placed.Side)
	require.Equal(t, "Market", sdk.placed.OrderType)
}

func TestDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newLinearProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("bybit-linear-data", provider, sdk)
	ws := &fakePublicWS{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDT-PERP.BYBIT")

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTicker,
	}))
	require.Equal(t, "tickers.BTCUSDT", ws.lastTopic)
	ws.emit("tickers.BTCUSDT", map[string]any{
		"topic": "tickers.BTCUSDT",
		"ts":    1000,
		"data":  map[string]string{"symbol": "BTCUSDT", "bid1Price": "9", "ask1Price": "11", "lastPrice": "10"},
	})
	tickerEvent := <-client.Events()
	require.NotNil(t, tickerEvent.Ticker)
	require.Equal(t, id, tickerEvent.Ticker.InstrumentID)
	require.True(t, decimal.RequireFromString("9").Equal(tickerEvent.Ticker.Bid))

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        50,
	}))
	require.Equal(t, "orderbook.50.BTCUSDT", ws.lastTopic)
	ws.emit("orderbook.50.BTCUSDT", map[string]any{
		"topic": "orderbook.50.BTCUSDT",
		"ts":    1000,
		"data": map[string]any{
			"s": "BTCUSDT",
			"b": [][]string{{"9", "1"}},
			"a": [][]string{{"11", "2"}},
		},
	})
	bookEvent := <-client.Events()
	require.NotNil(t, bookEvent.OrderBook)
	require.Equal(t, id, bookEvent.OrderBook.InstrumentID)
	require.True(t, decimal.RequireFromString("11").Equal(bookEvent.OrderBook.Asks[0].Price))

	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        50,
	}))
	require.Equal(t, "orderbook.50.BTCUSDT", ws.unsubTopic)
}

func TestDataClientRestSnapshotsUseVenueTimestamps(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newLinearProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("bybit-linear-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.BYBIT")

	ticker, err := client.FetchTicker(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(1000), ticker.Timestamp)

	book, err := client.FetchOrderBook(context.Background(), id, 50)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(2000), book.Timestamp)
}

func TestDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newLinearProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("bybit-linear-data", provider, sdk)
	ws := &fakePublicWS{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDT-PERP.BYBIT")

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeQuoteTick,
	}))
	require.Equal(t, "tickers.BTCUSDT", ws.lastTopic)
	ws.emit("tickers.BTCUSDT", map[string]any{
		"topic": "tickers.BTCUSDT",
		"ts":    1000,
		"data": map[string]string{
			"symbol":    "BTCUSDT",
			"bid1Price": "9",
			"bid1Size":  "1.25",
			"ask1Price": "11",
			"ask1Size":  "2.5",
			"lastPrice": "10",
		},
	})
	quoteEvent := <-client.Events()
	require.NotNil(t, quoteEvent.Quote)
	require.Equal(t, id, quoteEvent.Quote.InstrumentID)
	require.True(t, decimal.RequireFromString("1.25").Equal(quoteEvent.Quote.BidSize))

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTradeTick,
	}))
	require.Equal(t, "publicTrade.BTCUSDT", ws.lastTopic)
	ws.emit("publicTrade.BTCUSDT", map[string]any{
		"topic": "publicTrade.BTCUSDT",
		"ts":    2000,
		"data": []map[string]any{{
			"T": 2000,
			"s": "BTCUSDT",
			"S": "Buy",
			"v": "0.3",
			"p": "100.5",
			"i": "trade-1",
		}},
	})
	tradeEvent := <-client.Events()
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, model.TradeID("trade-1"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)

	barType := model.NewTimeBarType(id, time.Minute)
	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	}))
	require.Equal(t, "kline.1.BTCUSDT", ws.lastTopic)
	ws.emit("kline.1.BTCUSDT", map[string]any{
		"topic": "kline.1.BTCUSDT",
		"ts":    3000,
		"data": []map[string]any{{
			"start":     3000,
			"end":       60000,
			"interval":  "1",
			"open":      "100",
			"high":      "110",
			"low":       "95",
			"close":     "105",
			"volume":    "12.5",
			"confirm":   false,
			"timestamp": 60000,
		}},
	})
	barEvent := <-client.Events()
	require.NotNil(t, barEvent.Bar)
	require.Equal(t, barType.Canonical(), barEvent.Bar.BarType)
	require.True(t, decimal.RequireFromString("105").Equal(barEvent.Bar.Close))
}

func TestSpotDataClientStreamsQuoteTickFromTopOfBook(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("bybit-spot-data", provider, sdk)
	ws := &fakePublicWS{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDT-SPOT.BYBIT")

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeQuoteTick,
	}))
	require.Equal(t, "orderbook.1.BTCUSDT", ws.lastTopic)
	ws.emit("orderbook.1.BTCUSDT", map[string]any{
		"topic": "orderbook.1.BTCUSDT",
		"ts":    1000,
		"data": map[string]any{
			"s": "BTCUSDT",
			"b": [][]string{{"9", "1"}},
			"a": [][]string{{"11", "2"}},
		},
	})
	quoteEvent := <-client.Events()
	require.NotNil(t, quoteEvent.Quote)
	require.Equal(t, id, quoteEvent.Quote.InstrumentID)
	require.True(t, decimal.RequireFromString("2").Equal(quoteEvent.Quote.AskSize))
}

func TestExecutionClientPrivateStreamMapsOrdersExecutionsAndPositions(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newLinearProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("linear-acct", provider, sdk, "linear")
	ws := &fakePrivateWS{}
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.True(t, ws.connected)
	require.Contains(t, ws.handlers, "order")
	require.Contains(t, ws.handlers, "execution")
	require.Contains(t, ws.handlers, "position")

	ws.emit("order", bybitsdk.WSOrderMessage{
		Data: []bybitsdk.OrderRecord{{
			OrderID:     "100",
			OrderLinkID: "client-1",
			Symbol:      "BTCUSDT",
			Side:        "Buy",
			OrderType:   "Limit",
			Price:       "10",
			Qty:         "1",
			CumExecQty:  "0.4",
			OrderStatus: "PartiallyFilled",
			AvgPrice:    "10",
			UpdatedTime: "1000",
		}},
	})
	orderEvent := <-exec.Events()
	require.NotNil(t, orderEvent.Order)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)

	ws.emit("execution", bybitsdk.WSExecutionMessage{
		Data: []bybitsdk.ExecutionRecord{{
			ExecID:      "exec-1",
			OrderID:     "100",
			OrderLinkID: "client-1",
			Symbol:      "BTCUSDT",
			Side:        "Buy",
			ExecPrice:   "10",
			ExecQty:     "0.4",
			ExecFee:     "0.01",
			FeeCurrency: "USDT",
			ExecTime:    "1000",
		}},
	})
	fillEvent := <-exec.Events()
	require.NotNil(t, fillEvent.Fill)
	require.Equal(t, model.TradeID("exec-1"), fillEvent.Fill.TradeID)
	require.True(t, decimal.RequireFromString("0.4").Equal(fillEvent.Fill.Quantity))

	ws.emit("position", bybitsdk.WSPositionMessage{
		Data: []bybitsdk.PositionRecord{{
			Symbol:   "BTCUSDT",
			Side:     "Sell",
			Size:     "0.2",
			AvgPrice: "10",
		}},
	})
	positionEvent := <-exec.Events()
	require.NotNil(t, positionEvent.Position)
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)

	require.NoError(t, exec.ResubscribeExecution(context.Background()))
	require.Equal(t, 6, ws.subscribeCount)
}

type fakeSDK struct {
	placed bybitsdk.PlaceOrderRequest
}

func (f *fakeSDK) GetInstruments(_ context.Context, category string) ([]bybitsdk.Instrument, error) {
	switch category {
	case "spot":
		return []bybitsdk.Instrument{{
			Symbol:      "BTCUSDT",
			BaseCoin:    "BTC",
			QuoteCoin:   "USDT",
			Status:      "Trading",
			PriceFilter: bybitsdk.PriceFilter{TickSize: "0.01"},
			LotSizeFilter: bybitsdk.LotSizeFilter{
				BasePrecision: "0.0001",
				QtyStep:       "0.0001",
				MinOrderQty:   "0.0001",
			},
		}}, nil
	case "linear":
		return []bybitsdk.Instrument{{
			Symbol:      "BTCUSDT",
			BaseCoin:    "BTC",
			QuoteCoin:   "USDT",
			SettleCoin:  "USDT",
			Status:      "Trading",
			PriceFilter: bybitsdk.PriceFilter{TickSize: "0.1"},
			LotSizeFilter: bybitsdk.LotSizeFilter{
				QtyStep:     "0.001",
				MinOrderQty: "0.001",
			},
		}}, nil
	default:
		return nil, nil
	}
}

func (f *fakeSDK) GetTicker(context.Context, string, string) (*bybitsdk.Ticker, error) {
	return &bybitsdk.Ticker{LastPrice: "10", Bid1Price: "9", Ask1Price: "11", Time: "1000"}, nil
}

func (f *fakeSDK) GetOrderBook(context.Context, string, string, int) (*bybitsdk.OrderBook, error) {
	return &bybitsdk.OrderBook{
		Bids: [][]bybitsdk.NumberString{{"9", "1"}},
		Asks: [][]bybitsdk.NumberString{{"11", "1"}},
		TS:   2000,
	}, nil
}

func (f *fakeSDK) GetWalletBalance(context.Context, string, string) (*bybitsdk.WalletBalanceResult, error) {
	return &bybitsdk.WalletBalanceResult{List: []bybitsdk.WalletAccount{{
		AccountType: "UNIFIED",
		Coin: []bybitsdk.WalletCoin{{
			Coin:          "USDT",
			Equity:        "6",
			WalletBalance: "5",
			Locked:        "1",
		}},
	}}}, nil
}

func (f *fakeSDK) PlaceOrder(_ context.Context, req bybitsdk.PlaceOrderRequest) (*bybitsdk.OrderActionResponse, error) {
	f.placed = req
	return &bybitsdk.OrderActionResponse{OrderID: "100", OrderLinkID: req.OrderLinkID}, nil
}

func (f *fakeSDK) CancelOrder(context.Context, bybitsdk.CancelOrderRequest) (*bybitsdk.OrderActionResponse, error) {
	return &bybitsdk.OrderActionResponse{OrderID: "100"}, nil
}

func (f *fakeSDK) GetOpenOrders(context.Context, string, string) ([]bybitsdk.OrderRecord, error) {
	return []bybitsdk.OrderRecord{{
		OrderID:     "100",
		OrderLinkID: "client-1",
		Symbol:      "BTCUSDT",
		Side:        "Buy",
		OrderType:   "Limit",
		Price:       "10",
		Qty:         "1",
		CumExecQty:  "0",
		OrderStatus: "New",
	}}, nil
}

type fakePublicWS struct {
	connected  bool
	lastTopic  string
	unsubTopic string
	handlers   map[string]func(json.RawMessage)
}

func (f *fakePublicWS) Subscribe(_ context.Context, topic string, handler func(json.RawMessage)) error {
	f.connected = true
	f.lastTopic = topic
	if f.handlers == nil {
		f.handlers = make(map[string]func(json.RawMessage))
	}
	f.handlers[topic] = handler
	return nil
}

func (f *fakePublicWS) Unsubscribe(_ context.Context, topic string) error {
	f.unsubTopic = topic
	return nil
}

func (f *fakePublicWS) Close() error {
	f.connected = false
	return nil
}

func (f *fakePublicWS) emit(topic string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	f.handlers[topic](data)
}

type fakePrivateWS struct {
	connected      bool
	subscribeCount int
	handlers       map[string]func(json.RawMessage)
}

func (f *fakePrivateWS) Subscribe(_ context.Context, topic string, handler func(json.RawMessage)) error {
	f.connected = true
	f.subscribeCount++
	if f.handlers == nil {
		f.handlers = make(map[string]func(json.RawMessage))
	}
	f.handlers[topic] = handler
	return nil
}

func (f *fakePrivateWS) Close() error {
	f.connected = false
	return nil
}

func (f *fakePrivateWS) emit(topic string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	f.handlers[topic](data)
}
