package sdk

import (
	"context"
	"fmt"
)

func (c *Client) GetWalletBalance(ctx context.Context, accountType, coin string) (*WalletBalanceResult, error) {
	query := map[string]string{"accountType": accountType}
	if coin != "" {
		query["coin"] = coin
	}

	var resp responseEnvelope[WalletBalanceResult]
	err := c.getPrivate(ctx, "/v5/account/wallet-balance", query, &resp)
	if err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit sdk: get wallet balance failed: %d %s", resp.RetCode, resp.RetMsg)
	}
	return &resp.Result, nil
}

func (c *Client) GetFeeRates(ctx context.Context, category, symbol string) ([]FeeRateRecord, error) {
	var resp responseEnvelope[FeeRatesResult]
	err := c.getPrivate(ctx, "/v5/account/fee-rate", map[string]string{
		"category": category,
		"symbol":   symbol,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit sdk: get fee rates failed: %d %s", resp.RetCode, resp.RetMsg)
	}
	return resp.Result.List, nil
}

func (c *Client) GetPositions(ctx context.Context, category, symbol, settleCoin string) ([]PositionRecord, error) {
	query := map[string]string{
		"category":   category,
		"symbol":     symbol,
		"settleCoin": settleCoin,
	}
	var resp responseEnvelope[PositionsResult]
	err := c.getPrivate(ctx, "/v5/position/list", query, &resp)
	if err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit sdk: get positions failed: %d %s", resp.RetCode, resp.RetMsg)
	}
	return resp.Result.List, nil
}

func (c *Client) SetLeverage(ctx context.Context, req SetLeverageRequest) error {
	var resp responseEnvelope[map[string]any]
	err := c.postPrivate(ctx, "/v5/position/set-leverage", req, &resp)
	if err != nil {
		return err
	}
	if resp.RetCode == 110043 {
		return nil
	}
	if resp.RetCode != 0 {
		return fmt.Errorf("bybit sdk: set leverage failed: %d %s", resp.RetCode, resp.RetMsg)
	}
	return nil
}
