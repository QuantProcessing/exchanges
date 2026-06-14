package hyperliquid

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	hlsdk "github.com/QuantProcessing/exchanges/sdk/hyperliquid"
	hlperp "github.com/QuantProcessing/exchanges/sdk/hyperliquid/perp"
	hlspot "github.com/QuantProcessing/exchanges/sdk/hyperliquid/spot"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSpotClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	data := newSpotDataClient("hyperliquid-spot-data", provider, sdk)
	data.ws = &fakeSpotMarketWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newSpotExecutionClient("spot-acct", "", provider, sdk),
		InstrumentID: model.MustInstrumentID("PURR-USDC-SPOT.HYPERLIQUID"),
		Capabilities: (&SpotAdapter{}).Capabilities(),
	})
}

func TestPerpClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	data := newPerpDataClient("hyperliquid-perp-data", provider, sdk)
	data.ws = &fakePerpMarketWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:           provider,
		Data:               data,
		Execution:          newPerpExecutionClient("perp-acct", "0xabc", provider, sdk),
		InstrumentID:       model.MustInstrumentID("BTC-USD-PERP.HYPERLIQUID"),
		Capabilities:       (&PerpAdapter{}).Capabilities(),
		ExpectedMarginInit: decimal.RequireFromString("0.02"),
	})
}

func TestSpotSubmitMapsAssetAndOrderFields(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newSpotExecutionClient("acct", "", provider, sdk)

	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("PURR-USDC-SPOT.HYPERLIQUID"),
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("2"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, 1, sdk.placed.AssetID)
	require.True(t, sdk.placed.IsBuy)
	require.Equal(t, hlsdk.TifGtc, sdk.placed.OrderType.Limit.Tif)
}

func TestSpotDataClientStreamsBboAndOrderBook(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeSpotMarketWS{}
	client := newSpotDataClient("hyperliquid-spot-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("PURR-USDC-SPOT.HYPERLIQUID"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("PURR-USDC-SPOT.HYPERLIQUID"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, "PURR/USDC", ws.bboCoin)
	require.Equal(t, "PURR/USDC", ws.l2Coin)

	ws.bboHandler(hlsdk.WsBbo{
		Coin: "PURR/USDC",
		Time: 1710000000000,
		Bbo:  []hlsdk.WsLevel{{Px: "10", Sz: "1"}, {Px: "11", Sz: "2"}},
	})
	tickerEvent := requireMarketEvent(t, streaming.Events())
	require.Equal(t, model.MustInstrumentID("PURR-USDC-SPOT.HYPERLIQUID"), tickerEvent.Ticker.InstrumentID)
	require.Equal(t, "10", tickerEvent.Ticker.Bid.String())
	require.Equal(t, "11", tickerEvent.Ticker.Ask.String())
	require.Equal(t, "10.5", tickerEvent.Ticker.Last.String())

	ws.l2Handler(hlsdk.WsL2Book{
		Coin:   "PURR/USDC",
		Time:   1710000000001,
		Levels: [][]hlsdk.WsLevel{{{Px: "10", Sz: "1.2"}}, {{Px: "11", Sz: "0.7"}}},
	})
	bookEvent := requireMarketEvent(t, streaming.Events())
	require.Equal(t, "10", bookEvent.OrderBook.Bids[0].Price.String())
	require.Equal(t, "1.2", bookEvent.OrderBook.Bids[0].Size.String())
	require.Equal(t, "11", bookEvent.OrderBook.Asks[0].Price.String())

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("PURR-USDC-SPOT.HYPERLIQUID"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("PURR-USDC-SPOT.HYPERLIQUID"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, "PURR/USDC", ws.unsubBboCoin)
	require.Equal(t, "PURR/USDC", ws.unsubL2Coin)
}

func TestSpotDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeSpotMarketWS{}
	client := newSpotDataClient("hyperliquid-spot-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	id := model.MustInstrumentID("PURR-USDC-SPOT.HYPERLIQUID")
	tickerSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}
	quoteSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), tickerSub))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 1, ws.bboSubscriptions)

	ws.bboHandler(hlsdk.WsBbo{Coin: "PURR/USDC", Time: 1710000000000, Bbo: []hlsdk.WsLevel{{Px: "10", Sz: "1.2"}, {Px: "11", Sz: "0.7"}}})
	firstEvent := requireMarketEvent(t, streaming.Events())
	secondEvent := requireMarketEvent(t, streaming.Events())
	if firstEvent.Quote == nil {
		firstEvent, secondEvent = secondEvent, firstEvent
	}
	require.NotNil(t, firstEvent.Quote)
	require.Equal(t, id, firstEvent.Quote.InstrumentID)
	require.Equal(t, "10", firstEvent.Quote.BidPrice.String())
	require.Equal(t, "1.2", firstEvent.Quote.BidSize.String())
	require.Equal(t, "11", firstEvent.Quote.AskPrice.String())
	require.Equal(t, "0.7", firstEvent.Quote.AskSize.String())
	require.NotNil(t, secondEvent.Ticker)

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 0, ws.unsubBboSubscriptions)
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), tickerSub))
	require.Equal(t, 1, ws.unsubBboSubscriptions)

	tradeSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTradeTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, "PURR/USDC", ws.tradesCoin)
	ws.tradesHandler([]hlsdk.WsTrade{{Coin: "PURR/USDC", Side: string(hlsdk.SideBid), Px: "10.25", Sz: "2.5", Tid: 55, Hash: "0xtrade", Time: 1710000000001}})
	tradeEvent := requireMarketEvent(t, streaming.Events())
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, model.TradeID("55"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)
	require.Equal(t, "10.25", tradeEvent.Trade.Price.String())
	require.Equal(t, "2.5", tradeEvent.Trade.Size.String())
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, 1, ws.unsubTradesSubscriptions)

	barType := model.NewTimeBarType(id, time.Minute)
	barSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeBar, BarType: barType}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), barSub))
	require.Equal(t, "PURR/USDC", ws.candleCoin)
	require.Equal(t, "1m", ws.candleInterval)
	ws.candleHandler(hlsdk.WsCandle{S: "PURR/USDC", I: "1m", T: 1710000000000, TClose: 1710000059999, O: "10", H: "12", L: "9", C: "11", V: "100"})
	barEvent := requireMarketEvent(t, streaming.Events())
	require.NotNil(t, barEvent.Bar)
	require.Equal(t, barType.String(), barEvent.Bar.BarType.String())
	require.Equal(t, "10", barEvent.Bar.Open.String())
	require.Equal(t, "12", barEvent.Bar.High.String())
	require.Equal(t, "9", barEvent.Bar.Low.String())
	require.Equal(t, "11", barEvent.Bar.Close.String())
	require.Equal(t, "100", barEvent.Bar.Volume.String())
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), barSub))
	require.Equal(t, 1, ws.unsubCandleSubscriptions)
}

func TestSpotExecutionClientPrivateStreamMapsOrdersAndFills(t *testing.T) {
	sdk := &fakeSpotSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeSpotAccountWS{}
	exec := newSpotExecutionClient("acct", "0xabc", provider, sdk)
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.Equal(t, 1, ws.connects)
	require.Equal(t, "0xabc", ws.orderUser)
	require.Equal(t, "0xabc", ws.fillUser)
	require.Equal(t, 1, ws.orderSubscriptions)
	require.Equal(t, 1, ws.fillSubscriptions)

	ws.orderHandler([]hlsdk.WsOrderUpdate{{
		Order: hlsdk.WsOrder{
			Coin:      "PURR/USDC",
			Side:      string(hlsdk.SideBid),
			LimitPx:   "10",
			Sz:        "1.5",
			OrigSz:    "2",
			Oid:       100,
			Cliod:     "client-1",
			Timestamp: 1710000000000,
		},
		Status:          hlsdk.StatusOpen,
		StatusTimestamp: 1710000000001,
	}})
	orderEvent := requireExecutionEvent(t, exec.Events())
	require.Equal(t, model.OrderID("100"), orderEvent.Order.OrderID)
	require.Equal(t, model.ClientOrderID("client-1"), orderEvent.Order.ClientOrderID)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)
	require.Equal(t, "0.5", orderEvent.Order.FilledQuantity.String())
	require.Equal(t, "1.5", orderEvent.Order.LeavesQuantity.String())

	ws.fillHandler(hlsdk.WsUserFills{User: "0xabc", Fills: []hlsdk.WsUserFill{{
		Coin:     "PURR/USDC",
		Side:     string(hlsdk.SideBid),
		Px:       "10",
		Sz:       "0.5",
		Oid:      100,
		Tid:      55,
		Fee:      "0.01",
		FeeToken: "USDC",
		Time:     1710000000002,
	}}})
	fillEvent := requireExecutionEvent(t, exec.Events())
	require.Equal(t, model.TradeID("55"), fillEvent.Fill.TradeID)
	require.Equal(t, model.OrderID("100"), fillEvent.Fill.OrderID)
	require.Equal(t, model.OrderSideBuy, fillEvent.Fill.Side)
	require.Equal(t, "0.5", fillEvent.Fill.Quantity.String())

	var resubscriber venue.ExecutionResubscriber = exec
	require.NoError(t, resubscriber.ResubscribeExecution(context.Background()))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, 2, ws.orderSubscriptions)
	require.Equal(t, 2, ws.fillSubscriptions)
}

func TestSpotGenerateOrderStatusReportsUsesOpenOrders(t *testing.T) {
	sdk := &fakeSpotSDK{openOrders: []hlspot.Order{{
		Coin:    "PURR/USDC",
		Side:    string(hlsdk.SideAsk),
		LimitPx: "10",
		Sz:      "1",
		OrigSz:  "1.5",
		Oid:     100,
	}}}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newSpotExecutionClient("acct", "0xabc", provider, sdk)

	reports, err := exec.GenerateOrderStatusReports(context.Background(), model.MustInstrumentID("PURR-USDC-SPOT.HYPERLIQUID"))
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.OrderID("100"), reports[0].OrderID)
	require.Equal(t, model.OrderSideSell, reports[0].Side)
	require.Equal(t, model.OrderStatusPartiallyFilled, reports[0].Status)
	require.Equal(t, "0.5", reports[0].FilledQuantity.String())
	require.Equal(t, "1", reports[0].LeavesQuantity.String())
}

func TestPerpSubmitMapsAssetAndOrderFields(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newPerpExecutionClient("acct", "0xabc", provider, sdk)

	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USD-PERP.HYPERLIQUID"),
		ClientOrderID: "client-2",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("1.5"),
	})
	require.NoError(t, err)
	require.Equal(t, 0, sdk.placed.AssetID)
	require.False(t, sdk.placed.IsBuy)
	require.Equal(t, hlsdk.TifIoc, sdk.placed.OrderType.Limit.Tif)
}

func TestPerpDataClientStreamsBboAndOrderBook(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakePerpMarketWS{}
	client := newPerpDataClient("hyperliquid-perp-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USD-PERP.HYPERLIQUID"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USD-PERP.HYPERLIQUID"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, "BTC", ws.bboCoin)
	require.Equal(t, "BTC", ws.l2Coin)

	ws.bboHandler(hlsdk.WsBbo{
		Coin: "BTC",
		Time: 1710000000000,
		Bbo:  []hlsdk.WsLevel{{Px: "100", Sz: "1"}, {Px: "101", Sz: "2"}},
	})
	tickerEvent := requireMarketEvent(t, streaming.Events())
	require.Equal(t, model.MustInstrumentID("BTC-USD-PERP.HYPERLIQUID"), tickerEvent.Ticker.InstrumentID)
	require.Equal(t, "100", tickerEvent.Ticker.Bid.String())
	require.Equal(t, "101", tickerEvent.Ticker.Ask.String())
	require.Equal(t, "100.5", tickerEvent.Ticker.Last.String())

	ws.l2Handler(hlsdk.WsL2Book{
		Coin:   "BTC",
		Time:   1710000000001,
		Levels: [][]hlsdk.WsLevel{{{Px: "100", Sz: "1.2"}}, {{Px: "101", Sz: "0.7"}}},
	})
	bookEvent := requireMarketEvent(t, streaming.Events())
	require.Equal(t, "100", bookEvent.OrderBook.Bids[0].Price.String())
	require.Equal(t, "1.2", bookEvent.OrderBook.Bids[0].Size.String())
	require.Equal(t, "101", bookEvent.OrderBook.Asks[0].Price.String())

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USD-PERP.HYPERLIQUID"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USD-PERP.HYPERLIQUID"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, "BTC", ws.unsubBboCoin)
	require.Equal(t, "BTC", ws.unsubL2Coin)
}

func TestPerpDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakePerpMarketWS{}
	client := newPerpDataClient("hyperliquid-perp-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	id := model.MustInstrumentID("BTC-USD-PERP.HYPERLIQUID")
	tickerSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}
	quoteSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), tickerSub))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 1, ws.bboSubscriptions)

	ws.bboHandler(hlsdk.WsBbo{Coin: "BTC", Time: 1710000000000, Bbo: []hlsdk.WsLevel{{Px: "100", Sz: "1.2"}, {Px: "101", Sz: "0.7"}}})
	firstEvent := requireMarketEvent(t, streaming.Events())
	secondEvent := requireMarketEvent(t, streaming.Events())
	if firstEvent.Quote == nil {
		firstEvent, secondEvent = secondEvent, firstEvent
	}
	require.NotNil(t, firstEvent.Quote)
	require.Equal(t, id, firstEvent.Quote.InstrumentID)
	require.Equal(t, "100", firstEvent.Quote.BidPrice.String())
	require.Equal(t, "1.2", firstEvent.Quote.BidSize.String())
	require.Equal(t, "101", firstEvent.Quote.AskPrice.String())
	require.Equal(t, "0.7", firstEvent.Quote.AskSize.String())
	require.NotNil(t, secondEvent.Ticker)

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 0, ws.unsubBboSubscriptions)
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), tickerSub))
	require.Equal(t, 1, ws.unsubBboSubscriptions)

	tradeSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTradeTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, "BTC", ws.tradesCoin)
	ws.tradesHandler([]hlsdk.WsTrade{{Coin: "BTC", Side: string(hlsdk.SideBid), Px: "100.25", Sz: "0.25", Tid: 55, Hash: "0xtrade", Time: 1710000000001}})
	tradeEvent := requireMarketEvent(t, streaming.Events())
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, model.TradeID("55"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)
	require.Equal(t, "100.25", tradeEvent.Trade.Price.String())
	require.Equal(t, "0.25", tradeEvent.Trade.Size.String())
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, 1, ws.unsubTradesSubscriptions)

	barType := model.NewTimeBarType(id, time.Minute)
	barSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeBar, BarType: barType}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), barSub))
	require.Equal(t, "BTC", ws.candleCoin)
	require.Equal(t, "1m", ws.candleInterval)
	ws.candleHandler(hlsdk.WsCandle{S: "BTC", I: "1m", T: 1710000000000, TClose: 1710000059999, O: "100", H: "102", L: "99", C: "101", V: "5"})
	barEvent := requireMarketEvent(t, streaming.Events())
	require.NotNil(t, barEvent.Bar)
	require.Equal(t, barType.String(), barEvent.Bar.BarType.String())
	require.Equal(t, "100", barEvent.Bar.Open.String())
	require.Equal(t, "102", barEvent.Bar.High.String())
	require.Equal(t, "99", barEvent.Bar.Low.String())
	require.Equal(t, "101", barEvent.Bar.Close.String())
	require.Equal(t, "5", barEvent.Bar.Volume.String())
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), barSub))
	require.Equal(t, 1, ws.unsubCandleSubscriptions)
}

func TestPerpExecutionClientPrivateStreamMapsOrdersFillsAndPositions(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakePerpAccountWS{}
	exec := newPerpExecutionClient("acct", "0xabc", provider, sdk)
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.Equal(t, 1, ws.connects)
	require.Equal(t, "0xabc", ws.orderUser)
	require.Equal(t, "0xabc", ws.fillUser)
	require.Equal(t, "0xabc", ws.positionUser)
	require.Equal(t, 1, ws.orderSubscriptions)
	require.Equal(t, 1, ws.fillSubscriptions)
	require.Equal(t, 1, ws.positionSubscriptions)

	ws.orderHandler([]hlsdk.WsOrderUpdate{{
		Order: hlsdk.WsOrder{
			Coin:      "BTC",
			Side:      string(hlsdk.SideBid),
			LimitPx:   "100",
			Sz:        "1",
			OrigSz:    "1.5",
			Oid:       200,
			Cliod:     "client-1",
			Timestamp: 1710000000000,
		},
		Status:          hlsdk.StatusOpen,
		StatusTimestamp: 1710000000001,
	}})
	orderEvent := requireExecutionEvent(t, exec.Events())
	require.Equal(t, model.OrderID("200"), orderEvent.Order.OrderID)
	require.Equal(t, model.ClientOrderID("client-1"), orderEvent.Order.ClientOrderID)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)
	require.Equal(t, "0.5", orderEvent.Order.FilledQuantity.String())
	require.Equal(t, "1", orderEvent.Order.LeavesQuantity.String())

	ws.fillHandler(hlsdk.WsUserFills{User: "0xabc", Fills: []hlsdk.WsUserFill{{
		Coin:     "BTC",
		Side:     string(hlsdk.SideBid),
		Px:       "100",
		Sz:       "0.5",
		Oid:      200,
		Tid:      55,
		Fee:      "0.01",
		FeeToken: "USDC",
		Time:     1710000000002,
	}}})
	fillEvent := requireExecutionEvent(t, exec.Events())
	require.Equal(t, model.TradeID("55"), fillEvent.Fill.TradeID)
	require.Equal(t, model.OrderID("200"), fillEvent.Fill.OrderID)
	require.Equal(t, model.OrderSideBuy, fillEvent.Fill.Side)
	require.Equal(t, "0.5", fillEvent.Fill.Quantity.String())

	ws.positionHandler(mustPerpPosition(`{"time":1710000000003,"assetPositions":[{"position":{"coin":"BTC","szi":"-0.25","entryPx":"10000"}}]}`))
	positionEvent := requireExecutionEvent(t, exec.Events())
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)
	require.Equal(t, "0.25", positionEvent.Position.Quantity.String())
	require.Equal(t, "10000", positionEvent.Position.EntryPrice.String())

	var resubscriber venue.ExecutionResubscriber = exec
	require.NoError(t, resubscriber.ResubscribeExecution(context.Background()))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, 2, ws.orderSubscriptions)
	require.Equal(t, 2, ws.fillSubscriptions)
	require.Equal(t, 2, ws.positionSubscriptions)
}

func TestPerpGeneratePositionStatusReportsUsesPerpPositions(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newPerpExecutionClient("acct", "0xabc", provider, sdk)

	var generator venue.PositionStatusReportGenerator = exec
	reports, err := generator.GeneratePositionStatusReports(context.Background(), model.MustInstrumentID("BTC-USD-PERP.HYPERLIQUID"))
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.PositionSideLong, reports[0].Side)
	require.Equal(t, "0.75", reports[0].Quantity.String())
	require.Equal(t, "12000", reports[0].EntryPrice.String())
}

func requireMarketEvent(t *testing.T, events <-chan model.MarketEvent) model.MarketEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for market event")
		return model.MarketEvent{}
	}
}

func requireExecutionEvent(t *testing.T, events <-chan model.ExecutionEvent) model.ExecutionEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for execution event")
		return model.ExecutionEvent{}
	}
}

func mustPerpPosition(raw string) hlperp.PerpPosition {
	var position hlperp.PerpPosition
	if err := json.Unmarshal([]byte(raw), &position); err != nil {
		panic(err)
	}
	return position
}

type fakeSpotSDK struct {
	placed     hlspot.PlaceOrderRequest
	openOrders []hlspot.Order
}

func (f *fakeSpotSDK) GetSpotMeta(context.Context) (*hlspot.SpotMeta, error) {
	return &hlspot.SpotMeta{
		Tokens: []struct {
			Name        string `json:"name"`
			SzDecimals  int    `json:"szDecimals"`
			WeiDecimals int    `json:"weiDecimals"`
			Index       int    `json:"index"`
			TokenId     string `json:"tokenId"`
			IsCanonical bool   `json:"isCanonical"`
			FullName    string `json:"fullName,omitempty"`
		}{
			{Name: "USDC", SzDecimals: 6, Index: 0, IsCanonical: true},
			{Name: "PURR", SzDecimals: 2, Index: 1, IsCanonical: true},
		},
		Universe: []struct {
			Name        string `json:"name"`
			Index       int    `json:"index"`
			Tokens      []int  `json:"tokens"`
			IsCanonical bool   `json:"isCanonical"`
		}{{Name: "PURR/USDC", Index: 1, Tokens: []int{1, 0}, IsCanonical: true}},
	}, nil
}

func (f *fakeSpotSDK) AllMids(context.Context) (map[string]string, error) {
	return map[string]string{"PURR/USDC": "10"}, nil
}

func (f *fakeSpotSDK) L2Book(context.Context, string) (*hlspot.L2BookResponse, error) {
	return &hlspot.L2BookResponse{Levels: [][]hlspot.L2Level{{{Px: "9", Sz: "1"}}, {{Px: "11", Sz: "1"}}}}, nil
}

func (f *fakeSpotSDK) GetBalance() (*hlspot.Balance, error) {
	return &hlspot.Balance{Balances: []struct {
		Coin     string `json:"coin"`
		Token    int64  `json:"token"`
		Hold     string `json:"hold"`
		Total    string `json:"total"`
		EntryNtl string `json:"entryNtl"`
	}{{Coin: "USDC", Hold: "1", Total: "6"}}}, nil
}

func (f *fakeSpotSDK) PlaceOrder(_ context.Context, req hlspot.PlaceOrderRequest) (*hlspot.OrderStatus, error) {
	f.placed = req
	clientID := ""
	if req.ClientOrderID != nil {
		clientID = *req.ClientOrderID
	}
	return &hlspot.OrderStatus{Resting: &hlspot.OrderResting{Oid: 100, ClientID: &clientID}}, nil
}

func (f *fakeSpotSDK) CancelOrder(context.Context, hlspot.CancelOrderRequest) (*string, error) {
	status := "success"
	return &status, nil
}

func (f *fakeSpotSDK) UserOpenOrders(context.Context, string) ([]hlspot.Order, error) {
	return f.openOrders, nil
}

type fakeSpotMarketWS struct {
	connects                 int
	bboCoin                  string
	l2Coin                   string
	tradesCoin               string
	candleCoin               string
	candleInterval           string
	bboSubscriptions         int
	tradesSubscriptions      int
	candleSubscriptions      int
	unsubBboCoin             string
	unsubL2Coin              string
	unsubTradesCoin          string
	unsubCandleCoin          string
	unsubCandleInterval      string
	unsubBboSubscriptions    int
	unsubTradesSubscriptions int
	unsubCandleSubscriptions int
	bboHandler               func(hlsdk.WsBbo)
	l2Handler                func(hlsdk.WsL2Book)
	tradesHandler            func([]hlsdk.WsTrade)
	candleHandler            func(hlsdk.WsCandle)
}

func (f *fakeSpotMarketWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeSpotMarketWS) SubscribeBbo(coin string, handler func(hlsdk.WsBbo)) error {
	f.bboCoin = coin
	f.bboSubscriptions++
	f.bboHandler = handler
	return nil
}

func (f *fakeSpotMarketWS) SubscribeL2Book(coin string, handler func(hlsdk.WsL2Book)) error {
	f.l2Coin = coin
	f.l2Handler = handler
	return nil
}

func (f *fakeSpotMarketWS) SubscribeTrades(coin string, handler func([]hlsdk.WsTrade)) error {
	f.tradesCoin = coin
	f.tradesSubscriptions++
	f.tradesHandler = handler
	return nil
}

func (f *fakeSpotMarketWS) SubscribeCandle(coin string, interval string, handler func(hlsdk.WsCandle)) error {
	f.candleCoin = coin
	f.candleInterval = interval
	f.candleSubscriptions++
	f.candleHandler = handler
	return nil
}

func (f *fakeSpotMarketWS) UnsubscribeBbo(coin string) error {
	f.unsubBboCoin = coin
	f.unsubBboSubscriptions++
	return nil
}

func (f *fakeSpotMarketWS) UnsubscribeL2Book(coin string) error {
	f.unsubL2Coin = coin
	return nil
}

func (f *fakeSpotMarketWS) UnsubscribeTrades(coin string) error {
	f.unsubTradesCoin = coin
	f.unsubTradesSubscriptions++
	return nil
}

func (f *fakeSpotMarketWS) UnsubscribeCandle(coin string, interval string) error {
	f.unsubCandleCoin = coin
	f.unsubCandleInterval = interval
	f.unsubCandleSubscriptions++
	return nil
}

func (f *fakeSpotMarketWS) Close() {}

type fakeSpotAccountWS struct {
	connects           int
	orderUser          string
	fillUser           string
	orderSubscriptions int
	fillSubscriptions  int
	orderHandler       func([]hlsdk.WsOrderUpdate)
	fillHandler        func(hlsdk.WsUserFills)
}

func (f *fakeSpotAccountWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeSpotAccountWS) SubscribeOrderUpdates(user string, handler func([]hlsdk.WsOrderUpdate)) error {
	f.orderUser = user
	f.orderSubscriptions++
	f.orderHandler = handler
	return nil
}

func (f *fakeSpotAccountWS) SubscribeUserFills(user string, handler func(hlsdk.WsUserFills)) error {
	f.fillUser = user
	f.fillSubscriptions++
	f.fillHandler = handler
	return nil
}

func (f *fakeSpotAccountWS) Close() {}

type fakePerpSDK struct {
	placed hlperp.PlaceOrderRequest
}

func (f *fakePerpSDK) GetPrepMeta(context.Context) (*hlperp.PrepMeta, error) {
	return &hlperp.PrepMeta{Universe: []struct {
		Name        string `json:"name"`
		SzDecimals  int    `json:"szDecimals"`
		MaxLeverage int    `json:"maxLeverage"`
	}{{Name: "BTC", SzDecimals: 3, MaxLeverage: 50}}}, nil
}

func (f *fakePerpSDK) AllMids(context.Context) (map[string]string, error) {
	return map[string]string{"BTC": "10"}, nil
}

func (f *fakePerpSDK) L2Book(context.Context, string) (*hlperp.L2BookResponse, error) {
	return &hlperp.L2BookResponse{Levels: [][]hlperp.L2Level{{{Px: "9", Sz: "1"}}, {{Px: "11", Sz: "1"}}}}, nil
}

func (f *fakePerpSDK) GetBalance(context.Context) (*hlperp.PerpPosition, error) {
	position := mustPerpPosition(`{"withdrawable":"5","marginSummary":{"accountValue":"10"},"assetPositions":[{"position":{"coin":"BTC","szi":"0.75","entryPx":"12000"}}]}`)
	return &position, nil
}

func (f *fakePerpSDK) UserOpenOrders(context.Context, string) ([]hlperp.Order, error) {
	return []hlperp.Order{{Coin: "BTC", Side: "B", LimitPx: "10", Sz: "1", OrigSz: "1", Oid: 200}}, nil
}

func (f *fakePerpSDK) PlaceOrder(_ context.Context, req hlperp.PlaceOrderRequest) (*hlperp.OrderStatus, error) {
	f.placed = req
	clientID := ""
	if req.ClientOrderID != nil {
		clientID = *req.ClientOrderID
	}
	return &hlperp.OrderStatus{Resting: &hlperp.OrderResting{Oid: 200, ClientID: &clientID}}, nil
}

func (f *fakePerpSDK) CancelOrder(context.Context, hlperp.CancelOrderRequest) (*string, error) {
	status := "success"
	return &status, nil
}

type fakePerpMarketWS struct {
	connects                 int
	bboCoin                  string
	l2Coin                   string
	tradesCoin               string
	candleCoin               string
	candleInterval           string
	bboSubscriptions         int
	tradesSubscriptions      int
	candleSubscriptions      int
	unsubBboCoin             string
	unsubL2Coin              string
	unsubTradesCoin          string
	unsubCandleCoin          string
	unsubCandleInterval      string
	unsubBboSubscriptions    int
	unsubTradesSubscriptions int
	unsubCandleSubscriptions int
	bboHandler               func(hlsdk.WsBbo)
	l2Handler                func(hlsdk.WsL2Book)
	tradesHandler            func([]hlsdk.WsTrade)
	candleHandler            func(hlsdk.WsCandle)
}

func (f *fakePerpMarketWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakePerpMarketWS) SubscribeBbo(coin string, handler func(hlsdk.WsBbo)) error {
	f.bboCoin = coin
	f.bboSubscriptions++
	f.bboHandler = handler
	return nil
}

func (f *fakePerpMarketWS) SubscribeL2Book(coin string, handler func(hlsdk.WsL2Book)) error {
	f.l2Coin = coin
	f.l2Handler = handler
	return nil
}

func (f *fakePerpMarketWS) SubscribeTrades(coin string, handler func([]hlsdk.WsTrade)) error {
	f.tradesCoin = coin
	f.tradesSubscriptions++
	f.tradesHandler = handler
	return nil
}

func (f *fakePerpMarketWS) SubscribeCandle(coin string, interval string, handler func(hlsdk.WsCandle)) error {
	f.candleCoin = coin
	f.candleInterval = interval
	f.candleSubscriptions++
	f.candleHandler = handler
	return nil
}

func (f *fakePerpMarketWS) UnsubscribeBbo(coin string) error {
	f.unsubBboCoin = coin
	f.unsubBboSubscriptions++
	return nil
}

func (f *fakePerpMarketWS) UnsubscribeL2Book(coin string) error {
	f.unsubL2Coin = coin
	return nil
}

func (f *fakePerpMarketWS) UnsubscribeTrades(coin string) error {
	f.unsubTradesCoin = coin
	f.unsubTradesSubscriptions++
	return nil
}

func (f *fakePerpMarketWS) UnsubscribeCandle(coin string, interval string) error {
	f.unsubCandleCoin = coin
	f.unsubCandleInterval = interval
	f.unsubCandleSubscriptions++
	return nil
}

func (f *fakePerpMarketWS) Close() {}

type fakePerpAccountWS struct {
	connects              int
	orderUser             string
	fillUser              string
	positionUser          string
	orderSubscriptions    int
	fillSubscriptions     int
	positionSubscriptions int
	orderHandler          func([]hlsdk.WsOrderUpdate)
	fillHandler           func(hlsdk.WsUserFills)
	positionHandler       func(hlperp.PerpPosition)
}

func (f *fakePerpAccountWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakePerpAccountWS) SubscribeOrderUpdates(user string, handler func([]hlsdk.WsOrderUpdate)) error {
	f.orderUser = user
	f.orderSubscriptions++
	f.orderHandler = handler
	return nil
}

func (f *fakePerpAccountWS) SubscribeUserFills(user string, handler func(hlsdk.WsUserFills)) error {
	f.fillUser = user
	f.fillSubscriptions++
	f.fillHandler = handler
	return nil
}

func (f *fakePerpAccountWS) SubscribeWebData2(user string, handler func(hlperp.PerpPosition)) error {
	f.positionUser = user
	f.positionSubscriptions++
	f.positionHandler = handler
	return nil
}

func (f *fakePerpAccountWS) Close() {}
