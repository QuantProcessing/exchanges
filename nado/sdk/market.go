package nado

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"time"
)

// GetAssets returns the list of assets from V2 Assets endpoint.
func (c *Client) GetAssets(ctx context.Context) ([]AssetV2, error) {
	var assets []AssetV2
	if err := c.QueryGatewayV2(ctx, "/assets", nil, &assets); err != nil {
		return nil, err
	}
	return assets, nil
}

// GetPairs returns the list of products from V2 Pairs endpoint.
func (c *Client) GetPairs(ctx context.Context, market *string) ([]PairV2, error) {
	q := url.Values{}
	if market != nil {
		q.Set("market", *market)
	}

	var pairs []PairV2
	if err := c.QueryGatewayV2(ctx, "/pairs", q, &pairs); err != nil {
		return nil, err
	}
	return pairs, nil
}

// GetApr returns the APRs from V2 APR endpoint.
func (c *Client) GetApr(ctx context.Context) ([]AprV2, error) {
	var aps []AprV2
	if err := c.QueryGatewayV2(ctx, "/apr", nil, &aps); err != nil {
		return nil, err
	}
	return aps, nil
}

// GetOrderBook returns the orderbook for a ticker using V2.
func (c *Client) GetOrderBook(ctx context.Context, tickerID string, depth int) (*OrderBookV2, error) {
	q := url.Values{}
	q.Set("ticker_id", tickerID)
	q.Set("depth", strconv.Itoa(depth))

	var ob OrderBookV2
	if err := c.QueryGatewayV2(ctx, "/orderbook", q, &ob); err != nil {
		return nil, err
	}
	return &ob, nil
}

// GetTickers
func (c *Client) GetTickers(ctx context.Context, market MarketType, edge *bool) (TickerV2Map, error) {
	q := url.Values{}
	q.Set("market", string(market))

	if edge != nil {
		q.Set("edge", strconv.FormatBool(*edge))
	}
	var tickers TickerV2Map
	if err := c.QueryArchiveV2(ctx, "/tickers", q, &tickers); err != nil {
		return nil, err
	}
	return tickers, nil
}

// GetContracts returns the list of contracts from V2 Contracts endpoint.
func (c *Client) GetContracts(ctx context.Context, edge *bool) (ContractV2Map, error) {
	q := url.Values{}
	if edge != nil {
		q.Set("edge", strconv.FormatBool(*edge))
	}
	var contracts ContractV2Map
	if err := c.QueryArchiveV2(ctx, "/contracts", q, &contracts); err != nil {
		return nil, err
	}
	return contracts, nil
}

func (c *Client) GetTrades(ctx context.Context, tickerID string, limit *int, maxTradeID *int64) ([]TradeV2, error) {
	q := url.Values{}
	q.Set("ticker_id", tickerID)
	if limit != nil {
		q.Set("limit", strconv.Itoa(*limit))
	}
	if maxTradeID != nil {
		q.Set("max_trade_id", strconv.FormatInt(*maxTradeID, 10))
	}
	var trades []TradeV2
	if err := c.QueryGatewayV2(ctx, "/trades", q, &trades); err != nil {
		return nil, err
	}
	return trades, nil
}

// v1 endpoints api

func (c *Client) GetMarketPrice(ctx context.Context, productID int64) (*MarketPrice, error) {
	req := map[string]interface{}{
		"product_id": productID,
	}
	data, err := c.QueryGateWayV1(ctx, "POST", req)
	if err != nil {
		return nil, err
	}
	var resp MarketPrice
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetMarketPrices(ctx context.Context, productID []int) ([]MarketPrice, error) {
	req := map[string]interface{}{
		"type":        "market_price",
		"product_ids": productID,
	}
	data, err := c.QueryGateWayV1(ctx, "POST", req)
	if err != nil {
		return nil, err
	}
	var resp []MarketPrice
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) GetNonces(ctx context.Context) (*Nonce, error) {
	req := map[string]interface{}{
		"type":    "nonces",
		"address": c.address,
	}
	data, err := c.QueryGateWayV1(ctx, "GET", req)
	if err != nil {
		return nil, err
	}
	var resp Nonce
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetMarketLiquidity(ctx context.Context, productID int64, depth int) (*MarketLiquidity, error) {
	if depth > 100 || depth <= 0 {
		return nil, errors.New("depth must be between 1 and 100")
	}
	req := map[string]interface{}{
		"type":       "market_liquidity",
		"product_id": productID,
		"depth":      depth,
	}
	data, err := c.QueryGateWayV1(ctx, "GET", req)
	if err != nil {
		return nil, err
	}
	var resp MarketLiquidity
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetSymbols(ctx context.Context, productType *string) (*SymbolsInfo, error) {
	q := map[string]interface{}{
		"type": "symbols",
	}
	if productType != nil {
		q["product_type"] = *productType
	}
	data, err := c.QueryGateWayV1(ctx, "GET", q)
	if err != nil {
		return nil, err
	}
	var resp SymbolsInfo
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetContractsV1(ctx context.Context) (*ContractV1, error) {
	req := map[string]interface{}{
		"type": "contracts",
	}
	data, err := c.QueryGateWayV1(ctx, "GET", req)
	if err != nil {
		return nil, err
	}
	var resp ContractV1
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetCandlesticks queries historical candlesticks from the archive indexer.
func (c *Client) GetCandlesticks(ctx context.Context, req CandlestickRequest) ([]ArchiveCandlestick, error) {
	data, err := c.QueryArchiveV1(ctx, req)
	if err != nil {
		return nil, err
	}
	var resp CandlestickResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Candlesticks, nil
}

// GetFundingRate retrieves the funding rate for a specific product
// Returns per-hour funding rate (Nado uses 1-hour intervals)
func (c *Client) GetFundingRate(ctx context.Context, productID int64) (*FundingRateData, error) {
	req := map[string]interface{}{
		"funding_rate": map[string]interface{}{
			"product_id": productID,
		},
	}
	data, err := c.QueryArchiveV1(ctx, req)
	if err != nil {
		return nil, err
	}

	var fundingResp FundingRateResponse
	if err := json.Unmarshal(data, &fundingResp); err != nil {
		return nil, err
	}

	// Get symbol from product ID
	symbol, err := c.getSymbolForProduct(ctx, productID)
	if err != nil {
		// Use product ID as symbol if lookup fails
		symbol = fmt.Sprintf("Product-%d", productID)
	}

	return convertNadoFundingRateToStandardized(&fundingResp, symbol)
}

// GetAllFundingRates retrieves funding rates for all perp products
// Returns per-hour funding rates (Nado uses 1-hour intervals)
func (c *Client) GetAllFundingRates(ctx context.Context) ([]FundingRateData, error) {
	// First, get all perp symbols to find product IDs
	perpType := "perp"
	symbols, err := c.GetSymbols(ctx, &perpType)
	if err != nil {
		return nil, err
	}

	// Collect all product IDs
	var productIDs []int64
	productSymbols := make(map[int64]string)
	for _, sym := range symbols.Symbols {
		if sym.Type == "perp" {
			productIDs = append(productIDs, int64(sym.ProductID))
			productSymbols[int64(sym.ProductID)] = sym.Symbol
		}
	}

	if len(productIDs) == 0 {
		return []FundingRateData{}, nil
	}

	// Query all funding rates
	req := map[string]interface{}{
		"funding_rates": map[string]interface{}{
			"product_ids": productIDs,
		},
	}
	data, err := c.QueryArchiveV1(ctx, req)
	if err != nil {
		return nil, err
	}

	// Response is a map: product_id -> FundingRateResponse
	var fundingMap map[string]FundingRateResponse
	if err := json.Unmarshal(data, &fundingMap); err != nil {
		return nil, err
	}

	var result []FundingRateData
	for productIDStr, fundingResp := range fundingMap {
		productID, err := strconv.ParseInt(productIDStr, 10, 64)
		if err != nil {
			continue
		}

		symbol := productSymbols[productID]
		if symbol == "" {
			symbol = fmt.Sprintf("Product-%d", productID)
		}

		fundingData, err := convertNadoFundingRateToStandardized(&fundingResp, symbol)
		if err != nil {
			continue
		}
		result = append(result, *fundingData)
	}

	return result, nil
}

// getSymbolForProduct looks up the symbol for a given product ID
func (c *Client) getSymbolForProduct(ctx context.Context, productID int64) (string, error) {
	perpType := "perp"
	symbols, err := c.GetSymbols(ctx, &perpType)
	if err != nil {
		return "", err
	}

	for _, sym := range symbols.Symbols {
		if int64(sym.ProductID) == productID {
			return sym.Symbol, nil
		}
	}

	return "", fmt.Errorf("symbol not found for product ID: %d", productID)
}

// convertNadoFundingRateToStandardized converts Nado's funding rate to standardized format
func convertNadoFundingRateToStandardized(funding *FundingRateResponse, symbol string) (*FundingRateData, error) {
	// Parse funding_rate_x18 (24hr rate * 10^18)
	fundingRateX18 := new(big.Int)
	if _, ok := fundingRateX18.SetString(funding.FundingRateX18, 10); !ok {
		return nil, fmt.Errorf("invalid funding_rate_x18: %s", funding.FundingRateX18)
	}

	// Convert from x18 to float: funding_rate_x18 / 10^18
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	fundingRate := new(big.Rat).SetFrac(fundingRateX18, divisor)

	// This is the 24-hour rate, divide by 24 to get 1-hour rate
	hourlyRateBig := new(big.Rat).Quo(fundingRate, big.NewRat(24, 1))
	hourlyRate, _ := hourlyRateBig.Float64()

	// Parse update time
	updateTime, err := strconv.ParseInt(funding.UpdateTime, 10, 64)
	if err != nil {
		updateTime = time.Now().Unix()
	}

	// Calculate funding times (1-hour interval)
	now := time.Now().UTC()
	fundingTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)
	nextFundingTime := fundingTime.Add(1 * time.Hour)

	return &FundingRateData{
		ProductID:            funding.ProductID,
		Symbol:               symbol,
		FundingRate:          fmt.Sprintf("%.10f", hourlyRate),
		FundingIntervalHours: 1,
		FundingTime:          fundingTime.UnixMilli(),
		NextFundingTime:      nextFundingTime.UnixMilli(),
		UpdateTime:           updateTime,
	}, nil
}
