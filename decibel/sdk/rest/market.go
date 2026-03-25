package rest

import "context"

func (c *Client) GetMarkets(ctx context.Context) ([]Market, error) {
	var markets []Market
	if err := c.get(ctx, "/api/v1/markets", nil, &markets); err != nil {
		return nil, err
	}
	return markets, nil
}
