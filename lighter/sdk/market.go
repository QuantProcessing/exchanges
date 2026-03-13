package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// GetAssetDetails fetches asset details
func (c *Client) GetAssetDetails(ctx context.Context, assetIndex *int16) (*AssetDetailsResponse, error) {
	path := "/api/v1/assetDetails"
	if assetIndex != nil {
		path = fmt.Sprintf("%s?asset_index=%d", path, *assetIndex)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res AssetDetailsResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get asset details: %s", res.Msg)
	}
	return &res, nil
}

// GetOrderBookDetails fetches order book details
func (c *Client) GetOrderBookDetails(ctx context.Context, marketId *int, filter *string) (*OrderBookDetailsResponse, error) {
	params := url.Values{}
	if marketId != nil {
		params.Add("market_id", fmt.Sprintf("%d", *marketId))
	}
	if filter != nil {
		params.Add("filter", *filter)
	}

	path := "/api/v1/orderBookDetails"
	if queryString := params.Encode(); queryString != "" {
		path = fmt.Sprintf("%s?%s", path, queryString)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res OrderBookDetailsResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("unmarshal error: %w, code: %d, body: %s", err, resp.StatusCode, string(data))
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get order book details: %s", res.Msg)
	}
	return &res, nil
}

// GetOrderBooks fetches order books
func (c *Client) GetOrderBooks(ctx context.Context, marketId *int) (*OrderBooksResponse, error) {
	path := "/api/v1/orderBooks"
	if marketId != nil {
		path = fmt.Sprintf("%s?market_id=%d", path, *marketId)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res OrderBooksResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get order books: %s", res.Msg)
	}
	return &res, nil
}

// GetRecentTrades fetches recent trades
func (c *Client) GetRecentTrades(ctx context.Context, marketId int, limit int64) (*RecentTradesResponse, error) {
	path := fmt.Sprintf("/api/v1/recentTrades?market_id=%d&limit=%d", marketId, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res RecentTradesResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get recent trades: %s", res.Msg)
	}
	return &res, nil
}

// GetOrderBookOrders fetches detailed order book orders
func (c *Client) GetOrderBookOrders(ctx context.Context, marketId int, limit int64) (*OrderBookOrdersResponse, error) {
	path := fmt.Sprintf("/api/v1/orderBookOrders?market_id=%d&limit=%d", marketId, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res OrderBookOrdersResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get order book orders: %s", res.Msg)
	}
	return &res, nil
}

// GetFundingRates fetches current funding rates
func (c *Client) GetFundingRates(ctx context.Context) (*FundingRatesResponse, error) {
	path := "/api/v1/funding-rates"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res FundingRatesResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get funding rates: %s", res.Msg)
	}
	return &res, nil
}

// GetFundingRate retrieves the funding rate for a specific market id
// Returns per-hour funding rate (Lighter uses 1-hour interval natively)
func (c *Client) GetFundingRate(ctx context.Context, marketId int) (*FundingRateData, error) {
	rates, err := c.GetFundingRates(ctx)
	if err != nil {
		return nil, err
	}
	if rates.Code != 200 {
		return nil, fmt.Errorf("failed to get funding rate: %s", rates.Msg)
	}

	// Find the rate for the requested market id
	for _, rate := range rates.FundingRate {
		if rate.MarketId == marketId {
			return convertLighterFundingRateToStandardized(rate), nil
		}
	}

	return nil, fmt.Errorf("funding rate not found for market id: %d", marketId)
}

// GetAllFundingRates retrieves funding rates for all symbols
// Returns per-hour funding rates (Lighter uses 1-hour interval natively)
func (c *Client) GetAllFundingRates(ctx context.Context) ([]FundingRateData, error) {
	rates, err := c.GetFundingRates(ctx)
	if err != nil {
		return nil, err
	}
	if rates.Code != 200 {
		return nil, fmt.Errorf("failed to get funding rates: %s", rates.Msg)
	}

	var result []FundingRateData
	for _, rate := range rates.FundingRate {
		result = append(result, *convertLighterFundingRateToStandardized(rate))
	}

	return result, nil
}

// convertLighterFundingRateToStandardized converts Lighter funding rate to standardized format
// Lighter uses 1-hour funding intervals, so we calculate funding times based on the current hour
func convertLighterFundingRateToStandardized(funding *FundingRate) *FundingRateData {
	// Get current time in milliseconds
	now := time.Now().UTC()

	// Calculate funding time (start of current hour)
	fundingTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)

	// Calculate next funding time (start of next hour)
	nextFundingTime := fundingTime.Add(1 * time.Hour)

	return &FundingRateData{
		Symbol:               funding.Symbol,
		MarketId:             funding.MarketId,
		FundingRate:          fmt.Sprintf("%.10f", funding.Rate),
		FundingIntervalHours: 1, // Lighter always uses 1-hour interval
		FundingTime:          fundingTime.UnixMilli(),
		NextFundingTime:      nextFundingTime.UnixMilli(),
	}
}

// GetExchangeStats fetches exchange statistics
func (c *Client) GetExchangeStats(ctx context.Context) (*ExchangeStatsResponse, error) {
	path := "/api/v1/exchangeStats"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res ExchangeStatsResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// GetCandlesticks fetches candlesticks
func (c *Client) GetCandlesticks(ctx context.Context, marketId int, resolution string, startTimestamp, endTimestamp int64) (*CandlesticksResponse, error) {
	path := fmt.Sprintf("/api/v1/candlesticks?market_id=%d&resolution=%s&start_timestamp=%d&end_timestamp=%d", marketId, resolution, startTimestamp, endTimestamp)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res CandlesticksResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get candlesticks: %s", res.Msg)
	}
	return &res, nil
}

// GetFundingHistory fetches funding history
func (c *Client) GetFundingHistory(ctx context.Context, marketId *int, limit int64) (*FundingHistoryResponse, error) {
	path := fmt.Sprintf("/api/v1/fundings?limit=%d", limit)
	if marketId != nil {
		path = fmt.Sprintf("%s&market_id=%d", path, *marketId)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res FundingHistoryResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get funding history: %s", res.Msg)
	}
	return &res, nil
}

// GetTransferFeeInfo fetches transfer fee info
func (c *Client) GetTransferFeeInfo(ctx context.Context) (*TransferFeeInfoResponse, error) {
	path := "/api/v1/transferFeeInfo"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res TransferFeeInfoResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get transfer fee info: %s", res.Msg)
	}
	return &res, nil
}

// GetWithdrawalDelay fetches withdrawal delay
func (c *Client) GetWithdrawalDelay(ctx context.Context) (*WithdrawalDelayResponse, error) {
	path := "/api/v1/withdrawalDelay"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res WithdrawalDelayResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get withdrawal delay: %s", res.Msg)
	}
	return &res, nil
}

// GetAnnouncements fetches announcements
func (c *Client) GetAnnouncements(ctx context.Context) (*AnnouncementResponse, error) {
	path := "/api/v1/announcement"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res AnnouncementResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get announcements: %s", res.Msg)
	}
	return &res, nil
}

// GetL1Metadata fetches L1 metadata
func (c *Client) GetL1Metadata(ctx context.Context) (*L1MetadataResponse, error) {
	path := "/api/v1/l1Metadata"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res L1MetadataResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get L1 metadata: %s", res.Msg)
	}
	return &res, nil
}

// GetPublicPoolsMetadata fetches public pools metadata
func (c *Client) GetPublicPoolsMetadata(ctx context.Context) (*PublicPoolsMetadataResponse, error) {
	path := "/api/v1/publicPoolsMetadata"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res PublicPoolsMetadataResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("failed to get public pools metadata: %s", res.Msg)
	}
	return &res, nil
}
