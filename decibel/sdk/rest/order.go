package rest

import (
	"context"
	"net/url"
	"strconv"
)

func (c *Client) GetOpenOrders(ctx context.Context, account, cursor string, limit int) (*OpenOrdersResponse, error) {
	query := url.Values{}
	query.Set("account", account)
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	var resp OpenOrdersResponse
	if err := c.get(ctx, "/api/v1/open_orders", query, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
