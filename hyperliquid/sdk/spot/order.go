package spot

import (
	"context"
	"encoding/json"
	"fmt"

	hyperliquid "github.com/QuantProcessing/exchanges/hyperliquid/sdk"
)

func (c *Client) placeOrder(ctx context.Context, req PlaceOrderRequest) ([]byte, error) {
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
	if err := json.Unmarshal(data, res); err != nil {
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

func (c *Client) modifyOrder(ctx context.Context, req ModifyOrderRequest) ([]byte, error) {
	if c.PrivateKey == nil {
		return nil, hyperliquid.ErrCredentialsRequired
	}
	action, err := buildModifyOrderAction(req)
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
	data, err := c.modifyOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	res := new(hyperliquid.APIResponse[ModifyOrderResponse])
	if err := json.Unmarshal(data, res); err != nil {
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

func (c *Client) cancelOrder(ctx context.Context, req CancelOrderRequest) ([]byte, error) {
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
	if err := json.Unmarshal(data, res); err != nil {
		return nil, err
	}
	if res.Status != "ok" {
		return nil, fmt.Errorf("cancel order failed: %s", res.Status)
	}
	if err := res.Response.Data.Statuses.FirstError(); err != nil {
		return nil, err
	}
	var status string
	_ = json.Unmarshal(res.Response.Data.Statuses[0], &status)
	return &status, nil
}
