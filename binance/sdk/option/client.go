package option

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/QuantProcessing/exchanges/internal/mbx"
)

const BaseURL = "https://eapi.binance.com"

type Client struct {
	BaseURL    string
	APIKey     string
	SecretKey  string
	HTTPClient *http.Client
	Logger     *zap.SugaredLogger

	UsedWeight mbx.UsedWeight
	OrderCount mbx.OrderCount
}

func NewClient() *Client {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	logger := zap.NewNop().Sugar().Named("binance-option")

	if proxyURL := os.Getenv("PROXY"); proxyURL != "" {
		parsedURL, err := url.Parse(proxyURL)
		if err == nil {
			httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(parsedURL)}
		} else {
			logger.Warnw("Invalid proxy URL", "url", proxyURL, "error", err)
		}
	}

	return &Client{
		BaseURL:    BaseURL,
		HTTPClient: httpClient,
		Logger:     logger,
	}
}

func (c *Client) WithCredentials(apiKey, secretKey string) *Client {
	c.APIKey = apiKey
	c.SecretKey = secretKey
	return c
}

func (c *Client) WithBaseURL(baseURL string) *Client {
	c.BaseURL = baseURL
	return c
}

func (c *Client) call(ctx context.Context, method, endpoint string, params map[string]interface{}, signed bool, result interface{}) error {
	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return err
	}

	q := u.Query()
	for k, v := range params {
		q.Add(k, fmt.Sprintf("%v", v))
	}
	if signed {
		if c.APIKey == "" || c.SecretKey == "" {
			return fmt.Errorf("binance option sdk: signed endpoint requires api key and secret key")
		}
		q.Add("timestamp", fmt.Sprintf("%d", Timestamp()))
		queryString := q.Encode()
		u.RawQuery = queryString + "&signature=" + GenerateSignature(c.SecretKey, queryString)
	} else {
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return err
	}
	if c.APIKey != "" {
		req.Header.Add("X-MBX-APIKEY", c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	c.UsedWeight.UpdateByHeader(resp.Header)
	c.OrderCount.UpdateByHeader(resp.Header)

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		if rlErr := mbx.MapAPIError("BINANCE", resp.StatusCode, data, func(d []byte) (int, string, error) {
			var apiErr APIError
			if err := json.Unmarshal(d, &apiErr); err != nil {
				return 0, "", err
			}
			return apiErr.Code, apiErr.Message, nil
		}); rlErr != nil {
			return rlErr
		}
		var apiErr APIError
		if err := json.Unmarshal(data, &apiErr); err != nil {
			return fmt.Errorf("http error %d: %s", resp.StatusCode, string(data))
		}
		return &apiErr
	}

	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

func (c *Client) Get(ctx context.Context, endpoint string, params map[string]interface{}, signed bool, result interface{}) error {
	return c.call(ctx, http.MethodGet, endpoint, params, signed, result)
}

func (c *Client) Post(ctx context.Context, endpoint string, params map[string]interface{}, signed bool, result interface{}) error {
	return c.call(ctx, http.MethodPost, endpoint, params, signed, result)
}

func (c *Client) Delete(ctx context.Context, endpoint string, params map[string]interface{}, signed bool, result interface{}) error {
	return c.call(ctx, http.MethodDelete, endpoint, params, signed, result)
}

func (c *Client) HasCredentials() bool {
	return c != nil && c.APIKey != "" && c.SecretKey != ""
}

func GenerateSignature(secretKey, data string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	_, _ = h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func Timestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
