package bitget

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	bitgetsdk "github.com/QuantProcessing/exchanges/sdk/bitget"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSpotClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSpotProvider(sdk)
	data := newDataClient("bitget-spot-data", provider, sdk)
	data.ws = &fakePublicWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:         provider,
		Data:             data,
		Execution:        newExecutionClient("spot-acct", provider, sdk, "SPOT"),
		InstrumentID:     model.MustInstrumentID("BTC-USDT-SPOT.BITGET"),
		Capabilities:     (&Adapter{}).Capabilities(),
		ExpectedMakerFee: decimal.RequireFromString("0.001"),
		ExpectedTakerFee: decimal.RequireFromString("0.0015"),
	})
}

func TestPerpClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	data := newDataClient("bitget-perp-data", provider, sdk)
	data.ws = &fakePublicWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:         provider,
		Data:             data,
		Execution:        newExecutionClient("perp-acct", provider, sdk, "USDT-FUTURES"),
		InstrumentID:     model.MustInstrumentID("BTC-USDT-PERP.BITGET"),
		Capabilities:     (&Adapter{fundingRates: true}).Capabilities(),
		ExpectedMakerFee: decimal.RequireFromString("0.0002"),
		ExpectedTakerFee: decimal.RequireFromString("0.0006"),
	})
}

func TestPerpAdapterCapabilitiesDeclareFundingRatesOnlyForPerp(t *testing.T) {
	require.False(t, (&Adapter{}).Capabilities().MarketData.FundingRates)
	require.True(t, (&Adapter{fundingRates: true}).Capabilities().MarketData.FundingRates)
}

func TestInstrumentProviderNormalizesFeeMetadata(t *testing.T) {
	sdk := &fakeSDK{}
	spotProvider := newSpotProvider(sdk)
	require.NoError(t, spotProvider.LoadAll(context.Background()))
	spot, ok := spotProvider.Get(model.MustInstrumentID("BTC-USDT-SPOT.BITGET"))
	require.True(t, ok)
	require.Equal(t, "0.001", spot.MakerFee.String())
	require.Equal(t, "0.0015", spot.TakerFee.String())

	perpProvider := newPerpProvider(sdk)
	require.NoError(t, perpProvider.LoadAll(context.Background()))
	perp, ok := perpProvider.Get(model.MustInstrumentID("BTC-USDT-PERP.BITGET"))
	require.True(t, ok)
	require.Equal(t, "0.0002", perp.MakerFee.String())
	require.Equal(t, "0.0006", perp.TakerFee.String())
}

func TestSpotSubmitMapsOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk, "SPOT")

	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BITGET"),
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, "SPOT", sdk.placed.Category)
	require.Equal(t, "BTCUSDT", sdk.placed.Symbol)
	require.Equal(t, "buy", sdk.placed.Side)
	require.Equal(t, "limit", sdk.placed.OrderType)
	require.Equal(t, "gtc", sdk.placed.TimeInForce)
}

func TestPerpSubmitMapsOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk, "USDT-FUTURES")

	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-PERP.BITGET"),
		ClientOrderID: "client-2",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("2"),
	})
	require.NoError(t, err)
	require.Equal(t, "USDT-FUTURES", sdk.placed.Category)
	require.Equal(t, "BTCUSDT", sdk.placed.Symbol)
	require.Equal(t, "sell", sdk.placed.Side)
	require.Equal(t, "market", sdk.placed.OrderType)
	require.Equal(t, "ioc", sdk.placed.TimeInForce)
}

func TestDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("bitget-spot-data", provider, sdk)
	ws := &fakePublicWS{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDT-SPOT.BITGET")

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTicker,
	}))
	require.Equal(t, bitgetsdk.WSArg{InstType: "SPOT", Channel: "ticker", InstID: "BTCUSDT"}, ws.lastArg)
	ws.emit(bitgetsdk.WSArg{InstType: "SPOT", Channel: "ticker", InstID: "BTCUSDT"}, map[string]any{
		"arg":  ws.lastArg,
		"data": []map[string]string{{"symbol": "BTCUSDT", "bid1Price": "9", "ask1Price": "11", "lastPrice": "10", "ts": "1000"}},
	})
	tickerEvent := <-client.Events()
	require.NotNil(t, tickerEvent.Ticker)
	require.Equal(t, id, tickerEvent.Ticker.InstrumentID)
	require.True(t, decimal.RequireFromString("9").Equal(tickerEvent.Ticker.Bid))
	require.True(t, decimal.RequireFromString("11").Equal(tickerEvent.Ticker.Ask))

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        5,
	}))
	require.Equal(t, bitgetsdk.WSArg{InstType: "SPOT", Channel: "books5", InstID: "BTCUSDT"}, ws.lastArg)
	ws.emit(ws.lastArg, map[string]any{
		"arg": ws.lastArg,
		"data": []map[string]any{{
			"b":  [][]string{{"9", "1"}},
			"a":  [][]string{{"11", "2"}},
			"ts": "1000",
		}},
	})
	bookEvent := <-client.Events()
	require.NotNil(t, bookEvent.OrderBook)
	require.Equal(t, id, bookEvent.OrderBook.InstrumentID)
	require.True(t, decimal.RequireFromString("11").Equal(bookEvent.OrderBook.Asks[0].Price))
	require.True(t, decimal.RequireFromString("2").Equal(bookEvent.OrderBook.Asks[0].Size))

	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        5,
	}))
	require.Equal(t, bitgetsdk.WSArg{InstType: "SPOT", Channel: "books5", InstID: "BTCUSDT"}, ws.unsubArg)
}

func TestDataClientRestSnapshotsUseVenueTimestamps(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("bitget-spot-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-SPOT.BITGET")

	ticker, err := client.FetchTicker(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(1000), ticker.Timestamp)

	book, err := client.FetchOrderBook(context.Background(), id, 5)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(2000), book.Timestamp)
}

func TestDataClientFetchFundingRate(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("bitget-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.BITGET")

	funding, err := client.FetchFundingRate(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, funding.InstrumentID)
	require.True(t, decimal.RequireFromString("0.0009").Equal(funding.Rate))
	require.Equal(t, time.UnixMilli(1000), funding.Timestamp)
	require.True(t, funding.MarkPrice.IsZero())
	require.True(t, funding.IndexPrice.IsZero())
}

func TestDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("bitget-spot-data", provider, sdk)
	ws := &fakePublicWS{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDT-SPOT.BITGET")

	tickerSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}
	quoteSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}
	require.NoError(t, client.SubscribeMarketData(context.Background(), tickerSub))
	require.NoError(t, client.SubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 1, ws.subscribeCount)
	require.Equal(t, bitgetsdk.WSArg{InstType: "SPOT", Channel: "ticker", InstID: "BTCUSDT"}, ws.lastArg)
	ws.emit(ws.lastArg, map[string]any{
		"arg": ws.lastArg,
		"data": []map[string]string{{
			"symbol":    "BTCUSDT",
			"bid1Price": "9",
			"bid1Size":  "1.25",
			"ask1Price": "11",
			"ask1Size":  "2.5",
			"lastPrice": "10",
			"ts":        "1000",
		}},
	})
	tickerEvent := <-client.Events()
	require.NotNil(t, tickerEvent.Ticker)
	quoteEvent := <-client.Events()
	require.NotNil(t, quoteEvent.Quote)
	require.Equal(t, id, quoteEvent.Quote.InstrumentID)
	require.True(t, decimal.RequireFromString("2.5").Equal(quoteEvent.Quote.AskSize))

	require.NoError(t, client.UnsubscribeMarketData(context.Background(), quoteSub))
	require.Empty(t, ws.unsubArg.Channel)
	require.NoError(t, client.UnsubscribeMarketData(context.Background(), tickerSub))
	require.Equal(t, bitgetsdk.WSArg{InstType: "SPOT", Channel: "ticker", InstID: "BTCUSDT"}, ws.unsubArg)

	tradeSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTradeTick}
	require.NoError(t, client.SubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, bitgetsdk.WSArg{InstType: "SPOT", Channel: "trade", InstID: "BTCUSDT"}, ws.lastArg)
	ws.emit(ws.lastArg, map[string]any{
		"arg": ws.lastArg,
		"data": []map[string]string{{
			"execId": "trade-1",
			"price":  "100.5",
			"size":   "0.3",
			"side":   "buy",
			"ts":     "2000",
		}},
	})
	tradeEvent := <-client.Events()
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, model.TradeID("trade-1"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)

	barType := model.NewTimeBarType(id, time.Minute)
	barSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeBar, BarType: barType}
	require.NoError(t, client.SubscribeMarketData(context.Background(), barSub))
	require.Equal(t, bitgetsdk.WSArg{InstType: "SPOT", Channel: "candle1m", InstID: "BTCUSDT"}, ws.lastArg)
	ws.emit(ws.lastArg, map[string]any{
		"arg":  ws.lastArg,
		"data": [][]string{{"3000", "100", "110", "95", "105", "12.5", "1200"}},
	})
	barEvent := <-client.Events()
	require.NotNil(t, barEvent.Bar)
	require.Equal(t, barType.Canonical(), barEvent.Bar.BarType)
	require.True(t, decimal.RequireFromString("105").Equal(barEvent.Bar.Close))
}

func TestExecutionClientPrivateStreamMapsOrdersFillsAndPositions(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("perp-acct", provider, sdk, "USDT-FUTURES")
	ws := &fakePrivateWS{}
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.True(t, ws.connected)
	require.Contains(t, ws.handlers, "order")
	require.Contains(t, ws.handlers, "fill")
	require.Contains(t, ws.handlers, "position")

	ws.emit("order", bitgetsdk.WSOrderMessage{
		Data: []bitgetsdk.OrderRecord{{
			OrderID:     "100",
			ClientOID:   "client-1",
			Symbol:      "BTCUSDT",
			Side:        "buy",
			OrderType:   "limit",
			Price:       "10",
			Qty:         "1",
			FilledQty:   "0.4",
			OrderStatus: "partial_filled",
			AvgPrice:    "10",
			UpdatedTime: "1000",
		}},
	})
	orderEvent := <-exec.Events()
	require.NotNil(t, orderEvent.Order)
	require.Equal(t, model.OrderID("100"), orderEvent.Order.OrderID)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)

	ws.emit("fill", bitgetsdk.WSFillMessage{
		Data: []bitgetsdk.FillRecord{{
			OrderID:   "100",
			ClientOID: "client-1",
			ExecID:    "trade-1",
			Symbol:    "BTCUSDT",
			Side:      "buy",
			ExecPrice: "10",
			ExecQty:   "0.4",
			FeeDetail: []bitgetsdk.FeeDetail{{FeeCoin: "USDT", Fee: "0.01"}},
			ExecTime:  "1000",
		}},
	})
	fillEvent := <-exec.Events()
	require.NotNil(t, fillEvent.Fill)
	require.Equal(t, model.TradeID("trade-1"), fillEvent.Fill.TradeID)
	require.True(t, decimal.RequireFromString("0.4").Equal(fillEvent.Fill.Quantity))

	ws.emit("position", bitgetsdk.WSPositionMessage{
		Data: []bitgetsdk.PositionRecord{{
			Symbol:           "BTCUSDT",
			PosSide:          "short",
			Total:            "0.2",
			AverageOpenPrice: "10",
			UpdatedTime:      "1000",
		}},
	})
	positionEvent := <-exec.Events()
	require.NotNil(t, positionEvent.Position)
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)
	require.True(t, decimal.RequireFromString("0.2").Equal(positionEvent.Position.Quantity))

	require.NoError(t, exec.ResubscribeExecution(context.Background()))
	require.Equal(t, 6, ws.subscribeCount)
}

type fakeSDK struct {
	placed bitgetsdk.PlaceOrderRequest
}

func (f *fakeSDK) GetInstruments(_ context.Context, category, _ string) ([]bitgetsdk.Instrument, error) {
	switch category {
	case "SPOT":
		return []bitgetsdk.Instrument{{
			Symbol:             "BTCUSDT",
			Category:           "SPOT",
			BaseCoin:           "BTC",
			QuoteCoin:          "USDT",
			PriceMultiplier:    "0.01",
			QuantityMultiplier: "0.0001",
			MakerFeeRate:       "0.001",
			TakerFeeRate:       "0.0015",
			Status:             "online",
		}}, nil
	case "USDT-FUTURES":
		return []bitgetsdk.Instrument{{
			Symbol:             "BTCUSDT",
			Category:           "USDT-FUTURES",
			BaseCoin:           "BTC",
			QuoteCoin:          "USDT",
			PriceMultiplier:    "0.1",
			QuantityMultiplier: "0.001",
			MakerFeeRate:       "0.0002",
			TakerFeeRate:       "0.0006",
			Status:             "online",
		}}, nil
	default:
		return nil, nil
	}
}

func (f *fakeSDK) GetTicker(context.Context, string, string) (*bitgetsdk.Ticker, error) {
	return &bitgetsdk.Ticker{LastPrice: "10", Bid1Price: "9", Ask1Price: "11", Timestamp: "1000"}, nil
}

func (f *fakeSDK) GetOrderBook(context.Context, string, string, int) (*bitgetsdk.OrderBook, error) {
	return &bitgetsdk.OrderBook{
		Bids: [][]bitgetsdk.NumberString{{"9", "1"}},
		Asks: [][]bitgetsdk.NumberString{{"11", "1"}},
		TS:   "2000",
	}, nil
}

func (f *fakeSDK) GetHistoryFundRate(context.Context, string, string, int, int) ([]bitgetsdk.HistoryFundRateEntry, error) {
	return []bitgetsdk.HistoryFundRateEntry{{
		Symbol:      "BTCUSDT",
		FundingRate: "0.0009",
		FundingTime: "1000",
	}}, nil
}

func (f *fakeSDK) GetAccountAssets(context.Context) (*bitgetsdk.AccountAssets, error) {
	return &bitgetsdk.AccountAssets{Assets: []bitgetsdk.AccountAsset{{
		Coin:      "USDT",
		Available: "5",
		Frozen:    "1",
		Equity:    "6",
	}}}, nil
}

func (f *fakeSDK) PlaceOrder(_ context.Context, req *bitgetsdk.PlaceOrderRequest) (*bitgetsdk.PlaceOrderResponse, error) {
	f.placed = *req
	return &bitgetsdk.PlaceOrderResponse{OrderID: "100", ClientOID: req.ClientOID}, nil
}

func (f *fakeSDK) CancelOrder(context.Context, *bitgetsdk.CancelOrderRequest) (*bitgetsdk.CancelOrderResponse, error) {
	return &bitgetsdk.CancelOrderResponse{OrderID: "100"}, nil
}

func (f *fakeSDK) GetOpenOrders(context.Context, string, string) ([]bitgetsdk.OrderRecord, error) {
	return []bitgetsdk.OrderRecord{{
		OrderID:     "100",
		ClientOID:   "client-1",
		Symbol:      "BTCUSDT",
		Side:        "buy",
		OrderType:   "limit",
		Price:       "10",
		Qty:         "1",
		FilledQty:   "0",
		OrderStatus: "live",
	}}, nil
}

type fakePublicWS struct {
	connected      bool
	subscribeCount int
	lastArg        bitgetsdk.WSArg
	unsubArg       bitgetsdk.WSArg
	handlers       map[string]func(json.RawMessage)
}

func (f *fakePublicWS) Subscribe(_ context.Context, arg bitgetsdk.WSArg, handler func(json.RawMessage)) error {
	f.connected = true
	f.subscribeCount++
	f.lastArg = arg
	if f.handlers == nil {
		f.handlers = make(map[string]func(json.RawMessage))
	}
	f.handlers[arg.Channel] = handler
	return nil
}

func (f *fakePublicWS) Unsubscribe(_ context.Context, arg bitgetsdk.WSArg) error {
	f.unsubArg = arg
	return nil
}

func (f *fakePublicWS) Close() error {
	f.connected = false
	return nil
}

func (f *fakePublicWS) emit(arg bitgetsdk.WSArg, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	f.handlers[arg.Channel](data)
}

type fakePrivateWS struct {
	connected      bool
	subscribeCount int
	handlers       map[string]func(json.RawMessage)
}

func (f *fakePrivateWS) Subscribe(_ context.Context, arg bitgetsdk.WSArg, handler func(json.RawMessage)) error {
	f.connected = true
	f.subscribeCount++
	if f.handlers == nil {
		f.handlers = make(map[string]func(json.RawMessage))
	}
	f.handlers[arg.Topic] = handler
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
