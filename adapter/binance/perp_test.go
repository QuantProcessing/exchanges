package binance

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpClientsPassVenueContractSuite(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	data := newPerpDataClient("binance-perp-data", provider, sdk)
	data.ws = &fakePerpMarketStream{}
	testsuite.RunVenueContractSuite(t, testsuite.VenueContractConfig{
		Provider:     provider,
		Data:         data,
		Execution:    newPerpExecutionClient("perp-acct", provider, sdk),
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		Capabilities: (&PerpAdapter{}).Capabilities(),
	})
}

func TestPerpProviderLoadsExchangeInfo(t *testing.T) {
	provider := newPerpProvider(&fakePerpSDK{})
	require.NoError(t, provider.LoadAll(context.Background()))

	inst, ok := provider.Get(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"))
	require.True(t, ok)
	require.Equal(t, "BTCUSDT", inst.RawSymbol)
	require.Equal(t, model.InstrumentTypePerp, inst.Type)
	require.Equal(t, model.Currency("USDT"), inst.Settle)
}

func TestPerpDataAndExecutionClients(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	data := newPerpDataClient("binance-perp-data", provider, sdk)
	exec := newPerpExecutionClient("perp-acct", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")

	ticker, err := data.FetchTicker(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, decimal.RequireFromString("200"), ticker.Last)

	report, err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "perp-acct",
		InstrumentID:  id,
		ClientOrderID: "perp-client-1",
		Side:          model.OrderSideSell,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("0.2"),
		Price:         decimal.RequireFromString("200"),
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderID("77"), report.OrderID)
	require.Equal(t, "BTCUSDT", sdk.placed.Symbol)
}

func TestPerpDataClientRestSnapshotsUseVenueTimestamps(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	data := newPerpDataClient("binance-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")

	ticker, err := data.FetchTicker(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(1000), ticker.Timestamp)

	book, err := data.FetchOrderBook(context.Background(), id, 5)
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(2000), book.Timestamp)
}

func TestPerpDataClientFetchFundingRate(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	data := newPerpDataClient("binance-perp-data", provider, sdk)
	id := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")

	funding, err := data.FetchFundingRate(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, funding.InstrumentID)
	require.True(t, decimal.RequireFromString("0.0008").Equal(funding.Rate))
	require.True(t, decimal.RequireFromString("200").Equal(funding.MarkPrice))
	require.True(t, decimal.RequireFromString("199").Equal(funding.IndexPrice))
	require.Equal(t, 8*time.Hour, funding.FundingInterval)
	require.Equal(t, time.UnixMilli(1000), funding.Timestamp)
	require.Equal(t, time.UnixMilli(28800000), funding.NextFundingTime)
}

func TestPerpDataClientStreamsTickerAndOrderBook(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newPerpDataClient("binance-perp-data", provider, sdk)
	ws := &fakePerpMarketStream{}
	client.ws = ws
	id := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTicker,
	}))
	require.Equal(t, "BTCUSDT", ws.bookTickerSymbol)
	ws.bookTickerCB(&perp.WsBookTickerEvent{
		Symbol:       "BTCUSDT",
		BestBidPrice: "199",
		BestAskPrice: "201",
		EventTime:    1000,
	})
	tickerEvent := <-client.Events()
	require.NotNil(t, tickerEvent.Ticker)
	require.Equal(t, id, tickerEvent.Ticker.InstrumentID)
	require.True(t, decimal.RequireFromString("199").Equal(tickerEvent.Ticker.Bid))

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeQuoteTick,
	}))
	require.Equal(t, "BTCUSDT", ws.bookTickerSymbol)
	ws.bookTickerCB(&perp.WsBookTickerEvent{
		Symbol:       "BTCUSDT",
		BestBidPrice: "198",
		BestBidQty:   "1.5",
		BestAskPrice: "202",
		BestAskQty:   "2.5",
		EventTime:    2000,
	})
	quoteEvent := requireNextQuoteEvent(t, client.Events())
	require.NotNil(t, quoteEvent.Quote)
	require.Equal(t, id, quoteEvent.Quote.InstrumentID)
	require.Equal(t, "198", quoteEvent.Quote.BidPrice.String())
	require.Equal(t, "2.5", quoteEvent.Quote.AskSize.String())

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTradeTick,
	}))
	require.Equal(t, "BTCUSDT", ws.tradeSymbol)
	ws.tradeCB(&perp.WsAggTradeEvent{
		Symbol:       "BTCUSDT",
		AggTradeID:   43,
		Price:        "200.5",
		Quantity:     "0.4",
		TradeTime:    3000,
		EventTime:    3100,
		IsBuyerMaker: true,
	})
	tradeEvent := <-client.Events()
	require.NotNil(t, tradeEvent.Trade)
	require.Equal(t, id, tradeEvent.Trade.InstrumentID)
	require.Equal(t, "200.5", tradeEvent.Trade.Price.String())
	require.Equal(t, model.TradeID("43"), tradeEvent.Trade.TradeID)
	require.Equal(t, model.AggressorSideSeller, tradeEvent.Trade.AggressorSide)

	barType := model.NewTimeBarType(id, time.Minute)
	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	}))
	require.Equal(t, "BTCUSDT", ws.klineSymbol)
	require.Equal(t, "1m", ws.klineInterval)
	kline := &perp.WsKlineEvent{Symbol: "BTCUSDT", EventTime: 61000}
	kline.Kline.StartTime = 60000
	kline.Kline.OpenPrice = "200"
	kline.Kline.HighPrice = "210"
	kline.Kline.LowPrice = "195"
	kline.Kline.ClosePrice = "205"
	kline.Kline.Volume = "22.5"
	require.NoError(t, ws.klineCB(kline))
	barEvent := <-client.Events()
	require.NotNil(t, barEvent.Bar)
	require.Equal(t, barType.Canonical(), barEvent.Bar.BarType)
	require.Equal(t, "205", barEvent.Bar.Close.String())
	require.Equal(t, "22.5", barEvent.Bar.Volume.String())

	require.NoError(t, client.SubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        10,
	}))
	require.Equal(t, "BTCUSDT", ws.depthSymbol)
	require.Equal(t, 10, ws.depthLevels)
	ws.depthCB(&perp.WsDepthEvent{
		Symbol:    "BTCUSDT",
		EventTime: 1000,
		Bids:      [][]interface{}{{"199", "1"}},
		Asks:      [][]interface{}{{"201", "2"}},
	})
	bookEvent := <-client.Events()
	require.NotNil(t, bookEvent.OrderBook)
	require.Equal(t, id, bookEvent.OrderBook.InstrumentID)
	require.True(t, decimal.RequireFromString("201").Equal(bookEvent.OrderBook.Asks[0].Price))

	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        10,
	}))
	require.Equal(t, "BTCUSDT", ws.unsubDepthSymbol)
	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeTradeTick,
	}))
	require.Equal(t, "BTCUSDT", ws.unsubTradeSymbol)
	require.NoError(t, client.UnsubscribeMarketData(context.Background(), model.SubscribeMarketData{
		InstrumentID: id,
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	}))
	require.Equal(t, "BTCUSDT", ws.unsubKlineSymbol)
}

func TestPerpExecutionClientPrivateStreamMapsOrdersFillsAndPositions(t *testing.T) {
	sdk := &fakePerpSDK{}
	provider := newPerpProvider(sdk)
	require.NoError(t, provider.LoadAll(context.Background()))
	client := newPerpExecutionClient("perp-acct", provider, sdk)
	stream := &fakePerpAccountStream{}
	client.accountStream = stream

	require.NoError(t, client.Connect(context.Background()))
	require.True(t, stream.connected)
	orderUpdate := &perp.OrderUpdateEvent{}
	orderUpdate.Order.Symbol = "BTCUSDT"
	orderUpdate.Order.ClientOrderID = "perp-client-1"
	orderUpdate.Order.Side = "SELL"
	orderUpdate.Order.OrderType = "LIMIT"
	orderUpdate.Order.OrderStatus = "FILLED"
	orderUpdate.Order.OrderID = 77
	orderUpdate.Order.OriginalQty = "0.2"
	orderUpdate.Order.OriginalPrice = "200"
	orderUpdate.Order.AccumulatedFilledQty = "0.2"
	orderUpdate.Order.LastFilledQty = "0.2"
	orderUpdate.Order.LastFilledPrice = "200"
	orderUpdate.Order.Commission = "0.01"
	orderUpdate.Order.CommissionAsset = "USDT"
	orderUpdate.Order.TradeTime = 1000
	orderUpdate.Order.TradeID = 88
	stream.orderUpdateCB(orderUpdate)

	orderEvent := <-client.Events()
	require.NotNil(t, orderEvent.Order)
	require.Equal(t, model.OrderStatusFilled, orderEvent.Order.Status)
	fillEvent := <-client.Events()
	require.NotNil(t, fillEvent.Fill)
	require.Equal(t, model.TradeID("88"), fillEvent.Fill.TradeID)
	require.Equal(t, model.OrderSideSell, fillEvent.Fill.Side)

	accountUpdate := &perp.AccountUpdateEvent{}
	accountUpdate.UpdateData.Balances = append(accountUpdate.UpdateData.Balances, struct {
		Asset              string `json:"a"`
		WalletBalance      string `json:"wb"`
		CrossWalletBalance string `json:"cw"`
		BalanceChange      string `json:"bc"`
	}{Asset: "USDT", WalletBalance: "20", CrossWalletBalance: "18"})
	accountUpdate.UpdateData.Positions = append(accountUpdate.UpdateData.Positions, struct {
		Symbol              string `json:"s"`
		PositionAmount      string `json:"pa"`
		EntryPrice          string `json:"ep"`
		AccumulatedRealized string `json:"cr"`
		UnrealizedPnL       string `json:"up"`
		MarginType          string `json:"mt"`
		IsolatedWallet      string `json:"iw"`
		PositionSide        string `json:"ps"`
	}{Symbol: "BTCUSDT", PositionAmount: "-0.2", EntryPrice: "200", PositionSide: "SHORT"})
	stream.accountUpdateCB(accountUpdate)

	accountEvent := <-client.Events()
	require.NotNil(t, accountEvent.Account)
	require.Equal(t, model.AccountID("perp-acct"), accountEvent.Account.AccountID)
	positionEvent := <-client.Events()
	require.NotNil(t, positionEvent.Position)
	require.Equal(t, model.PositionSideShort, positionEvent.Position.Side)

	require.NoError(t, client.ResubscribeExecution(context.Background()))
	require.Equal(t, 2, stream.connects)
}

type fakePerpSDK struct {
	placed perp.PlaceOrderParams
}

func (f *fakePerpSDK) ExchangeInfo(context.Context) (*perp.ExchangeInfoResponse, error) {
	return &perp.ExchangeInfoResponse{Symbols: []perp.SymbolInfo{{
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

func (f *fakePerpSDK) Ticker(context.Context, string) (*perp.TickerResponse, error) {
	return &perp.TickerResponse{Symbol: "BTCUSDT", LastPrice: "200", CloseTime: 1000}, nil
}

func (f *fakePerpSDK) Depth(context.Context, string, int) (*perp.DepthResponse, error) {
	return &perp.DepthResponse{
		E:    2000,
		T:    1900,
		Bids: [][]string{{"199", "1"}},
		Asks: [][]string{{"201", "2"}},
	}, nil
}

func (f *fakePerpSDK) GetFundingRate(context.Context, string) (*perp.FundingRateData, error) {
	return &perp.FundingRateData{
		Symbol:               "BTCUSDT",
		MarkPrice:            "200",
		IndexPrice:           "199",
		LastFundingRate:      "0.0008",
		HourlyFundingRate:    "0.0001000000",
		NextFundingTime:      28800000,
		Time:                 1000,
		FundingIntervalHours: 8,
		FundingTime:          0,
	}, nil
}

func (f *fakePerpSDK) GetAccount(context.Context) (*perp.AccountResponse, error) {
	return &perp.AccountResponse{Assets: []struct {
		Asset                  string `json:"asset"`
		WalletBalance          string `json:"walletBalance"`
		UnrealizedProfit       string `json:"unrealizedProfit"`
		MarginBalance          string `json:"marginBalance"`
		MaintMargin            string `json:"maintMargin"`
		InitialMargin          string `json:"initialMargin"`
		PositionInitialMargin  string `json:"positionInitialMargin"`
		OpenOrderInitialMargin string `json:"openOrderInitialMargin"`
		MaxWithdrawAmount      string `json:"maxWithdrawAmount"`
		CrossWalletBalance     string `json:"crossWalletBalance"`
		CrossUnPnl             string `json:"crossUnPnl"`
		AvailableBalance       string `json:"availableBalance"`
		MarginAvailable        bool   `json:"marginAvailable"`
		UpdateTime             int64  `json:"updateTime"`
	}{{Asset: "USDT", WalletBalance: "20", AvailableBalance: "18"}}}, nil
}

func (f *fakePerpSDK) PlaceOrder(_ context.Context, p perp.PlaceOrderParams) (*perp.OrderResponse, error) {
	f.placed = p
	return &perp.OrderResponse{Symbol: p.Symbol, OrderID: 77, ClientOrderID: p.NewClientOrderID, Status: "NEW", Side: p.Side, Type: p.Type, OrigQty: p.Quantity, Price: p.Price}, nil
}

func (f *fakePerpSDK) CancelOrder(context.Context, perp.CancelOrderParams) (*perp.OrderResponse, error) {
	return &perp.OrderResponse{OrderID: 77, Status: "CANCELED"}, nil
}

func (f *fakePerpSDK) GetOpenOrders(context.Context, string) ([]perp.OrderResponse, error) {
	return []perp.OrderResponse{{Symbol: "BTCUSDT", OrderID: 77, Status: "NEW"}}, nil
}

type fakePerpMarketStream struct {
	connected        bool
	bookTickerSymbol string
	bookTickerCB     func(*perp.WsBookTickerEvent) error
	depthSymbol      string
	depthLevels      int
	depthCB          func(*perp.WsDepthEvent) error
	tradeSymbol      string
	tradeCB          func(*perp.WsAggTradeEvent) error
	klineSymbol      string
	klineInterval    string
	klineCB          func(*perp.WsKlineEvent) error
	unsubDepthSymbol string
	unsubTradeSymbol string
	unsubKlineSymbol string
}

func (f *fakePerpMarketStream) Connect() error {
	f.connected = true
	return nil
}
func (f *fakePerpMarketStream) Close()                  { f.connected = false }
func (f *fakePerpMarketStream) IsConnected() bool       { return f.connected }
func (f *fakePerpMarketStream) SetPostReconnect(func()) {}
func (f *fakePerpMarketStream) SubscribeBookTicker(symbol string, cb func(*perp.WsBookTickerEvent) error) error {
	f.bookTickerSymbol = symbol
	f.bookTickerCB = cb
	return nil
}
func (f *fakePerpMarketStream) SubscribeLimitOrderBook(symbol string, levels int, _ string, cb func(*perp.WsDepthEvent) error) error {
	f.depthSymbol = symbol
	f.depthLevels = levels
	f.depthCB = cb
	return nil
}
func (f *fakePerpMarketStream) SubscribeAggTrade(symbol string, cb func(*perp.WsAggTradeEvent) error) error {
	f.tradeSymbol = symbol
	f.tradeCB = cb
	return nil
}
func (f *fakePerpMarketStream) SubscribeKline(symbol string, interval string, cb func(*perp.WsKlineEvent) error) error {
	f.klineSymbol = symbol
	f.klineInterval = interval
	f.klineCB = cb
	return nil
}
func (f *fakePerpMarketStream) UnsubscribeBookTicker(symbol string) error {
	f.bookTickerSymbol = symbol
	return nil
}
func (f *fakePerpMarketStream) UnsubscribeLimitOrderBook(symbol string, _ int, _ string) error {
	f.unsubDepthSymbol = symbol
	return nil
}
func (f *fakePerpMarketStream) UnsubscribeAggTrade(symbol string) error {
	f.unsubTradeSymbol = symbol
	return nil
}
func (f *fakePerpMarketStream) UnsubscribeKline(symbol string, _ string) error {
	f.unsubKlineSymbol = symbol
	return nil
}

type fakePerpAccountStream struct {
	connected       bool
	connects        int
	orderUpdateCB   func(*perp.OrderUpdateEvent)
	accountUpdateCB func(*perp.AccountUpdateEvent)
	resubscribeCB   func()
}

func (f *fakePerpAccountStream) Connect() error {
	f.connected = true
	f.connects++
	return nil
}
func (f *fakePerpAccountStream) Close() {
	f.connected = false
}
func (f *fakePerpAccountStream) SetOnResubscribe(cb func()) {
	f.resubscribeCB = cb
}
func (f *fakePerpAccountStream) SubscribeOrderUpdate(cb func(*perp.OrderUpdateEvent)) {
	f.orderUpdateCB = cb
}
func (f *fakePerpAccountStream) SubscribeAccountUpdate(cb func(*perp.AccountUpdateEvent)) {
	f.accountUpdateCB = cb
}
func (f *fakePerpAccountStream) SubscribeAccountConfigUpdate(func(*perp.AccountConfigUpdateEvent)) {}
