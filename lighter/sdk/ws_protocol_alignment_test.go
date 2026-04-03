package lighter

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestWSClientPrivateChannelHelpersAlignWithOfficialChannels(t *testing.T) {
	authToken := "auth-token"
	callback := func([]byte) {}

	cases := []struct {
		name            string
		subscribe       func(*WebsocketClient) error
		unsubscribe     func(*WebsocketClient) error
		expectedChannel string
		expectedAuth    string
	}{
		{
			name:            "account all assets",
			subscribe:       func(c *WebsocketClient) error { return c.SubscribeAccountAllAssets(1234, authToken, callback) },
			unsubscribe:     func(c *WebsocketClient) error { return c.UnsubscribeAccountAllAssets(1234) },
			expectedChannel: "account_all_assets/1234",
			expectedAuth:    authToken,
		},
		{
			name:            "spot average entry prices",
			subscribe:       func(c *WebsocketClient) error { return c.SubscribeAccountSpotAvgEntryPrices(1234, authToken, callback) },
			unsubscribe:     func(c *WebsocketClient) error { return c.UnsubscribeAccountSpotAvgEntryPrices(1234) },
			expectedChannel: "account_spot_avg_entry_prices/1234",
			expectedAuth:    authToken,
		},
		{
			name:            "pool data",
			subscribe:       func(c *WebsocketClient) error { return c.SubscribePoolData(1234, authToken, callback) },
			unsubscribe:     func(c *WebsocketClient) error { return c.UnsubscribePoolData(1234) },
			expectedChannel: "pool_data/1234",
			expectedAuth:    authToken,
		},
		{
			name:            "pool info",
			subscribe:       func(c *WebsocketClient) error { return c.SubscribePoolInfo(1234, authToken, callback) },
			unsubscribe:     func(c *WebsocketClient) error { return c.UnsubscribePoolInfo(1234) },
			expectedChannel: "pool_info/1234",
			expectedAuth:    authToken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewWebsocketClient(context.Background())
			rec := &jsonCaptureConn{}
			client.conn = rec

			require.NoError(t, tc.subscribe(client))

			sub := client.Subscriptions[tc.expectedChannel]
			require.NotNil(t, sub)
			require.NotNil(t, sub.authToken)
			require.Equal(t, tc.expectedAuth, *sub.authToken)
			require.Equal(t, map[string]string{
				"type":    "subscribe",
				"channel": tc.expectedChannel,
				"auth":    tc.expectedAuth,
			}, rec.lastPayload)

			require.NoError(t, tc.unsubscribe(client))
			require.Equal(t, map[string]string{
				"type":    "unsubscribe",
				"channel": tc.expectedChannel,
			}, rec.lastPayload)
		})
	}
}

func TestWSModelTypesDecodeExtendedWebsocketFields(t *testing.T) {
	var order Order
	require.NoError(t, json.Unmarshal([]byte(`{
		"status":"canceled-invalid-balance",
		"integrator_fee_collector_index":"99",
		"integrator_taker_fee":"12",
		"integrator_maker_fee":"3",
		"transaction_time":1773854156686065
	}`), &order))
	require.Equal(t, OrderStatusCanceledInvalidBalance, order.Status)
	require.Equal(t, "99", order.IntegratorFeeCollectorIndex)
	require.Equal(t, "12", order.IntegratorTakerFee)
	require.Equal(t, "3", order.IntegratorMakerFee)
	require.Equal(t, int64(1773854156686065), order.TransactionTime)

	var trade Trade
	require.NoError(t, json.Unmarshal([]byte(`{
		"trade_id":16164557907,
		"trade_id_str":"16164557907",
		"ask_id_str":"281476612587355",
		"bid_id_str":"562948334068259",
		"ask_client_id":363283,
		"ask_client_id_str":"363283",
		"bid_client_id":23004521241,
		"bid_client_id_str":"23004521241",
		"transaction_time":1773854156686065,
		"ask_account_pnl":"1.25",
		"bid_account_pnl":"-1.25"
	}`), &trade))
	require.Equal(t, "16164557907", trade.TradeIdStr)
	require.Equal(t, "281476612587355", trade.AskIdStr)
	require.Equal(t, "562948334068259", trade.BidIdStr)
	require.Equal(t, int64(363283), trade.AskClientId)
	require.Equal(t, "363283", trade.AskClientIdStr)
	require.Equal(t, int64(23004521241), trade.BidClientId)
	require.Equal(t, "23004521241", trade.BidClientIdStr)
	require.Equal(t, int64(1773854156686065), trade.TransactionTime)
	require.Equal(t, "1.25", trade.AskAccountPnl)
	require.Equal(t, "-1.25", trade.BidAccountPnl)

	var position Position
	require.NoError(t, json.Unmarshal([]byte(`{"total_discount":"4.2"}`), &position))
	require.Equal(t, "4.2", position.TotalDiscount)

	var funding PositionFunding
	require.NoError(t, json.Unmarshal([]byte(`{"discount":"0.1"}`), &funding))
	require.Equal(t, "0.1", funding.Discount)

	var tx Tx
	require.NoError(t, json.Unmarshal([]byte(`{
		"type":15,
		"executed_at":1700000000005,
		"api_key_index":2,
		"transaction_time":1700000000005000
	}`), &tx))
	require.Equal(t, uint8(15), tx.Type)
	require.Equal(t, int64(1700000000005), tx.ExecutedAt)
	require.Equal(t, 2, tx.APIKeyIndex)
	require.Equal(t, int64(1700000000005000), tx.TransactionTime)
}

func TestWSEventsDecodeNewOfficialPayloadShapes(t *testing.T) {
	var tradeEvent WsTradeEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"channel":"trade:0",
		"type":"update/trade",
		"nonce":7,
		"liquidation_trades":[{"trade_id":2,"trade_id_str":"2"}],
		"trades":[{"trade_id":1,"trade_id_str":"1"}]
	}`), &tradeEvent))
	require.Equal(t, int64(7), tradeEvent.Nonce)
	require.Len(t, tradeEvent.Trades, 1)
	require.Len(t, tradeEvent.LiquidationTrades, 1)
	require.Equal(t, "1", tradeEvent.Trades[0].TradeIdStr)

	var assetsEvent WsAccountAllAssetsEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"channel":"account_all_assets:1234",
		"type":"update/account_all_assets",
		"timestamp":1773158679717,
		"assets":{"1":{"symbol":"ETH","asset_id":1,"balance":"7.1","locked_balance":"0.0"}}
	}`), &assetsEvent))
	require.Equal(t, int64(1773158679717), assetsEvent.Timestamp)
	require.Equal(t, "ETH", assetsEvent.Assets["1"].Symbol)

	var avgEntryEvent WsAccountSpotAvgEntryPricesEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"channel":"account_spot_avg_entry_prices:1234",
		"type":"subscribed/account_spot_avg_entry_prices",
		"timestamp":1773158679717,
		"avg_entry_prices":{"1":{"asset_id":1,"avg_entry_price":"1850.45","asset_size":"0.0123","last_trade_id":13472591098}}
	}`), &avgEntryEvent))
	require.Equal(t, "1850.45", avgEntryEvent.AvgEntryPrices["1"].AvgEntryPrice)
	require.Equal(t, int64(13472591098), avgEntryEvent.AvgEntryPrices["1"].LastTradeID)

	var poolDataEvent WsPoolDataEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"channel":"pool_data:1234",
		"type":"subscribed/pool_data",
		"account":1234,
		"shares":[{"public_pool_index":1,"shares_amount":100,"entry_usdc":"1000","principal_amount":"900","entry_timestamp":1773158679717}]
	}`), &poolDataEvent))
	require.Equal(t, int64(1234), poolDataEvent.Account)
	require.Len(t, poolDataEvent.Shares, 1)
	require.Equal(t, "900", poolDataEvent.Shares[0].PrincipalAmount)

	var poolInfoEvent WsPoolInfoEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"channel":"pool_info:1234",
		"type":"subscribed/pool_info",
		"pool_info":{
			"status":1,
			"operator_fee":"0.1",
			"min_operator_share_rate":"0.2",
			"total_shares":10,
			"operator_shares":3,
			"annual_percentage_yield":12.5,
			"daily_returns":{"timestamp":1773158679717,"daily_return":1.5},
			"share_prices":{"timestamp":1773158679717,"share_price":101.2}
		}
	}`), &poolInfoEvent))
	require.Equal(t, "0.1", poolInfoEvent.PoolInfo.OperatorFee)
	require.NotNil(t, poolInfoEvent.PoolInfo.DailyReturns)
	require.Equal(t, float64(1.5), poolInfoEvent.PoolInfo.DailyReturns.DailyReturn)
	require.NotNil(t, poolInfoEvent.PoolInfo.SharePrice)
	require.Equal(t, float64(101.2), poolInfoEvent.PoolInfo.SharePrice.SharePrice)
}

type jsonCaptureConn struct {
	lastPayload map[string]string
}

func (c *jsonCaptureConn) ReadMessage() (int, []byte, error) {
	return 0, nil, errors.New("not implemented")
}

func (c *jsonCaptureConn) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	var payload map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	c.lastPayload = payload
	return nil
}

func (c *jsonCaptureConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	if messageType != websocket.PingMessage {
		return nil
	}
	return nil
}

func (c *jsonCaptureConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *jsonCaptureConn) Close() error {
	return nil
}
