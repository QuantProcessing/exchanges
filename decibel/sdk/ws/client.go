package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/gorilla/websocket"
)

const (
	MainnetWSURL = "wss://api.mainnet.aptoslabs.com/decibel/ws"
	TestnetWSURL = "wss://api.testnet.aptoslabs.com/decibel/ws"
)

type wsConn interface {
	ReadJSON(v any) error
	WriteJSON(v any) error
	Close() error
}

type dialFunc func(ctx context.Context, url string, header http.Header) (wsConn, error)

type Client struct {
	URL                  string
	Origin               string
	APIKey               string
	ReconnectWait        time.Duration
	PingInterval         time.Duration
	MaxReconnectAttempts int
	StatusNormalizer     func(string) exchanges.OrderStatus

	ctx    context.Context
	cancel context.CancelFunc

	mu                  sync.RWMutex
	writeMu             sync.Mutex
	conn                wsConn
	depthHandlers       map[string][]func(MarketDepthMessage)
	orderHandlers       map[string][]func(UserOrderHistoryMessage)
	orderUpdateHandlers map[string][]func(OrderUpdateMessage)
	tradeHandlers       map[string][]func(UserTradesMessage)
	positionHandlers    map[string][]func(UserPositionsMessage)
	subscriptionReqs    map[string]subscriptionRequest
	dial                dialFunc
}

func NewClient(parent context.Context, apiKey string) *Client {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)
	return &Client{
		URL:                  MainnetWSURL,
		Origin:               originForWSURL(MainnetWSURL),
		APIKey:               apiKey,
		ReconnectWait:        time.Second,
		PingInterval:         30 * time.Second,
		MaxReconnectAttempts: 3,
		StatusNormalizer:     NormalizeOrderStatus,
		ctx:                  ctx,
		cancel:               cancel,
		depthHandlers:        make(map[string][]func(MarketDepthMessage)),
		orderHandlers:        make(map[string][]func(UserOrderHistoryMessage)),
		orderUpdateHandlers:  make(map[string][]func(OrderUpdateMessage)),
		tradeHandlers:        make(map[string][]func(UserTradesMessage)),
		positionHandlers:     make(map[string][]func(UserPositionsMessage)),
		subscriptionReqs:     make(map[string]subscriptionRequest),
		dial:                 defaultDial,
	}
}

func (c *Client) Connect() error {
	conn, isNew, err := c.connectOnce()
	if err != nil {
		return err
	}
	if !isNew {
		return nil
	}

	if err := c.replaySubscriptions(conn); err != nil {
		_ = conn.Close()
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
		return err
	}

	go c.readLoop(conn)
	if c.PingInterval > 0 {
		go c.pingLoop(conn)
	}

	return nil
}

func (c *Client) Close() error {
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *Client) Subscribe(topic string, handler func(MarketDepthMessage)) error {
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("subscribe: topic is required")
	}

	req := subscriptionRequest{
		Subscribe: subscriptionTopic{Topic: topic},
	}

	c.mu.Lock()
	c.depthHandlers[topic] = append(c.depthHandlers[topic], handler)
	c.subscriptionReqs[topic] = req
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil
	}
	return c.writeJSON(conn, req)
}

func (c *Client) SubscribeUserOrderHistory(userAddr string, handler func(UserOrderHistoryMessage)) error {
	userAddr = strings.TrimSpace(userAddr)
	if userAddr == "" {
		return fmt.Errorf("subscribe user order history: user address is required")
	}

	topic := "user_order_history:" + userAddr
	req := subscriptionRequest{
		Subscribe: subscriptionTopic{Topic: topic},
	}

	c.mu.Lock()
	c.orderHandlers[topic] = append(c.orderHandlers[topic], handler)
	c.subscriptionReqs[topic] = req
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil
	}
	return c.writeJSON(conn, req)
}

func (c *Client) SubscribeOrderUpdates(userAddr string, handler func(OrderUpdateMessage)) error {
	userAddr = strings.TrimSpace(userAddr)
	if userAddr == "" {
		return fmt.Errorf("subscribe order updates: user address is required")
	}

	topic := "order_updates:" + userAddr
	req := subscriptionRequest{
		Subscribe: subscriptionTopic{Topic: topic},
	}

	c.mu.Lock()
	c.orderUpdateHandlers[topic] = append(c.orderUpdateHandlers[topic], handler)
	c.subscriptionReqs[topic] = req
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil
	}
	return c.writeJSON(conn, req)
}

func (c *Client) SubscribeUserTrades(userAddr string, handler func(UserTradesMessage)) error {
	userAddr = strings.TrimSpace(userAddr)
	if userAddr == "" {
		return fmt.Errorf("subscribe user trades: user address is required")
	}

	topic := "user_trades:" + userAddr
	req := subscriptionRequest{
		Subscribe: subscriptionTopic{Topic: topic},
	}

	c.mu.Lock()
	c.tradeHandlers[topic] = append(c.tradeHandlers[topic], handler)
	c.subscriptionReqs[topic] = req
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil
	}
	return c.writeJSON(conn, req)
}

func (c *Client) SubscribeUserPositions(userAddr string, handler func(UserPositionsMessage)) error {
	userAddr = strings.TrimSpace(userAddr)
	if userAddr == "" {
		return fmt.Errorf("subscribe user positions: user address is required")
	}

	topic := "user_positions:" + userAddr
	req := subscriptionRequest{
		Subscribe: subscriptionTopic{Topic: topic},
	}

	c.mu.Lock()
	c.positionHandlers[topic] = append(c.positionHandlers[topic], handler)
	c.subscriptionReqs[topic] = req
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil
	}
	return c.writeJSON(conn, req)
}

func (c *Client) connectOnce() (wsConn, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn, false, nil
	}

	conn, err := c.dial(c.ctx, c.URL, c.authHeaders())
	if err != nil {
		return nil, false, err
	}

	c.conn = conn
	return conn, true, nil
}

func (c *Client) authHeaders() http.Header {
	header := http.Header{}
	header.Set("Accept", "application/json")
	if strings.TrimSpace(c.APIKey) != "" {
		header.Set("Authorization", "Bearer "+strings.TrimSpace(c.APIKey))
	}
	header.Set("Origin", c.origin())
	return header
}

func (c *Client) origin() string {
	if strings.TrimSpace(c.Origin) != "" {
		return c.Origin
	}
	return originForWSURL(c.URL)
}

func (c *Client) readLoop(conn wsConn) {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		var raw json.RawMessage
		if err := conn.ReadJSON(&raw); err != nil {
			if c.ctx.Err() != nil {
				return
			}
			c.handleDisconnect(conn, err)
			return
		}

		c.dispatch(raw)
	}
}

func (c *Client) pingLoop(conn wsConn) {
	interval := c.PingInterval
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			current := c.conn
			c.mu.RUnlock()

			if current != conn {
				return
			}

			if err := c.writeJSON(conn, pingRequest{Type: "ping"}); err != nil {
				c.handleDisconnect(conn, err)
				return
			}
		}
	}
}

func (c *Client) handleDisconnect(conn wsConn, _ error) {
	c.mu.Lock()
	if c.conn != conn {
		c.mu.Unlock()
		return
	}
	_ = conn.Close()
	c.conn = nil
	c.mu.Unlock()

	c.reconnect()
}

func (c *Client) reconnect() {
	attempts := 0

	for {
		if c.ctx.Err() != nil {
			return
		}

		if c.MaxReconnectAttempts > 0 && attempts >= c.MaxReconnectAttempts {
			return
		}
		attempts++

		if c.ReconnectWait > 0 {
			timer := time.NewTimer(c.ReconnectWait)
			select {
			case <-c.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}

		conn, err := c.dial(c.ctx, c.URL, c.authHeaders())
		if err != nil {
			continue
		}

		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()

		if err := c.replaySubscriptions(conn); err != nil {
			_ = conn.Close()
			c.mu.Lock()
			if c.conn == conn {
				c.conn = nil
			}
			c.mu.Unlock()
			continue
		}

		go c.readLoop(conn)
		if c.PingInterval > 0 {
			go c.pingLoop(conn)
		}
		return
	}
}

func (c *Client) dispatch(raw json.RawMessage) {
	var envelope struct {
		Topic string `json:"topic"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return
	}

	switch {
	case envelope.Type == "pong":
		return
	case strings.HasPrefix(envelope.Topic, "depth:"):
		var msg MarketDepthMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		c.mu.RLock()
		handlers := append([]func(MarketDepthMessage){}, c.depthHandlers[msg.Topic]...)
		c.mu.RUnlock()
		for _, handler := range handlers {
			if handler != nil {
				handler(msg)
			}
		}
	case strings.HasPrefix(envelope.Topic, "user_order_history:"):
		var msg UserOrderHistoryMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		normalizer := c.statusNormalizer()
		for i := range msg.Orders {
			msg.Orders[i].NormalizedStatus = normalizer(msg.Orders[i].Status)
		}
		c.mu.RLock()
		handlers := append([]func(UserOrderHistoryMessage){}, c.orderHandlers[msg.Topic]...)
		c.mu.RUnlock()
		for _, handler := range handlers {
			if handler != nil {
				handler(msg)
			}
		}
	case strings.HasPrefix(envelope.Topic, "order_updates:"):
		var msg OrderUpdateMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		normalizer := c.statusNormalizer()
		if msg.Order.NormalizedStatus == "" || msg.Order.NormalizedStatus == exchanges.OrderStatusUnknown {
			msg.Order.NormalizedStatus = normalizer(msg.Order.Status)
			if msg.Order.NormalizedStatus == exchanges.OrderStatusUnknown {
				msg.Order.NormalizedStatus = normalizer(msg.Order.Order.Status)
			}
		}
		c.mu.RLock()
		handlers := append([]func(OrderUpdateMessage){}, c.orderUpdateHandlers[msg.Topic]...)
		c.mu.RUnlock()
		for _, handler := range handlers {
			if handler != nil {
				handler(msg)
			}
		}
	case strings.HasPrefix(envelope.Topic, "user_trades:"):
		var msg UserTradesMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		c.mu.RLock()
		handlers := append([]func(UserTradesMessage){}, c.tradeHandlers[msg.Topic]...)
		c.mu.RUnlock()
		for _, handler := range handlers {
			if handler != nil {
				handler(msg)
			}
		}
	case strings.HasPrefix(envelope.Topic, "user_positions:"):
		var msg UserPositionsMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		c.mu.RLock()
		handlers := append([]func(UserPositionsMessage){}, c.positionHandlers[msg.Topic]...)
		c.mu.RUnlock()
		for _, handler := range handlers {
			if handler != nil {
				handler(msg)
			}
		}
	}
}

func (c *Client) writeJSON(conn wsConn, payload any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteJSON(payload)
}

func (c *Client) replaySubscriptions(conn wsConn) error {
	c.mu.RLock()
	reqs := make([]subscriptionRequest, 0, len(c.subscriptionReqs))
	for _, req := range c.subscriptionReqs {
		reqs = append(reqs, req)
	}
	c.mu.RUnlock()

	for _, req := range reqs {
		if err := c.writeJSON(conn, req); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) statusNormalizer() func(string) exchanges.OrderStatus {
	if c.StatusNormalizer != nil {
		return c.StatusNormalizer
	}
	return NormalizeOrderStatus
}

func defaultDial(ctx context.Context, wsURL string, header http.Header) (wsConn, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func originForWSURL(wsURL string) string {
	parsed, err := url.Parse(wsURL)
	if err != nil {
		return wsURL
	}

	switch parsed.Scheme {
	case "wss":
		parsed.Scheme = "https"
	case "ws":
		parsed.Scheme = "http"
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return wsURL
	}

	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

var _ wsConn = (*websocket.Conn)(nil)
