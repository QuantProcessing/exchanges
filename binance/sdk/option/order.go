package option

import (
	"context"
	"strings"
)

func (c *Client) PlaceOrder(ctx context.Context, p PlaceOrderParams) (*OrderResponse, error) {
	params := map[string]interface{}{
		"symbol":   strings.ToUpper(p.Symbol),
		"side":     strings.ToUpper(p.Side),
		"type":     strings.ToUpper(p.Type),
		"quantity": p.Quantity,
	}
	if p.Price != "" {
		params["price"] = p.Price
	}
	if p.TimeInForce != "" {
		params["timeInForce"] = strings.ToUpper(p.TimeInForce)
	}
	if p.ClientOrderID != "" {
		params["clientOrderId"] = p.ClientOrderID
	}
	if p.ReduceOnly {
		params["reduceOnly"] = "true"
	}

	var res OrderResponse
	if err := c.Post(ctx, "/eapi/v1/order", params, true, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) CancelOrder(ctx context.Context, p CancelOrderParams) (*OrderResponse, error) {
	params := map[string]interface{}{"symbol": strings.ToUpper(p.Symbol)}
	if p.OrderID != "" {
		params["orderId"] = p.OrderID
	}
	if p.ClientOrderID != "" {
		params["clientOrderId"] = p.ClientOrderID
	}

	var res OrderResponse
	if err := c.Delete(ctx, "/eapi/v1/order", params, true, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) CancelAllOpenOrders(ctx context.Context, p CancelAllOrdersParams) error {
	params := map[string]interface{}{}
	if p.Symbol != "" {
		params["symbol"] = strings.ToUpper(p.Symbol)
	}
	var res struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	return c.Delete(ctx, "/eapi/v1/allOpenOrders", params, true, &res)
}

func (c *Client) GetOrder(ctx context.Context, symbol, orderID, clientOrderID string) (*OrderResponse, error) {
	params := map[string]interface{}{"symbol": strings.ToUpper(symbol)}
	if orderID != "" {
		params["orderId"] = orderID
	}
	if clientOrderID != "" {
		params["clientOrderId"] = clientOrderID
	}

	var res OrderResponse
	if err := c.Get(ctx, "/eapi/v1/order", params, true, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) GetOpenOrders(ctx context.Context, symbol string) ([]OrderResponse, error) {
	params := map[string]interface{}{}
	if symbol != "" {
		params["symbol"] = strings.ToUpper(symbol)
	}

	var res []OrderResponse
	if err := c.Get(ctx, "/eapi/v1/openOrders", params, true, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) GetOrderHistory(ctx context.Context, symbol string) ([]OrderResponse, error) {
	params := map[string]interface{}{}
	if symbol != "" {
		params["symbol"] = strings.ToUpper(symbol)
	}

	var res []OrderResponse
	if err := c.Get(ctx, "/eapi/v1/historyOrders", params, true, &res); err != nil {
		return nil, err
	}
	return res, nil
}
