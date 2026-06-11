package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const defaultWSURL = "wss://ws.backpack.exchange"

type wsSubscribeRequest struct {
	Method    string   `json:"method"`
	Params    []string `json:"params"`
	Signature []string `json:"signature,omitempty"`
}

type WSClient struct {
	url        string
	apiKey     string
	privateKey string

	mu       sync.RWMutex
	writeMu  sync.Mutex
	conn     *websocket.Conn
	handlers map[string]func(json.RawMessage)
}

func NewWSClient() *WSClient {
	return &WSClient{
		url:      defaultWSURL,
		handlers: make(map[string]func(json.RawMessage)),
	}
}

func (c *WSClient) WithCredentials(apiKey, privateKey string) *WSClient {
	c.apiKey = apiKey
	c.privateKey = privateKey
	return c
}

func (c *WSClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil)
	if err != nil {
		return err
	}
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})
	c.conn = conn

	go c.readLoop(conn)
	return nil
}

func (c *WSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}
	_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(5*time.Second))
	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *WSClient) Subscribe(ctx context.Context, stream string, private bool, handler func(json.RawMessage)) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}

	req := wsSubscribeRequest{
		Method: "SUBSCRIBE",
		Params: []string{stream},
	}
	if private {
		timestamp := time.Now().UnixMilli()
		payload := buildSigningPayload("subscribe", nil, timestamp, defaultRecvWindow)
		signature, err := signPayload(c.privateKey, payload)
		if err != nil {
			return err
		}
		req.Signature = []string{
			c.apiKey,
			signature,
			strconv.FormatInt(timestamp, 10),
			strconv.FormatInt(defaultRecvWindow, 10),
		}
	}

	c.mu.Lock()
	c.handlers[stream] = handler
	c.mu.Unlock()

	if err := c.writeJSON(req); err != nil {
		c.mu.Lock()
		delete(c.handlers, stream)
		c.mu.Unlock()
		return err
	}
	return nil
}

func (c *WSClient) Unsubscribe(ctx context.Context, stream string) error {
	_ = ctx

	c.mu.Lock()
	delete(c.handlers, stream)
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil
	}

	return c.writeJSON(wsSubscribeRequest{
		Method: "UNSUBSCRIBE",
		Params: []string{stream},
	})
}

func (c *WSClient) writeJSON(v any) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("backpack ws: not connected")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteJSON(v)
}

func (c *WSClient) readLoop(conn *websocket.Conn) {
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			if c.conn == conn {
				c.conn = nil
			}
			c.mu.Unlock()
			return
		}

		var env StreamEnvelope
		if err := json.Unmarshal(payload, &env); err != nil {
			continue
		}
		if env.Stream == "" {
			continue
		}

		c.mu.RLock()
		handler := c.handlers[env.Stream]
		c.mu.RUnlock()
		if handler != nil {
			handler(env.Data)
		}
	}
}
