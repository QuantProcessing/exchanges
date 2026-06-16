package okx

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	okxsdk "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSpotClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSpotProvider(sdk)
	data := newDataClient("okx-spot-data", provider, sdk)
	data.ws = &fakePublicWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newExecutionClient("spot-acct", provider, sdk, "SPOT", "cash"),
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.OKX"),
		Capabilities: (&Adapter{}).Capabilities(),
	})
}

func TestSwapClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSwapProvider(sdk)
	data := newDataClient("okx-swap-data", provider, sdk)
	data.ws = &fakePublicWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newExecutionClient("swap-acct", provider, sdk, "SWAP", "cross"),
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.OKX"),
		Capabilities: (&Adapter{fundingRates: true}).Capabilities(),
	})
}

func TestSwapAdapterCapabilitiesDeclareFundingRatesOnlyForPerp(t *testing.T) {
	require.False(t, (&Adapter{}).Capabilities().MarketData.FundingRates)
	require.True(t, (&Adapter{fundingRates: true}).Capabilities().MarketData.FundingRates)
}

func TestSpotSubmitMapsCashOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSpotProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk, "SPOT", "cash")

	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.OKX"),
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, "BTC-USDT", sdk.placed.InstId)
	require.Equal(t, "cash", sdk.placed.TdMode)
	require.Equal(t, "buy", sdk.placed.Side)
	require.Equal(t, "limit", sdk.placed.OrdType)
}

func TestSwapSubmitMapsCrossOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSwapProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk, "SWAP", "cross")

	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-PERP.OKX"),
		ClientOrderID: "client-2",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeMarket,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("2"),
	})
	require.NoError(t, err)
	require.Equal(t, "BTC-USDT-SWAP", sdk.placed.InstId)
	require.Equal(t, "cross", sdk.placed.TdMode)
	require.Equal(t, "sell", sdk.placed.Side)
	require.Equal(t, "market", sdk.placed.OrdType)
}

func TestDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSwapProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("okx-swap-data", provider, sdk)
	ws := &fakePublicWS{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDT-PERP.OKX")

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTicker,
	}))
	require.Equal(t, "BTC-USDT-SWAP", ws.tickerInstID)
	ws.emitTicker(&okxsdk.Ticker{InstId: "BTC-USDT-SWAP", BidPx: "9", AskPx: "11", Last: "10", Ts: "1000"})
	tickerEvent := <-client.Events()
	require.NotNil(t, tickerEvent.Ticker)
	require.Equal(t, id, tickerEvent.Ticker.InstrumentID)
	require.True(t, decimal.RequireFromString("9").Equal(tickerEvent.Ticker.Bid))
	require.True(t, decimal.RequireFromString("11").Equal(tickerEvent.Ticker.Ask))

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        400,
	}))
	require.Equal(t, "BTC-USDT-SWAP", ws.bookInstID)
	ws.emitBook(&okxsdk.OrderBook{
		Bids: [][]string{{"9", "1", "0", "1"}},
		Asks: [][]string{{"11", "2", "0", "1"}},
		Ts:   "1000",
	}, "snapshot")
	bookEvent := <-client.Events()
	require.NotNil(t, bookEvent.OrderBook)
	require.Equal(t, id, bookEvent.OrderBook.InstrumentID)
	require.True(t, decimal.RequireFromString("11").Equal(bookEvent.OrderBook.Asks[0].Price))
	require.True(t, decimal.RequireFromString("2").Equal(bookEvent.OrderBook.Asks[0].Size))

	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        400,
	}))
	require.Equal(t, okxsdk.WsSubscribeArgs{Channel: "books", InstId: "BTC-USDT-SWAP"}, ws.unsubArg)
}

func TestDataClientRestSnapshotsUseVenueTimestamps(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSwapProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("okx-swap-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.OKX")

	ticker, err := client.FetchTicker(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(1000), ticker.Timestamp)

	book, err := client.FetchOrderBook(context.Background(), id, 400)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(2000), book.Timestamp)
}

func TestDataClientFetchFundingRate(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSwapProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("okx-swap-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.OKX")

	funding, err := client.FetchFundingRate(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, funding.InstrumentID)
	require.True(t, decimal.RequireFromString("0.0006").Equal(funding.Rate))
	require.Equal(t, 8*time.Hour, funding.FundingInterval)
	require.Equal(t, time.UnixMilli(1000), funding.Timestamp)
	require.Equal(t, time.UnixMilli(28800000), funding.NextFundingTime)
}

func TestDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSwapProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("okx-swap-data", provider, sdk)
	ws := &fakePublicWS{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDT-PERP.OKX")

	tickerSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}
	quoteSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}
	require.NoError(t, client.SubscribeMarketData(context.Background(), tickerSub))
	require.NoError(t, client.SubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 1, ws.subscribeCount)
	require.Equal(t, "BTC-USDT-SWAP", ws.tickerInstID)
	ws.emitTicker(&okxsdk.Ticker{InstId: "BTC-USDT-SWAP", BidPx: "9", BidSz: "1.25", AskPx: "11", AskSz: "2.5", Last: "10", Ts: "1000"})
	tickerEvent := <-client.Events()
	require.NotNil(t, tickerEvent.Ticker)
	quoteEvent := <-client.Events()
	require.NotNil(t, quoteEvent.Quote)
	require.Equal(t, id, quoteEvent.Quote.InstrumentID)
	require.True(t, decimal.RequireFromString("2.5").Equal(quoteEvent.Quote.AskSize))

	require.NoError(t, client.UnsubscribeMarketData(context.Background(), quoteSub))
	require.Empty(t, ws.unsubArg.Channel)
	require.NoError(t, client.UnsubscribeMarketData(context.Background(), tickerSub))
	require.Equal(t, okxsdk.WsSubscribeArgs{Channel: "tickers", InstId: "BTC-USDT-SWAP"}, ws.unsubArg)

	tradeSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTradeTick}
	require.NoError(t, client.SubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, "BTC-USDT-SWAP", ws.tradeInstID)
	ws.emitTrade(&okxsdk.PublicTrade{InstId: "BTC-USDT-SWAP", TradeId: "trade-1", Px: "100.5", Sz: "0.3", Side: "buy", Ts: "2000"})
	tradeEvent := <-client.Events()
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, model.TradeID("trade-1"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)

	barType := model.NewTimeBarType(id, time.Minute)
	barSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeBar, BarType: barType}
	require.NoError(t, client.SubscribeMarketData(context.Background(), barSub))
	require.Equal(t, okxsdk.WsSubscribeArgs{Channel: "candle1m", InstId: "BTC-USDT-SWAP"}, ws.candleArg)
	ws.emitCandle(okxsdk.Candle{"3000", "100", "110", "95", "105", "12.5", "1200", "1300", "0"})
	barEvent := <-client.Events()
	require.NotNil(t, barEvent.Bar)
	require.Equal(t, barType.Canonical(), barEvent.Bar.BarType)
	require.True(t, decimal.RequireFromString("105").Equal(barEvent.Bar.Close))
	require.True(t, barEvent.Bar.IsRevision)
}

func TestExecutionClientPrivateStreamMapsOrdersFillsAndPositions(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newSwapProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("swap-acct", provider, sdk, "SWAP", "cross")
	ws := &fakePrivateWS{}
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.True(t, ws.connected)
	require.Equal(t, "SWAP", ws.ordersInstType)
	require.Equal(t, "SWAP", ws.positionsInstType)

	ws.emitOrder(&okxsdk.Order{
		InstId:    "BTC-USDT-SWAP",
		OrdId:     "100",
		ClOrdId:   "client-1",
		State:     okxsdk.OrderStatusPartiallyFilled,
		Side:      okxsdk.SideBuy,
		OrdType:   okxsdk.OrderTypeLimit,
		Sz:        "1",
		AccFillSz: "0.4",
		Px:        "10",
		AvgPx:     "10",
		FillPx:    "10",
		FillSz:    "0.4",
		FillTime:  "1000",
		TradeId:   "trade-1",
		Fee:       "0.01",
		FeeCcy:    "USDT",
		UTime:     "1000",
	})
	orderEvent := <-exec.Events()
	require.NotNil(t, orderEvent.Order)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)
	require.Equal(t, model.OrderID("100"), orderEvent.Order.OrderID)
	fillEvent := <-exec.Events()
	require.NotNil(t, fillEvent.Fill)
	require.Equal(t, model.TradeID("trade-1"), fillEvent.Fill.TradeID)
	require.True(t, decimal.RequireFromString("0.4").Equal(fillEvent.Fill.Quantity))

	ws.emitPosition(&okxsdk.Position{
		InstId:  "BTC-USDT-SWAP",
		PosId:   "pos-1",
		PosSide: okxsdk.PosSideShort,
		Pos:     "0.2",
		AvgPx:   "10",
		UTime:   "1000",
	})
	positionEvent := <-exec.Events()
	require.NotNil(t, positionEvent.Position)
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)
	require.True(t, decimal.RequireFromString("0.2").Equal(positionEvent.Position.Quantity))

	require.NoError(t, exec.ResubscribeExecution(context.Background()))
	require.Equal(t, 4, ws.subscribeCount)
}

type fakeSDK struct {
	placed okxsdk.OrderRequest
}

func (f *fakeSDK) GetInstruments(_ context.Context, instType string) ([]okxsdk.Instrument, error) {
	switch instType {
	case "SPOT":
		return []okxsdk.Instrument{{
			InstId:   "BTC-USDT",
			InstType: "SPOT",
			BaseCcy:  "BTC",
			QuoteCcy: "USDT",
			TickSz:   "0.01",
			LotSz:    "0.0001",
			MinSz:    "0.0001",
			State:    "live",
		}}, nil
	case "SWAP":
		return []okxsdk.Instrument{{
			InstId:     "BTC-USDT-SWAP",
			InstType:   "SWAP",
			InstFamily: "BTC-USDT",
			Uly:        "BTC-USDT",
			SettleCcy:  "USDT",
			CtValCcy:   "BTC",
			TickSz:     "0.1",
			LotSz:      "0.01",
			MinSz:      "0.01",
			State:      "live",
		}}, nil
	default:
		return nil, nil
	}
}

func (f *fakeSDK) GetTicker(_ context.Context, instID string) ([]okxsdk.Ticker, error) {
	return []okxsdk.Ticker{{InstId: instID, Last: "10", BidPx: "9", AskPx: "11", Ts: "1000"}}, nil
}

func (f *fakeSDK) GetOrderBook(_ context.Context, instID string, _ *int) ([]okxsdk.OrderBook, error) {
	return []okxsdk.OrderBook{{Bids: [][]string{{"9", "1", "0", "1"}}, Asks: [][]string{{"11", "1", "0", "1"}}, Ts: "2000"}}, nil
}

func (f *fakeSDK) GetFundingRate(context.Context, string) (*okxsdk.FundingRateData, error) {
	return &okxsdk.FundingRateData{
		Symbol:               "BTC-USDT-SWAP",
		FundingRate:          "0.0006",
		FundingIntervalHours: 8,
		FundingTime:          "1000",
		NextFundingTime:      "28800000",
	}, nil
}

func (f *fakeSDK) GetAccountBalance(context.Context, *string) ([]okxsdk.Balance, error) {
	return []okxsdk.Balance{{Details: []okxsdk.BalanceDetail{{
		Ccy:       "USDT",
		AvailBal:  "5",
		FrozenBal: "1",
		Eq:        "6",
	}}}}, nil
}

func (f *fakeSDK) GetPositions(_ context.Context, instType, instID *string) ([]okxsdk.Position, error) {
	_ = instType
	if instID == nil {
		return nil, nil
	}
	return []okxsdk.Position{{
		InstId:  *instID,
		PosId:   "pos-1",
		PosSide: okxsdk.PosSideLong,
		Pos:     "0.1",
		AvgPx:   "10",
		UTime:   "1000",
	}}, nil
}

func (f *fakeSDK) PlaceOrder(_ context.Context, req *okxsdk.OrderRequest) ([]okxsdk.OrderId, error) {
	f.placed = *req
	return []okxsdk.OrderId{{OrdId: "100", ClOrdId: value(req.ClOrdId), SCode: "0"}}, nil
}

func (f *fakeSDK) CancelOrder(context.Context, string, string, string) ([]okxsdk.OrderId, error) {
	return []okxsdk.OrderId{{OrdId: "100", SCode: "0"}}, nil
}

func (f *fakeSDK) GetOrders(_ context.Context, instType, instID *string) ([]okxsdk.Order, error) {
	_ = instType
	if instID == nil {
		return nil, nil
	}
	return []okxsdk.Order{{
		InstId:    *instID,
		OrdId:     "100",
		ClOrdId:   "client-1",
		State:     okxsdk.OrderStatusLive,
		Side:      okxsdk.SideBuy,
		OrdType:   okxsdk.OrderTypeLimit,
		Sz:        "1",
		AccFillSz: "0",
		Px:        "10",
	}}, nil
}

func value(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type fakePublicWS struct {
	connected      bool
	subscribeCount int
	tickerInstID   string
	bookInstID     string
	tradeInstID    string
	candleArg      okxsdk.WsSubscribeArgs
	unsubArg       okxsdk.WsSubscribeArgs
	tickerHandler  func(*okxsdk.Ticker)
	bookHandler    func(*okxsdk.OrderBook, string)
	tradeHandler   func(*okxsdk.PublicTrade)
	candleHandler  func(okxsdk.Candle)
}

func (f *fakePublicWS) Connect() error {
	f.connected = true
	return nil
}

func (f *fakePublicWS) SubscribeTicker(instID string, handler func(*okxsdk.Ticker)) error {
	f.connected = true
	f.subscribeCount++
	f.tickerInstID = instID
	f.tickerHandler = handler
	return nil
}

func (f *fakePublicWS) SubscribeOrderBook(instID string, handler func(*okxsdk.OrderBook, string)) error {
	f.connected = true
	f.subscribeCount++
	f.bookInstID = instID
	f.bookHandler = handler
	return nil
}

func (f *fakePublicWS) SubscribeTrades(instID string, handler func(*okxsdk.PublicTrade)) error {
	f.connected = true
	f.subscribeCount++
	f.tradeInstID = instID
	f.tradeHandler = handler
	return nil
}

func (f *fakePublicWS) SubscribeCandles(instID string, channel string, handler func(okxsdk.Candle)) error {
	f.connected = true
	f.subscribeCount++
	f.candleArg = okxsdk.WsSubscribeArgs{Channel: channel, InstId: instID}
	f.candleHandler = handler
	return nil
}

func (f *fakePublicWS) Unsubscribe(arg okxsdk.WsSubscribeArgs) error {
	f.unsubArg = arg
	return nil
}

func (f *fakePublicWS) Close() error {
	f.connected = false
	return nil
}

func (f *fakePublicWS) emitTicker(ticker *okxsdk.Ticker) {
	f.tickerHandler(ticker)
}

func (f *fakePublicWS) emitBook(book *okxsdk.OrderBook, action string) {
	f.bookHandler(book, action)
}

func (f *fakePublicWS) emitTrade(trade *okxsdk.PublicTrade) {
	f.tradeHandler(trade)
}

func (f *fakePublicWS) emitCandle(candle okxsdk.Candle) {
	f.candleHandler(candle)
}

type fakePrivateWS struct {
	connected         bool
	subscribeCount    int
	ordersInstType    string
	positionsInstType string
	orderHandler      func(*okxsdk.Order)
	positionHandler   func(*okxsdk.Position)
}

func (f *fakePrivateWS) Connect() error {
	f.connected = true
	return nil
}

func (f *fakePrivateWS) SubscribeOrders(instType string, instID *string, handler func(*okxsdk.Order)) error {
	f.connected = true
	f.subscribeCount++
	f.ordersInstType = instType
	f.orderHandler = handler
	return nil
}

func (f *fakePrivateWS) SubscribePositions(instType string, handler func(*okxsdk.Position)) error {
	f.connected = true
	f.subscribeCount++
	f.positionsInstType = instType
	f.positionHandler = handler
	return nil
}

func (f *fakePrivateWS) Close() error {
	f.connected = false
	return nil
}

func (f *fakePrivateWS) emitOrder(order *okxsdk.Order) {
	f.orderHandler(order)
}

func (f *fakePrivateWS) emitPosition(position *okxsdk.Position) {
	f.positionHandler(position)
}
