package perp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QuantProcessing/exchanges/hyperliquid/sdk"
)

// UserOpenOrders
func (c *Client) UserOpenOrders(ctx context.Context, user string) ([]Order, error) {
	req := map[string]string{
		"type": "openOrders",
		"user": user,
	}
	data, err := c.Post(ctx, "/info", req)
	if err != nil {
		return nil, err
	}
	var res []Order
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// OrderStatus
func (c *Client) OrderStatus(ctx context.Context, user string, oid int64) (*OrderStatusInfo, error) {
	req := map[string]any{
		"type": "orderStatus",
		"user": user,
		"oid":  oid,
	}
	data, err := c.Post(ctx, "/info", req)
	if err != nil {
		return nil, err
	}
	var res OrderStatusQueryResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return &res.OrderStatus, nil
}

// Transaction Helpers

func (c *Client) placeOrder(ctx context.Context, req PlaceOrderRequest) (data []byte, err error) {
	if c.PrivateKey == nil {
		return nil, hyperliquid.ErrCredentialsRequired
	}
	action, err := buildPlaceOrderAction(req)
	if err != nil {
		return nil, err
	}
	nonce := c.GetNextNonce()
	sig, err := hyperliquid.SignL1Action(c.PrivateKey, action, c.Vault, nonce, c.ExpiresAfter, true)
	if err != nil {
		return nil, err
	}

	return c.PostAction(ctx, action, sig, nonce)
}

func (c *Client) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*OrderStatus, error) {
	data, err := c.placeOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	res := new(hyperliquid.APIResponse[PlaceOrderResponse])
	err = json.Unmarshal(data, res)
	if err != nil {
		return nil, err
	}
	if res.Status != "ok" {
		return nil, fmt.Errorf("place order failed: %s", res.Status)
	}
	status := res.Response.Data.Statuses[0]
	if status.Error != nil {
		return nil, fmt.Errorf("place order failed: %s", *status.Error)
	}
	return &status, nil
}

// Modify

func (c *Client) newModifyOrdersAction(orders []ModifyOrderRequest) (hyperliquid.BatchModifyAction, error) {
	modifies := make([]hyperliquid.ModifyOrderAction, len(orders))
	for i, req := range orders {
		modify, err := buildModifyOrderAction(req)
		if err != nil {
			return hyperliquid.BatchModifyAction{}, fmt.Errorf("failed to create modify request %d: %w", i, err)
		}
		modify.Type = ""
		modifies[i] = modify
	}

	return hyperliquid.BatchModifyAction{
		Type:     "batchModify",
		Modifies: modifies,
	}, nil
}

func (c *Client) modifyOrders(ctx context.Context, req []ModifyOrderRequest) (data []byte, err error) {
	if c.PrivateKey == nil {
		return nil, hyperliquid.ErrCredentialsRequired
	}
	action, err := c.newModifyOrdersAction(req)
	if err != nil {
		return nil, err
	}
	nonce := c.GetNextNonce()
	sig, err := hyperliquid.SignL1Action(c.PrivateKey, action, c.Vault, nonce, c.ExpiresAfter, true)
	if err != nil {
		return nil, err
	}

	return c.PostAction(ctx, action, sig, nonce)
}

func (c *Client) ModifyOrder(ctx context.Context, req ModifyOrderRequest) (*OrderStatus, error) {
	data, err := c.modifyOrders(ctx, []ModifyOrderRequest{req})
	if err != nil {
		return nil, err
	}
	res := new(hyperliquid.APIResponse[ModifyOrderResponse])
	err = json.Unmarshal(data, res)
	if err != nil {
		return nil, err
	}
	if res.Status != "ok" {
		return nil, fmt.Errorf("modify order failed: %s", res.Status)
	}
	status := res.Response.Data.Statuses[0]
	if status.Error != nil {
		return nil, fmt.Errorf("modify order failed: %s", *status.Error)
	}
	return &status, nil
}

// Cancel Order

func (c *Client) cancelOrder(ctx context.Context, req CancelOrderRequest) (data []byte, err error) {
	if c.PrivateKey == nil {
		return nil, hyperliquid.ErrCredentialsRequired
	}
	action, err := buildCancelOrderAction(req)
	if err != nil {
		return nil, err
	}
	nonce := c.GetNextNonce()
	sig, err := hyperliquid.SignL1Action(c.PrivateKey, action, c.Vault, nonce, c.ExpiresAfter, true)
	if err != nil {
		return nil, err
	}

	return c.PostAction(ctx, action, sig, nonce)
}

func (c *Client) CancelOrder(ctx context.Context, req CancelOrderRequest) (*string, error) {
	data, err := c.cancelOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	res := new(hyperliquid.APIResponse[CancelOrderResponse])
	err = json.Unmarshal(data, res)
	if err != nil {
		return nil, err
	}
	if res.Status != "ok" {
		return nil, fmt.Errorf("cancel order failed: %s", res.Status)
	}
	if err := res.Response.Data.Statuses.FirstError(); err != nil {
		return nil, err
	}
	var status string
	json.Unmarshal(res.Response.Data.Statuses[0], &status)
	return &status, nil
}
