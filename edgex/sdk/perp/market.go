
package perp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

var (
	ErrInvalidParam = errors.New("invalid param")
)

// Public API

func (c *Client) GetExchangeInfo(ctx context.Context) (*ExchangeInfo, error) {
	var res ExchangeInfo
	err := c.call(ctx, http.MethodGet, "/api/v1/public/meta/getMetaData", nil, false, &res)
	return &res, err
}

func (c *Client) GetTicker(ctx context.Context, contractId string) (*Ticker, error) {
	var res []Ticker
	err := c.call(ctx, http.MethodGet, "/api/v1/public/quote/getTicker", map[string]interface{}{"contractId": contractId}, false, &res)
	if len(res) == 0 {
		return nil, ErrInvalidParam
	}
	return &res[0], err
}

func (c *Client) GetOrderBook(ctx context.Context, contractId string, level int) (*OrderBook, error) {
	var res []OrderBook
	err := c.call(ctx, http.MethodGet, "/api/v1/public/quote/getDepth", map[string]interface{}{"contractId": contractId, "level": level}, false, &res)
	if len(res) == 0 {
		return nil, ErrInvalidParam
	}
	return &res[0], err
}

func (c *Client) GetKline(ctx context.Context,
	contractId, priceType, klineType string, size int,
	offsetData, filterBeginKlineTimeInclusive, filterEndKlineTimeExclusive string) (*GetKlineResponse, error) {
	// check params
	if size <= 0 || size > 1000 {
		return nil, ErrInvalidParam
	}
	var res GetKlineResponse
	err := c.call(ctx, http.MethodGet, "/api/v1/public/quote/getKline", map[string]interface{}{
		"contractId":                    contractId,
		"priceType":                     priceType,
		"klineType":                     klineType,
		"size":                          size,                          // Number to retrieve. Must be greater than 0 and less than or equal to 1000
		"offsetData":                    offsetData,                    // Pagination offset. If empty, get the first page
		"filterBeginKlineTimeInclusive": filterBeginKlineTimeInclusive, // Query start time (if 0, means from current time). Returns in descending order by time
		"filterEndKlineTimeExclusive":   filterEndKlineTimeExclusive,   // Query end time
	}, false, &res)
	return &res, err
}

func (c *Client) GetExchangeLongShortRatio(ctx context.Context, rang, filterContractIdList, filterExchangeList *string) (*ExchangeLongShortRatio, error) {
	var params = make(map[string]interface{})
	if rang != nil {
		params["rang"] = *rang
	}
	if filterContractIdList != nil {
		params["filterContractIdList"] = *filterContractIdList
	}
	if filterExchangeList != nil {
		params["filterExchangeList"] = *filterExchangeList
	}
	var res ExchangeLongShortRatio
	err := c.call(ctx, http.MethodGet, "/api/v1/public/quote/getExchangeLongShortRatio", params, false, &res)
	return &res, err
}

// GetFundingRate retrieves the latest funding rate for a specific contract.
// Returns per-hour funding rate (converted from the settlement interval rate)
func (c *Client) GetFundingRate(ctx context.Context, contractId string) (*FundingRateData, error) {
	params := make(map[string]interface{})
	if contractId != "" {
		params["contractId"] = contractId
	}

	var data []FundingRateData
	err := c.call(ctx, http.MethodGet, "/api/v1/public/funding/getLatestFundingRate", params, false, &data)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, ErrInvalidParam
	}

	// Convert funding rate to per-hour rate
	converted, err := convertEdgexFundingRateToHourly(&data[0])
	if err != nil {
		return nil, err
	}

	return converted, nil
}

// GetAllFundingRates retrieves the latest funding rates for all contracts.
// Returns per-hour funding rates (converted from settlement interval rates)
func (c *Client) GetAllFundingRates(ctx context.Context) ([]FundingRateData, error) {
	// First, get all contract information to retrieve contract IDs
	exchangeInfo, err := c.GetExchangeInfo(ctx)
	if err != nil {
		return nil, err
	}

	// Collect all contract IDs
	var contractIds []string
	for _, contract := range exchangeInfo.ContractList {
		contractIds = append(contractIds, contract.ContractId)
	}

	if len(contractIds) == 0 {
		return []FundingRateData{}, nil
	}

	// Build query parameters with multiple contractId fields
	params := url.Values{}
	for _, contractId := range contractIds {
		params.Add("contractId", contractId)
	}

	var data []FundingRateData
	endpoint := "/api/v1/public/funding/getLatestFundingRate?" + params.Encode()
	err = c.call(ctx, http.MethodGet, endpoint, nil, false, &data)
	if err != nil {
		return nil, err
	}

	// Convert all funding rates to per-hour rates
	var result []FundingRateData
	for i := range data {
		converted, err := convertEdgexFundingRateToHourly(&data[i])
		if err != nil {
			// Skip rates that can't be converted
			continue
		}
		result = append(result, *converted)
	}

	return result, nil
}

// convertEdgexFundingRateToHourly converts EdgeX funding rate to per-hour rate
func convertEdgexFundingRateToHourly(data *FundingRateData) (*FundingRateData, error) {
	// Parse funding interval in minutes
	intervalMin, err := strconv.ParseFloat(data.FundingRateIntervalMin, 64)
	if err != nil || intervalMin == 0 {
		return nil, fmt.Errorf("invalid funding rate interval: %s", data.FundingRateIntervalMin)
	}

	// Convert to hours
	intervalHours := intervalMin / 60.0

	// Parse original funding rate
	rate, err := strconv.ParseFloat(data.FundingRate, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse funding rate: %w", err)
	}

	// Convert to per-hour rate
	hourlyRate := rate / intervalHours

	// Calculate next funding time from current funding timestamp
	fundingTimestamp, err := strconv.ParseInt(data.FundingTimestamp, 10, 64)
	if err == nil {
		// Add interval in milliseconds
		intervalMs := int64(intervalMin * 60 * 1000)
		nextFundingTimestamp := fundingTimestamp + intervalMs
		data.NextFundingTime = strconv.FormatInt(nextFundingTimestamp, 10)
	}

	// Create new data with converted rate
	result := *data
	result.FundingRate = fmt.Sprintf("%.10f", hourlyRate)
	return &result, nil
}
