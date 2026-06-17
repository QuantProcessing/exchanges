package lighter

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	lightersdk "github.com/QuantProcessing/exchanges/sdk/lighter"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	data := newDataClient("lighter-perp-data", provider, sdk)
	data.ws = &fakeMarketWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:            provider,
		Data:                data,
		Execution:           newExecutionClient("perp-acct", provider, sdk),
		InstrumentID:        model.MustInstrumentID("BTC-USDC-PERP.LIGHTER"),
		Capabilities:        (&Adapter{}).Capabilities(),
		ExpectedMakerFee:    decimal.RequireFromString("0.0002"),
		ExpectedTakerFee:    decimal.RequireFromString("0.0005"),
		ExpectedMarginInit:  decimal.RequireFromString("0.05"),
		ExpectedMarginMaint: decimal.RequireFromString("0.025"),
	})
}

func TestInstrumentProviderNormalizesFeeAndMarginMetadata(t *testing.T) {
	provider := newPerpProvider(&fakeSDK{})
	require.NoError(t, provider.LoadAll(context.Background()))

	inst, ok := provider.Get(model.MustInstrumentID("BTC-USDC-PERP.LIGHTER"))
	require.True(t, ok)
	require.Equal(t, "0.0002", inst.MakerFee.String())
	require.Equal(t, "0.0005", inst.TakerFee.String())
	require.Equal(t, "0.05", inst.MarginInit.String())
	require.Equal(t, "0.025", inst.MarginMaint.String())
}

func TestDataClientFetchFundingRate(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("lighter-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDC-PERP.LIGHTER")

	funding, err := client.FetchFundingRate(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, funding.InstrumentID)
	require.True(t, decimal.RequireFromString("0.0002").Equal(funding.Rate))
	require.Equal(t, time.Hour, funding.FundingInterval)
	require.Equal(t, time.Hour, funding.NextFundingTime.Sub(funding.Timestamp))
	require.Equal(t, 0, funding.Timestamp.Minute())
	require.Equal(t, 0, funding.Timestamp.Second())
}

func TestSubmitMapsDecimalsToLighterIntegerUnits(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk)

	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDC-PERP.LIGHTER"),
		ClientOrderID: "42",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, 7, sdk.placed.MarketId)
	require.Equal(t, uint32(1), sdk.placed.IsAsk)
	require.Equal(t, int64(500), sdk.placed.BaseAmount)
	require.Equal(t, uint32(1000), sdk.placed.Price)
	require.Equal(t, int64(42), sdk.placed.ClientOrderId)
}

func TestDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeMarketWS{}
	client := newDataClient("lighter-perp-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDC-PERP.LIGHTER"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDC-PERP.LIGHTER"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, 7, ws.tickerMarketID)
	require.Equal(t, 7, ws.bookMarketID)

	ws.emitTicker(t, `{"channel":"ticker/7","type":"update/ticker","timestamp":1710000000000,"ticker":{"b":{"price":"100","size":"1"},"a":{"price":"101","size":"2"}}}`)
	tickerEvent := <-streaming.Events()
	require.Equal(t, model.MustInstrumentID("BTC-USDC-PERP.LIGHTER"), tickerEvent.Ticker.InstrumentID)
	require.Equal(t, "100", tickerEvent.Ticker.Bid.String())
	require.Equal(t, "101", tickerEvent.Ticker.Ask.String())

	ws.emitBook(t, `{"channel":"order_book/7","type":"update/order_book","timestamp":1710000000001,"order_book":{"bids":[{"price":"100","size":"1.2"}],"asks":[{"price":"101","size":"0.7"}]}}`)
	bookEvent := <-streaming.Events()
	require.Equal(t, "100", bookEvent.OrderBook.Bids[0].Price.String())
	require.Equal(t, "101", bookEvent.OrderBook.Asks[0].Price.String())

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDC-PERP.LIGHTER"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDC-PERP.LIGHTER"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        20,
	}))
	require.Equal(t, 7, ws.unsubTickerMarketID)
	require.Equal(t, 7, ws.unsubBookMarketID)
}

func TestDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeMarketWS{}
	client := newDataClient("lighter-perp-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	id := model.MustInstrumentID("BTC-USDC-PERP.LIGHTER")
	tickerSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}
	quoteSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), tickerSub))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 1, ws.tickerSubscriptions)

	ws.emitTicker(t, `{"channel":"ticker/7","type":"update/ticker","timestamp":1710000000000,"ticker":{"b":{"price":"100","size":"1.2"},"a":{"price":"101","size":"0.7"}}}`)
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
	require.Equal(t, 7, ws.tradeMarketID)
	require.Equal(t, 1, ws.tradeSubscriptions)
	ws.emitTrade(t, `{"channel":"trade/7","type":"update/trade","nonce":9,"trades":[{"trade_id":55,"trade_id_str":"55","market_id":7,"price":"100.25","size":"0.25","is_maker_ask":true,"transaction_time":1710000000001000}]}`)
	tradeEvent := <-streaming.Events()
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, id, tradeEvent.Trade.InstrumentID)
	require.Equal(t, model.TradeID("55"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)
	require.Equal(t, "100.25", tradeEvent.Trade.Price.String())
	require.Equal(t, "0.25", tradeEvent.Trade.Size.String())
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, 1, ws.unsubTradeSubscriptions)
}

func TestExecutionClientPrivateStreamMapsOrdersFillsAndPositions(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeAccountWS{}
	exec := newExecutionClient("acct", provider, sdk)
	exec.privateWS = ws
	exec.accountIndex = 123
	exec.authToken = "auth-token"

	require.NoError(t, exec.Connect(context.Background()))
	require.Equal(t, 1, ws.connects)
	require.Equal(t, int64(123), ws.ordersAccount)
	require.Equal(t, "auth-token", ws.ordersAuth)
	require.Equal(t, 1, ws.orderSubscriptions)
	require.Equal(t, 1, ws.tradeSubscriptions)
	require.Equal(t, 1, ws.positionSubscriptions)

	ws.emitOrders(t, `{"channel":"account_all_orders/123","type":"update/account_all_orders","orders":{"7":[{"order_id":"100","client_order_id":"42","market_index":7,"side":"buy","price":"100","initial_base_amount":"1.5","filled_base_amount":"0.5","remaining_base_amount":"1","status":"partially-filled","type":"limit","updated_at":1710000000000}]}}`)
	orderEvent := <-exec.Events()
	require.Equal(t, model.OrderID("100"), orderEvent.Order.OrderID)
	require.Equal(t, model.OrderStatusPartiallyFilled, orderEvent.Order.Status)
	require.Equal(t, "1", orderEvent.Order.LeavesQuantity.String())

	ws.emitTrades(t, `{"channel":"account_all_trades/123","type":"update/account_all_trades","trades":{"7":[{"trade_id":55,"trade_id_str":"55","market_id":7,"price":"100","size":"0.5","bid_id":100,"bid_id_str":"100","bid_account_id":123,"taker_fee":0,"transaction_time":1710000000001000}]}}`)
	fillEvent := <-exec.Events()
	require.Equal(t, model.TradeID("55"), fillEvent.Fill.TradeID)
	require.Equal(t, model.OrderID("100"), fillEvent.Fill.OrderID)
	require.Equal(t, model.OrderSideBuy, fillEvent.Fill.Side)
	require.Equal(t, "0.5", fillEvent.Fill.Quantity.String())

	ws.emitPositions(t, `{"channel":"account_all_positions/123","type":"update/account_all_positions","positions":{"7":{"market_id":7,"position":"-0.25","avg_entry_price":"10000","sign":-1}}}`)
	positionEvent := <-exec.Events()
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)
	require.Equal(t, "0.25", positionEvent.Position.Quantity.String())
	require.Equal(t, "10000", positionEvent.Position.EntryPrice.String())

	require.NoError(t, exec.ResubscribeExecution(context.Background()))
	require.Equal(t, 2, ws.connects)
	require.Equal(t, 2, ws.orderSubscriptions)
	require.Equal(t, 2, ws.tradeSubscriptions)
	require.Equal(t, 2, ws.positionSubscriptions)
}

func TestGeneratePositionStatusReportsUsesAccountPositions(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk)

	reports, err := exec.GeneratePositionStatusReports(context.Background(), model.MustInstrumentID("BTC-USDC-PERP.LIGHTER"))
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, model.PositionSideLong, reports[0].Side)
	require.Equal(t, "0.75", reports[0].Quantity.String())
	require.Equal(t, "12000", reports[0].EntryPrice.String())
}

type fakeSDK struct {
	placed lightersdk.CreateOrderRequest
}

func (f *fakeSDK) GetOrderBookDetails(context.Context, *int, *string) (*lightersdk.OrderBookDetailsResponse, error) {
	return &lightersdk.OrderBookDetailsResponse{OrderBookDetails: []*lightersdk.OrderBookDetail{{
		Symbol:                       "BTC-USDC",
		MarketId:                     7,
		MarketType:                   "perp",
		Status:                       "active",
		TakerFee:                     "0.0005",
		MakerFee:                     "0.0002",
		SizeDecimals:                 3,
		PriceDecimals:                2,
		SupportedSizeDecimals:        3,
		DefaultInitialMarginFraction: 500,
		MaintenanceMarginFraction:    250,
	}}}, nil
}

func (f *fakeSDK) GetOrderBookOrders(context.Context, int, int64) (*lightersdk.OrderBookOrdersResponse, error) {
	return &lightersdk.OrderBookOrdersResponse{Bids: []lightersdk.Bid{{Price: "9", RemainingBaseAmount: "1"}}, Asks: []lightersdk.Ask{{Price: "11", RemainingBaseAmount: "1"}}}, nil
}

func (f *fakeSDK) GetFundingRates(context.Context) (*lightersdk.FundingRatesResponse, error) {
	return &lightersdk.FundingRatesResponse{
		Code: 200,
		FundingRate: []*lightersdk.FundingRate{
			{Symbol: "ETH-USDC", MarketId: 8, Exchange: "lighter", Rate: 0.0001},
			{Symbol: "BTC-USDC", MarketId: 7, Exchange: "lighter", Rate: 0.0002},
		},
	}, nil
}

func (f *fakeSDK) GetAccount(context.Context) (*lightersdk.AccountResponse, error) {
	return &lightersdk.AccountResponse{Accounts: []*lightersdk.Account{{
		AvailableBalance: "5",
		Collateral:       "6",
		Positions: []*lightersdk.Position{{
			MarketId:      7,
			Position:      "0.75",
			AvgEntryPrice: "12000",
			Sign:          1,
		}},
	}}}, nil
}

func (f *fakeSDK) GetAccountActiveOrders(context.Context, int) (*lightersdk.AccountActiveOrdersResponse, error) {
	return &lightersdk.AccountActiveOrdersResponse{Orders: []*lightersdk.Order{{OrderId: "100", ClientOrderId: "42", MarketIndex: 7, Side: "sell", Price: "10", InitialBaseAmount: "1", FilledBaseAmount: "0", Status: lightersdk.OrderStatusOpen, OrderType: lightersdk.OrderTypeRespLimit}}}, nil
}

func (f *fakeSDK) PlaceOrder(_ context.Context, req lightersdk.CreateOrderRequest) (*lightersdk.CreateOrderResponse, error) {
	f.placed = req
	return &lightersdk.CreateOrderResponse{TxHash: "tx-100"}, nil
}

func (f *fakeSDK) CancelOrder(context.Context, lightersdk.CancelOrderRequest) (*lightersdk.CancelOrderResponse, error) {
	return &lightersdk.CancelOrderResponse{TxHash: "tx-cancel"}, nil
}

type fakeMarketWS struct {
	connects                 int
	tickerMarketID           int
	bookMarketID             int
	tradeMarketID            int
	tickerSubscriptions      int
	tradeSubscriptions       int
	unsubTickerMarketID      int
	unsubBookMarketID        int
	unsubTradeMarketID       int
	unsubTickerSubscriptions int
	unsubTradeSubscriptions  int
	tickerHandler            func([]byte)
	orderBookHandler         func([]byte)
	tradeHandler             func([]byte)
}

func (f *fakeMarketWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeMarketWS) SubscribeTicker(marketID int, cb func([]byte)) error {
	f.tickerMarketID = marketID
	f.tickerSubscriptions++
	f.tickerHandler = cb
	return nil
}

func (f *fakeMarketWS) SubscribeOrderBook(marketID int, cb func([]byte)) error {
	f.bookMarketID = marketID
	f.orderBookHandler = cb
	return nil
}

func (f *fakeMarketWS) SubscribeTrades(marketID int, cb func([]byte)) error {
	f.tradeMarketID = marketID
	f.tradeSubscriptions++
	f.tradeHandler = cb
	return nil
}

func (f *fakeMarketWS) UnsubscribeTicker(marketID int) error {
	f.unsubTickerMarketID = marketID
	f.unsubTickerSubscriptions++
	return nil
}

func (f *fakeMarketWS) UnsubscribeOrderBook(marketID int) error {
	f.unsubBookMarketID = marketID
	return nil
}

func (f *fakeMarketWS) UnsubscribeTrades(marketID int) error {
	f.unsubTradeMarketID = marketID
	f.unsubTradeSubscriptions++
	return nil
}

func (f *fakeMarketWS) Close() {}

func (f *fakeMarketWS) emitTicker(t *testing.T, raw string) {
	t.Helper()
	require.NotNil(t, f.tickerHandler)
	f.tickerHandler([]byte(raw))
}

func (f *fakeMarketWS) emitBook(t *testing.T, raw string) {
	t.Helper()
	require.NotNil(t, f.orderBookHandler)
	f.orderBookHandler([]byte(raw))
}

func (f *fakeMarketWS) emitTrade(t *testing.T, raw string) {
	t.Helper()
	require.NotNil(t, f.tradeHandler)
	f.tradeHandler([]byte(raw))
}

type fakeAccountWS struct {
	connects              int
	ordersAccount         int64
	ordersAuth            string
	orderSubscriptions    int
	tradeSubscriptions    int
	positionSubscriptions int
	orderHandler          func([]byte)
	tradeHandler          func([]byte)
	positionHandler       func([]byte)
}

func (f *fakeAccountWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeAccountWS) SubscribeAccountAllOrders(accountID int64, authToken string, cb func([]byte)) error {
	f.ordersAccount = accountID
	f.ordersAuth = authToken
	f.orderSubscriptions++
	f.orderHandler = cb
	return nil
}

func (f *fakeAccountWS) SubscribeAccountAllTrades(accountID int64, authToken string, cb func([]byte)) error {
	f.tradeSubscriptions++
	f.tradeHandler = cb
	return nil
}

func (f *fakeAccountWS) SubscribeAccountAllPositions(accountID int64, authToken string, cb func([]byte)) error {
	f.positionSubscriptions++
	f.positionHandler = cb
	return nil
}

func (f *fakeAccountWS) Close() {}

func (f *fakeAccountWS) emitOrders(t *testing.T, raw string) {
	t.Helper()
	var event lightersdk.WsAccountAllOrdersEvent
	require.NoError(t, json.Unmarshal([]byte(raw), &event))
	f.orderHandler([]byte(raw))
}

func (f *fakeAccountWS) emitTrades(t *testing.T, raw string) {
	t.Helper()
	var event lightersdk.WsAccountAllTradesEvent
	require.NoError(t, json.Unmarshal([]byte(raw), &event))
	f.tradeHandler([]byte(raw))
}

func (f *fakeAccountWS) emitPositions(t *testing.T, raw string) {
	t.Helper()
	var event lightersdk.WsAccountAllPositionsEvent
	require.NoError(t, json.Unmarshal([]byte(raw), &event))
	f.positionHandler([]byte(raw))
}
