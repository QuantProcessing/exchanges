package sdk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetWalletBalanceSignsRequestAndParsesResponse(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/account/wallet-balance", r.URL.Path)
			require.Equal(t, "UNIFIED", r.URL.Query().Get("accountType"))
			require.Equal(t, "api-key", r.Header.Get("X-BAPI-API-KEY"))
			require.NotEmpty(t, r.Header.Get("X-BAPI-TIMESTAMP"))
			require.NotEmpty(t, r.Header.Get("X-BAPI-RECV-WINDOW"))
			require.NotEmpty(t, r.Header.Get("X-BAPI-SIGN"))
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"accountType":"UNIFIED","totalEquity":"11","totalAvailableBalance":"9","totalPerpUPL":"2","totalWalletBalance":"9","coin":[{"coin":"USDT","equity":"11","walletBalance":"9","locked":"1","unrealisedPnl":"2","cumRealisedPnl":"3","usdValue":"11"}]}]}}`), nil
		}),
	}

	got, err := client.GetWalletBalance(context.Background(), "UNIFIED", "")
	require.NoError(t, err)
	require.Len(t, got.List, 1)
	require.Equal(t, "UNIFIED", got.List[0].AccountType)
	require.Len(t, got.List[0].Coin, 1)
	require.Equal(t, "USDT", got.List[0].Coin[0].Coin)
}

func TestGetFeeRatesParsesList(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/account/fee-rate", r.URL.Path)
			require.Equal(t, "linear", r.URL.Query().Get("category"))
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"symbol":"BTCUSDT","makerFeeRate":"0.0001","takerFeeRate":"0.0006"}]}}`), nil
		}),
	}

	got, err := client.GetFeeRates(context.Background(), "linear", "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "0.0001", got[0].MakerFeeRate)
}

func TestGetPositionsParsesList(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/position/list", r.URL.Path)
			require.Equal(t, "linear", r.URL.Query().Get("category"))
			require.Equal(t, "USDT", r.URL.Query().Get("settleCoin"))
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"symbol":"BTCUSDT","side":"Buy","size":"0.5","avgPrice":"50000","leverage":"10","unrealisedPnl":"100","cumRealisedPnl":"5","liqPrice":"45000"}]}}`), nil
		}),
	}

	got, err := client.GetPositions(context.Background(), "linear", "", "USDT")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "BTCUSDT", got[0].Symbol)
}

func TestSetLeveragePostsBody(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v5/position/set-leverage", r.URL.Path)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var payload map[string]string
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Equal(t, "linear", payload["category"])
			require.Equal(t, "BTCUSDT", payload["symbol"])
			require.Equal(t, "6", payload["buyLeverage"])
			require.Equal(t, "6", payload["sellLeverage"])
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{}}`), nil
		}),
	}

	err := client.SetLeverage(context.Background(), SetLeverageRequest{
		Category:     "linear",
		Symbol:       "BTCUSDT",
		BuyLeverage:  "6",
		SellLeverage: "6",
	})
	require.NoError(t, err)
}

func TestSetLeverageTreatsNotModifiedAsSuccess(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(`{"retCode":110043,"retMsg":"leverage not modified","result":{}}`), nil
		}),
	}

	err := client.SetLeverage(context.Background(), SetLeverageRequest{
		Category:     "linear",
		Symbol:       "BTCUSDT",
		BuyLeverage:  "6",
		SellLeverage: "6",
	})
	require.NoError(t, err)
}
