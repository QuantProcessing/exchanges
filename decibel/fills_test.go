package decibel

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	decibelws "github.com/QuantProcessing/exchanges/decibel/sdk/ws"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapUserTradeReturnsExecutionDetails(t *testing.T) {
	adp := newTestDecibelAdapter(t, &stubDecibelRESTClient{}, &stubDecibelWSClient{}, &stubDecibelAptosClient{accountAddress: "0xaccount"})
	adp.quoteCurrency = exchanges.QuoteCurrencyUSDC

	fill := adp.mapUserTrade("", decibelws.UserTradeItem{
		TradeID:       "3647276",
		OrderID:       "45678",
		ClientOrderID: "order_123",
		Market:        "0xbtc",
		Action:        "Open Long",
		Price:         decimal.RequireFromString("50125.75"),
		Size:          decimal.RequireFromString("1.5"),
		FeeAmount:     decimal.RequireFromString("25.06"),
		UnixMS:        1699564800000,
	})

	require.NotNil(t, fill)
	require.Equal(t, "3647276", fill.TradeID)
	require.Equal(t, "45678", fill.OrderID)
	require.Equal(t, "order_123", fill.ClientOrderID)
	require.Equal(t, "BTC", fill.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, fill.Side)
	require.True(t, decimal.RequireFromString("50125.75").Equal(fill.Price))
	require.True(t, decimal.RequireFromString("1.5").Equal(fill.Quantity))
	require.True(t, decimal.RequireFromString("25.06").Equal(fill.Fee))
	require.Equal(t, "USDC", fill.FeeAsset)
	require.False(t, fill.IsMaker)
	require.Equal(t, int64(1699564800000), fill.Timestamp)
}

func TestMapUserTradeTreatsRebateAsMakerFill(t *testing.T) {
	adp := newTestDecibelAdapter(t, &stubDecibelRESTClient{}, &stubDecibelWSClient{}, &stubDecibelAptosClient{accountAddress: "0xaccount"})

	fill := adp.mapUserTrade("", decibelws.UserTradeItem{
		Market:    "0xbtc",
		Action:    "Close Long",
		Price:     decimal.RequireFromString("50120"),
		Size:      decimal.RequireFromString("0.2"),
		FeeAmount: decimal.RequireFromString("1.25"),
		IsRebate:  true,
	})

	require.NotNil(t, fill)
	require.Equal(t, exchanges.OrderSideSell, fill.Side)
	require.True(t, decimal.RequireFromString("-1.25").Equal(fill.Fee))
	require.True(t, fill.IsMaker)
}

func TestWatchFillsSubscribesAndMapsUserTrades(t *testing.T) {
	wsClient := &stubDecibelWSClient{}
	adp := newTestDecibelAdapter(t, &stubDecibelRESTClient{}, wsClient, &stubDecibelAptosClient{accountAddress: "0xaccount"})

	updates := make(chan *exchanges.Fill, 1)
	require.NoError(t, adp.WatchFills(context.Background(), func(fill *exchanges.Fill) {
		updates <- fill
	}))
	require.Equal(t, "user_trades:0xaccount", wsClient.tradeTopic)

	wsClient.emitUserTrades(decibelws.UserTradesMessage{
		Topic: "user_trades:0xaccount",
		Trades: []decibelws.UserTradeItem{
			{
				TradeID:       "1",
				OrderID:       "2",
				ClientOrderID: "cli-1",
				Market:        "0xbtc",
				Action:        "Open Short",
				Price:         decimal.RequireFromString("50000"),
				Size:          decimal.RequireFromString("0.3"),
				FeeAmount:     decimal.RequireFromString("0.4"),
				UnixMS:        1711286400005,
			},
		},
	})

	select {
	case fill := <-updates:
		require.Equal(t, "1", fill.TradeID)
		require.Equal(t, "2", fill.OrderID)
		require.Equal(t, "cli-1", fill.ClientOrderID)
		require.Equal(t, "BTC", fill.Symbol)
		require.Equal(t, exchanges.OrderSideSell, fill.Side)
		require.True(t, decimal.RequireFromString("50000").Equal(fill.Price))
		require.True(t, decimal.RequireFromString("0.3").Equal(fill.Quantity))
		require.True(t, decimal.RequireFromString("0.4").Equal(fill.Fee))
	case <-time.After(time.Second):
		t.Fatal("expected WatchFills callback")
	}
}
