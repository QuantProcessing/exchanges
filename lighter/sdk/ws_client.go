package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"


	"github.com/gorilla/websocket"
)

const (
	MainnetWSURL = "wss://mainnet.zklighter.elliot.ai/stream"
)

type subscription struct {
	authToken *string
	handler   func([]byte)
}

type WebsocketClient struct {
	URL     string
	Conn    *websocket.Conn
	Mu      sync.RWMutex
	WriteMu sync.Mutex
	// Subscriptions maps channel name -> subscription (auth + handler)
	Subscriptions map[string]*subscription
	// PendingRequests maps request ID -> response channel for transaction tracking
	PendingRequests map[string]chan *TxResponse
	pendingMu       sync.RWMutex
	Logger          *zap.SugaredLogger

	// Reconnect logic
	ReconnectWait time.Duration

	// Error handling
	OnError func(error)

	ctx    context.Context
	cancel context.CancelFunc
}

func NewWebsocketClient(ctx context.Context) *WebsocketClient {
	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)

	return &WebsocketClient{
		URL:             MainnetWSURL,
		Subscriptions:   make(map[string]*subscription),
		PendingRequests: make(map[string]chan *TxResponse),
		Logger:          zap.NewNop().Sugar().Named("lighter"),
		ReconnectWait:   1 * time.Second,
		OnError:         func(err error) {},
		ctx:             ctx,
		cancel:          cancel,
	}
}

func (c *WebsocketClient) Connect() error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.Conn != nil {
		return nil
	}

	// Use internal 10 second timeout
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.URL, nil)
	if err != nil {
		return err
	}

	c.Conn = conn

	go c.readLoop()
	go c.pingLoop()

	return nil
}

func (c *WebsocketClient) Close() {
	// Cancel context to stop loops
	c.cancel()

	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.Conn != nil {
		c.Conn.Close()
		c.Conn = nil
	}
}

func (c *WebsocketClient) readLoop() {
	defer func() {
		// Clean up connection
		c.Mu.Lock()
		if c.Conn != nil {
			c.Conn.Close()
			c.Conn = nil
		}
		c.Mu.Unlock()

		// Trigger reconnect if not manually canceled
		if c.ctx.Err() == nil {
			c.reconnect()
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			c.Logger.Debug("Read loop stopping due to context cancellation")
			return
		default:
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				// Check if intentionally closed
				if c.ctx.Err() != nil {
					c.Logger.Debug("Read loop stopping due to context cancellation")
					return
				}
				// Log unexpected errors
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.Logger.Errorw("websocket unexpected close", "error", err)
				} else {
					c.Logger.Debugw("websocket read error", "error", err)
				}
				return
			}
			// Extend read deadline on any message received
			c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			c.HandleMessage(message)
		}
	}
}

func (c *WebsocketClient) pingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.WriteMu.Lock()
			if c.Conn != nil {
				err := c.Conn.WriteJSON(map[string]string{"type": "ping"})
				if err != nil {
					c.Logger.Errorw("websocket ping error", "error", err)
					c.WriteMu.Unlock()
					return
				}
				c.Logger.Debugw("websocket ping sent")
			}
			c.WriteMu.Unlock()
		}
	}
}

func (c *WebsocketClient) reconnect() {
	time.Sleep(c.ReconnectWait)
	c.Logger.Infow("reconnecting...")
	if err := c.Connect(); err != nil {
		c.Logger.Errorw("reconnect failed", "error", err)
		go c.reconnect()
		return
	}

	// After successful reconnect, re-subscribe all existing channels
	c.resubscribeAll()
}

// resubscribeAll re-sends subscribe requests for all stored subscriptions.
// It does NOT change the in-memory Subscriptions map; it only restores
// server-side state after reconnect.
func (c *WebsocketClient) resubscribeAll() {
	c.Mu.RLock()
	subsSnapshot := make(map[string]*subscription, len(c.Subscriptions))
	for ch, sub := range c.Subscriptions {
		subsSnapshot[ch] = sub
	}
	c.Mu.RUnlock()

	for ch, sub := range subsSnapshot {
		params := map[string]string{
			"channel": ch,
			"type":    "subscribe",
		}
		if sub.authToken != nil {
			params["auth"] = *sub.authToken
		}
		if err := c.Send(params); err != nil {
			c.Logger.Errorw("failed to resubscribe channel", "channel", ch, "error", err)
		} else {
			c.Logger.Infow("resubscribed channel", "channel", ch)
		}
	}
}

func (c *WebsocketClient) HandleMessage(message []byte) {
	c.Logger.Debugw("Received message", "msg", string(message))

	// Try to parse as transaction response first
	var txResp TxResponse
	if err := json.Unmarshal(message, &txResp); err == nil && txResp.ID != "" {
		c.pendingMu.RLock()
		if ch, ok := c.PendingRequests[txResp.ID]; ok {
			select {
			case ch <- &txResp:
				c.Logger.Debugw("delivered tx response", "id", txResp.ID, "code", txResp.Code)
			default:
				c.Logger.Warnw("tx response channel blocked", "id", txResp.ID)
			}
		} else {
			c.Logger.Warnw("tx response for unregistered ID", "id", txResp.ID, "code", txResp.Code, "msg", message)
		}
		c.pendingMu.RUnlock()
		return
	}

	// Otherwise, parse as channel-based message
	var msg struct {
		Channel string `json:"channel"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal(message, &msg); err != nil {
		c.Logger.Errorw("error unmarshaling message", "error", err)
		return
	}

	// Handle server heartbeat messages: {"type": "ping"}
	if msg.Type == "ping" {
		if err := c.Send(map[string]string{"type": "pong"}); err != nil {
			c.Logger.Errorw("failed to send pong", "error", err)
		} else {
			c.Logger.Debugw("sent pong in response to ping")
		}
		return
	}

	c.Mu.RLock()
	defer c.Mu.RUnlock()

	channel := strings.ReplaceAll(msg.Channel, ":", "/")
	if sub, ok := c.Subscriptions[channel]; ok {
		go sub.handler(message)
		return
	}
}

// Subscribe registers a handler for a channel.
func (c *WebsocketClient) Subscribe(channel string, authToken *string, handler func([]byte)) error {
	// Avoid duplicate logical subscriptions
	c.Mu.Lock()
	if _, ok := c.Subscriptions[channel]; ok {
		c.Mu.Unlock()
		return fmt.Errorf("duplicate subscription to channel %s", channel)
	}
	// Copy auth token so we don't hold pointer to caller's stack variable
	var tokenCopy *string
	if authToken != nil {
		t := *authToken
		tokenCopy = &t
	}
	c.Subscriptions[channel] = &subscription{
		authToken: tokenCopy,
		handler:   handler,
	}
	c.Mu.Unlock()

	params := map[string]string{
		"channel": channel,
		"type":    "subscribe",
	}
	if tokenCopy != nil {
		params["auth"] = *tokenCopy
	}
	return c.Send(params)
}

func (c *WebsocketClient) Unsubscribe(channel string) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	delete(c.Subscriptions, channel)

	// docs do not show this action
	return c.Send(map[string]string{
		"channel": channel,
		"type":    "unsubscribe",
	})
}

func (c *WebsocketClient) Send(v any) error {
	c.WriteMu.Lock()
	defer c.WriteMu.Unlock()

	if c.Conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	return c.Conn.WriteJSON(v)
}

// RegisterPendingRequest creates a response channel for a request ID with timeout
func (c *WebsocketClient) RegisterPendingRequest(id string) chan *TxResponse {
	ch := make(chan *TxResponse, 1)
	c.pendingMu.Lock()
	c.PendingRequests[id] = ch
	c.pendingMu.Unlock()
	return ch
}

// UnregisterPendingRequest removes a pending request
func (c *WebsocketClient) UnregisterPendingRequest(id string) {
	c.pendingMu.Lock()
	if ch, ok := c.PendingRequests[id]; ok {
		close(ch)
		delete(c.PendingRequests, id)
	}
	c.pendingMu.Unlock()
}
