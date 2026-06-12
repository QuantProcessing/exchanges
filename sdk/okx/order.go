package okx

import (
	"context"
	"net/url"
)

// PlaceOrder submits a new order.
func (c *Client) PlaceOrder(ctx context.Context, req *OrderRequest) ([]OrderId, error) {
	return Request[OrderId](c, ctx, MethodPost, "/api/v5/trade/order", req, true)
}

// ModifyOrder amends an incomplete order.
func (c *Client) ModifyOrder(ctx context.Context, req *ModifyOrderRequest) ([]OrderId, error) {
	return Request[OrderId](c, ctx, MethodPost, "/api/v5/trade/amend-order", req, true)
}

// CancelOrder cancels an incomplete order.
func (c *Client) CancelOrder(ctx context.Context, instId, ordId, clOrdId string) ([]OrderId, error) {
	req := map[string]string{
		"instId": instId,
	}
	if ordId != "" {
		req["ordId"] = ordId
	}
	if clOrdId != "" {
		req["clOrdId"] = clOrdId
	}

	return Request[OrderId](c, ctx, MethodPost, "/api/v5/trade/cancel-order", req, true)
}

// CancelOrders cancels multiple orders (max 20).
func (c *Client) CancelOrders(ctx context.Context, reqs []CancelOrderRequest) ([]OrderId, error) {
	return Request[OrderId](c, ctx, MethodPost, "/api/v5/trade/cancel-batch-orders", reqs, true)
}

// ClosePosition closes a position.
func (c *Client) ClosePosition(ctx context.Context, instId, mgnMode string) ([]ClosePosition, error) {
	req := map[string]string{
		"instId":  instId,
		"mgnMode": mgnMode,
		"autoCxl": "true", // default true: auto cancel incomplete orders
	}

	return Request[ClosePosition](c, ctx, MethodPost, "/api/v5/trade/close-position", req, true)
}

// GetOrder retrieves order details.
func (c *Client) GetOrder(ctx context.Context, instId, ordId, clOrdId string) ([]Order, error) {
	params := url.Values{}
	params.Add("instId", instId)
	if ordId != "" {
		params.Add("ordId", ordId)
	}
	if clOrdId != "" {
		params.Add("clOrdId", clOrdId)
	}

	path := "/api/v5/trade/order"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return Request[Order](c, ctx, MethodGet, path, nil, true)
}

// GetOrders retrieves pending orders.
// instType: SPOT, MARGIN, SWAP, FUTURES, OPTION
// instId: optional, Instrument ID
func (c *Client) GetOrders(ctx context.Context, instType, instId *string) ([]Order, error) {
	params := url.Values{}
	if instType != nil {
		params.Add("instType", *instType)
	}
	if instId != nil {
		params.Add("instId", *instId)
	}

	path := "/api/v5/trade/orders-pending"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return Request[Order](c, ctx, MethodGet, path, nil, true)
}
