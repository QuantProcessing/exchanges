
package spot

import (
	"context"
	"fmt"
	"net/http"
)

// Order Operations

func (c *Client) PlaceOrder(ctx context.Context, params PlaceOrderParams, symbol *Symbol, coin *Coin) (*CreateOrderData, error) {
	return nil, fmt.Errorf("not supported")
}

func (c *Client) CancelOrder(ctx context.Context, orderId string) (*CancelOrderData, error) {
	params := map[string]interface{}{
		"accountId":   c.AccountID,
		"orderIdList": []string{orderId},
	}
	var resData CancelOrderData
	err := c.call(ctx, http.MethodPost, "/api/v1/private/user/order/cancelOrderById", params, true, &resData)
	return &resData, err
}

func (c *Client) CancelAllOrders(ctx context.Context) error {
	params := map[string]interface{}{
		"accountId": c.AccountID,
	}
	return c.call(ctx, http.MethodPost, "/api/v1/private/user/order/cancelAllOrder", params, true, nil)
}

func (c *Client) GetOrdersByIds(ctx context.Context, orderIds []string) ([]Order, error) {
	var res []Order
	params := map[string]interface{}{
		"accountId":   c.AccountID,
		"orderIdList": orderIds,
	}
	// Endpoint guessed from user link/pattern: getOrdersByAccountIdAndOrderIdsBatch
	// Path: /api/v1/private/user/order/getOrdersByAccountIdAndOrderIdsBatch
	err := c.call(ctx, http.MethodPost, "/api/v1/private/user/order/getOrdersByAccountIdAndOrderIdsBatch", params, true, &res)
	return res, err
}
