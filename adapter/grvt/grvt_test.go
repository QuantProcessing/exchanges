package grvt

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	grvtsdk "github.com/QuantProcessing/exchanges/sdk/grvt"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	data := newDataClient("grvt-perp-data", provider, sdk)
	data.ws = &fakeMarketWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newExecutionClient("perp-acct", provider, sdk, 7),
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.GRVT"),
		Capabilities: (&Adapter{}).Capabilities(),
	})
}

func TestDataClientFetchFundingRate(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("grvt-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.GRVT")

	funding, err := client.FetchFundingRate(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, funding.InstrumentID)
	require.True(t, decimal.RequireFromString("0.0003").Equal(funding.Rate))
	require.Equal(t, 8*time.Hour, funding.FundingInterval)
	require.Equal(t, time.UnixMilli(1000), funding.Timestamp)
	require.Equal(t, time.UnixMilli(28800000), funding.NextFundingTime)
}

func TestSubmitMapsOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk, 7)

	report, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-PERP.GRVT"),
		ClientOrderID: "client-1",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderID("placed-1"), report.OrderID)
	require.Equal(t, uint64(7), sdk.created.Order.SubAccountID)
	require.False(t, sdk.created.Order.IsMarket)
	require.Equal(t, grvtsdk.IOC, sdk.created.Order.TimeInForce)
	require.Equal(t, "client-1", sdk.created.Order.Metadata.ClientOrderID)
	require.Len(t, sdk.created.Order.Legs, 1)
	require.Equal(t, "BTC_USDT_Perp", sdk.created.Order.Legs[0].Instrument)
	require.False(t, sdk.created.Order.Legs[0].IsBuyintAsset)
	require.Equal(t, "0.5", sdk.created.Order.Legs[0].Size)
	require.Equal(t, "10", sdk.created.Order.Legs[0].LimitPrice)
}

func TestDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeMarketWS{}
	client := newDataClient("grvt-perp-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.GRVT"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.GRVT"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        50,
	}))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, "BTC_USDT_Perp", ws.tickerInstrument)
	require.Equal(t, grvtsdk.TickerSnapRate1000, ws.tickerInterval)
	require.Equal(t, "BTC_USDT_Perp", ws.bookInstrument)
	require.Equal(t, grvtsdk.OrderBookSnapDepth50, ws.bookDepth)

	require.NoError(t, ws.tickerHandler(grvtsdk.WsFeeData[grvtsdk.Ticker]{Feed: grvtsdk.Ticker{
		Instrument:   "BTC_USDT_Perp",
		LastPrice:    "100.5",
		BestBidPrice: "100",
		BestAskPrice: "101",
		EventTime:    "1710000000000",
	}}))
	tickerEvent := <-streaming.Events()
	require.Equal(t, "100", tickerEvent.Ticker.Bid.String())
	require.Equal(t, "101", tickerEvent.Ticker.Ask.String())
	require.Equal(t, "100.5", tickerEvent.Ticker.Last.String())

	require.NoError(t, ws.bookHandler(grvtsdk.WsFeeData[grvtsdk.OrderBook]{Feed: grvtsdk.OrderBook{
		Instrument: "BTC_USDT_Perp",
		EventTime:  "1710000000001",
		Bids:       []grvtsdk.OrderBookLevel{{Price: "100", Size: "1.2"}},
		Asks:       []grvtsdk.OrderBookLevel{{Price: "101", Size: "0.7"}},
	}}))
	bookEvent := <-streaming.Events()
	require.Equal(t, "100", bookEvent.OrderBook.Bids[0].Price.String())
	require.Equal(t, "101", bookEvent.OrderBook.Asks[0].Price.String())

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.GRVT"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.GRVT"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        50,
	}))
	require.Equal(t, "BTC_USDT_Perp", ws.unsubTickerInstrument)
	require.Equal(t, "BTC_USDT_Perp", ws.unsubBookInstrument)
}

func TestDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeMarketWS{}
	client := newDataClient("grvt-perp-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	id := model.MustInstrumentID("BTC-USDT-PERP.GRVT")
	tickerSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}
	quoteSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), tickerSub))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 1, ws.tickerSubscriptions)

	require.NoError(t, ws.tickerHandler(grvtsdk.WsFeeData[grvtsdk.Ticker]{Feed: grvtsdk.Ticker{
		Instrument:   "BTC_USDT_Perp",
		LastPrice:    "100.5",
		BestBidPrice: "100",
		BestBidSize:  "1.2",
		BestAskPrice: "101",
		BestAskSize:  "0.7",
		EventTime:    "1710000000000",
	}}))
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
	require.NotNil(t, secondEvent.Ticker)

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 0, ws.unsubTickerSubscriptions)
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), tickerSub))
	require.Equal(t, 1, ws.unsubTickerSubscriptions)

	tradeSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTradeTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, grvtsdk.TradeLimit50, ws.tradeLimit)
	require.NoError(t, ws.tradeHandler(grvtsdk.WsFeeData[grvtsdk.Trade]{Feed: grvtsdk.Trade{
		Instrument:   "BTC_USDT_Perp",
		TradeId:      "trade-1",
		Price:        "100.25",
		Size:         "0.25",
		IsTakerBuyer: true,
		EventTime:    "1710000000001",
	}}))
	tradeEvent := <-streaming.Events()
	require.Equal(t, model.TradeID("trade-1"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)
	require.Equal(t, "100.25", tradeEvent.Trade.Price.String())
	require.Equal(t, "0.25", tradeEvent.Trade.Size.String())
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, 1, ws.unsubTradeSubscriptions)

	barType := model.NewTimeBarType(id, time.Minute)
	barSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeBar, BarType: barType}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), barSub))
	require.Equal(t, grvtsdk.KlineInterval1m, ws.klineInterval)
	require.Equal(t, grvtsdk.KlineTypeTrade, ws.klineType)
	require.NoError(t, ws.klineHandler(grvtsdk.WsFeeData[grvtsdk.KLine]{Feed: grvtsdk.KLine{
		Instrument: "BTC_USDT_Perp",
		OpenTime:   "1710000000000",
		CloseTime:  "1710000059999",
		Open:       "100",
		High:       "102",
		Low:        "99",
		Close:      "101",
		VolumeB:    "5",
	}}))
	barEvent := <-streaming.Events()
	require.Equal(t, barType.String(), barEvent.Bar.BarType.String())
	require.Equal(t, "100", barEvent.Bar.Open.String())
	require.Equal(t, "102", barEvent.Bar.High.String())
	require.Equal(t, "99", barEvent.Bar.Low.String())
	require.Equal(t, "101", barEvent.Bar.Close.String())
	require.Equal(t, "5", barEvent.Bar.Volume.String())
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), barSub))
	require.Equal(t, 1, ws.unsubKlineSubscriptions)
}

func TestExecutionClientPrivateStreamMapsOrdersFillsAndPositions(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeAccountWS{}
	exec := newExecutionClient("acct", provider, sdk, 7)
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.Equal(t, 1, ws.connects)
	require.Equal(t, "all", ws.orderInstrument)
	require.Equal(t, "all", ws.fillInstrument)
	require.Equal(t, "all", ws.positionInstrument)

	require.NoError(t, ws.orderHandler(grvtsdk.WsFeeData[grvtsdk.Order]{Feed: grvtsdk.Order{
		OrderID:  "order-1",
		IsMarket: false,
		Legs: []grvtsdk.OrderLeg{{
			Instrument:    "BTC_USDT_Perp",
			Size:          "1.5",
			LimitPrice:    "100",
			IsBuyintAsset: true,
		}},
		Metadata: grvtsdk.OrderMetadata{ClientOrderID: "client-1"},
		State:    grvtsdk.OrderState{Status: grvtsdk.OrderStatusOpen, TradedSize: []string{"0.5"}, BookSize: []string{"1"}, AvgFillPrice: []string{"100"}, UpdateTime: "1710000000000"},
	}}))
	orderEvent := <-exec.Events()
	require.Equal(t, model.OrderID("order-1"), orderEvent.Order.OrderID)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)
	require.Equal(t, "1", orderEvent.Order.LeavesQuantity.String())

	require.NoError(t, ws.fillHandler(grvtsdk.WsFeeData[grvtsdk.WsFill]{Feed: grvtsdk.WsFill{
		Instrument:    "BTC_USDT_Perp",
		IsBuyer:       true,
		Size:          "0.5",
		Price:         "100",
		Fee:           "0.01",
		TradeID:       "trade-1",
		OrderID:       "order-1",
		ClientOrderID: "client-1",
		EventTime:     "1710000000001",
	}}))
	fillEvent := <-exec.Events()
	require.Equal(t, model.TradeID("trade-1"), fillEvent.Fill.TradeID)
	require.Equal(t, model.OrderID("order-1"), fillEvent.Fill.OrderID)
	require.Equal(t, model.OrderSideBuy, fillEvent.Fill.Side)
	require.Equal(t, "0.5", fillEvent.Fill.Quantity.String())

	require.NoError(t, ws.positionHandler(grvtsdk.WsFeeData[grvtsdk.Position]{Feed: grvtsdk.Position{
		Instrument: "BTC_USDT_Perp",
		Size:       "-0.25",
		EntryPrice: "10000",
		EventTime:  "1710000000002",
	}}))
	positionEvent := <-exec.Events()
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)
	require.Equal(t, "0.25", positionEvent.Position.Quantity.String())
	require.Equal(t, "10000", positionEvent.Position.EntryPrice.String())

	require.NoError(t, exec.ResubscribeExecution(context.Background()))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, 2, ws.orderSubscriptions)
	require.Equal(t, 2, ws.fillSubscriptions)
	require.Equal(t, 2, ws.positionSubscriptions)
}

func TestGeneratePositionStatusReportsUsesAccountSummaryPositions(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk, 7)

	reports, err := exec.GeneratePositionStatusReports(context.Background(), model.MustInstrumentID("BTC-USDT-PERP.GRVT"))
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.PositionSideLong, reports[0].Side)
	require.Equal(t, "0.75", reports[0].Quantity.String())
	require.Equal(t, "12000", reports[0].EntryPrice.String())
}

type fakeSDK struct {
	created          grvtsdk.CreateOrderRequest
	openOrdersSymbol string
}

func (f *fakeSDK) GetInstruments(context.Context) ([]grvtsdk.Instrument, error) {
	return []grvtsdk.Instrument{{
		Instrument:     "BTC_USDT_Perp",
		InstrumentHash: "0x1",
		Base:           "BTC",
		Quote:          "USDT",
		Kind:           "PERPETUAL",
		TickSize:       "0.1",
		MinSize:        "0.001",
	}}, nil
}

func (f *fakeSDK) GetTicker(context.Context, string) (*grvtsdk.GetTickerResponse, error) {
	return &grvtsdk.GetTickerResponse{Result: grvtsdk.Ticker{
		Instrument:   "BTC_USDT_Perp",
		LastPrice:    "10",
		BestBidPrice: "9",
		BestAskPrice: "11",
	}}, nil
}

func (f *fakeSDK) GetOrderBook(context.Context, string, int) (*grvtsdk.GetOrderBookResponse, error) {
	return &grvtsdk.GetOrderBookResponse{Result: grvtsdk.OrderBook{
		Instrument: "BTC_USDT_Perp",
		Bids:       []grvtsdk.OrderBookLevel{{Price: "9", Size: "1"}},
		Asks:       []grvtsdk.OrderBookLevel{{Price: "11", Size: "1"}},
	}}, nil
}

func (f *fakeSDK) GetFundingRate(context.Context, string) (*grvtsdk.FundingRateData, error) {
	return &grvtsdk.FundingRateData{
		Instrument:           "BTC_USDT_Perp",
		FundingRate:          "0.0003",
		FundingIntervalHours: 8,
		FundingTime:          "1000",
		NextFundingTime:      "28800000",
	}, nil
}

func (f *fakeSDK) GetAccountSummary(context.Context) (*grvtsdk.GetAccountSummaryResponse, error) {
	return &grvtsdk.GetAccountSummaryResponse{Result: grvtsdk.AccountSummary{
		SubAccountID:     "7",
		SettleCurrency:   "USDT",
		AvailableBalance: "90",
		TotalEquity:      "100",
		Position: []grvtsdk.Position{{
			Instrument: "BTC_USDT_Perp",
			Size:       "0.75",
			EntryPrice: "12000",
			EventTime:  "1710000000000",
		}},
		SpotBalance: []struct {
			Currency   string `json:"c"`
			Balance    string `json:"b"`
			IndexPrice string `json:"ip"`
		}{{Currency: "USDT", Balance: "100", IndexPrice: "1"}},
	}}, nil
}

func (f *fakeSDK) GetOpenOrders(_ context.Context, symbol string) ([]grvtsdk.Order, error) {
	f.openOrdersSymbol = symbol
	return []grvtsdk.Order{{
		OrderID:      "open-1",
		SubAccountID: "7",
		IsMarket:     false,
		TimeInForce:  grvtsdk.GTT,
		Legs:         []grvtsdk.OrderLeg{{Instrument: "BTC_USDT_Perp", Size: "1", LimitPrice: "10", IsBuyintAsset: true}},
		Metadata:     grvtsdk.OrderMetadata{ClientOrderID: "client-open"},
		State:        grvtsdk.OrderState{Status: grvtsdk.OrderStatusOpen, TradedSize: []string{"0"}},
	}}, nil
}

func (f *fakeSDK) CreateOrder(_ context.Context, req *grvtsdk.CreateOrderRequest, _ map[string]grvtsdk.Instrument) (*grvtsdk.CreateOrderResponse, error) {
	f.created = *req
	return &grvtsdk.CreateOrderResponse{Result: grvtsdk.Order{
		OrderID:  "placed-1",
		IsMarket: req.Order.IsMarket,
		Legs:     req.Order.Legs,
		Metadata: req.Order.Metadata,
		State:    grvtsdk.OrderState{Status: grvtsdk.OrderStatusOpen},
	}}, nil
}

func (f *fakeSDK) CancelOrder(context.Context, string) error { return nil }

type fakeMarketWS struct {
	connects                 int
	tickerInstrument         string
	tickerInterval           grvtsdk.TickerSnapRate
	tickerSubscriptions      int
	bookInstrument           string
	bookInterval             grvtsdk.OrderBookSnapRate
	bookDepth                grvtsdk.OrderBookSnapDepth
	tradeInstrument          string
	tradeLimit               grvtsdk.TradeLimit
	klineInstrument          string
	klineInterval            grvtsdk.KlineInterval
	klineType                grvtsdk.KlineType
	unsubTickerInstrument    string
	unsubBookInstrument      string
	unsubTickerSubscriptions int
	unsubTradeSubscriptions  int
	unsubKlineSubscriptions  int
	tickerHandler            func(grvtsdk.WsFeeData[grvtsdk.Ticker]) error
	bookHandler              func(grvtsdk.WsFeeData[grvtsdk.OrderBook]) error
	tradeHandler             func(grvtsdk.WsFeeData[grvtsdk.Trade]) error
	klineHandler             func(grvtsdk.WsFeeData[grvtsdk.KLine]) error
}

func (f *fakeMarketWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeMarketWS) SubscribeTickerSnap(instrument string, interval grvtsdk.TickerSnapRate, cb func(grvtsdk.WsFeeData[grvtsdk.Ticker]) error) error {
	f.tickerInstrument = instrument
	f.tickerInterval = interval
	f.tickerSubscriptions++
	f.tickerHandler = cb
	return nil
}

func (f *fakeMarketWS) SubscribeOrderbookSnap(instrument string, interval grvtsdk.OrderBookSnapRate, depth grvtsdk.OrderBookSnapDepth, cb func(grvtsdk.WsFeeData[grvtsdk.OrderBook]) error) error {
	f.bookInstrument = instrument
	f.bookInterval = interval
	f.bookDepth = depth
	f.bookHandler = cb
	return nil
}

func (f *fakeMarketWS) SubscribeTrade(instrument string, limit grvtsdk.TradeLimit, cb func(grvtsdk.WsFeeData[grvtsdk.Trade]) error) error {
	f.tradeInstrument = instrument
	f.tradeLimit = limit
	f.tradeHandler = cb
	return nil
}

func (f *fakeMarketWS) SubscribeKline(instrument string, interval grvtsdk.KlineInterval, typ grvtsdk.KlineType, cb func(grvtsdk.WsFeeData[grvtsdk.KLine]) error) error {
	f.klineInstrument = instrument
	f.klineInterval = interval
	f.klineType = typ
	f.klineHandler = cb
	return nil
}

func (f *fakeMarketWS) UnsubscribeTickerSnap(instrument string, interval grvtsdk.TickerSnapRate) error {
	f.unsubTickerInstrument = instrument
	f.unsubTickerSubscriptions++
	return nil
}

func (f *fakeMarketWS) UnsubscribeOrderbookSnap(instrument string, interval grvtsdk.OrderBookSnapRate, depth grvtsdk.OrderBookSnapDepth) error {
	f.unsubBookInstrument = instrument
	return nil
}

func (f *fakeMarketWS) UnsubscribeTrade(instrument string, limit grvtsdk.TradeLimit) error {
	f.unsubTradeSubscriptions++
	return nil
}

func (f *fakeMarketWS) UnsubscribeKline(instrument string, interval grvtsdk.KlineInterval, typ grvtsdk.KlineType) error {
	f.unsubKlineSubscriptions++
	return nil
}

func (f *fakeMarketWS) Close() {}

type fakeAccountWS struct {
	connects              int
	orderInstrument       string
	fillInstrument        string
	positionInstrument    string
	orderSubscriptions    int
	fillSubscriptions     int
	positionSubscriptions int
	orderHandler          func(grvtsdk.WsFeeData[grvtsdk.Order]) error
	fillHandler           func(grvtsdk.WsFeeData[grvtsdk.WsFill]) error
	positionHandler       func(grvtsdk.WsFeeData[grvtsdk.Position]) error
}

func (f *fakeAccountWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeAccountWS) SubscribeOrderUpdate(instrument string, cb func(grvtsdk.WsFeeData[grvtsdk.Order]) error) error {
	f.orderInstrument = instrument
	f.orderSubscriptions++
	f.orderHandler = cb
	return nil
}

func (f *fakeAccountWS) SubscribeFill(instrument string, cb func(grvtsdk.WsFeeData[grvtsdk.WsFill]) error) error {
	f.fillInstrument = instrument
	f.fillSubscriptions++
	f.fillHandler = cb
	return nil
}

func (f *fakeAccountWS) SubscribePositions(instrument string, cb func(grvtsdk.WsFeeData[grvtsdk.Position]) error) error {
	f.positionInstrument = instrument
	f.positionSubscriptions++
	f.positionHandler = cb
	return nil
}

func (f *fakeAccountWS) Close() {}
