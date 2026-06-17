package edgex

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	edgexperp "github.com/QuantProcessing/exchanges/sdk/edgex/perp"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	data := newDataClient("edgex-perp-data", provider, sdk)
	data.ws = &fakeMarketWS{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:            provider,
		Data:                data,
		Execution:           newExecutionClient("perp-acct", provider, sdk),
		InstrumentID:        model.MustInstrumentID("BTC-USDT-PERP.EDGEX"),
		Capabilities:        (&Adapter{}).Capabilities(),
		ExpectedMakerFee:    decimal.RequireFromString("0.0002"),
		ExpectedTakerFee:    decimal.RequireFromString("0.001"),
		ExpectedMarginInit:  decimal.RequireFromString("0.05"),
		ExpectedMarginMaint: decimal.RequireFromString("0.025"),
	})
}

func TestInstrumentProviderNormalizesFeeAndMarginMetadata(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))

	inst, ok := provider.Get(model.MustInstrumentID("BTC-USDT-PERP.EDGEX"))
	require.True(t, ok)
	require.Equal(t, "0.0002", inst.MakerFee.String())
	require.Equal(t, "0.001", inst.TakerFee.String())
	require.Equal(t, "0.05", inst.MarginInit.String())
	require.Equal(t, "0.025", inst.MarginMaint.String())
}

func TestDataClientFetchFundingRate(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("edgex-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.EDGEX")

	funding, err := client.FetchFundingRate(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, funding.InstrumentID)
	require.True(t, decimal.RequireFromString("0.0004").Equal(funding.Rate))
	require.Equal(t, 8*time.Hour, funding.FundingInterval)
	require.Equal(t, time.UnixMilli(1000), funding.Timestamp)
	require.Equal(t, time.UnixMilli(28801000), funding.NextFundingTime)
	require.True(t, sdk.getAllFundingRatesCalled)
	require.Empty(t, sdk.getFundingRateContractIDs)
}

func TestDataClientFetchFundingRateReturnsAllFundingError(t *testing.T) {
	allFundingErr := errors.New("all funding request failed")
	sdk := &fakeSDK{allFundingRatesErr: allFundingErr}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newDataClient("edgex-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.EDGEX")

	_, err := client.FetchFundingRate(context.Background(), id)
	require.ErrorIs(t, err, allFundingErr)
	require.True(t, sdk.getAllFundingRatesCalled)
	require.Empty(t, sdk.getFundingRateContractIDs)
}

func TestSubmitMapsOrderRequest(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	exec := newExecutionClient("acct", provider, sdk)

	report, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-PERP.EDGEX"),
		ClientOrderID: "client-1",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceIOC,
		Quantity:      decimal.RequireFromString("0.5"),
		Price:         decimal.RequireFromString("10"),
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderID("placed-1"), report.OrderID)
	require.Equal(t, "100", sdk.placed.ContractId)
	require.Equal(t, string(edgexperp.SideSell), sdk.placed.Side)
	require.Equal(t, string(edgexperp.OrderTypeLimit), sdk.placed.Type)
	require.Equal(t, string(edgexperp.TimeInForceImmediateOrCancel), sdk.placed.TimeInForce)
	require.Equal(t, "0.5", sdk.placed.Quantity)
	require.Equal(t, "10", sdk.placed.Price)
	require.Equal(t, "100", sdk.placeContract.ContractId)
	require.Equal(t, "2", sdk.placeQuote.CoinId)
}

func TestDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeMarketWS{}
	client := newDataClient("edgex-perp-data", provider, sdk)
	client.ws = ws

	var streaming venue.StreamingDataClient = client
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.EDGEX"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.EDGEX"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        15,
	}))

	require.Equal(t, 2, ws.connects)
	require.Equal(t, "100", ws.tickerContract)
	require.Equal(t, "100", ws.bookContract)
	require.Equal(t, edgexperp.OrderBookDepth15, ws.bookDepth)

	ws.emitTicker(t, `{"type":"snapshot","channel":"ticker.100","content":{"data":[{"contractId":"100","lastPrice":"100.5","close":"100.4"}]}}`)
	tickerEvent := <-streaming.Events()
	require.Equal(t, model.MustInstrumentID("BTC-USDT-PERP.EDGEX"), tickerEvent.Ticker.InstrumentID)
	require.Equal(t, "100.5", tickerEvent.Ticker.Last.String())

	ws.emitBook(t, `{"type":"snapshot","channel":"depth.100.15","content":{"data":[{"contractId":"100","bids":[{"price":"100","size":"1.2"}],"asks":[{"price":"101","size":"0.7"}]}]}}`)
	bookEvent := <-streaming.Events()
	require.Equal(t, "100", bookEvent.OrderBook.Bids[0].Price.String())
	require.Equal(t, "101", bookEvent.OrderBook.Asks[0].Price.String())

	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.EDGEX"),
		Type:         model.MarketDataTypeTicker,
	}))
	require.NoError(t, streaming.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.EDGEX"),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        15,
	}))
	require.Equal(t, "100", ws.unsubTickerContract)
	require.Equal(t, "100", ws.unsubBookContract)
	require.Equal(t, edgexperp.OrderBookDepth15, ws.unsubBookDepth)
}

func TestDataClientStreamsNautilusMarketDataTypes(t *testing.T) {
	sdk := &fakeSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	ws := &fakeMarketWS{}
	client := newDataClient("edgex-perp-data", provider, sdk)
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDT-PERP.EDGEX")

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeQuoteTick,
	}))
	require.Equal(t, "100", ws.bookContract)
	require.Equal(t, edgexperp.OrderBookDepth15, ws.bookDepth)
	ws.emitBook(t, `{"type":"snapshot","channel":"depth.100.15","content":{"data":[{"contractId":"100","bids":[{"price":"100","size":"1.25"}],"asks":[{"price":"101","size":"2.5"}]}]}}`)
	quoteEvent := <-client.Events()
	require.NotNil(t, quoteEvent.Quote)
	require.Equal(t, id, quoteEvent.Quote.InstrumentID)
	require.Equal(t, "2.5", quoteEvent.Quote.AskSize.String())

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTradeTick,
	}))
	require.Equal(t, "100", ws.tradeContract)
	ws.emitTrade(t, `{"type":"snapshot","channel":"trades.100","content":{"data":[{"ticketId":"trade-1","time":"2000","price":"100.5","size":"0.3","contractId":"100","isBuyerMaker":false}]}}`)
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
	require.Equal(t, "100", ws.klineContract)
	require.Equal(t, edgexperp.PriceTypeLastPrice, ws.klinePriceType)
	require.Equal(t, edgexperp.KlineInterval1m, ws.klineInterval)
	ws.emitKline(t, `{"type":"snapshot","channel":"kline.LAST_PRICE.100.MINUTE_1","content":{"data":[{"contractId":"100","klineTime":"3000","open":"100","high":"110","low":"95","close":"105","size":"12.5"}]}}`)
	barEvent := <-client.Events()
	require.NotNil(t, barEvent.Bar)
	require.Equal(t, barType.Canonical(), barEvent.Bar.BarType)
	require.Equal(t, "105", barEvent.Bar.Close.String())
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
	require.Equal(t, 1, ws.orderSubscriptions)
	require.Equal(t, 1, ws.fillSubscriptions)
	require.Equal(t, 1, ws.positionSubscriptions)

	ws.emitOrders([]edgexperp.Order{{
		Id:            "order-1",
		ContractId:    "100",
		ClientOrderId: "client-1",
		Side:          edgexperp.SideBuy,
		Type:          edgexperp.OrderTypeLimit,
		Status:        edgexperp.OrderStatusFilled,
		Size:          "1.5",
		CumFillSize:   "1.5",
		CumFillValue:  "150",
		Price:         "100",
		UpdatedTime:   "1710000000000",
	}})
	orderEvent := <-exec.Events()
	require.Equal(t, model.OrderID("order-1"), orderEvent.Order.OrderID)
	require.Equal(t, model.OrderStatusFilled, orderEvent.Order.Status)
	require.Equal(t, "0", orderEvent.Order.LeavesQuantity.String())
	require.Equal(t, "100", orderEvent.Order.AveragePrice.String())

	ws.emitFills([]edgexperp.OrderFillTransaction{{
		Id:         "fill-1",
		ContractId: "100",
		OrderId:    "order-1",
		OrderSide:  string(edgexperp.SideBuy),
		FillPrice:  "100",
		FillSize:   "0.5",
		FillFee:    "0.01",
		MatchTime:  "1710000000001",
	}})
	fillEvent := <-exec.Events()
	require.Equal(t, model.TradeID("fill-1"), fillEvent.Fill.TradeID)
	require.Equal(t, model.OrderID("order-1"), fillEvent.Fill.OrderID)
	require.Equal(t, "0.5", fillEvent.Fill.Quantity.String())
	require.Equal(t, model.Currency("USDT"), fillEvent.Fill.FeeCurrency)

	ws.emitPositions([]edgexperp.PositionInfo{{
		ContractId:  "100",
		OpenSize:    "-0.25",
		OpenValue:   "2500",
		UpdatedTime: "1710000000002",
	}})
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

type fakeSDK struct {
	placed                    edgexperp.PlaceOrderParams
	placeContract             *edgexperp.Contract
	placeQuote                *edgexperp.Coin
	getAllFundingRatesCalled  bool
	getFundingRateContractIDs []string
	allFundingRatesErr        error
}

func (f *fakeSDK) GetExchangeInfo(context.Context) (*edgexperp.ExchangeInfo, error) {
	return &edgexperp.ExchangeInfo{
		CoinList: []edgexperp.Coin{
			{CoinId: "1", CoinName: "BTC", StepSize: "0.000001", StarkExAssetId: "0x1", StarkExResolution: "1000000"},
			{CoinId: "2", CoinName: "USDT", StepSize: "0.000001", StarkExAssetId: "0x2", StarkExResolution: "1000000"},
		},
		ContractList: []edgexperp.Contract{{
			ContractId:          "100",
			ContractName:        "BTCUSDT",
			BaseCoinId:          "1",
			QuoteCoinId:         "2",
			TickSize:            "0.1",
			StepSize:            "0.001",
			EnableTrade:         true,
			DefaultTakerFeeRate: "0.001",
			DefaultMakerFeeRate: "0.0002",
			RiskTierList: []edgexperp.RiskTier{{
				MaxLeverage:           "20",
				MaintenanceMarginRate: "0.025",
			}},
			StarkExSyntheticAssetId: "0x3",
			StarkExResolution:       "1000000",
		}},
	}, nil
}

func (f *fakeSDK) GetTicker(context.Context, string) (*edgexperp.Ticker, error) {
	return &edgexperp.Ticker{ContractId: "100", LastPrice: "10", Close: "10"}, nil
}

func (f *fakeSDK) GetOrderBook(context.Context, string, int) (*edgexperp.OrderBook, error) {
	return &edgexperp.OrderBook{
		ContractId: "100",
		Bids:       []edgexperp.Level{{Price: "9", Size: "1"}},
		Asks:       []edgexperp.Level{{Price: "11", Size: "1"}},
	}, nil
}

func (f *fakeSDK) GetFundingRate(_ context.Context, contractID string) (*edgexperp.FundingRateData, error) {
	f.getFundingRateContractIDs = append(f.getFundingRateContractIDs, contractID)
	return &edgexperp.FundingRateData{
		ContractId:             "100",
		FundingRate:            "0.0004",
		OraclePrice:            "200",
		MarkPrice:              "201",
		IndexPrice:             "199",
		FundingTimestamp:       "1000",
		FundingRateIntervalMin: "480",
	}, nil
}

func (f *fakeSDK) GetAllFundingRates(context.Context) ([]edgexperp.FundingRateData, error) {
	f.getAllFundingRatesCalled = true
	return []edgexperp.FundingRateData{
		{ContractId: "200", FundingRate: "0.0001", OraclePrice: "100", MarkPrice: "101", IndexPrice: "99", FundingTimestamp: "900", FundingRateIntervalMin: "240"},
		{ContractId: "100", FundingRate: "0.0004", OraclePrice: "200", MarkPrice: "201", IndexPrice: "199", FundingTimestamp: "1000", FundingRateIntervalMin: "480"},
	}, f.allFundingRatesErr
}

func (f *fakeSDK) GetAccountAsset(context.Context) (*edgexperp.AccountAsset, error) {
	return &edgexperp.AccountAsset{
		CollateralList: []edgexperp.Collateral{{CoinId: "2", Amount: "100"}},
		CollateralAssetModelList: []edgexperp.CollateralAssetModel{{
			CoinId:          "2",
			AvailableAmount: "90",
			TotalEquity:     "100",
		}},
	}, nil
}

func (f *fakeSDK) GetOpenOrders(context.Context, *string) ([]edgexperp.Order, error) {
	return []edgexperp.Order{{
		Id:            "open-1",
		ContractId:    "100",
		Side:          edgexperp.SideBuy,
		Price:         "10",
		Size:          "1",
		ClientOrderId: "client-open",
		Type:          edgexperp.OrderTypeLimit,
		Status:        edgexperp.OrderStatusOpen,
		CumFillSize:   "0",
	}}, nil
}

func (f *fakeSDK) PlaceOrder(_ context.Context, params edgexperp.PlaceOrderParams, contract *edgexperp.Contract, quoteCoin *edgexperp.Coin) (*edgexperp.CreateOrderData, error) {
	f.placed = params
	f.placeContract = contract
	f.placeQuote = quoteCoin
	return &edgexperp.CreateOrderData{OrderId: "placed-1"}, nil
}

func (f *fakeSDK) CancelOrder(context.Context, string) (*edgexperp.CancelOrderData, error) {
	return &edgexperp.CancelOrderData{}, nil
}

type fakeMarketWS struct {
	connects            int
	tickerContract      string
	bookContract        string
	bookDepth           edgexperp.OrderBookDepth
	tradeContract       string
	klineContract       string
	klinePriceType      edgexperp.PriceType
	klineInterval       edgexperp.KlineInterval
	unsubTickerContract string
	unsubBookContract   string
	unsubBookDepth      edgexperp.OrderBookDepth
	unsubTradeContract  string
	unsubKlineContract  string
	tickerHandler       func(*edgexperp.WsTickerEvent)
	orderBookHandler    func(*edgexperp.WsDepthEvent)
	tradeHandler        func(*edgexperp.WsTradeEvent)
	klineHandler        func(*edgexperp.WsKlineEvent)
}

func (f *fakeMarketWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeMarketWS) SubscribeTicker(contractID string, callback func(*edgexperp.WsTickerEvent)) error {
	f.tickerContract = contractID
	f.tickerHandler = callback
	return nil
}

func (f *fakeMarketWS) SubscribeOrderBook(contractID string, depth edgexperp.OrderBookDepth, callback func(*edgexperp.WsDepthEvent)) error {
	f.bookContract = contractID
	f.bookDepth = depth
	f.orderBookHandler = callback
	return nil
}

func (f *fakeMarketWS) SubscribeTrades(contractID string, callback func(*edgexperp.WsTradeEvent)) error {
	f.tradeContract = contractID
	f.tradeHandler = callback
	return nil
}

func (f *fakeMarketWS) SubscribeKline(contractID string, priceType edgexperp.PriceType, interval edgexperp.KlineInterval, callback func(*edgexperp.WsKlineEvent)) error {
	f.klineContract = contractID
	f.klinePriceType = priceType
	f.klineInterval = interval
	f.klineHandler = callback
	return nil
}

func (f *fakeMarketWS) UnsubscribeTicker(contractID string) error {
	f.unsubTickerContract = contractID
	return nil
}

func (f *fakeMarketWS) UnsubscribeOrderBook(contractID string, depth edgexperp.OrderBookDepth) error {
	f.unsubBookContract = contractID
	f.unsubBookDepth = depth
	return nil
}

func (f *fakeMarketWS) UnsubscribeTrades(contractID string) error {
	f.unsubTradeContract = contractID
	return nil
}

func (f *fakeMarketWS) UnsubscribeKline(contractID string, _ edgexperp.PriceType, _ edgexperp.KlineInterval) error {
	f.unsubKlineContract = contractID
	return nil
}

func (f *fakeMarketWS) Close() {}

func (f *fakeMarketWS) emitTicker(t *testing.T, raw string) {
	t.Helper()
	var event edgexperp.WsTickerEvent
	require.NoError(t, json.Unmarshal([]byte(raw), &event))
	f.tickerHandler(&event)
}

func (f *fakeMarketWS) emitBook(t *testing.T, raw string) {
	t.Helper()
	var event edgexperp.WsDepthEvent
	require.NoError(t, json.Unmarshal([]byte(raw), &event))
	f.orderBookHandler(&event)
}

func (f *fakeMarketWS) emitTrade(t *testing.T, raw string) {
	t.Helper()
	var event edgexperp.WsTradeEvent
	require.NoError(t, json.Unmarshal([]byte(raw), &event))
	f.tradeHandler(&event)
}

func (f *fakeMarketWS) emitKline(t *testing.T, raw string) {
	t.Helper()
	var event edgexperp.WsKlineEvent
	require.NoError(t, json.Unmarshal([]byte(raw), &event))
	f.klineHandler(&event)
}

type fakeAccountWS struct {
	connects              int
	orderSubscriptions    int
	fillSubscriptions     int
	positionSubscriptions int
	orderHandler          func([]edgexperp.Order)
	fillHandler           func([]edgexperp.OrderFillTransaction)
	positionHandler       func([]edgexperp.PositionInfo)
}

func (f *fakeAccountWS) Connect() error {
	f.connects++
	return nil
}

func (f *fakeAccountWS) SubscribeOrderUpdate(handler func([]edgexperp.Order)) {
	f.orderSubscriptions++
	f.orderHandler = handler
}

func (f *fakeAccountWS) SubscribeOrderFillUpdate(handler func([]edgexperp.OrderFillTransaction)) {
	f.fillSubscriptions++
	f.fillHandler = handler
}

func (f *fakeAccountWS) SubscribePositionUpdate(handler func([]edgexperp.PositionInfo)) {
	f.positionSubscriptions++
	f.positionHandler = handler
}

func (f *fakeAccountWS) Close() {}

func (f *fakeAccountWS) emitOrders(orders []edgexperp.Order) {
	f.orderHandler(orders)
}

func (f *fakeAccountWS) emitFills(fills []edgexperp.OrderFillTransaction) {
	f.fillHandler(fills)
}

func (f *fakeAccountWS) emitPositions(positions []edgexperp.PositionInfo) {
	f.positionHandler(positions)
}
