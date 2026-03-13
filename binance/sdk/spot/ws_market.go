package spot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

type WsMarketClient struct {
	*WsClient
}

func NewWsMarketClient(ctx context.Context) *WsMarketClient {
	client := &WsMarketClient{
		WsClient: NewWsClient(ctx, WSBaseURL),
	}
	client.WsClient.Logger = zap.NewNop().Sugar().Named("binance-spot-market")
	client.Handler = client.handleMessage
	return client
}

func (c *WsMarketClient) handleMessage(message []byte) {
	c.Logger.Debugw("Received", "msg", string(message))

	// trim space
	message = bytes.TrimSpace(message)
	if len(message) == 0 {
		return
	}

	if message[0] == '[' {
		c.handleArrayMessage(message)
	} else {
		c.handleObjectMessage(message)
	}
}

func (c *WsMarketClient) handleArrayMessage(message []byte) {
	var events []struct {
		EventType interface{} `json:"e"`
		EventTime int64       `json:"E"`
	}
	if err := json.Unmarshal(message, &events); err != nil {
		c.Logger.Errorw("error unmarshalling array message", "error", err, "raw_msg", string(message))
		return
	}

	if len(events) == 0 {
		return
	}

	// use first event type
	var eventType string
	switch v := events[0].EventType.(type) {
	case string:
		eventType = v
	case float64:
		// Some events might send integer as number?
		eventType = fmt.Sprintf("%.0f", v)
	default:
		c.Logger.Debugw("unknown event type format", "type", fmt.Sprintf("%T", events[0].EventType))
		return
	}

	if eventType == "" {
		c.Logger.Debug("event type not found in array message")
		return
	}

	key := fmt.Sprintf("!%s@arr", eventType)
	if mappedKey, ok := ArrayEventMap[eventType]; ok {
		key = mappedKey
	}
	c.CallSubscription(key, message)
}

func (c *WsMarketClient) handleObjectMessage(message []byte) {
	var event struct {
		EventType string `json:"e"`
		EventTime int64  `json:"E"`
		Symbol    string `json:"s"`
		// kline specific
		Kline struct {
			Interval string `json:"i"`
		} `json:"k"`
	}
	if err := json.Unmarshal(message, &event); err != nil {
		c.Logger.Errorw("error unmarshalling object message", "error", err)
		return
	}

	// collect all potential keys
	var keys []string

	eventName, ok := SingleEventMap[event.EventType]
	if ok {
		stream := fmt.Sprintf("%s@%s", strings.ToLower(event.Symbol), eventName)
		keys = append(keys, stream)
	}

	// special handle !bookTicker
	if event.EventType == "bookTicker" {
		keys = append(keys, "!bookTicker")
	}

	// special handle kline
	if event.EventType == "kline" && event.Symbol != "" && event.Kline.Interval != "" {
		stream := fmt.Sprintf("%s@kline_%s", strings.ToLower(event.Symbol), event.Kline.Interval)
		keys = append(keys, stream)
	}

	// dispatch
	for _, key := range keys {
		c.CallSubscription(key, message)
	}

	if len(keys) == 0 {
		c.Logger.Debugw("No routing keys generated for event", "msg", string(message))
	}
}

// SubscribeDepth
func (c *WsMarketClient) SubscribeIncrementOrderBook(symbol string, interval string, callback func(*WsDepthEvent) error) error {
	channel := fmt.Sprintf("%s@depth@%s", symbol, interval)
	return c.Subscribe(channel, func(data []byte) error {
		var wsData WsDepthEvent
		if err := json.Unmarshal(data, &wsData); err != nil {
			return err
		}
		return callback(&wsData)
	})
}

// SubscribeLimitOrderBook (Partial Depth)
func (c *WsMarketClient) SubscribeLimitOrderBook(symbol string, levels int, interval string, callback func(*DepthEvent) error) error {
	// interval: 100ms or 1000ms. default 100ms usually.
	if interval == "" {
		interval = "100ms"
	}
	channel := fmt.Sprintf("%s@depth%d@%s", strings.ToLower(symbol), levels, interval)
	return c.Subscribe(channel, func(data []byte) error {
		var wsData DepthEvent
		if err := json.Unmarshal(data, &wsData); err != nil {
			return err
		}
		return callback(&wsData)
	})
}

// SubscribeBookTicker
func (c *WsMarketClient) SubscribeBookTicker(symbol string, callback func(*BookTickerEvent) error) error {
	channel := fmt.Sprintf("%s@bookTicker", strings.ToLower(symbol))
	return c.Subscribe(channel, func(data []byte) error {
		var wsData BookTickerEvent
		if err := json.Unmarshal(data, &wsData); err != nil {
			return err
		}
		return callback(&wsData)
	})
}

// SubscribeAggTrade
func (c *WsMarketClient) SubscribeAggTrade(symbol string, callback func(*AggTradeEvent) error) error {
	channel := fmt.Sprintf("%s@aggTrade", strings.ToLower(symbol))
	return c.Subscribe(channel, func(data []byte) error {
		var wsData AggTradeEvent
		if err := json.Unmarshal(data, &wsData); err != nil {
			return err
		}
		return callback(&wsData)
	})
}

// SubscribeTrade
func (c *WsMarketClient) SubscribeTrade(symbol string, callback func(*TradeEvent) error) error {
	channel := fmt.Sprintf("%s@trade", strings.ToLower(symbol))
	return c.Subscribe(channel, func(data []byte) error {
		var wsData TradeEvent
		if err := json.Unmarshal(data, &wsData); err != nil {
			return err
		}
		return callback(&wsData)
	})
}

// SubscribeKline
func (c *WsMarketClient) SubscribeKline(symbol string, interval string, callback func(*KlineEvent) error) error {
	channel := fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), interval)
	return c.Subscribe(channel, func(data []byte) error {
		var wsData KlineEvent
		if err := json.Unmarshal(data, &wsData); err != nil {
			return err
		}
		return callback(&wsData)
	})
}

// Unsubscribe methods

func (c *WsMarketClient) UnsubscribeDepth(symbol string) error {
	channel := fmt.Sprintf("%s@depth", strings.ToLower(symbol))
	return c.Unsubscribe(channel)
}

func (c *WsMarketClient) UnsubscribeLimitOrderBook(symbol string, levels int, interval string) error {
	if interval == "" {
		interval = "100ms"
	}
	channel := fmt.Sprintf("%s@depth%d@%s", strings.ToLower(symbol), levels, interval)
	return c.Unsubscribe(channel)
}

func (c *WsMarketClient) UnsubscribeBookTicker(symbol string) error {
	channel := fmt.Sprintf("%s@bookTicker", strings.ToLower(symbol))
	return c.Unsubscribe(channel)
}

func (c *WsMarketClient) UnsubscribeAggTrade(symbol string) error {
	channel := fmt.Sprintf("%s@aggTrade", strings.ToLower(symbol))
	return c.Unsubscribe(channel)
}

func (c *WsMarketClient) UnsubscribeTrade(symbol string) error {
	channel := fmt.Sprintf("%s@trade", strings.ToLower(symbol))
	return c.Unsubscribe(channel)
}

func (c *WsMarketClient) UnsubscribeKline(symbol string, interval string) error {
	channel := fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), interval)
	return c.Unsubscribe(channel)
}

func (c *WsMarketClient) SubscribeAllMiniTicker(callback func([]*WsMiniTickerEvent) error) error {
	channel := "!miniTicker@arr"
	return c.Subscribe(channel, func(data []byte) error {
		var wsData []*WsMiniTickerEvent
		if err := json.Unmarshal(data, &wsData); err != nil {
			return err
		}
		return callback(wsData)
	})
}
