package sdk

import (
	"context"
)

func (c *Client) Buy(ctx context.Context, req OrderRequest) (*OrderResult, error) {
	return c.placeOrder(ctx, "/api/v2/private/buy", req)
}

func (c *Client) Sell(ctx context.Context, req OrderRequest) (*OrderResult, error) {
	return c.placeOrder(ctx, "/api/v2/private/sell", req)
}

func (c *Client) placeOrder(ctx context.Context, path string, req OrderRequest) (*OrderResult, error) {
	query := map[string]string{
		"instrument_name": req.InstrumentName,
		"amount":          req.Amount,
		"type":            req.Type,
		"label":           req.Label,
		"time_in_force":   req.TimeInForce,
	}
	if req.Price != "" {
		query["price"] = req.Price
	}
	if req.ReduceOnly {
		query["reduce_only"] = "true"
	}
	if req.PostOnly {
		query["post_only"] = "true"
	}

	var res OrderResult
	if err := c.privateGet(ctx, path, query, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) (*OrderRecord, error) {
	var res OrderRecord
	if err := c.privateGet(ctx, "/api/v2/private/cancel", map[string]string{"order_id": orderID}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) CancelAll(ctx context.Context) (int64, error) {
	var res int64
	if err := c.privateGet(ctx, "/api/v2/private/cancel_all", nil, &res); err != nil {
		return 0, err
	}
	return res, nil
}

func (c *Client) CancelAllByInstrument(ctx context.Context, instrumentName string) (int64, error) {
	var res int64
	if err := c.privateGet(ctx, "/api/v2/private/cancel_all_by_instrument", map[string]string{
		"instrument_name": instrumentName,
	}, &res); err != nil {
		return 0, err
	}
	return res, nil
}

func (c *Client) GetOrderState(ctx context.Context, orderID string) (*OrderRecord, error) {
	var res OrderRecord
	if err := c.privateGet(ctx, "/api/v2/private/get_order_state", map[string]string{"order_id": orderID}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) GetOpenOrdersByInstrument(ctx context.Context, instrumentName string) ([]OrderRecord, error) {
	var res []OrderRecord
	if err := c.privateGet(ctx, "/api/v2/private/get_open_orders_by_instrument", map[string]string{
		"instrument_name": instrumentName,
	}, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) GetOrderHistoryByInstrument(ctx context.Context, instrumentName string, count int) ([]OrderRecord, error) {
	query := map[string]string{"instrument_name": instrumentName}
	if count > 0 {
		query["count"] = int64String(int64(count))
	}
	var res []OrderRecord
	if err := c.privateGet(ctx, "/api/v2/private/get_order_history_by_instrument", query, &res); err != nil {
		return nil, err
	}
	return res, nil
}
