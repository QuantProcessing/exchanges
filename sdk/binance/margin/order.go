package margin

import (
	"context"
)

// PlaceOrder places a margin order
// Endpoint: POST /sapi/v1/margin/order
func (c *Client) PlaceOrder(ctx context.Context, params *PlaceOrderParams) (*OrderResponseFull, error) {
	p := map[string]interface{}{
		"symbol": params.Symbol,
		"side":   params.Side,
		"type":   params.Type,
	}

	if params.Quantity > 0 {
		p["quantity"] = params.Quantity
	}
	if params.QuoteOrderQty > 0 {
		p["quoteOrderQty"] = params.QuoteOrderQty
	}
	if params.Price > 0 {
		p["price"] = params.Price
	}
	if params.TimeInForce != "" {
		p["timeInForce"] = params.TimeInForce
	}
	if params.NewClientOrderID != "" {
		p["newClientOrderId"] = params.NewClientOrderID
	}
	if params.SideEffectType != "" {
		p["sideEffectType"] = params.SideEffectType
	}
	if params.IsIsolated {
		p["isIsolated"] = "TRUE"
	}

	// Always ask for FULL response for better debugging/info
	p["newOrderRespType"] = "FULL"

	var res OrderResponseFull
	err := c.Post(ctx, "/sapi/v1/margin/order", p, true, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// GetOrder queries a margin order
// Endpoint: GET /sapi/v1/margin/order
func (c *Client) GetOrder(ctx context.Context, symbol string, orderID int64, origClientOrderID string, isIsolated bool) (*MarginOrder, error) {
	params := map[string]interface{}{
		"symbol": symbol,
	}
	if orderID > 0 {
		params["orderId"] = orderID
	}
	if origClientOrderID != "" {
		params["origClientOrderId"] = origClientOrderID
	}
	if isIsolated {
		params["isIsolated"] = "TRUE"
	}

	var res MarginOrder
	err := c.Get(ctx, "/sapi/v1/margin/order", params, true, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// CancelOrder excels a margin order
// Endpoint: DELETE /sapi/v1/margin/order
func (c *Client) CancelOrder(ctx context.Context, symbol string, orderID int64, origClientOrderID string, isIsolated bool) (*MarginOrder, error) {
	params := map[string]interface{}{
		"symbol": symbol,
	}
	if orderID > 0 {
		params["orderId"] = orderID
	}
	if origClientOrderID != "" {
		params["origClientOrderId"] = origClientOrderID
	}
	if isIsolated {
		params["isIsolated"] = "TRUE"
	}

	var res MarginOrder
	err := c.Delete(ctx, "/sapi/v1/margin/order", params, true, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
