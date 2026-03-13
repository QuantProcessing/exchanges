package lighter

import (
	"context"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/lighter/sdk"
)

// TokenManager manages the lifecycle of Lighter API authentication tokens.
// It manages two types of tokens:
// 1. Read-Token: A long-lived API token used for WebSocket subscriptions (read-only).
// 2. Write-Token: A short-lived (8h) locally signed AuthToken used for write operations.
type TokenManager struct {
	client *lighter.Client

	// Tokens
	readToken  string
	writeToken string

	// State
	writeExpiry time.Time

	mu       sync.RWMutex
	stopChan chan struct{}
}

// NewTokenManager creates a new TokenManager
func NewTokenManager(client *lighter.Client, roToken string) *TokenManager {
	return &TokenManager{
		client:    client,
		readToken: roToken,
		stopChan:  make(chan struct{}),
	}
}

// Start initializes tokens and enters the refresh loop
func (tm *TokenManager) Start(ctx context.Context) error {

	// Initialize Write Token (Short-lived)
	if err := tm.refreshWriteToken(); err != nil {
		return err
	}

	// 3. Start refresh loop for Write Token
	go tm.refreshLoop(ctx)

	return nil
}

// Stop stops the background refresh loop
func (tm *TokenManager) Stop() {
	close(tm.stopChan)
}

// GetReadToken returns the long-lived read token for WS subscriptions.
func (tm *TokenManager) GetReadToken() (string, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.readToken == "" {
		// Fallback: try to get write token if read token is missing (should not happen after Start)
		return tm.writeToken, nil
	}
	return tm.readToken, nil
}

// GetWriteToken returns the short-lived write token.
// Refreshes if expired.
func (tm *TokenManager) GetWriteToken() (string, error) {
	tm.mu.RLock()
	token := tm.writeToken
	expiry := tm.writeExpiry
	tm.mu.RUnlock()

	if token == "" || time.Now().After(expiry) {
		if err := tm.refreshWriteToken(); err != nil {
			return "", err
		}
		tm.mu.RLock()
		token = tm.writeToken
		tm.mu.RUnlock()
	}

	return token, nil
}

// refreshWriteToken generates a new local write token (AuthToken)
func (tm *TokenManager) refreshWriteToken() error {
	// Lighter AuthTokens max 8 hours
	const tokenDuration = 8 * time.Hour

	expiry := time.Now().Add(tokenDuration)
	token, err := tm.client.CreateAuthToken(expiry)
	if err != nil {
		// TODO: logger.Error("Failed to refresh write token", "error", err)
		return err
	}

	tm.mu.Lock()
	tm.writeToken = token
	tm.writeExpiry = expiry
	tm.mu.Unlock()

	// TODO: logger.Info("Refreshed write token", "expiry", expiry)
	return nil
}

func (tm *TokenManager) refreshLoop(ctx context.Context) {
	// Refresh Write Token every 7 hours (valid for 8h)
	ticker := time.NewTicker(7 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tm.stopChan:
			return
		case <-ticker.C:
			if err := tm.refreshWriteToken(); err != nil {
				// TODO: logger.Error("Write token refresh failed in loop", "error", err)
			}
		}
	}
}
