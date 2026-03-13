
package spot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

)

const (
	BaseURL = "https://spot.edgex.exchange"
)

type Client struct {
	BaseURL         string
	starkPrivateKey string // Private Key
	AccountID       string // Account ID
	HTTPClient      *http.Client
	Logger          *zap.SugaredLogger
}

func NewClient() *Client {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Check for proxy in environment
	proxyURL := os.Getenv("PROXY")
	if proxyURL != "" {
		parsedURL, err := url.Parse(proxyURL)
		if err == nil {
			httpClient.Transport = &http.Transport{
				Proxy: http.ProxyURL(parsedURL),
			}
		}
	}

	return &Client{
		BaseURL:    BaseURL,
		HTTPClient: httpClient,
		Logger:     zap.NewNop().Sugar().Named("edgex-perp"),
	}
}

func (c *Client) WithCredentials(starkPrivateKey, accountID string) *Client {
	c.starkPrivateKey = starkPrivateKey
	c.AccountID = accountID
	return c
}

func (c *Client) call(ctx context.Context, method, endpoint string, params map[string]interface{}, signed bool, result interface{}) error {
	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return err
	}

	// Prepare Query Params
	q := u.Query()
	for k, v := range params {
		q.Add(k, fmt.Sprintf("%v", v))
	}

	var body io.Reader
	var bodyString string

	if method == http.MethodGet {
		u.RawQuery = q.Encode()
	} else {
		// For non-GET, we usually send JSON body
		if len(params) > 0 {
			jsonBytes, err := json.Marshal(params)
			if err != nil {
				return err
			}
			body = strings.NewReader(string(jsonBytes))
			bodyString = string(jsonBytes)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return err
	}

	if method != http.MethodGet && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if signed {
		timestamp := time.Now().UnixMilli()

		sig, err := GenerateSignature(c.starkPrivateKey, timestamp, method, endpoint, bodyString, params)
		if err != nil {
			return fmt.Errorf("failed to generate signature: %w", err)
		}

		req.Header.Set("X-edgeX-Api-Timestamp", fmt.Sprintf("%d", timestamp))
		req.Header.Set("X-edgeX-Api-Signature", sig)
	}

	c.Logger.Debugw("Request", "method", method, "url", u.String())
	if bodyString != "" {
		c.Logger.Debugw("Body", "content", bodyString)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	c.Logger.Debugw("Response", "body", string(data))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http error %d: %s", resp.StatusCode, string(data))
	}

	if result != nil {
		var apiResp APIResponse
		if err := json.Unmarshal(data, &apiResp); err != nil {
			// Try unmarshal directly to result if not wrapped
			if err := json.Unmarshal(data, result); err != nil {
				return fmt.Errorf("failed to unmarshal response: %w", err)
			}
			return nil
		}

		if apiResp.Code != "0" && apiResp.Code != "SUCCESS" && apiResp.Code != "" {
			return fmt.Errorf("api error %s: %s", apiResp.Code, apiResp.Message)
		}

		if len(apiResp.Data) > 0 {
			if err := json.Unmarshal(apiResp.Data, result); err != nil {
				return fmt.Errorf("failed to unmarshal data: %w", err)
			}
		} else {
			// Sometimes result is the whole response?
			// Let's assume standard format for now.
		}
	}

	return nil
}
