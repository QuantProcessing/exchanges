package nado

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	nadosdk "github.com/QuantProcessing/exchanges/sdk/nado"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	data := newDataClient("nado-perp-data", provider, sdk)
	data.ws = &fakeMarketWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:            provider,
		Data:                data,
		Execution:           newExecutionClient("perp-acct", provider, sdk, "sender-1"),
		InstrumentID:        model.MustInstrumentID("BTC-USDC-PERP.NADO"),
		Capabilities:        (&Adapter{}).Capabilities(),
		ExpectedMakerFee:    decimal.RequireFromString("0.0002"),
		ExpectedTakerFee:    decimal.RequireFromString("0.0005"),
		ExpectedMarginInit:  decimal.RequireFromString("0.10"),
		ExpectedMarginMaint: decimal.RequireFromString("0.05"),
	})
}

func TestInstrumentProviderNormalizesFeeAndMarginMetadata(t *testing.T) {
	provider := newPerpProvider(&fakeSDK{})
	require.NoError(t, provider.LoadAll(context.Background()))

	inst, ok := provider.Get(model.MustInstrumentID("BTC-USDC-PERP.NADO"))
	require.True(t, ok)
	require.Equal(t, "0.0002", inst.MakerFee.String())
	require.Equal(t, "0.0005", inst.TakerFee.String())
	require.Equal(t, "0.1", inst.MarginInit.String())
	require.Equal(t, "0.05", inst.MarginMaint.String())
}

func TestDataClientFetchFundingRate(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("nado-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDC-PERP.NADO")

	funding, err := client.FetchFundingRate(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, funding.InstrumentID)
	require.True(t, decimal.RequireFromString("0.0001").Equal(funding.Rate))
	require.Equal(t, time.Hour, funding.FundingInterval)
	require.Equal(t, time.UnixMilli(1200000), funding.Timestamp)
	require.Equal(t, time.UnixMilli(7200000), funding.NextFundingTime)
	require.True(t, sdk.getAllFundingRatesCalled)
	require.Empty(t, sdk.getFundingRateProductIDs)
}

func TestDataClientFetchFundingRateUsesCurrentTimestampWhenRawUpdateTimeMissing(t *testing.T) {
	sdk := &fakeSDK{fundingRates: map[string]nadosdk.FundingRateResponse{
		"9": {
			ProductID:      9,
			FundingRateX18: "2400000000000000",
		},
	}}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("nado-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDC-PERP.NADO")

	before := time.Now()
	funding, err := client.FetchFundingRate(context.Background(), id)
	require.NoError(t, err)
	require.False(t, funding.Timestamp.Before(before))
}

func TestSubmitMapsOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk, "sender-1")

	_, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDC-PERP.NADO"),
		ClientOrderID: "client-1",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, int64(9), sdk.placed.ProductId)
	require.Equal(t, "10", sdk.placed.Price)
	require.Equal(t, "0.5", sdk.placed.Amount)
	require.Equal(t, nadosdk.OrderSideSell, sdk.placed.Side)
}

func TestDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("nado-perp-data", provider, sdk)
	ws := &fakeMarketWS{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDC-PERP.NADO")

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTicker,
	}))
	require.Equal(t, int64(9), ws.tickerProductID)
	ws.emitTicker(&nadosdk.Ticker{ProductId: 9, BidPrice: "9", AskPrice: "11", Timestamp: "1000"})
	tickerEvent := <-client.Events()
	require.NotNil(t, tickerEvent.Ticker)
	require.Equal(t, id, tickerEvent.Ticker.InstrumentID)
	require.True(t, decimal.RequireFromString("9").Equal(tickerEvent.Ticker.Bid))
	require.True(t, decimal.RequireFromString("11").Equal(tickerEvent.Ticker.Ask))

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        25,
	}))
	require.Equal(t, int64(9), ws.bookProductID)
	ws.emitOrderBook(&nadosdk.OrderBook{
		ProductId: 9,
		Bids:      [][2]string{{"9", "1"}},
		Asks:      [][2]string{{"11", "2"}},
	})
	bookEvent := <-client.Events()
	require.NotNil(t, bookEvent.OrderBook)
	require.Equal(t, id, bookEvent.OrderBook.InstrumentID)
	require.True(t, decimal.RequireFromString("11").Equal(bookEvent.OrderBook.Asks[0].Price))
	require.True(t, decimal.RequireFromString("2").Equal(bookEvent.OrderBook.Asks[0].Size))

	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        25,
	}))
	require.Equal(t, int64(9), ws.unsubBookProductID)
}

func TestDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("nado-perp-data", provider, sdk)
	ws := &fakeMarketWS{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDC-PERP.NADO")

	tickerSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}
	quoteSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}
	require.NoError(t, client.SubscribeMarketData(context.Background(), tickerSub))
	require.NoError(t, client.SubscribeMarketData(context.Background(), quoteSub))
	require.Equal(t, 1, ws.subscribeCount)
	require.Equal(t, int64(9), ws.tickerProductID)
	ws.emitTicker(&nadosdk.Ticker{ProductId: 9, BidPrice: "9", BidQty: "1.25", AskPrice: "11", AskQty: "2.5", Timestamp: "1000"})
	tickerEvent := <-client.Events()
	require.NotNil(t, tickerEvent.Ticker)
	quoteEvent := <-client.Events()
	require.NotNil(t, quoteEvent.Quote)
	require.Equal(t, id, quoteEvent.Quote.InstrumentID)
	require.True(t, decimal.RequireFromString("2.5").Equal(quoteEvent.Quote.AskSize))

	require.NoError(t, client.UnsubscribeMarketData(context.Background(), quoteSub))
	require.Zero(t, ws.unsubTickerProductID)
	require.NoError(t, client.UnsubscribeMarketData(context.Background(), tickerSub))
	require.Equal(t, int64(9), ws.unsubTickerProductID)

	tradeSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTradeTick}
	require.NoError(t, client.SubscribeMarketData(context.Background(), tradeSub))
	require.Equal(t, int64(9), ws.tradeProductID)
	ws.emitTrade(&nadosdk.Trade{ProductId: 9, Price: "100.5", TakerQty: "0.3", IsTakerBuyer: true, Timestamp: "2000"})
	tradeEvent := <-client.Events()
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, model.AggressorSideBuyer, tradeEvent.Trade.AggressorSide)
	require.True(t, decimal.RequireFromString("0.3").Equal(tradeEvent.Trade.Size))

	barType := model.NewTimeBarType(id, time.Minute)
	barSub := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeBar, BarType: barType}
	require.NoError(t, client.SubscribeMarketData(context.Background(), barSub))
	require.Equal(t, int64(9), ws.candleProductID)
	require.Equal(t, int32(60), ws.candleGranularity)
	ws.emitCandlestick(&nadosdk.Candlestick{
		ProductId:   9,
		Granularity: 60,
		Timestamp:   "3000",
		OpenX18:     "100000000000000000000",
		HighX18:     "110000000000000000000",
		LowX18:      "95000000000000000000",
		CloseX18:    "105000000000000000000",
		Volume:      "12.5",
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
	exec := newExecutionClient("perp-acct", provider, sdk, "sender-1")
	ws := &fakeAccountWS{}
	exec.privateWS = ws

	require.NoError(t, exec.Connect(context.Background()))
	require.True(t, ws.connected)
	require.Nil(t, ws.ordersProductID)
	require.Nil(t, ws.fillsProductID)
	require.Nil(t, ws.positionsProductID)

	ws.emitOrder(&nadosdk.OrderUpdate{ProductId: 9, Digest: "digest-1", Amount: "1", Reason: "placed"})
	orderEvent := <-exec.Events()
	require.NotNil(t, orderEvent.Order)
	require.Equal(t, model.OrderID("digest-1"), orderEvent.Order.OrderID)
	require.Equal(t, model.OrderStatusAccepted, orderEvent.Order.Status)

	ws.emitFill(&nadosdk.Fill{
		Digest:    "digest-1",
		TradeId:   "trade-1",
		ProductId: 9,
		Price:     "10",
		Size:      "0.4",
		Side:      "buy",
		Fee:       "0.01",
		Time:      1000,
	})
	fillEvent := <-exec.Events()
	require.NotNil(t, fillEvent.Fill)
	require.Equal(t, model.TradeID("trade-1"), fillEvent.Fill.TradeID)
	require.True(t, decimal.RequireFromString("0.4").Equal(fillEvent.Fill.Quantity))

	ws.emitPosition(&nadosdk.PositionChange{
		ProductId:  9,
		Amount:     "-0.2",
		EntryPrice: "10",
		Side:       "short",
	})
	positionEvent := <-exec.Events()
	require.NotNil(t, positionEvent.Position)
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)
	require.True(t, decimal.RequireFromString("0.2").Equal(positionEvent.Position.Quantity))

	require.NoError(t, exec.ResubscribeExecution(context.Background()))
	require.Equal(t, 6, ws.subscribeCount)
}

type fakeSDK struct {
	placed                   nadosdk.ClientOrderInput
	getAllFundingRatesCalled bool
	getFundingRateProductIDs []int64
	fundingRates             map[string]nadosdk.FundingRateResponse
}

func (f *fakeSDK) GetContracts(context.Context, *bool) (nadosdk.ContractV2Map, error) {
	return nadosdk.ContractV2Map{"BTC-USDC": {ProductID: 9, TickerID: "BTC-USDC", BaseCurrency: "BTC", QuoteCurrency: "USDC", ProductType: "perp", LastPrice: 10, NextFundingRateTimestamp: 7200000}}, nil
}

func (f *fakeSDK) GetSymbols(context.Context, *string) (*nadosdk.SymbolsInfo, error) {
	return &nadosdk.SymbolsInfo{Symbols: map[string]nadosdk.Symbol{
		"BTC-USDC": {
			Type:                     "perp",
			ProductID:                9,
			Symbol:                   "BTC-USDC",
			MakerFeeRateX18:          "200000000000000",
			TakerFeeRateX18:          "500000000000000",
			LongWeightInitialX18:     "900000000000000000",
			LongWeightMaintenanceX18: "950000000000000000",
		},
	}}, nil
}

func (f *fakeSDK) GetOrderBook(context.Context, string, int) (*nadosdk.OrderBookV2, error) {
	return &nadosdk.OrderBookV2{Bids: [][2]float64{{9, 1}}, Asks: [][2]float64{{11, 1}}}, nil
}

func (f *fakeSDK) GetFundingRate(_ context.Context, productID int64) (*nadosdk.FundingRateResponse, error) {
	f.getFundingRateProductIDs = append(f.getFundingRateProductIDs, productID)
	return &nadosdk.FundingRateResponse{
		ProductID:      9,
		FundingRateX18: "2400000000000000",
		UpdateTime:     "1200000",
	}, nil
}

func (f *fakeSDK) GetAllFundingRates(context.Context) (map[string]nadosdk.FundingRateResponse, error) {
	f.getAllFundingRatesCalled = true
	if f.fundingRates != nil {
		return f.fundingRates, nil
	}
	return map[string]nadosdk.FundingRateResponse{
		"8": {ProductID: 8, FundingRateX18: "4800000000000000"},
		"9": {ProductID: 9, FundingRateX18: "2400000000000000", UpdateTime: "1200000"},
	}, nil
}

func (f *fakeSDK) GetAccount(context.Context) (*nadosdk.AccountInfo, error) {
	bal := nadosdk.Balance{ProductID: 9}
	bal.Balance.Amount = "6"
	return &nadosdk.AccountInfo{PerpBalances: []nadosdk.Balance{bal}}, nil
}

func (f *fakeSDK) GetAccountProductOrders(context.Context, int64, string) (*nadosdk.AccountProductOrders, error) {
	return &nadosdk.AccountProductOrders{Orders: []nadosdk.Order{{ProductID: 9, Digest: "digest-1", PriceX18: "10", Amount: "1", UnfilledAmount: "1", OrderType: "limit"}}}, nil
}

func (f *fakeSDK) PlaceOrder(_ context.Context, input nadosdk.ClientOrderInput) (*nadosdk.PlaceOrderResponse, error) {
	f.placed = input
	return &nadosdk.PlaceOrderResponse{Digest: "digest-placed"}, nil
}

func (f *fakeSDK) CancelOrders(context.Context, nadosdk.CancelOrdersInput) (*nadosdk.CancelOrdersResponse, error) {
	return &nadosdk.CancelOrdersResponse{}, nil
}

type fakeMarketWS struct {
	connected            bool
	subscribeCount       int
	tickerProductID      int64
	bookProductID        int64
	tradeProductID       int64
	candleProductID      int64
	candleGranularity    int32
	unsubTickerProductID int64
	unsubBookProductID   int64
	unsubTradeProductID  int64
	unsubCandleProductID int64
	tickerHandler        func(*nadosdk.Ticker)
	bookHandler          func(*nadosdk.OrderBook)
	tradeHandler         func(*nadosdk.Trade)
	candleHandler        func(*nadosdk.Candlestick)
}

func (f *fakeMarketWS) Connect() error {
	f.connected = true
	return nil
}

func (f *fakeMarketWS) SubscribeTicker(productID int64, handler func(*nadosdk.Ticker)) error {
	f.connected = true
	f.subscribeCount++
	f.tickerProductID = productID
	f.tickerHandler = handler
	return nil
}

func (f *fakeMarketWS) SubscribeOrderBook(productID int64, handler func(*nadosdk.OrderBook)) error {
	f.connected = true
	f.subscribeCount++
	f.bookProductID = productID
	f.bookHandler = handler
	return nil
}

func (f *fakeMarketWS) SubscribeTrades(productID int64, handler func(*nadosdk.Trade)) error {
	f.connected = true
	f.subscribeCount++
	f.tradeProductID = productID
	f.tradeHandler = handler
	return nil
}

func (f *fakeMarketWS) SubscribeLatestCandlestick(productID int64, granularity int32, handler func(*nadosdk.Candlestick)) error {
	f.connected = true
	f.subscribeCount++
	f.candleProductID = productID
	f.candleGranularity = granularity
	f.candleHandler = handler
	return nil
}

func (f *fakeMarketWS) UnsubscribeTicker(productID int64) error {
	f.unsubTickerProductID = productID
	return nil
}

func (f *fakeMarketWS) UnsubscribeOrderBook(productID int64) error {
	f.unsubBookProductID = productID
	return nil
}

func (f *fakeMarketWS) UnsubscribeTrades(productID int64) error {
	f.unsubTradeProductID = productID
	return nil
}

func (f *fakeMarketWS) UnsubscribeLatestCandlestick(productID int64, _ int32) error {
	f.unsubCandleProductID = productID
	return nil
}

func (f *fakeMarketWS) Close() {
	f.connected = false
}

func (f *fakeMarketWS) emitTicker(ticker *nadosdk.Ticker) {
	f.tickerHandler(ticker)
}

func (f *fakeMarketWS) emitOrderBook(book *nadosdk.OrderBook) {
	f.bookHandler(book)
}

func (f *fakeMarketWS) emitTrade(trade *nadosdk.Trade) {
	f.tradeHandler(trade)
}

func (f *fakeMarketWS) emitCandlestick(candle *nadosdk.Candlestick) {
	f.candleHandler(candle)
}

type fakeAccountWS struct {
	connected          bool
	subscribeCount     int
	ordersProductID    *int64
	fillsProductID     *int64
	positionsProductID *int64
	orderHandler       func(*nadosdk.OrderUpdate)
	fillHandler        func(*nadosdk.Fill)
	positionHandler    func(*nadosdk.PositionChange)
}

func (f *fakeAccountWS) Connect() error {
	f.connected = true
	return nil
}

func (f *fakeAccountWS) SubscribeOrders(productID *int64, handler func(*nadosdk.OrderUpdate)) error {
	f.connected = true
	f.subscribeCount++
	f.ordersProductID = productID
	f.orderHandler = handler
	return nil
}

func (f *fakeAccountWS) SubscribeFills(productID *int64, handler func(*nadosdk.Fill)) error {
	f.connected = true
	f.subscribeCount++
	f.fillsProductID = productID
	f.fillHandler = handler
	return nil
}

func (f *fakeAccountWS) SubscribePositions(productID *int64, handler func(*nadosdk.PositionChange)) error {
	f.connected = true
	f.subscribeCount++
	f.positionsProductID = productID
	f.positionHandler = handler
	return nil
}

func (f *fakeAccountWS) Close() {
	f.connected = false
}

func (f *fakeAccountWS) emitOrder(order *nadosdk.OrderUpdate) {
	f.orderHandler(order)
}

func (f *fakeAccountWS) emitFill(fill *nadosdk.Fill) {
	f.fillHandler(fill)
}

func (f *fakeAccountWS) emitPosition(position *nadosdk.PositionChange) {
	f.positionHandler(position)
}
