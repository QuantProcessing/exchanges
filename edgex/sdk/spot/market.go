
package spot

import (
	"context"
	"errors"
	"net/http"
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

func (c *Client) GetTicker(ctx context.Context, instrumentId string) (*Ticker, error) {
	var res []Ticker
	err := c.call(ctx, http.MethodGet, "/api/v1/public/quote/getTicker", map[string]interface{}{"instrumentId": instrumentId}, false, &res)
	if len(res) == 0 {
		return nil, ErrInvalidParam
	}
	return &res[0], err
}

func (c *Client) GetOrderBook(ctx context.Context, instrumentId string, level int) (*OrderBook, error) {
	var res OrderBook
	err := c.call(ctx, http.MethodGet, "/api/v1/public/quote/getDepth", map[string]interface{}{"instrumentId": instrumentId, "level": level}, false, &res)
	return &res, err
}

func (c *Client) GetKline(ctx context.Context,
	instrumentId string, priceType PriceType, klineType KlineType, size int,
	offsetData, filterBeginKlineTimeInclusive, filterEndKlineTimeExclusive string) (*GetKlineResponse, error) {
	// check params
	if size <= 0 || size > 1000 {
		return nil, ErrInvalidParam
	}
	var res GetKlineResponse
	err := c.call(ctx, http.MethodGet, "/api/v1/public/quote/getKline", map[string]interface{}{
		"instrumentId":                  instrumentId,
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
