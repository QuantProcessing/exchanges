package rest

import (
	"context"
	"net/url"
	"strconv"
)

func (c *Client) GetOpenOrders(ctx context.Context, account string, limit int, offset int) (*OpenOrdersResponse, error) {
	query := url.Values{}
	query.Set("account", account)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}

	var resp OpenOrdersResponse
	if err := c.get(ctx, "/api/v1/open_orders", query, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetOrderHistory(ctx context.Context, account string, limit int, offset int) (*OpenOrdersResponse, error) {
	query := url.Values{}
	query.Set("account", account)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}

	var resp OpenOrdersResponse
	if err := c.get(ctx, "/api/v1/order_history", query, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetOrder(
	ctx context.Context,
	account string,
	market string,
	orderID string,
	clientOrderID string,
) (*OrderResponse, error) {
	query := url.Values{}
	query.Set("account", account)
	query.Set("market", market)
	if orderID != "" {
		query.Set("order_id", orderID)
	}
	if clientOrderID != "" {
		query.Set("client_order_id", clientOrderID)
	}

	var resp OrderResponse
	if err := c.get(ctx, "/api/v1/orders", query, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetOrderByID(ctx context.Context, account, orderID string) (*OpenOrder, error) {
	offset := 0

	for {
		resp, err := c.GetOpenOrders(ctx, account, 100, offset)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			break
		}

		for _, order := range resp.Items {
			if order.OrderID == orderID {
				orderCopy := order
				return &orderCopy, nil
			}
		}

		offset += len(resp.Items)
		if len(resp.Items) == 0 || offset >= resp.TotalCount {
			break
		}
	}

	offset = 0
	for {
		resp, err := c.GetOrderHistory(ctx, account, 100, offset)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			break
		}

		for _, order := range resp.Items {
			if order.OrderID == orderID {
				orderCopy := order
				return &orderCopy, nil
			}
		}

		offset += len(resp.Items)
		if len(resp.Items) == 0 || offset >= resp.TotalCount {
			break
		}
	}

	return nil, nil
}
