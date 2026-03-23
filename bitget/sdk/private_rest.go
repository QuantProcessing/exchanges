package sdk

import (
	"context"
	"fmt"
)

func (c *Client) PlaceOrder(ctx context.Context, req *PlaceOrderRequest) (*PlaceOrderResponse, error) {
	var out responseEnvelope[PlaceOrderResponse]
	err := c.postPrivate(ctx, "/api/v3/trade/place-order", req, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: place order failed: %s %s", out.Code, out.Msg)
	}
	return &out.Data, nil
}

func (c *Client) CancelOrder(ctx context.Context, req *CancelOrderRequest) (*CancelOrderResponse, error) {
	var out responseEnvelope[CancelOrderResponse]
	err := c.postPrivate(ctx, "/api/v3/trade/cancel-order", req, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: cancel order failed: %s %s", out.Code, out.Msg)
	}
	return &out.Data, nil
}

func (c *Client) CancelAllOrders(ctx context.Context, req *CancelAllOrdersRequest) error {
	var out responseEnvelope[any]
	err := c.postPrivate(ctx, "/api/v3/trade/cancel-symbol-order", req, &out)
	if err != nil {
		return err
	}
	if out.Code != "00000" {
		return fmt.Errorf("bitget sdk: cancel all orders failed: %s %s", out.Code, out.Msg)
	}
	return nil
}

func (c *Client) ModifyOrder(ctx context.Context, req *ModifyOrderRequest) (*CancelOrderResponse, error) {
	var out responseEnvelope[CancelOrderResponse]
	err := c.postPrivate(ctx, "/api/v3/trade/modify-order", req, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: modify order failed: %s %s", out.Code, out.Msg)
	}
	return &out.Data, nil
}

func (c *Client) GetOrder(ctx context.Context, category, symbol, orderID, clientOID string) (*OrderRecord, error) {
	var out responseEnvelope[OrderRecord]
	err := c.getPrivate(ctx, "/api/v3/trade/order-info", map[string]string{
		"category":  category,
		"symbol":    symbol,
		"orderId":   orderID,
		"clientOid": clientOID,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: get order failed: %s %s", out.Code, out.Msg)
	}
	return &out.Data, nil
}

func (c *Client) GetOpenOrders(ctx context.Context, category, symbol string) ([]OrderRecord, error) {
	var out responseEnvelope[OrderList]
	err := c.getPrivate(ctx, "/api/v3/trade/unfilled-orders", map[string]string{
		"category": category,
		"symbol":   symbol,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: get open orders failed: %s %s", out.Code, out.Msg)
	}
	return out.Data.List, nil
}

func (c *Client) GetOrderHistory(ctx context.Context, category, symbol string) ([]OrderRecord, error) {
	var out responseEnvelope[OrderList]
	err := c.getPrivate(ctx, "/api/v3/trade/history-orders", map[string]string{
		"category": category,
		"symbol":   symbol,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: get order history failed: %s %s", out.Code, out.Msg)
	}
	return out.Data.List, nil
}

func (c *Client) GetAccountAssets(ctx context.Context) (*AccountAssets, error) {
	var out responseEnvelope[AccountAssets]
	err := c.getPrivate(ctx, "/api/v3/account/assets", nil, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: get account assets failed: %s %s", out.Code, out.Msg)
	}
	return &out.Data, nil
}

func (c *Client) GetCurrentPositions(ctx context.Context, category, symbol string) ([]PositionRecord, error) {
	var out responseEnvelope[PositionList]
	err := c.getPrivate(ctx, "/api/v3/position/current-position", map[string]string{
		"category": category,
		"symbol":   symbol,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("bitget sdk: get positions failed: %s %s", out.Code, out.Msg)
	}
	return out.Data.List, nil
}

func (c *Client) SetLeverage(ctx context.Context, req *SetLeverageRequest) error {
	var out responseEnvelope[any]
	err := c.postPrivate(ctx, "/api/v3/account/set-leverage", req, &out)
	if err != nil {
		return err
	}
	if out.Code != "00000" {
		return fmt.Errorf("bitget sdk: set leverage failed: %s %s", out.Code, out.Msg)
	}
	return nil
}
