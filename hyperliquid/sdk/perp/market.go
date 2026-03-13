package perp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// L2Book

type L2BookResponse struct {
	Coin   string      `json:"coin"`
	Levels [][]L2Level `json:"levels"`
	Time   int64       `json:"time"`
}

type L2Level struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int    `json:"n"`
}

func (c *Client) L2Book(ctx context.Context, coin string) (*L2BookResponse, error) {
	req := map[string]string{
		"type": "l2Book",
		"coin": coin,
	}
	data, err := c.Post(ctx, "/info", req)
	if err != nil {
		return nil, err
	}
	var res L2BookResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

// AllMids

func (c *Client) AllMids(ctx context.Context) (map[string]string, error) {
	req := map[string]string{
		"type": "allMids",
	}
	data, err := c.Post(ctx, "/info", req)
	if err != nil {
		return nil, err
	}
	var res map[string]string
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// CandleSnapshot

type Candle struct {
	T      int64  `json:"t"` // Open time
	TClose int64  `json:"T"` // Close time
	S      string `json:"s"` // Symbol
	I      string `json:"i"` // Interval
	O      string `json:"o"` // Open
	C      string `json:"c"` // Close
	H      string `json:"h"` // High
	L      string `json:"l"` // Low
	V      string `json:"v"` // Volume
	N      int64  `json:"n"` // Number of trades
}

type CandleSnapshotRequest struct {
	Coin      string `json:"coin"`
	Interval  string `json:"interval"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
}

func (c *Client) CandleSnapshot(ctx context.Context, coin string, interval string, startTime, endTime int64) ([]Candle, error) {
	data, err := c.Post(ctx, "/info", map[string]any{
		"type": "candleSnapshot",
		"req": CandleSnapshotRequest{
			Coin:      coin,
			Interval:  interval,
			StartTime: startTime,
			EndTime:   endTime,
		},
	})
	if err != nil {
		return nil, err
	}
	var res []Candle
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetPrepMeta (Metadata)

func (c *Client) GetPrepMeta(ctx context.Context) (*PrepMeta, error) {
	data, err := c.Post(ctx, "/info", map[string]string{
		"type": "meta",
	})
	if err != nil {
		return nil, err
	}
	var res PrepMeta
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// GetFundingRate retrieves the current funding rate for a specific coin.
// It uses the metaAndAssetCtxs endpoint which provides real-time asset contexts
// including mark price, funding rate, open interest, etc.
func (c *Client) GetFundingRate(ctx context.Context, coin string) (*FundingRate, error) {
	// Request metaAndAssetCtxs to get all asset contexts
	req := map[string]string{
		"type": "metaAndAssetCtxs",
	}
	data, err := c.Post(ctx, "/info", req)
	if err != nil {
		return nil, err
	}

	// The response is an array of two elements:
	// [0] is meta (contains universe array with coin names)
	// [1] is the array of asset contexts (funding rates, etc.)
	// The indices match: universe[i] corresponds to assetCtxs[i]
	var response []json.RawMessage
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	if len(response) < 2 {
		return nil, fmt.Errorf("unexpected response format: expected 2 elements, got %d", len(response))
	}

	// Parse the meta object to get universe
	var meta struct {
		Universe []struct {
			Name string `json:"name"`
		} `json:"universe"`
	}
	if err := json.Unmarshal(response[0], &meta); err != nil {
		return nil, err
	}

	// Parse the asset contexts array
	var assetCtxs MetaAndAssetCtxsResponse
	if err := json.Unmarshal(response[1], &assetCtxs); err != nil {
		return nil, err
	}

	// Match coin by index
	for i, uni := range meta.Universe {
		if uni.Name == coin {
			if i >= len(assetCtxs) {
				return nil, fmt.Errorf("asset context not found for coin: %s", coin)
			}

			// Calculate funding times (1-hour interval)
			now := time.Now().UTC()
			fundingTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)
			nextFundingTime := fundingTime.Add(1 * time.Hour)

			return &FundingRate{
				Coin:                 coin,
				FundingRate:          assetCtxs[i].Funding,
				FundingIntervalHours: 1,
				FundingTime:          fundingTime.UnixMilli(),
				NextFundingTime:      nextFundingTime.UnixMilli(),
			}, nil
		}
	}

	return nil, fmt.Errorf("funding rate not found for coin: %s", coin)
}

// GetAllFundingRates retrieves funding rates for all available coins.
// Returns a map where keys are coin names (e.g., "BTC", "ETH") and values are funding rates.
func (c *Client) GetAllFundingRates(ctx context.Context) (map[string]string, error) {
	// Request metaAndAssetCtxs to get all asset contexts
	req := map[string]string{
		"type": "metaAndAssetCtxs",
	}
	data, err := c.Post(ctx, "/info", req)
	if err != nil {
		return nil, err
	}

	// The response is an array of two elements:
	// [0] is meta (contains universe array with coin names)
	// [1] is the array of asset contexts (funding rates, etc.)
	// The indices match: universe[i] corresponds to assetCtxs[i]
	var response []json.RawMessage
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	if len(response) < 2 {
		return nil, fmt.Errorf("unexpected response format: expected 2 elements, got %d", len(response))
	}

	// Parse the meta object to get universe
	var meta struct {
		Universe []struct {
			Name string `json:"name"`
		} `json:"universe"`
	}
	if err := json.Unmarshal(response[0], &meta); err != nil {
		return nil, err
	}

	// Parse the asset contexts array
	var assetCtxs MetaAndAssetCtxsResponse
	if err := json.Unmarshal(response[1], &assetCtxs); err != nil {
		return nil, err
	}

	// Build the map of coin name to funding rate
	result := make(map[string]string)
	for i, uni := range meta.Universe {
		if i < len(assetCtxs) {
			result[uni.Name] = assetCtxs[i].Funding
		}
	}

	return result, nil
}
