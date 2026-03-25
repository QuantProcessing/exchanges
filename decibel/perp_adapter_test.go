package decibel

import (
	"context"
	"sync"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	decibelaptos "github.com/QuantProcessing/exchanges/decibel/sdk/aptos"
	decibelrest "github.com/QuantProcessing/exchanges/decibel/sdk/rest"
	decibelws "github.com/QuantProcessing/exchanges/decibel/sdk/ws"
	aptossdkapi "github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type stubDecibelRESTClient struct {
	getTickerFn           func(context.Context, string) (*decibelrest.Ticker, error)
	getOrderBookFn        func(context.Context, string, int) (*decibelrest.OrderBookSnapshot, error)
	getAccountOverviewFn  func(context.Context, string) (*decibelrest.AccountOverview, error)
	getAccountPositionsFn func(context.Context, string) ([]decibelrest.AccountPosition, error)
	getOpenOrdersFn       func(context.Context, string, int, int) (*decibelrest.OpenOrdersResponse, error)
	getOrderHistoryFn     func(context.Context, string, int, int) (*decibelrest.OpenOrdersResponse, error)
	getOrderFn            func(context.Context, string, string, string, string) (*decibelrest.OrderResponse, error)
	getOrderByIDFn        func(context.Context, string, string) (*decibelrest.OpenOrder, error)
}

func (c *stubDecibelRESTClient) GetMarkets(context.Context) ([]decibelrest.Market, error) {
	panic("unexpected GetMarkets call")
}

func (c *stubDecibelRESTClient) GetTicker(ctx context.Context, market string) (*decibelrest.Ticker, error) {
	if c.getTickerFn == nil {
		panic("unexpected GetTicker call")
	}
	return c.getTickerFn(ctx, market)
}

func (c *stubDecibelRESTClient) GetOrderBook(ctx context.Context, market string, limit int) (*decibelrest.OrderBookSnapshot, error) {
	if c.getOrderBookFn == nil {
		panic("unexpected GetOrderBook call")
	}
	return c.getOrderBookFn(ctx, market, limit)
}

func (c *stubDecibelRESTClient) GetAccountOverview(ctx context.Context, account string) (*decibelrest.AccountOverview, error) {
	if c.getAccountOverviewFn == nil {
		panic("unexpected GetAccountOverview call")
	}
	return c.getAccountOverviewFn(ctx, account)
}

func (c *stubDecibelRESTClient) GetAccountPositions(ctx context.Context, account string) ([]decibelrest.AccountPosition, error) {
	if c.getAccountPositionsFn == nil {
		panic("unexpected GetAccountPositions call")
	}
	return c.getAccountPositionsFn(ctx, account)
}

func (c *stubDecibelRESTClient) GetOpenOrders(ctx context.Context, account string, limit int, offset int) (*decibelrest.OpenOrdersResponse, error) {
	if c.getOpenOrdersFn == nil {
		return &decibelrest.OpenOrdersResponse{}, nil
	}
	return c.getOpenOrdersFn(ctx, account, limit, offset)
}

func (c *stubDecibelRESTClient) GetOrderHistory(ctx context.Context, account string, limit int, offset int) (*decibelrest.OpenOrdersResponse, error) {
	if c.getOrderHistoryFn == nil {
		return &decibelrest.OpenOrdersResponse{}, nil
	}
	return c.getOrderHistoryFn(ctx, account, limit, offset)
}

func (c *stubDecibelRESTClient) GetOrder(ctx context.Context, account, market, orderID, clientOrderID string) (*decibelrest.OrderResponse, error) {
	if c.getOrderFn == nil {
		return nil, nil
	}
	return c.getOrderFn(ctx, account, market, orderID, clientOrderID)
}

func (c *stubDecibelRESTClient) GetOrderByID(ctx context.Context, account, orderID string) (*decibelrest.OpenOrder, error) {
	if c.getOrderByIDFn == nil {
		panic("unexpected GetOrderByID call")
	}
	return c.getOrderByIDFn(ctx, account, orderID)
}

type stubDecibelWSClient struct {
	mu sync.Mutex

	connectCalls        int
	closeCalls          int
	depthHandlers       map[string]func(decibelws.MarketDepthMessage)
	orderHistoryHandler func(decibelws.UserOrderHistoryMessage)
	orderUpdateHandler  func(decibelws.OrderUpdateMessage)
	positionHandler     func(decibelws.UserPositionsMessage)
	orderHistoryTopic   string
	orderUpdateTopic    string
	positionTopic       string
}

func (c *stubDecibelWSClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectCalls++
	return nil
}

func (c *stubDecibelWSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeCalls++
	return nil
}

func (c *stubDecibelWSClient) Subscribe(topic string, handler func(decibelws.MarketDepthMessage)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.depthHandlers == nil {
		c.depthHandlers = make(map[string]func(decibelws.MarketDepthMessage))
	}
	c.depthHandlers[topic] = handler
	return nil
}

func (c *stubDecibelWSClient) SubscribeUserOrderHistory(userAddr string, handler func(decibelws.UserOrderHistoryMessage)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.orderHistoryTopic = "user_order_history:" + userAddr
	c.orderHistoryHandler = handler
	return nil
}

func (c *stubDecibelWSClient) SubscribeOrderUpdates(userAddr string, handler func(decibelws.OrderUpdateMessage)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.orderUpdateTopic = "order_updates:" + userAddr
	c.orderUpdateHandler = handler
	return nil
}

func (c *stubDecibelWSClient) SubscribeUserPositions(userAddr string, handler func(decibelws.UserPositionsMessage)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.positionTopic = "user_positions:" + userAddr
	c.positionHandler = handler
	return nil
}

func (c *stubDecibelWSClient) emitDepth(topic string, msg decibelws.MarketDepthMessage) {
	c.mu.Lock()
	handler := c.depthHandlers[topic]
	c.mu.Unlock()
	if handler != nil {
		handler(msg)
	}
}

func (c *stubDecibelWSClient) emitOrderHistory(msg decibelws.UserOrderHistoryMessage) {
	c.mu.Lock()
	handler := c.orderHistoryHandler
	c.mu.Unlock()
	if handler != nil {
		handler(msg)
	}
}

func (c *stubDecibelWSClient) emitOrderUpdate(msg decibelws.OrderUpdateMessage) {
	c.mu.Lock()
	handler := c.orderUpdateHandler
	c.mu.Unlock()
	if handler != nil {
		handler(msg)
	}
}

type stubDecibelAptosClient struct {
	accountAddress string
	placeOrderFn   func(decibelaptos.PlaceOrderRequest) (*aptossdkapi.SubmitTransactionResponse, error)
	cancelOrderFn  func(decibelaptos.CancelOrderRequest) (*aptossdkapi.SubmitTransactionResponse, error)
}

func (c *stubDecibelAptosClient) AccountAddress() string {
	return c.accountAddress
}

func (c *stubDecibelAptosClient) PlaceOrder(req decibelaptos.PlaceOrderRequest) (*aptossdkapi.SubmitTransactionResponse, error) {
	if c.placeOrderFn == nil {
		panic("unexpected PlaceOrder call")
	}
	return c.placeOrderFn(req)
}

func (c *stubDecibelAptosClient) CancelOrder(req decibelaptos.CancelOrderRequest) (*aptossdkapi.SubmitTransactionResponse, error) {
	if c.cancelOrderFn == nil {
		panic("unexpected CancelOrder call")
	}
	return c.cancelOrderFn(req)
}

func TestDecibelAdapterFetchesUseExpectedSDKSeams(t *testing.T) {
	var (
		seenTickerMarket     string
		seenOrderBookMarket  string
		seenOrderBookLimit   int
		seenOverviewAccount  string
		seenPositionsAccount string
		seenOpenOrdersAcct   string
		seenOrderLookupAcct  string
		seenOrderLookupID    string
	)

	restClient := &stubDecibelRESTClient{
		getTickerFn: func(_ context.Context, market string) (*decibelrest.Ticker, error) {
			seenTickerMarket = market
			return &decibelrest.Ticker{
				Market:    market,
				LastPrice: decimal.RequireFromString("50000"),
				MarkPrice: decimal.RequireFromString("50010"),
				BidPrice:  decimal.RequireFromString("49990"),
				AskPrice:  decimal.RequireFromString("50020"),
				Timestamp: 1711286400000,
			}, nil
		},
		getOrderBookFn: func(_ context.Context, market string, limit int) (*decibelrest.OrderBookSnapshot, error) {
			seenOrderBookMarket = market
			seenOrderBookLimit = limit
			return &decibelrest.OrderBookSnapshot{
				Market: market,
				Bids: []decibelrest.OrderBookLevel{
					{Price: decimal.RequireFromString("49990"), Size: decimal.RequireFromString("1.25")},
				},
				Asks: []decibelrest.OrderBookLevel{
					{Price: decimal.RequireFromString("50020"), Size: decimal.RequireFromString("0.75")},
				},
				Timestamp: 1711286400001,
			}, nil
		},
		getAccountOverviewFn: func(_ context.Context, account string) (*decibelrest.AccountOverview, error) {
			seenOverviewAccount = account
			return &decibelrest.AccountOverview{
				Account:          account,
				TotalBalance:     decimal.RequireFromString("1000"),
				AvailableBalance: decimal.RequireFromString("800"),
				UnrealizedPnL:    decimal.RequireFromString("12.5"),
			}, nil
		},
		getAccountPositionsFn: func(_ context.Context, account string) ([]decibelrest.AccountPosition, error) {
			seenPositionsAccount = account
			return []decibelrest.AccountPosition{
				{
					Market:       "0xbtc",
					Size:         decimal.RequireFromString("2"),
					EntryPrice:   decimal.RequireFromString("49000"),
					UserLeverage: decimal.RequireFromString("5"),
				},
			}, nil
		},
		getOpenOrdersFn: func(_ context.Context, account string, _ int, _ int) (*decibelrest.OpenOrdersResponse, error) {
			seenOpenOrdersAcct = account
			return &decibelrest.OpenOrdersResponse{
				Items: []decibelrest.OpenOrder{
					{
						OrderID:       "77",
						ClientOrderID: "cli-77",
						Market:        "0xbtc",
						OrderType:     "limit",
						Status:        "open",
						UnixMS:        1711286400002,
						OrigSize:      decimal.RequireFromString("0.5"),
						RemainingSize: decimal.RequireFromString("0.5"),
						Price:         decimal.RequireFromString("48000"),
					},
				},
				TotalCount: 1,
			}, nil
		},
		getOrderHistoryFn: func(_ context.Context, account string, _ int, _ int) (*decibelrest.OpenOrdersResponse, error) {
			require.Equal(t, "0xsubaccount", account)
			return &decibelrest.OpenOrdersResponse{
				Items: []decibelrest.OpenOrder{
					{
						OrderID:       "78",
						ClientOrderID: "cli-78",
						Market:        "0xbtc",
						OrderType:     "market",
						Status:        "filled",
						UnixMS:        1711286400010,
						OrigSize:      decimal.RequireFromString("0.25"),
						RemainingSize: decimal.Zero,
						Price:         decimal.RequireFromString("50010"),
						IsBuy:         true,
					},
				},
				TotalCount: 1,
			}, nil
		},
		getOrderByIDFn: func(_ context.Context, account, orderID string) (*decibelrest.OpenOrder, error) {
			seenOrderLookupAcct = account
			seenOrderLookupID = orderID
			return &decibelrest.OpenOrder{
				OrderID:       orderID,
				ClientOrderID: "cli-77",
				Market:        "0xbtc",
				OrderType:     "limit",
				Status:        "open",
				UnixMS:        1711286400003,
				OrigSize:      decimal.RequireFromString("0.5"),
				RemainingSize: decimal.RequireFromString("0.5"),
				Price:         decimal.RequireFromString("48000"),
			}, nil
		},
	}

	adp := newTestDecibelAdapter(t, restClient, &stubDecibelWSClient{}, &stubDecibelAptosClient{accountAddress: "0xaccount"})
	ctx := context.Background()

	ticker, err := adp.FetchTicker(ctx, "BTC")
	require.NoError(t, err)
	require.Equal(t, "0xbtc", seenTickerMarket)
	require.Equal(t, "BTC", ticker.Symbol)
	require.True(t, decimal.RequireFromString("50020").Equal(ticker.Ask))

	book, err := adp.FetchOrderBook(ctx, "BTC", 25)
	require.NoError(t, err)
	require.Equal(t, "0xbtc", seenOrderBookMarket)
	require.Equal(t, 25, seenOrderBookLimit)
	require.Equal(t, "BTC", book.Symbol)
	require.Len(t, book.Bids, 1)
	require.Len(t, book.Asks, 1)

	account, err := adp.FetchAccount(ctx)
	require.NoError(t, err)
	require.Equal(t, "0xsubaccount", seenOverviewAccount)
	require.Equal(t, "0xsubaccount", seenPositionsAccount)
	require.Equal(t, "0xsubaccount", seenOpenOrdersAcct)
	require.True(t, decimal.RequireFromString("1000").Equal(account.TotalBalance))
	require.Len(t, account.Positions, 1)
	require.Len(t, account.Orders, 1)
	require.Equal(t, "BTC", account.Positions[0].Symbol)
	require.Equal(t, exchanges.PositionSideLong, account.Positions[0].Side)

	balance, err := adp.FetchBalance(ctx)
	require.NoError(t, err)
	require.True(t, decimal.RequireFromString("800").Equal(balance))

	positions, err := adp.FetchPositions(ctx)
	require.NoError(t, err)
	require.Len(t, positions, 1)
	require.Equal(t, account.Positions, positions)

	openOrders, err := adp.FetchOpenOrders(ctx, "BTC")
	require.NoError(t, err)
	require.Len(t, openOrders, 1)
	require.Equal(t, "BTC", openOrders[0].Symbol)

	orders, err := adp.FetchOrders(ctx, "BTC")
	require.NoError(t, err)
	require.Len(t, orders, 1)
	require.Equal(t, "78", orders[0].OrderID)
	require.Equal(t, exchanges.OrderStatusFilled, orders[0].Status)

	order, err := adp.FetchOrderByID(ctx, "77", "BTC")
	require.NoError(t, err)
	require.Equal(t, "0xsubaccount", seenOrderLookupAcct)
	require.Equal(t, "77", seenOrderLookupID)
	require.Equal(t, "77", order.OrderID)
	require.Equal(t, "BTC", order.Symbol)

	details, err := adp.FetchSymbolDetails(ctx, "BTC")
	require.NoError(t, err)
	require.Equal(t, int32(2), details.PricePrecision)
	require.Equal(t, int32(3), details.QuantityPrecision)

	feeRate, err := adp.FetchFeeRate(ctx, "BTC")
	require.NoError(t, err)
	require.True(t, feeRate.Maker.IsZero())
	require.True(t, feeRate.Taker.IsZero())
}

func TestDecibelAdapterWatchOrderBookWaitsForInitialSync(t *testing.T) {
	wsClient := &stubDecibelWSClient{}
	adp := newTestDecibelAdapter(t, &stubDecibelRESTClient{}, wsClient, &stubDecibelAptosClient{accountAddress: "0xaccount"})

	updates := make(chan *exchanges.OrderBook, 2)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- adp.WatchOrderBook(ctx, "BTC", func(ob *exchanges.OrderBook) {
			updates <- ob
		})
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("WatchOrderBook returned before snapshot: %v", err)
	default:
	}

	wsClient.emitDepth("depth:0xbtc:1", decibelws.MarketDepthMessage{
		Topic:      "depth:0xbtc:1",
		Market:     "0xbtc",
		UpdateType: decibelws.DepthUpdateDelta,
		Bids: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("49990"), Size: decimal.RequireFromString("1")},
		},
	})

	time.Sleep(25 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("WatchOrderBook returned on delta-only bootstrap: %v", err)
	default:
	}

	wsClient.emitDepth("depth:0xbtc:1", decibelws.MarketDepthMessage{
		Topic:      "depth:0xbtc:1",
		Market:     "0xbtc",
		UpdateType: decibelws.DepthUpdateSnapshot,
		Bids: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("49990"), Size: decimal.RequireFromString("2")},
		},
		Asks: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50010"), Size: decimal.RequireFromString("3")},
		},
		Timestamp: 1711286400004,
	})

	require.NoError(t, <-done)
	local := adp.GetLocalOrderBook("BTC", 5)
	require.NotNil(t, local)
	require.Len(t, local.Bids, 1)
	require.Len(t, local.Asks, 1)

	deadline := time.After(time.Second)
	for {
		select {
		case update := <-updates:
			if len(update.Bids) == 0 || len(update.Asks) == 0 {
				continue
			}
			require.Equal(t, "BTC", update.Symbol)
			require.Len(t, update.Bids, 1)
			require.Len(t, update.Asks, 1)
			return
		case <-deadline:
			t.Fatal("expected orderbook callback after snapshot")
		}
	}
}

func TestDecibelAdapterWatchOrdersReconcilesPlacedOrderID(t *testing.T) {
	wsClient := &stubDecibelWSClient{}
	aptosClient := &stubDecibelAptosClient{accountAddress: "0xaccount"}
	adp := newTestDecibelAdapter(t, &stubDecibelRESTClient{}, wsClient, aptosClient)

	updates := make(chan *exchanges.Order, 1)
	require.NoError(t, adp.WatchOrders(context.Background(), func(order *exchanges.Order) {
		updates <- order
	}))
	require.Equal(t, "order_updates:0xaccount", wsClient.orderUpdateTopic)
	require.Equal(t, "user_order_history:0xaccount", wsClient.orderHistoryTopic)

	aptosClient.placeOrderFn = func(req decibelaptos.PlaceOrderRequest) (*aptossdkapi.SubmitTransactionResponse, error) {
		require.Equal(t, "0xsubaccount", req.SubaccountAddr)
		require.Equal(t, "0xbtc", req.MarketAddr)
		require.NotNil(t, req.ClientOrderID)
		require.Equal(t, "cli-1", *req.ClientOrderID)
		require.Equal(t, decibelaptos.TimeInForceGoodTillCancelled, req.TimeInForce)
		require.NotNil(t, req.Encoder)

		go func() {
			time.Sleep(20 * time.Millisecond)
			wsClient.emitOrderUpdate(decibelws.OrderUpdateMessage{
				Topic: "order_updates:0xaccount",
				Order: decibelws.OrderUpdateRecord{
					Status:           "Open",
					NormalizedStatus: exchanges.OrderStatusNew,
					Order: decibelws.OrderUpdateItem{
						OrderID:       "42",
						ClientOrderID: "cli-1",
						Market:        "0xbtc",
						OrderType:     "limit",
						Side:          "buy",
						Price:         decimal.RequireFromString("48000"),
						OrigSize:      decimal.RequireFromString("0.25"),
						RemainingSize: decimal.RequireFromString("0.25"),
						UnixMS:        1711286400005,
					},
				},
			})
		}()

		return &aptossdkapi.SubmitTransactionResponse{Hash: "0xtxhash"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:      "BTC",
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.25"),
		Price:       decimal.RequireFromString("48000"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "cli-1",
	})
	require.NoError(t, err)
	require.Equal(t, "42", order.OrderID)
	require.Equal(t, "cli-1", order.ClientOrderID)
	require.Equal(t, exchanges.OrderStatusNew, order.Status)

	select {
	case update := <-updates:
		require.Equal(t, "42", update.OrderID)
		require.Equal(t, "cli-1", update.ClientOrderID)
		require.Equal(t, exchanges.OrderStatusNew, update.Status)
	case <-time.After(time.Second):
		t.Fatal("expected WatchOrders callback")
	}
}

func TestDecibelAdapterPlaceOrderQuantizesPriceAndSizeToMarketSteps(t *testing.T) {
	aptosClient := &stubDecibelAptosClient{accountAddress: "0xaccount"}
	adp := newTestDecibelAdapter(t, &stubDecibelRESTClient{
		getTickerFn: func(_ context.Context, market string) (*decibelrest.Ticker, error) {
			return &decibelrest.Ticker{
				Market:    market,
				LastPrice: decimal.RequireFromString("50000"),
				MarkPrice: decimal.RequireFromString("50000"),
				Timestamp: 1711286400000,
			}, nil
		},
		getOpenOrdersFn: func(_ context.Context, account string, limit int, offset int) (*decibelrest.OpenOrdersResponse, error) {
			return &decibelrest.OpenOrdersResponse{}, nil
		},
	}, &stubDecibelWSClient{}, aptosClient)

	aptosClient.placeOrderFn = func(req decibelaptos.PlaceOrderRequest) (*aptossdkapi.SubmitTransactionResponse, error) {
		require.True(t, decimal.RequireFromString("48000.10").Equal(req.Price))
		require.True(t, decimal.RequireFromString("0.250").Equal(req.Size))
		return &aptossdkapi.SubmitTransactionResponse{Hash: "0xtxhash"}, nil
	}

	order, err := adp.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:      "BTC",
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.2504"),
		Price:       decimal.RequireFromString("48000.129"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "cli-quantized",
	})
	require.NoError(t, err)
	require.Empty(t, order.OrderID)
	require.Equal(t, exchanges.OrderStatusPending, order.Status)
}

func newTestDecibelAdapter(
	t *testing.T,
	restClient decibelRESTClient,
	wsClient decibelWSClient,
	aptosClient decibelAptosClient,
) *Adapter {
	t.Helper()

	prevLookup := lookupDecibelTransactionOrderEvents
	prevTimeout := orderReconcileTimeout
	lookupDecibelTransactionOrderEvents = func(context.Context, string) ([]aptosTxEvent, error) {
		return nil, nil
	}
	orderReconcileTimeout = 100 * time.Millisecond
	t.Cleanup(func() {
		lookupDecibelTransactionOrderEvents = prevLookup
		orderReconcileTimeout = prevTimeout
	})

	cache, err := newMarketMetadataCache([]decibelrest.Market{
		{
			MarketAddr: "0xbtc",
			MarketName: "BTC-USDC-PERP",
			Mode:       "perp",
			MinSize:    decimal.RequireFromString("0.001"),
			LotSize:    decimal.RequireFromString("0.001"),
			TickSize:   decimal.RequireFromString("0.1"),
			PxDecimals: 2,
			SzDecimals: 3,
		},
	})
	require.NoError(t, err)

	adp := &Adapter{
		BaseAdapter:    exchanges.NewBaseAdapter("DECIBEL", exchanges.MarketTypePerp, exchanges.NopLogger),
		apiKey:         "api-key",
		privateKey:     "private-key",
		subaccountAddr: "0xsubaccount",
		accountAddr:    "0xaccount",
		rest:           restClient,
		ws:             wsClient,
		aptos:          aptosClient,
		markets:        cache,
	}
	adp.initRuntimeState()
	adp.SetSymbolDetails(symbolDetailsFromMetadataCache(cache))
	return adp
}
