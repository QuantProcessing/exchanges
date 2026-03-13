//go:build grvt

package grvt

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// Market Data Methods

func (c *Client) GetInstruments(ctx context.Context) ([]Instrument, error) {
	req := map[string]interface{}{} // Empty request
	resp, err := c.Post(ctx, c.MarketDataURL+"/lite/v1/all_instruments", req, false)
	if err != nil {
		return nil, err
	}

	var result GetInstrumentsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return result.Result, nil
}

func (c *Client) GetOrderBook(ctx context.Context, instrument string, limit int) (*GetOrderBookResponse, error) {
	req := GetOrderBookRequest{
		Instrument: instrument,
		Depth:      limit,
	}
	resp, err := c.Post(ctx, c.MarketDataURL+"/lite/v1/book", req, false)
	if err != nil {
		return nil, err
	}

	var result GetOrderBookResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetMiniTicker(ctx context.Context, instrument string) (*GetMiniTickerResponse, error) {
	req := GetMiniTickerRequest{
		Instrument: instrument,
	}
	resp, err := c.Post(ctx, c.MarketDataURL+"/lite/v1/mini", req, false)
	if err != nil {
		return nil, err
	}

	var result GetMiniTickerResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetTicker(ctx context.Context, instrument string) (*GetTickerResponse, error) {
	req := GetTickerRequest{
		Instrument: instrument,
	}
	resp, err := c.Post(ctx, c.MarketDataURL+"/lite/v1/ticker", req, false)
	if err != nil {
		return nil, err
	}

	var result GetTickerResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetTrade(ctx context.Context, instrument string, limit int) (*GetTradeResponse, error) {
	req := GetTradeRequest{
		Instrument: instrument,
	}
	resp, err := c.Post(ctx, c.MarketDataURL+"/lite/v1/trade", req, false)
	if err != nil {
		return nil, err
	}

	var result GetTradeResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetKLine(ctx context.Context, instrument string, interval string, klineType string, startTime *string, endTime *string, limit *int64, cursor *string) (*GetKLineResponse, error) {
	req := GetKLineRequest{
		Instrument: instrument,
		Interval:   interval,
		KlineType:  klineType,
		StartTime:  startTime,
		EndTime:    endTime,
		Limit:      limit,
		Cursor:     cursor,
	}
	resp, err := c.Post(ctx, c.MarketDataURL+"/lite/v1/kline", req, false)
	if err != nil {
		return nil, err
	}

	var result GetKLineResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetHistoricalFundingRate retrieves historical funding rate data
// Renamed from GetFundingRate to clarify it queries historical data
func (c *Client) GetHistoricalFundingRate(ctx context.Context, instrument string, startTime *string, endTime *string, limit *int64, cursor *string, aggType *string) (*GetFundingRateResponse, error) {
	req := GetFundingRateRequest{
		Instrument: instrument,
		StartTime:  startTime,
		EndTime:    endTime,
		Limit:      limit,
		Cursor:     cursor,
		AggType:    aggType,
	}
	resp, err := c.Post(ctx, c.MarketDataURL+"/lite/v1/funding", req, false)
	if err != nil {
		return nil, err
	}

	var result GetFundingRateResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetFundingRate retrieves the current real-time funding rate for a specific instrument
// Returns per-hour funding rate (converted from the settlement interval rate)
func (c *Client) GetFundingRate(ctx context.Context, instrument string) (*FundingRateData, error) {
	// First, get instrument info to find funding interval
	instruments, err := c.GetInstruments(ctx)
	if err != nil {
		return nil, err
	}

	// Find the instrument and get its funding interval
	var fundingIntervalHours int64
	found := false
	for _, inst := range instruments {
		if inst.Instrument == instrument {
			fundingIntervalHours = inst.FundingIntervalHours
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("instrument not found: %s", instrument)
	}

	// Get ticker data
	ticker, err := c.GetTicker(ctx, instrument)
	if err != nil {
		return nil, err
	}

	// Convert funding rate to per-hour rate
	hourlyRate, err := convertToHourlyRate(ticker.Result.FundingRate, fundingIntervalHours)
	if err != nil {
		return nil, fmt.Errorf("failed to convert funding rate: %w", err)
	}

	// Calculate funding time from next funding time - interval
	fundingTime := ""
	if ticker.Result.NextFundingTime != "" {
		nextTime, err := strconv.ParseInt(ticker.Result.NextFundingTime, 10, 64)
		if err == nil {
			// Subtract interval in nanoseconds (NextFundingTime appears to be in nanoseconds)
			intervalNs := fundingIntervalHours * 3600 * 1000000000
			currentTime := nextTime - intervalNs
			fundingTime = strconv.FormatInt(currentTime, 10)
		}
	}

	return &FundingRateData{
		Instrument:           ticker.Result.Instrument,
		FundingRate:          hourlyRate,
		FundingIntervalHours: fundingIntervalHours,
		FundingTime:          fundingTime,
		NextFundingTime:      ticker.Result.NextFundingTime,
	}, nil
}

// GetAllFundingRates retrieves real-time funding rates for all instruments
// Returns per-hour funding rates (converted from settlement interval rates)
func (c *Client) GetAllFundingRates(ctx context.Context) ([]FundingRateData, error) {
	instruments, err := c.GetInstruments(ctx)
	if err != nil {
		return nil, err
	}

	var result []FundingRateData
	for _, inst := range instruments {
		// Only query perpetual futures (which have funding rates)
		if inst.Kind != "PERPETUAL" {
			continue
		}

		// Get funding rate data for this instrument (already converted to hourly)
		fundingData, err := c.GetFundingRate(ctx, inst.Instrument)
		if err != nil {
			// Continue with other instruments if one fails
			continue
		}

		result = append(result, *fundingData)
	}

	return result, nil
}

// convertToHourlyRate converts a period funding rate to per-hour rate
func convertToHourlyRate(periodRate string, intervalHours int64) (string, error) {
	if intervalHours == 0 {
		return "", fmt.Errorf("invalid interval hours: 0")
	}

	rate, err := strconv.ParseFloat(periodRate, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse rate: %w", err)
	}

	hourlyRate := rate / float64(intervalHours)
	return fmt.Sprintf("%.10f", hourlyRate), nil
}
