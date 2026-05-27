package okx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestPlaceOrderWSUsesInstIdCodeAndSurfacesSubCode(t *testing.T) {
	type capturedRequest struct {
		Op   string `json:"op"`
		ID   string `json:"id"`
		Args []struct {
			InstIdCode int64  `json:"instIdCode"`
			InstId     string `json:"instId,omitempty"`
			Side       string `json:"side"`
			TdMode     string `json:"tdMode"`
			OrdType    string `json:"ordType"`
			Sz         string `json:"sz"`
		} `json:"args"`
	}

	var captured capturedRequest
	wsURL := newOrderActionWSServer(t, func(payload []byte) []byte {
		require.NoError(t, json.Unmarshal(payload, &captured))
		require.Len(t, captured.Args, 1)
		return []byte(`{"id":"` + captured.ID + `","code":"0","msg":"","data":[{"ordId":"","clOrdId":"cid-1","sCode":"51000","sMsg":"order rejected","subCode":"51149","ts":"1"}]}`)
	})

	wsClient := NewWSClient(context.Background()).WithCredentials("api-key", "secret-key", "passphrase")
	wsClient.URL = wsURL
	require.NoError(t, wsClient.Connect())
	t.Cleanup(func() {
		if wsClient.Conn != nil {
			_ = wsClient.Conn.Close()
		}
	})

	instIdCode := int64(123456)
	_, err := wsClient.PlaceOrderWS(&OrderRequest{
		InstId:     "BTC-USDT-SWAP",
		InstIdCode: &instIdCode,
		TdMode:     "isolated",
		Side:       "buy",
		OrdType:    "limit",
		Sz:         "1",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "sCode=51000")
	require.Contains(t, err.Error(), "subCode=51149")
	require.Equal(t, "order", captured.Op)
	require.Equal(t, int64(123456), captured.Args[0].InstIdCode)
	require.Empty(t, captured.Args[0].InstId, "WS order payload should rely on instIdCode instead of instId")
}

func TestCancelOrderWSUsesInstIdCode(t *testing.T) {
	type capturedRequest struct {
		Op   string `json:"op"`
		ID   string `json:"id"`
		Args []struct {
			InstIdCode int64   `json:"instIdCode"`
			InstId     string  `json:"instId,omitempty"`
			OrdId      *string `json:"ordId,omitempty"`
			ClOrdId    *string `json:"clOrdId,omitempty"`
		} `json:"args"`
	}

	var captured capturedRequest
	wsURL := newOrderActionWSServer(t, func(payload []byte) []byte {
		require.NoError(t, json.Unmarshal(payload, &captured))
		require.Len(t, captured.Args, 1)
		return []byte(`{"id":"` + captured.ID + `","code":"0","msg":"","data":[{"ordId":"cancelled-1","clOrdId":"","sCode":"0","sMsg":"","subCode":"","ts":"1"}]}`)
	})

	wsClient := NewWSClient(context.Background()).WithCredentials("api-key", "secret-key", "passphrase")
	wsClient.URL = wsURL
	require.NoError(t, wsClient.Connect())
	t.Cleanup(func() {
		if wsClient.Conn != nil {
			_ = wsClient.Conn.Close()
		}
	})

	instIdCode := int64(654321)
	ordID := "cancel-me"
	_, err := wsClient.CancelOrderWS(instIdCode, &ordID, nil)
	require.NoError(t, err)
	require.Equal(t, "cancel-order", captured.Op)
	require.Equal(t, int64(654321), captured.Args[0].InstIdCode)
	require.Empty(t, captured.Args[0].InstId, "WS cancel payload should rely on instIdCode instead of instId")
	require.NotNil(t, captured.Args[0].OrdId)
	require.Equal(t, "cancel-me", *captured.Args[0].OrdId)
}

func TestModifyOrderWSUsesInstIdCode(t *testing.T) {
	type capturedRequest struct {
		Op   string `json:"op"`
		ID   string `json:"id"`
		Args []struct {
			InstIdCode int64   `json:"instIdCode"`
			InstId     string  `json:"instId,omitempty"`
			OrdId      *string `json:"ordId,omitempty"`
			NewSz      *string `json:"newSz,omitempty"`
		} `json:"args"`
	}

	var captured capturedRequest
	wsURL := newOrderActionWSServer(t, func(payload []byte) []byte {
		require.NoError(t, json.Unmarshal(payload, &captured))
		require.Len(t, captured.Args, 1)
		return []byte(`{"id":"` + captured.ID + `","code":"0","msg":"","data":[{"ordId":"amended-1","clOrdId":"","sCode":"0","sMsg":"","subCode":"","ts":"1"}]}`)
	})

	wsClient := NewWSClient(context.Background()).WithCredentials("api-key", "secret-key", "passphrase")
	wsClient.URL = wsURL
	require.NoError(t, wsClient.Connect())
	t.Cleanup(func() {
		if wsClient.Conn != nil {
			_ = wsClient.Conn.Close()
		}
	})

	instIdCode := int64(999001)
	ordID := "order-1"
	newSz := "2"
	_, err := wsClient.ModifyOrderWS(&ModifyOrderRequest{
		InstId:     "BTC-USDT-SWAP",
		InstIdCode: &instIdCode,
		OrdId:      &ordID,
		NewSz:      &newSz,
	})
	require.NoError(t, err)
	require.Equal(t, "amend-order", captured.Op)
	require.Equal(t, int64(999001), captured.Args[0].InstIdCode)
	require.Empty(t, captured.Args[0].InstId, "WS amend payload should rely on instIdCode instead of instId")
	require.NotNil(t, captured.Args[0].OrdId)
	require.Equal(t, "order-1", *captured.Args[0].OrdId)
	require.NotNil(t, captured.Args[0].NewSz)
	require.Equal(t, "2", *captured.Args[0].NewSz)
}

func newOrderActionWSServer(t *testing.T, onAction func(payload []byte) []byte) string {
	t.Helper()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}

			if strings.Contains(string(payload), `"op":"login"`) {
				require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"login","code":"0","msg":""}`)))
				continue
			}

			require.NoError(t, conn.WriteMessage(websocket.TextMessage, onAction(payload)))
			return
		}
	}))
	t.Cleanup(server.Close)

	return "ws" + strings.TrimPrefix(server.URL, "http")
}
