package rest

import (
	"context"
	"net/url"
)

func (c *Client) GetAccountOverview(ctx context.Context, account string) (*AccountOverview, error) {
	query := url.Values{}
	query.Set("account", account)

	var overview AccountOverview
	if err := c.get(ctx, "/api/v1/account_overviews", query, &overview); err != nil {
		return nil, err
	}
	return &overview, nil
}

func (c *Client) GetAccountPositions(ctx context.Context, account string) ([]AccountPosition, error) {
	query := url.Values{}
	query.Set("account", account)

	var positions []AccountPosition
	if err := c.get(ctx, "/api/v1/account_positions", query, &positions); err != nil {
		return nil, err
	}
	return positions, nil
}
