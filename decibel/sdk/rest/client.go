package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

const (
	MainnetBaseURL = "https://api.mainnet.aptoslabs.com/decibel"
	TestnetBaseURL = "https://api.testnet.aptoslabs.com/decibel"
)

type Client struct {
	BaseURL    string
	Origin     string
	APIKey     string
	HTTPClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL:    MainnetBaseURL,
		Origin:     originForBaseURL(MainnetBaseURL),
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) get(ctx context.Context, path string, query url.Values, dest any) error {
	u, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + path)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Origin", c.origin())

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return normalizeError(resp.StatusCode, body)
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) origin() string {
	if c.Origin != "" {
		return c.Origin
	}
	return originForBaseURL(c.BaseURL)
}

func originForBaseURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return baseURL
	}
	return u.Scheme + "://" + u.Host
}

func normalizeError(statusCode int, body []byte) error {
	apiErr := parseAPIError(body)
	code := strings.ToUpper(strings.TrimSpace(apiErr.Code))
	message := strings.TrimSpace(apiErr.Message)
	if message == "" {
		message = strings.TrimSpace(apiErr.ErrorMessage)
	}
	if message == "" {
		message = strings.TrimSpace(string(body))
	}

	lowerMessage := strings.ToLower(message)
	var sentinel error

	switch {
	case statusCode == http.StatusUnauthorized, statusCode == http.StatusForbidden,
		strings.Contains(code, "AUTH"), strings.Contains(lowerMessage, "auth"), strings.Contains(lowerMessage, "token"):
		sentinel = exchanges.ErrAuthFailed
	case statusCode == http.StatusTooManyRequests,
		strings.Contains(code, "RATE"), strings.Contains(lowerMessage, "rate limit"), strings.Contains(lowerMessage, "slow down"):
		sentinel = exchanges.ErrRateLimited
	case strings.Contains(code, "ORDER_NOT_FOUND"), strings.Contains(lowerMessage, "order not found"):
		sentinel = exchanges.ErrOrderNotFound
	case strings.Contains(code, "MARKET_NOT_FOUND"), strings.Contains(code, "SYMBOL_NOT_FOUND"),
		strings.Contains(lowerMessage, "market not found"), strings.Contains(lowerMessage, "symbol not found"):
		sentinel = exchanges.ErrSymbolNotFound
	case strings.Contains(code, "PRECISION"), strings.Contains(lowerMessage, "precision"),
		strings.Contains(lowerMessage, "tick size"), strings.Contains(lowerMessage, "decimal"):
		sentinel = exchanges.ErrInvalidPrecision
	}

	return exchanges.NewExchangeError("DECIBEL", code, message, sentinel)
}

func parseAPIError(body []byte) APIError {
	var apiErr APIError
	_ = json.Unmarshal(body, &apiErr)
	return apiErr
}
