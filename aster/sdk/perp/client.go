package perp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"go.uber.org/zap"

)

const (
	BaseURL = "https://fapi.asterdex.com"
)

type Client struct {
	BaseURL    string
	APIKey     string
	SecretKey  string
	HTTPClient *http.Client
	Logger     *zap.SugaredLogger
	Debug      bool
}

func NewClient() *Client {
	httpClient := &http.Client{}
	l := zap.NewNop().Sugar().Named("aster-perp")

	// Check for proxy in environment
	proxyURL := os.Getenv("PROXY")

	if proxyURL != "" {
		parsedURL, err := url.Parse(proxyURL)
		if err == nil {
			httpClient.Transport = &http.Transport{
				Proxy: http.ProxyURL(parsedURL),
			}
			if os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1" {
				l.Debugw("Using proxy", "url", proxyURL)
			}
		} else {
			l.Warnw("Invalid proxy URL", "url", proxyURL, "error", err)
		}
	}

	return &Client{
		BaseURL:    BaseURL,
		HTTPClient: httpClient,
		Logger:     l,
		Debug:      os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1",
	}
}

func (c *Client) WithCredentials(apiKey, secretKey string) *Client {
	c.APIKey = apiKey
	c.SecretKey = secretKey
	return c
}

func (c *Client) WithBaseURL(url string) *Client {
	c.BaseURL = url
	return c
}

func (c *Client) WithDebug(debug bool) *Client {
	c.Debug = debug
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
		q.Add("timestamp", fmt.Sprintf("%d", Timestamp()))
		queryString := q.Encode()
		sig := GenerateSignature(c.SecretKey, queryString)
		u.RawQuery = queryString + "&signature=" + sig
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

	if c.Debug {
		c.Logger.Debugw("Request", "method", method, "url", u.String())
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

	if c.Debug {
		c.Logger.Debugw("Response", "body", string(data))
	}

	if resp.StatusCode >= 400 {
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

func (c *Client) Put(ctx context.Context, endpoint string, params map[string]interface{}, signed bool, result interface{}) error {
	return c.call(ctx, http.MethodPut, endpoint, params, signed, result)
}
