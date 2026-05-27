package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBaseURL = "https://www.deribit.com"

type Client struct {
	baseURL     string
	apiKey      string
	secretKey   string
	accessToken string
	tokenExpiry time.Time
	HTTPClient  *http.Client
}

func NewClient() *Client {
	return &Client{
		baseURL: defaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = baseURL
	return c
}

func (c *Client) WithCredentials(apiKey, secretKey string) *Client {
	c.apiKey = apiKey
	c.secretKey = secretKey
	return c
}

func (c *Client) HasCredentials() bool {
	return c != nil && c.apiKey != "" && c.secretKey != ""
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e rpcError) Error() string {
	return fmt.Sprintf("deribit sdk: %d %s", e.Code, e.Message)
}

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

func (c *Client) get(ctx context.Context, path string, query map[string]string, out any) error {
	return c.doGet(ctx, path, query, "", out)
}

func (c *Client) privateGet(ctx context.Context, path string, query map[string]string, out any) error {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return err
	}
	return c.doGet(ctx, path, query, c.accessToken, out)
}

func (c *Client) ensureAuthenticated(ctx context.Context) error {
	if !c.HasCredentials() {
		return fmt.Errorf("deribit sdk: private endpoint requires client_id and client_secret")
	}
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-30*time.Second)) {
		return nil
	}
	var auth AuthResult
	if err := c.doGet(ctx, "/api/v2/public/auth", map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     c.apiKey,
		"client_secret": c.secretKey,
	}, "", &auth); err != nil {
		return err
	}
	if auth.AccessToken == "" {
		return fmt.Errorf("deribit sdk: auth response missing access_token")
	}
	c.accessToken = auth.AccessToken
	expires := auth.ExpiresIn
	if expires <= 0 {
		expires = 300
	}
	c.tokenExpiry = time.Now().Add(time.Duration(expires) * time.Second)
	return nil
}

func (c *Client) doGet(ctx context.Context, path string, query map[string]string, bearerToken string, out any) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	q := u.Query()
	for key, value := range query {
		if value != "" {
			q.Set(key, value)
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("deribit sdk: GET %s returned %s", path, resp.Status)
	}

	var envelope rpcEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return *envelope.Error
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Result, out)
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}
