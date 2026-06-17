package aster

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	asterperp "github.com/QuantProcessing/exchanges/sdk/aster/perp"
	asterspot "github.com/QuantProcessing/exchanges/sdk/aster/spot"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSpotClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	data := newSpotDataClient("aster-spot-data", provider, sdk)
	data.ws = &fakeAsterSpotMarketWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newSpotExecutionClient("spot-acct", provider, sdk),
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Capabilities: (&SpotAdapter{}).Capabilities(),
	})
}

func TestPerpClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	data := newPerpDataClient("aster-perp-data", provider, sdk)
	data.ws = &fakeAsterPerpMarketWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newPerpExecutionClient("perp-acct", provider, sdk),
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Capabilities: (&PerpAdapter{}).Capabilities(),
	})
}

func TestSpotSubmitMapsOrderRequest(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newSpotExecutionClient("acct", provider, sdk)
	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", sdk.spotPlaced.Symbol)
	require.Equal(t, "BUY", sdk.spotPlaced.Side)
}

func TestSpotDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeAsterSpotMarketWS{}
	client := newSpotDataClient("aster-spot-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Type:         model.MarketDataTypeQuoteTick,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Type:         model.MarketDataTypeTradeTick,
	}))
	barType := model.NewTimeBarType(model.MustInstrumentID("BTC-USDT-SPOT.ASTER"), time.Minute)
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, 5, ws.connects)
	require.Equal(t, "BTCUSDT", ws.bookTickerSymbol)
	require.Equal(t, "BTCUSDT", ws.tradeSymbol)
	require.Equal(t, "BTCUSDT", ws.klineSymbol)
	require.Equal(t, "1m", ws.klineInterval)
	require.Equal(t, "BTCUSDT", ws.depthSymbol)

	require.NoError(t, ws.bookTickerHandler(&asterspot.BookTickerEvent{Symbol: "BTCUSDT", BestBidPrice: "100", BestBidQty: "1.5", BestAskPrice: "101", BestAskQty: "2.5"}))
	tickerEvent := requireAsterMarketEvent(t, streaming.Events())
	require.Equal(t, "100", tickerEvent.Ticker.Bid.String())
	require.Equal(t, "101", tickerEvent.Ticker.Ask.String())
	require.Equal(t, "100.5", tickerEvent.Ticker.Last.String())
	quoteEvent := requireAsterQuoteEvent(t, streaming.Events())
	require.Equal(t, "100", quoteEvent.Quote.BidPrice.String())
	require.Equal(t, "2.5", quoteEvent.Quote.AskSize.String())

	require.NoError(t, ws.tradeHandler(&asterspot.AggTradeEvent{WsEventHeader: asterspot.WsEventHeader{EventTime: 1710000000100, Symbol: "BTCUSDT"}, AggTradeID: 42, Price: "100.5", Quantity: "0.3", TradeTime: 1710000000090, IsBuyerMaker: false}))
	tradeEvent := requireAsterMarketEvent(t, streaming.Events())
	require.Equal(t, "100.5", tradeEvent.Trade.Price.String())
	require.Equal(t, model.TradeID("42"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)

	kline := &asterspot.KlineEvent{WsEventHeader: asterspot.WsEventHeader{EventTime: 1710000060000, Symbol: "BTCUSDT"}}
	kline.Kline.StartTime = 1710000000000
	kline.Kline.CloseTime = 1710000060000
	kline.Kline.OpenPrice = "100"
	kline.Kline.HighPrice = "110"
	kline.Kline.LowPrice = "95"
	kline.Kline.ClosePrice = "105"
	kline.Kline.Volume = "12.5"
	require.NoError(t, ws.klineHandler(kline))
	barEvent := requireAsterMarketEvent(t, streaming.Events())
	require.Equal(t, barType.Canonical(), barEvent.Bar.BarType)
	require.Equal(t, "105", barEvent.Bar.Close.String())
	require.Equal(t, "12.5", barEvent.Bar.Volume.String())

	require.NoError(t, ws.depthHandler(&asterspot.DepthEvent{WsEventHeader: asterspot.WsEventHeader{EventTime: 1710000000000, Symbol: "BTCUSDT"}, Bids: [][]string{{"100", "1.2"}}, Asks: [][]string{{"101", "0.7"}}}))
	bookEvent := requireAsterMarketEvent(t, streaming.Events())
	require.Equal(t, "100", bookEvent.OrderBook.Bids[0].Price.String())
	require.Equal(t, "101", bookEvent.OrderBook.Asks[0].Price.String())

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.Empty(t, ws.unsubBookTickerStream)
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Type:         model.MarketDataTypeQuoteTick,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Type:         model.MarketDataTypeTradeTick,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.ASTER"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, "btcusdt@bookTicker", ws.unsubBookTickerStream)
	require.Equal(t, "btcusdt@aggTrade", ws.unsubTradeStream)
	require.Equal(t, "btcusdt@kline_1m", ws.unsubKlineStream)
	require.Equal(t, "BTCUSDT", ws.unsubDepthSymbol)
}

func TestSpotDataClientRestTickerUsesVenueTimestamp(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newSpotDataClient("aster-spot-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-SPOT.ASTER")

	ticker, err := client.FetchTicker(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(1000), ticker.Timestamp)
}

func TestSpotExecutionClientPrivateStreamMapsOrdersFillsAndBalances(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeAsterSpotAccountWS{}
	exec := newSpotExecutionClient("acct", provider, sdk)
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.Equal(t, 1, ws.connects)
	require.Equal(t, 1, ws.executionSubscriptions)
	require.Equal(t, 1, ws.accountSubscriptions)

	ws.executionHandler(&asterspot.ExecutionReportEvent{
		EventTime:                1710000000000,
		Symbol:                   "BTCUSDT",
		ClientOrderID:            "client-1",
		Side:                     "BUY",
		OrderType:                "LIMIT",
		Quantity:                 "1.5",
		Price:                    "100",
		OrderStatus:              "PARTIALLY_FILLED",
		OrderID:                  100,
		LastExecutedQuantity:     "0.5",
		CumulativeFilledQuantity: "0.5",
		LastExecutedPrice:        "100",
		CommissionAmount:         "0.01",
		CommissionAsset:          "USDT",
		TransactionTime:          1710000000001,
		TradeID:                  55,
	})
	orderEvent := requireAsterExecutionEvent(t, exec.Events())
	require.Equal(t, model.OrderID("100"), orderEvent.Order.OrderID)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)
	require.Equal(t, "1", orderEvent.Order.LeavesQuantity.String())
	fillEvent := requireAsterExecutionEvent(t, exec.Events())
	require.Equal(t, model.TradeID("55"), fillEvent.Fill.TradeID)
	require.Equal(t, "0.5", fillEvent.Fill.Quantity.String())

	ws.accountHandler(mustSpotAccountPosition(`{"e":"outboundAccountPosition","E":1710000000002,"B":[{"a":"USDT","f":"5","l":"1"}]}`))
	accountEvent := requireAsterExecutionEvent(t, exec.Events())
	require.Equal(t, model.Currency("USDT"), accountEvent.Account.Balances[0].Currency)
	require.Equal(t, "6", accountEvent.Account.Balances[0].Total)

	var resubscriber venue.ExecutionResubscriber = exec
	require.NoError(t, resubscriber.ResubscribeExecution(context.Background()))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, 2, ws.executionSubscriptions)
	require.Equal(t, 2, ws.accountSubscriptions)
}

func TestPerpDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeAsterPerpMarketWS{}
	client := newPerpDataClient("aster-perp-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Type:         model.MarketDataTypeQuoteTick,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Type:         model.MarketDataTypeTradeTick,
	}))
	barType := model.NewTimeBarType(model.MustInstrumentID("BTC-USDT-PERP.ASTER"), time.Minute)
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, 5, ws.connects)
	require.Equal(t, "btcusdt", ws.bookTickerSymbol)
	require.Equal(t, "btcusdt", ws.tradeSymbol)
	require.Equal(t, "btcusdt", ws.klineSymbol)
	require.Equal(t, "1m", ws.klineInterval)
	require.Equal(t, "btcusdt", ws.depthSymbol)

	require.NoError(t, ws.bookTickerHandler(&asterperp.WsBookTickerEvent{Symbol: "BTCUSDT", EventTime: 1710000000000, BestBidPrice: "100", BestBidQty: "1.5", BestAskPrice: "101", BestAskQty: "2.5"}))
	tickerEvent := requireAsterMarketEvent(t, streaming.Events())
	require.Equal(t, "100", tickerEvent.Ticker.Bid.String())
	require.Equal(t, "101", tickerEvent.Ticker.Ask.String())
	quoteEvent := requireAsterQuoteEvent(t, streaming.Events())
	require.Equal(t, "100", quoteEvent.Quote.BidPrice.String())
	require.Equal(t, "2.5", quoteEvent.Quote.AskSize.String())

	require.NoError(t, ws.tradeHandler(&asterperp.WsAggTradeEvent{EventTime: 1710000000100, Symbol: "BTCUSDT", AggTradeID: 43, Price: "100.5", Quantity: "0.4", TradeTime: 1710000000090, IsBuyerMaker: true}))
	tradeEvent := requireAsterMarketEvent(t, streaming.Events())
	require.Equal(t, "100.5", tradeEvent.Trade.Price.String())
	require.Equal(t, model.TradeID("43"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideSeller, tradeEvent.Trade.AggressorSide)

	kline := &asterperp.WsKlineEvent{EventTime: 1710000060000, Symbol: "BTCUSDT"}
	kline.Kline.StartTime = 1710000000000
	kline.Kline.EndTime = 1710000060000
	kline.Kline.OpenPrice = "100"
	kline.Kline.HighPrice = "110"
	kline.Kline.LowPrice = "95"
	kline.Kline.ClosePrice = "105"
	kline.Kline.Volume = "12.5"
	require.NoError(t, ws.klineHandler(kline))
	barEvent := requireAsterMarketEvent(t, streaming.Events())
	require.Equal(t, barType.Canonical(), barEvent.Bar.BarType)
	require.Equal(t, "105", barEvent.Bar.Close.String())

	require.NoError(t, ws.depthHandler(&asterperp.WsDepthEvent{Symbol: "BTCUSDT", EventTime: 1710000000001, Bids: [][]interface{}{{"100", "1.2"}}, Asks: [][]interface{}{{"101", "0.7"}}}))
	bookEvent := requireAsterMarketEvent(t, streaming.Events())
	require.Equal(t, "100", bookEvent.OrderBook.Bids[0].Price.String())
	require.Equal(t, "101", bookEvent.OrderBook.Asks[0].Price.String())

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.Empty(t, ws.unsubBookTickerSymbol)
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Type:         model.MarketDataTypeQuoteTick,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Type:         model.MarketDataTypeTradeTick,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.ASTER"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, "btcusdt", ws.unsubBookTickerSymbol)
	require.Equal(t, "btcusdt", ws.unsubTradeSymbol)
	require.Equal(t, "btcusdt", ws.unsubKlineSymbol)
	require.Equal(t, "btcusdt", ws.unsubDepthSymbol)
}

func TestPerpDataClientRestSnapshotsUseVenueTimestamps(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newPerpDataClient("aster-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.ASTER")

	ticker, err := client.FetchTicker(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(1000), ticker.Timestamp)

	book, err := client.FetchOrderBook(context.Background(), id, 20)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(2000), book.Timestamp)
}

func TestPerpDataClientFetchFundingRate(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newPerpDataClient("aster-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.ASTER")

	funding, err := client.FetchFundingRate(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, funding.InstrumentID)
	require.True(t, decimal.RequireFromString("0.0008").Equal(funding.Rate))
	require.Zero(t, funding.FundingInterval)
	require.Equal(t, time.UnixMilli(1000), funding.Timestamp)
	require.Equal(t, time.UnixMilli(28800000), funding.NextFundingTime)
	require.True(t, sdk.getAllFundingRatesCalled)
	require.Empty(t, sdk.getFundingRateSymbols)
}

func TestPerpDataClientFetchFundingRateReturnsAllFundingError(t *testing.T) {
	allFundingErr := errors.New("all funding request failed")
	sdk := &fakePerpSDK{allFundingRatesErr: allFundingErr}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newPerpDataClient("aster-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.ASTER")

	_, err := client.FetchFundingRate(context.Background(), id)
	require.ErrorIs(t, err, allFundingErr)
	require.True(t, sdk.getAllFundingRatesCalled)
	require.Empty(t, sdk.getFundingRateSymbols)
}

func TestPerpExecutionClientPrivateStreamMapsOrdersFillsAndPositions(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeAsterPerpAccountWS{}
	exec := newPerpExecutionClient("acct", provider, sdk)
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.Equal(t, 1, ws.connects)
	require.Equal(t, 1, ws.orderSubscriptions)
	require.Equal(t, 1, ws.accountSubscriptions)

	order := &asterperp.OrderUpdateEvent{EventTime: 1710000000000, TransactionTime: 1710000000001}
	order.Order.Symbol = "BTCUSDT"
	order.Order.ClientOrderID = "client-1"
	order.Order.Side = "BUY"
	order.Order.OrderType = "LIMIT"
	order.Order.OriginalQty = "1.5"
	order.Order.OriginalPrice = "100"
	order.Order.OrderStatus = "PARTIALLY_FILLED"
	order.Order.OrderID = 200
	order.Order.LastFilledQty = "0.5"
	order.Order.AccumulatedFilledQty = "0.5"
	order.Order.LastFilledPrice = "100"
	order.Order.Commission = "0.01"
	order.Order.CommissionAsset = "USDT"
	order.Order.TradeID = 55
	ws.orderHandler(order)
	orderEvent := requireAsterExecutionEvent(t, exec.Events())
	require.Equal(t, model.OrderID("200"), orderEvent.Order.OrderID)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)
	require.Equal(t, "1", orderEvent.Order.LeavesQuantity.String())
	fillEvent := requireAsterExecutionEvent(t, exec.Events())
	require.Equal(t, model.TradeID("55"), fillEvent.Fill.TradeID)
	require.Equal(t, "0.5", fillEvent.Fill.Quantity.String())

	ws.accountHandler(mustPerpAccountUpdate(`{"e":"ACCOUNT_UPDATE","E":1710000000002,"a":{"P":[{"s":"BTCUSDT","pa":"-0.25","ep":"10000","ps":"BOTH"}]}}`))
	positionEvent := requireAsterExecutionEvent(t, exec.Events())
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)
	require.Equal(t, "0.25", positionEvent.Position.Quantity.String())
	require.Equal(t, "10000", positionEvent.Position.EntryPrice.String())

	var resubscriber venue.ExecutionResubscriber = exec
	require.NoError(t, resubscriber.ResubscribeExecution(context.Background()))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, 2, ws.orderSubscriptions)
	require.Equal(t, 2, ws.accountSubscriptions)
}

func TestPerpGeneratePositionStatusReportsUsesAccountPositions(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newPerpExecutionClient("acct", provider, sdk)

	var generator venue.PositionStatusReportGenerator = exec
	reports, err := generator.GeneratePositionStatusReports(context.Background(), model.MustInstrumentID("BTC-USDT-PERP.ASTER"))
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.PositionSideLong, reports[0].Side)
	require.Equal(t, "0.75", reports[0].Quantity.String())
	require.Equal(t, "12000", reports[0].EntryPrice.String())
}

func requireAsterMarketEvent(t *testing.T, events <-chan model.MarketEvent) model.MarketEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for market event")
		return model.MarketEvent{}
	}
}

func requireAsterQuoteEvent(t *testing.T, events <-chan model.MarketEvent) model.MarketEvent {
	t.Helper()
	for i := 0; i < 3; i++ {
		event := requireAsterMarketEvent(t, events)
		if event.Quote != nil {
			return event
		}
	}
	require.Fail(t, "quote event not found")
	return model.MarketEvent{}
}

func requireAsterExecutionEvent(t *testing.T, events <-chan model.ExecutionEvent) model.ExecutionEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for execution event")
		return model.ExecutionEvent{}
	}
}

func mustSpotAccountPosition(raw string) *asterspot.AccountPositionEvent {
	var event asterspot.AccountPositionEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		panic(err)
	}
	return &event
}

func mustPerpAccountUpdate(raw string) *asterperp.AccountUpdateEvent {
	var event asterperp.AccountUpdateEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		panic(err)
	}
	return &event
}

func mustPerpAccount(raw string) *asterperp.AccountResponse {
	var account asterperp.AccountResponse
	if err := json.Unmarshal([]byte(raw), &account); err != nil {
		panic(err)
	}
	return &account
}

type fakeSpotSDK struct {
	spotPlaced asterspot.PlaceOrderParams
}

func (f *fakeSpotSDK) ExchangeInfo(context.Context) (*asterspot.ExchangeInfoResponse, error) {
	return &asterspot.ExchangeInfoResponse{Symbols: []struct {
		Symbol             string                   `json:"symbol"`
		Status             string                   `json:"status"`
		BaseAsset          string                   `json:"baseAsset"`
		QuoteAsset         string                   `json:"quoteAsset"`
		PricePrecision     int                      `json:"pricePrecision"`
		QuantityPrecision  int                      `json:"quantityPrecision"`
		BaseAssetPrecision int                      `json:"baseAssetPrecision"`
		QuotePrecision     int                      `json:"quotePrecision"`
		Filters            []map[string]interface{} `json:"filters"`
	}{{
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

func (f *fakeSpotSDK) Ticker(context.Context, string) (*asterspot.TickerResponse, error) {
	return &asterspot.TickerResponse{Symbol: "BTCUSDT", LastPrice: "10", BidPrice: "9", AskPrice: "11", CloseTime: 1000}, nil
}

func (f *fakeSpotSDK) Depth(context.Context, string, int) (*asterspot.DepthResponse, error) {
	return &asterspot.DepthResponse{Bids: [][]string{{"9", "1"}}, Asks: [][]string{{"11", "1"}}}, nil
}

func (f *fakeSpotSDK) GetAccount(context.Context) (*asterspot.AccountResponse, error) {
	return &asterspot.AccountResponse{Balances: []struct {
		Asset  string `json:"asset"`
		Free   string `json:"free"`
		Locked string `json:"locked"`
	}{{Asset: "USDT", Free: "5", Locked: "1"}}}, nil
}

func (f *fakeSpotSDK) PlaceOrder(_ context.Context, p asterspot.PlaceOrderParams) (*asterspot.OrderResponse, error) {
	f.spotPlaced = p
	return &asterspot.OrderResponse{Symbol: p.Symbol, OrderID: 100, ClientOrderID: p.NewClientOrderID, Status: "NEW", Side: p.Side, Type: p.Type, OrigQty: p.Quantity, Price: p.Price}, nil
}

func (f *fakeSpotSDK) CancelOrder(context.Context, string, int64, string) (*asterspot.CancelOrderResponse, error) {
	return &asterspot.CancelOrderResponse{OrderID: 100, Status: "CANCELED"}, nil
}

func (f *fakeSpotSDK) GetOpenOrders(context.Context, string) ([]asterspot.OrderResponse, error) {
	return []asterspot.OrderResponse{{Symbol: "BTCUSDT", OrderID: 100, Status: "NEW"}}, nil
}

type fakePerpSDK struct {
	perpPlaced               asterperp.PlaceOrderParams
	getAllFundingRatesCalled bool
	getFundingRateSymbols    []string
	allFundingRatesErr       error
}

func (f *fakePerpSDK) ExchangeInfo(context.Context) (*asterperp.ExchangeInfoResponse, error) {
	return &asterperp.ExchangeInfoResponse{Symbols: []asterperp.SymbolInfo{{
		Symbol:      "BTCUSDT",
		Status:      "TRADING",
		BaseAsset:   "BTC",
		QuoteAsset:  "USDT",
		MarginAsset: "USDT",
		Filters: []map[string]interface{}{
			{"filterType": "PRICE_FILTER", "tickSize": "0.1"},
			{"filterType": "LOT_SIZE", "stepSize": "0.001"},
		},
	}}}, nil
}

func (f *fakePerpSDK) Ticker(context.Context, string) (*asterperp.TickerResponse, error) {
	return &asterperp.TickerResponse{Symbol: "BTCUSDT", LastPrice: "20", CloseTime: 1000}, nil
}

func (f *fakePerpSDK) Depth(context.Context, string, int) (*asterperp.DepthResponse, error) {
	return &asterperp.DepthResponse{E: 2000, T: 1900, Bids: [][]string{{"19", "1"}}, Asks: [][]string{{"21", "1"}}}, nil
}

func (f *fakePerpSDK) GetFundingRate(_ context.Context, symbol string) (*asterperp.FundingRateData, error) {
	f.getFundingRateSymbols = append(f.getFundingRateSymbols, symbol)
	return &asterperp.FundingRateData{
		Symbol:          "BTCUSDT",
		LastFundingRate: "0.0008",
		MarkPrice:       "200",
		IndexPrice:      "199",
		NextFundingTime: 28800000,
		Time:            1000,
	}, nil
}

func (f *fakePerpSDK) GetAllFundingRates(context.Context) ([]asterperp.FundingRateData, error) {
	f.getAllFundingRatesCalled = true
	return []asterperp.FundingRateData{
		{Symbol: "ETHUSDT", LastFundingRate: "0.0001", MarkPrice: "100", IndexPrice: "99", NextFundingTime: 14400000, Time: 900},
		{Symbol: "BTCUSDT", LastFundingRate: "0.0008", MarkPrice: "200", IndexPrice: "199", NextFundingTime: 28800000, Time: 1000},
	}, f.allFundingRatesErr
}

func (f *fakePerpSDK) GetAccount(context.Context) (*asterperp.AccountResponse, error) {
	return mustPerpAccount(`{"assets":[{"asset":"USDT","walletBalance":"20","availableBalance":"18"}],"positions":[{"symbol":"BTCUSDT","positionAmt":"0.75","entryPrice":"12000","positionSide":"BOTH","updateTime":1710000000000}]}`), nil
}

func (f *fakePerpSDK) PlaceOrder(_ context.Context, p asterperp.PlaceOrderParams) (*asterperp.OrderResponse, error) {
	f.perpPlaced = p
	return &asterperp.OrderResponse{Symbol: p.Symbol, OrderID: 200, ClientOrderID: p.NewClientOrderID, Status: "NEW", Side: p.Side, Type: string(p.Type), OrigQty: p.Quantity, Price: p.Price}, nil
}

func (f *fakePerpSDK) CancelOrder(context.Context, asterperp.CancelOrderParams) (*asterperp.OrderResponse, error) {
	return &asterperp.OrderResponse{OrderID: 200, Status: "CANCELED"}, nil
}

func (f *fakePerpSDK) GetOpenOrders(context.Context, string) ([]asterperp.OrderResponse, error) {
	return []asterperp.OrderResponse{{Symbol: "BTCUSDT", OrderID: 200, Status: "NEW"}}, nil
}

type fakeAsterSpotMarketWS struct {
	connects              int
	bookTickerSymbol      string
	depthSymbol           string
	depth                 int
	speed                 string
	unsubBookTickerStream string
	unsubTradeStream      string
	unsubKlineStream      string
	unsubDepthSymbol      string
	bookTickerHandler     func(*asterspot.BookTickerEvent) error
	tradeHandler          func(*asterspot.AggTradeEvent) error
	klineHandler          func(*asterspot.KlineEvent) error
	depthHandler          func(*asterspot.DepthEvent) error
	tradeSymbol           string
	klineSymbol           string
	klineInterval         string
}

func (f *fakeAsterSpotMarketWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeAsterSpotMarketWS) SubscribeBookTicker(symbol string, handler func(*asterspot.BookTickerEvent) error) error {
	f.bookTickerSymbol = symbol
	f.bookTickerHandler = handler
	return nil
}

func (f *fakeAsterSpotMarketWS) SubscribeLimitOrderBook(symbol string, depth int, speed string, handler func(*asterspot.DepthEvent) error) error {
	f.depthSymbol = symbol
	f.depth = depth
	f.speed = speed
	f.depthHandler = handler
	return nil
}

func (f *fakeAsterSpotMarketWS) SubscribeAggTrade(symbol string, handler func(*asterspot.AggTradeEvent) error) error {
	f.tradeSymbol = symbol
	f.tradeHandler = handler
	return nil
}

func (f *fakeAsterSpotMarketWS) SubscribeKline(symbol string, interval string, handler func(*asterspot.KlineEvent) error) error {
	f.klineSymbol = symbol
	f.klineInterval = interval
	f.klineHandler = handler
	return nil
}

func (f *fakeAsterSpotMarketWS) Unsubscribe(stream string) error {
	switch {
	case strings.Contains(stream, "@aggTrade"):
		f.unsubTradeStream = stream
	case strings.Contains(stream, "@kline_"):
		f.unsubKlineStream = stream
	default:
		f.unsubBookTickerStream = stream
	}
	return nil
}

func (f *fakeAsterSpotMarketWS) UnsubscribeLimitOrderBook(symbol string, depth int, speed string) error {
	f.unsubDepthSymbol = symbol
	return nil
}

func (f *fakeAsterSpotMarketWS) Close() {}

type fakeAsterSpotAccountWS struct {
	connects               int
	executionSubscriptions int
	accountSubscriptions   int
	executionHandler       func(*asterspot.ExecutionReportEvent)
	accountHandler         func(*asterspot.AccountPositionEvent)
}

func (f *fakeAsterSpotAccountWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeAsterSpotAccountWS) SubscribeExecutionReport(handler func(*asterspot.ExecutionReportEvent)) {
	f.executionSubscriptions++
	f.executionHandler = handler
}

func (f *fakeAsterSpotAccountWS) SubscribeAccountPosition(handler func(*asterspot.AccountPositionEvent)) {
	f.accountSubscriptions++
	f.accountHandler = handler
}

func (f *fakeAsterSpotAccountWS) Close() {}

type fakeAsterPerpMarketWS struct {
	connects              int
	bookTickerSymbol      string
	depthSymbol           string
	depth                 int
	speed                 string
	unsubBookTickerSymbol string
	unsubTradeSymbol      string
	unsubKlineSymbol      string
	unsubDepthSymbol      string
	bookTickerHandler     func(*asterperp.WsBookTickerEvent) error
	tradeHandler          func(*asterperp.WsAggTradeEvent) error
	klineHandler          func(*asterperp.WsKlineEvent) error
	depthHandler          func(*asterperp.WsDepthEvent) error
	tradeSymbol           string
	klineSymbol           string
	klineInterval         string
}

func (f *fakeAsterPerpMarketWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeAsterPerpMarketWS) SubscribeBookTicker(symbol string, handler func(*asterperp.WsBookTickerEvent) error) error {
	f.bookTickerSymbol = symbol
	f.bookTickerHandler = handler
	return nil
}

func (f *fakeAsterPerpMarketWS) SubscribeLimitOrderBook(symbol string, depth int, speed string, handler func(*asterperp.WsDepthEvent) error) error {
	f.depthSymbol = symbol
	f.depth = depth
	f.speed = speed
	f.depthHandler = handler
	return nil
}

func (f *fakeAsterPerpMarketWS) SubscribeAggTrade(symbol string, handler func(*asterperp.WsAggTradeEvent) error) error {
	f.tradeSymbol = symbol
	f.tradeHandler = handler
	return nil
}

func (f *fakeAsterPerpMarketWS) SubscribeKline(symbol string, interval string, handler func(*asterperp.WsKlineEvent) error) error {
	f.klineSymbol = symbol
	f.klineInterval = interval
	f.klineHandler = handler
	return nil
}

func (f *fakeAsterPerpMarketWS) UnsubscribeBookTicker(symbol string) error {
	f.unsubBookTickerSymbol = symbol
	return nil
}

func (f *fakeAsterPerpMarketWS) UnsubscribeAggTrade(symbol string) error {
	f.unsubTradeSymbol = symbol
	return nil
}

func (f *fakeAsterPerpMarketWS) UnsubscribeKline(symbol string, _ string) error {
	f.unsubKlineSymbol = symbol
	return nil
}

func (f *fakeAsterPerpMarketWS) UnsubscribeLimitOrderBook(symbol string, depth int, speed string) error {
	f.unsubDepthSymbol = symbol
	return nil
}

func (f *fakeAsterPerpMarketWS) Close() {}

type fakeAsterPerpAccountWS struct {
	connects             int
	orderSubscriptions   int
	accountSubscriptions int
	orderHandler         func(*asterperp.OrderUpdateEvent)
	accountHandler       func(*asterperp.AccountUpdateEvent)
}

func (f *fakeAsterPerpAccountWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeAsterPerpAccountWS) SubscribeOrderUpdate(handler func(*asterperp.OrderUpdateEvent)) {
	f.orderSubscriptions++
	f.orderHandler = handler
}

func (f *fakeAsterPerpAccountWS) SubscribeAccountUpdate(handler func(*asterperp.AccountUpdateEvent)) {
	f.accountSubscriptions++
	f.accountHandler = handler
}

func (f *fakeAsterPerpAccountWS) Close() {}
