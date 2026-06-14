package standx

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	standxsdk "github.com/QuantProcessing/exchanges/sdk/standx"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	data := newDataClient("standx-perp-data", provider, sdk)
	data.ws = &fakeMarketWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:           provider,
		Data:               data,
		Execution:          newExecutionClient("perp-acct", provider, sdk),
		InstrumentID:       model.MustInstrumentID("BTC-USDT-PERP.STANDX"),
		Capabilities:       (&Adapter{}).Capabilities(),
		ExpectedMakerFee:   decimal.RequireFromString("0.0001"),
		ExpectedTakerFee:   decimal.RequireFromString("0.0006"),
		ExpectedMarginInit: decimal.RequireFromString("0.05"),
	})
}

func TestInstrumentProviderNormalizesFeeAndMarginMetadata(t *testing.T) {
	provider := newPerpProvider(&fakeSDK{})
	require.NoError(t, provider.LoadAll(context.Background()))

	inst, ok := provider.Get(model.MustInstrumentID("BTC-USDT-PERP.STANDX"))
	require.True(t, ok)
	require.Equal(t, "0.0001", inst.MakerFee.String())
	require.Equal(t, "0.0006", inst.TakerFee.String())
	require.Equal(t, "0.05", inst.MarginInit.String())
}

func TestSubmitMapsOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk)

	report, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-PERP.STANDX"),
		ClientOrderID: "client-1",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderID("client-1"), report.OrderID)
	require.Equal(t, "BTC-USDT", sdk.created.Symbol)
	require.Equal(t, standxsdk.SideSell, sdk.created.Side)
	require.Equal(t, standxsdk.OrderTypeLimit, sdk.created.OrderType)
	require.Equal(t, standxsdk.TimeInForceIOC, sdk.created.TimeInForce)
	require.Equal(t, "0.5", sdk.created.Qty)
	require.Equal(t, "10", sdk.created.Price)
	require.Equal(t, "client-1", sdk.created.ClientOrdID)
}

func TestDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeMarketWS{}
	client := newDataClient("standx-perp-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.STANDX"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.STANDX"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, "BTC-USDT", ws.priceSymbol)
	require.Equal(t, "BTC-USDT", ws.depthSymbol)

	require.NoError(t, ws.priceHandler([]byte(`{"symbol":"BTC-USDT","last_price":"100.5","mid_price":"100.4","spread":["100","101"],"time":"1710000000000"}`)))
	tickerEvent := <-streaming.Events()
	require.Equal(t, "100", tickerEvent.Ticker.Bid.String())
	require.Equal(t, "101", tickerEvent.Ticker.Ask.String())
	require.Equal(t, "100.5", tickerEvent.Ticker.Last.String())

	require.NoError(t, ws.depthHandler([]byte(`{"symbol":"BTC-USDT","bids":[["100","1.2"]],"asks":[["101","0.7"]]}`)))
	bookEvent := <-streaming.Events()
	require.Equal(t, "100", bookEvent.OrderBook.Bids[0].Price.String())
	require.Equal(t, "101", bookEvent.OrderBook.Asks[0].Price.String())

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.STANDX"),
		Type:         model.MarketDataTypeTicker,
	}))
}

func TestDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeMarketWS{}
	client := newDataClient("standx-perp-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	id := model.MustInstrumentID("BTC-USDT-PERP.STANDX")
	bookSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeOrderBook, Depth: 20}
	quoteSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), bookSub))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 1, ws.depthSubscriptions)

	require.NoError(t, ws.depthHandler([]byte(`{"symbol":"BTC-USDT","bids":[["100","1.2"]],"asks":[["101","0.7"]]}`)))
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
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), bookSub))

	tradeSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTradeTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, "BTC-USDT", ws.tradeSymbol)
	require.Equal(t, 1, ws.tradeSubscriptions)
	require.NoError(t, ws.tradeHandler([]byte(`[{"symbol":"BTC-USDT","price":"100.25","qty":"0.25","is_buyer_taker":true,"time":"1710000000001"}]`)))
	tradeEvent := <-streaming.Events()
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, id, tradeEvent.Trade.InstrumentID)
	require.Equal(t, model.TradeID("BTC-USDT:1710000000001:100.25:0.25"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)
	require.Equal(t, "100.25", tradeEvent.Trade.Price.String())
	require.Equal(t, "0.25", tradeEvent.Trade.Size.String())
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), tradeSub))
}

func TestExecutionClientPrivateStreamMapsOrdersFillsAndPositions(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeAccountWS{}
	exec := newExecutionClient("acct", provider, sdk)
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.Equal(t, 1, ws.connects)
	require.Equal(t, 1, ws.auths)
	require.Equal(t, 1, ws.orderSubscriptions)
	require.Equal(t, 1, ws.tradeSubscriptions)
	require.Equal(t, 1, ws.positionSubscriptions)

	ws.orderHandler(&standxsdk.Order{ID: 100, ClOrdID: "client-1", Symbol: "BTC-USDT", Side: string(standxsdk.SideBuy), OrderType: string(standxsdk.OrderTypeLimit), Status: standxsdk.OrderStatusOpen, Qty: "1.5", FillQty: "0.5", Price: "100", FillAvgPrice: "100", UpdatedAt: "1710000000000"})
	orderEvent := <-exec.Events()
	require.Equal(t, model.OrderID("100"), orderEvent.Order.OrderID)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)
	require.Equal(t, "1", orderEvent.Order.LeavesQuantity.String())

	ws.tradeHandler(&standxsdk.Trade{ID: 55, OrderID: 100, Symbol: "BTC-USDT", Side: string(standxsdk.SideBuy), Price: "100", Qty: "0.5", FeeQty: "0.01", FeeAsset: "USDT", UpdatedAt: "1710000000001"})
	fillEvent := <-exec.Events()
	require.Equal(t, model.TradeID("55"), fillEvent.Fill.TradeID)
	require.Equal(t, model.OrderID("100"), fillEvent.Fill.OrderID)
	require.Equal(t, "0.5", fillEvent.Fill.Quantity.String())

	ws.positionHandler(&standxsdk.Position{Symbol: "BTC-USDT", Qty: "-0.25", EntryPrice: "10000", UpdatedAt: "1710000000002"})
	positionEvent := <-exec.Events()
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)
	require.Equal(t, "0.25", positionEvent.Position.Quantity.String())
	require.Equal(t, "10000", positionEvent.Position.EntryPrice.String())

	require.NoError(t, exec.ResubscribeExecution(context.Background()))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, 2, ws.auths)
	require.Equal(t, 2, ws.orderSubscriptions)
	require.Equal(t, 2, ws.tradeSubscriptions)
	require.Equal(t, 2, ws.positionSubscriptions)
}

func TestGeneratePositionStatusReportsUsesQueryPositions(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk)

	reports, err := exec.GeneratePositionStatusReports(context.Background(), model.MustInstrumentID("BTC-USDT-PERP.STANDX"))
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.PositionSideLong, reports[0].Side)
	require.Equal(t, "0.75", reports[0].Quantity.String())
	require.Equal(t, "12000", reports[0].EntryPrice.String())
}

type fakeSDK struct {
	created standxsdk.CreateOrderRequest
}

func (f *fakeSDK) QuerySymbolInfo(context.Context, string) ([]standxsdk.SymbolInfo, error) {
	return []standxsdk.SymbolInfo{{
		BaseAsset:         "BTC",
		QuoteAsset:        "USDT",
		Symbol:            "BTC-USDT",
		Enabled:           true,
		MakerFee:          "0.0001",
		TakerFee:          "0.0006",
		MaxLeverage:       "20",
		PriceTickDecimals: 1,
		QtyTickDecimals:   3,
		MinOrderQty:       "0.001",
	}}, nil
}

func (f *fakeSDK) QuerySymbolMarket(context.Context, string) (standxsdk.SymbolMarket, error) {
	return standxsdk.SymbolMarket{Symbol: "BTC-USDT", Bid1: "9", Ask1: "11", LastPrice: "10"}, nil
}

func (f *fakeSDK) QueryDepthBook(context.Context, string, int) (standxsdk.DepthBook, error) {
	return standxsdk.DepthBook{Symbol: "BTC-USDT", Bids: [][]string{{"9", "1"}}, Asks: [][]string{{"11", "1"}}}, nil
}

func (f *fakeSDK) QueryBalances(context.Context) (*standxsdk.Balance, error) {
	return &standxsdk.Balance{CrossAvailable: "90", Locked: "10", Balance: "100", Equity: "100"}, nil
}

func (f *fakeSDK) QueryUserAllOpenOrders(context.Context, string) ([]standxsdk.Order, error) {
	return []standxsdk.Order{{
		ID:          100,
		ClOrdID:     "client-open",
		Symbol:      "BTC-USDT",
		Side:        string(standxsdk.SideBuy),
		OrderType:   string(standxsdk.OrderTypeLimit),
		Status:      standxsdk.OrderStatusOpen,
		Qty:         "1",
		FillQty:     "0",
		Price:       "10",
		TimeInForce: string(standxsdk.TimeInForceGTC),
	}}, nil
}

func (f *fakeSDK) QueryPositions(context.Context, string) ([]standxsdk.Position, error) {
	return []standxsdk.Position{{Symbol: "BTC-USDT", Qty: "0.75", EntryPrice: "12000", UpdatedAt: "1710000000000"}}, nil
}

func (f *fakeSDK) CreateOrder(_ context.Context, req standxsdk.CreateOrderRequest, _ map[string]string) (*standxsdk.APIResponse, error) {
	f.created = req
	return &standxsdk.APIResponse{Code: 0, Message: "ok", RequestID: "req-1"}, nil
}

func (f *fakeSDK) CancelOrder(context.Context, standxsdk.CancelOrderRequest) (*standxsdk.APIResponse, error) {
	return &standxsdk.APIResponse{Code: 0, Message: "ok"}, nil
}

type fakeMarketWS struct {
	connects           int
	priceSymbol        string
	depthSymbol        string
	tradeSymbol        string
	depthSubscriptions int
	tradeSubscriptions int
	priceHandler       func([]byte) error
	depthHandler       func([]byte) error
	tradeHandler       func([]byte) error
}

func (f *fakeMarketWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeMarketWS) SubscribePrice(symbol string, handler func([]byte) error) error {
	f.priceSymbol = symbol
	f.priceHandler = handler
	return nil
}

func (f *fakeMarketWS) SubscribeDepthBook(symbol string, handler func([]byte) error) error {
	f.depthSymbol = symbol
	f.depthSubscriptions++
	f.depthHandler = handler
	return nil
}

func (f *fakeMarketWS) SubscribePublicTrade(symbol string, handler func([]byte) error) error {
	f.tradeSymbol = symbol
	f.tradeSubscriptions++
	f.tradeHandler = handler
	return nil
}

func (f *fakeMarketWS) Close() {}

type fakeAccountWS struct {
	connects              int
	auths                 int
	orderSubscriptions    int
	tradeSubscriptions    int
	positionSubscriptions int
	orderHandler          func(*standxsdk.Order)
	tradeHandler          func(*standxsdk.Trade)
	positionHandler       func(*standxsdk.Position)
}

func (f *fakeAccountWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeAccountWS) Auth() error {
	f.auths++
	return nil
}

func (f *fakeAccountWS) SubscribeOrderUpdate(handler func(*standxsdk.Order)) error {
	f.orderSubscriptions++
	f.orderHandler = handler
	return nil
}

func (f *fakeAccountWS) SubscribeTradeUpdate(handler func(*standxsdk.Trade)) error {
	f.tradeSubscriptions++
	f.tradeHandler = handler
	return nil
}

func (f *fakeAccountWS) SubscribePositionUpdate(handler func(*standxsdk.Position)) error {
	f.positionSubscriptions++
	f.positionHandler = handler
	return nil
}

func (f *fakeAccountWS) Close() {}
