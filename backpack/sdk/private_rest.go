package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultRecvWindow = int64(5000)

func (c *Client) signedDo(ctx context.Context, method, path, instruction string, query map[string]string, body any, out any) error {
	query = filterEmptyParams(query)
	timestamp := time.Now().UnixMilli()
	headers, err := buildSignedHeaders(c.apiKey, c.privateKey, instruction, query, timestamp, defaultRecvWindow)
	if err != nil {
		return err
	}

	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	q := u.Query()
	for key, value := range query {
		if value != "" {
			q.Set(key, value)
		}
	}
	u.RawQuery = q.Encode()

	var bodyReader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(payload)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("backpack sdk: %s %s returned %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) GetAccount(ctx context.Context) (*AccountSettings, error) {
	var out AccountSettings
	err := c.signedDo(ctx, http.MethodGet, "/api/v1/account", "accountQuery", nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetBalances(ctx context.Context) (map[string]CapitalBalance, error) {
	var out map[string]CapitalBalance
	err := c.signedDo(ctx, http.MethodGet, "/api/v1/capital", "balanceQuery", nil, nil, &out)
	return out, err
}

func (c *Client) GetOpenOrders(ctx context.Context, marketType, symbol string) ([]Order, error) {
	query := map[string]string{
		"marketType": marketType,
		"symbol":     symbol,
	}
	var out []Order
	err := c.signedDo(ctx, http.MethodGet, "/api/v1/orders", "orderQueryAll", query, nil, &out)
	return out, err
}

func (c *Client) GetOpenPositions(ctx context.Context, symbol string) ([]Position, error) {
	query := map[string]string{
		"symbol": symbol,
	}
	var out []Position
	err := c.signedDo(ctx, http.MethodGet, "/api/v1/position", "positionQuery", query, nil, &out)
	return out, err
}

func (c *Client) ExecuteOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
	signParams := map[string]string{
		"symbol":    req.Symbol,
		"side":      req.Side,
		"orderType": req.OrderType,
		"quantity":  req.Quantity,
	}
	if req.Price != "" {
		signParams["price"] = req.Price
	}
	if req.TimeInForce != "" {
		signParams["timeInForce"] = req.TimeInForce
	}
	if req.ReduceOnly {
		signParams["reduceOnly"] = "true"
	}
	if req.ClientID != 0 {
		signParams["clientId"] = strconv.FormatUint(uint64(req.ClientID), 10)
	}

	var out Order
	err := c.signedDo(ctx, http.MethodPost, "/api/v1/order", "orderExecute", signParams, req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PlaceOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
	return c.ExecuteOrder(ctx, req)
}

func (c *Client) CancelOrder(ctx context.Context, req CancelOrderRequest) (*Order, error) {
	signParams := map[string]string{
		"orderId": req.OrderID,
		"symbol":  req.Symbol,
	}
	var out Order
	err := c.signedDo(ctx, http.MethodDelete, "/api/v1/order", "orderCancel", signParams, req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CancelOpenOrders(ctx context.Context, symbol, marketType string) error {
	signParams := map[string]string{
		"symbol":     symbol,
		"marketType": marketType,
	}
	return c.signedDo(ctx, http.MethodDelete, "/api/v1/orders", "orderCancelAll", signParams, signParams, nil)
}
