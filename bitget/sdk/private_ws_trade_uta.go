package sdk

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type utaTradeRequest struct {
	Op       string `json:"op"`
	ID       string `json:"id"`
	Category string `json:"category"`
	Topic    string `json:"topic"`
	Args     []any  `json:"args"`
}

type utaTradeResponse struct {
	Event    string `json:"event"`
	ID       string `json:"id"`
	Category string `json:"category"`
	Topic    string `json:"topic"`
	Args     []struct {
		Symbol    string `json:"symbol"`
		OrderID   string `json:"orderId"`
		ClientOID string `json:"clientOid"`
		CTime     string `json:"cTime"`
	} `json:"args"`
	Code NumberString `json:"code"`
	Msg  string       `json:"msg"`
}

func (c *PrivateWSClient) PlaceOrderUTAWS(req *PlaceOrderRequest) (*PlaceOrderResponse, error) {
	resp, err := c.sendUTATradeRequest("place-order", req.Category, map[string]any{
		"symbol":      req.Symbol,
		"orderType":   req.OrderType,
		"qty":         req.Qty,
		"side":        req.Side,
		"price":       req.Price,
		"timeInForce": req.TimeInForce,
		"reduceOnly":  req.ReduceOnly,
		"clientOid":   req.ClientOID,
	})
	if err != nil {
		return nil, err
	}
	return &PlaceOrderResponse{
		OrderID:   firstTradeArg(resp).OrderID,
		ClientOID: firstTradeArg(resp).ClientOID,
	}, nil
}

func (c *PrivateWSClient) CancelOrderUTAWS(req *CancelOrderRequest) (*CancelOrderResponse, error) {
	resp, err := c.sendUTATradeRequest("cancel-order", req.Category, map[string]any{
		"symbol":    req.Symbol,
		"orderId":   req.OrderID,
		"clientOid": req.ClientOID,
	})
	if err != nil {
		return nil, err
	}
	return &CancelOrderResponse{
		OrderID:   firstTradeArg(resp).OrderID,
		ClientOID: firstTradeArg(resp).ClientOID,
	}, nil
}

func (c *PrivateWSClient) ModifyOrderUTAWS(req *ModifyOrderRequest) (*CancelOrderResponse, error) {
	resp, err := c.sendUTATradeRequest("modify-order", req.Category, map[string]any{
		"symbol":       req.Symbol,
		"orderId":      req.OrderID,
		"clientOid":    req.ClientOID,
		"newQty":       req.NewQty,
		"newPrice":     req.NewPrice,
		"newClientOid": req.NewClientID,
	})
	if err != nil {
		return nil, err
	}
	return &CancelOrderResponse{
		OrderID:   firstTradeArg(resp).OrderID,
		ClientOID: firstTradeArg(resp).ClientOID,
	}, nil
}

func (c *PrivateWSClient) sendUTATradeRequest(topic, category string, payload map[string]any) (*utaTradeResponse, error) {
	pruned := make(map[string]any, len(payload))
	for key, value := range payload {
		switch v := value.(type) {
		case string:
			if v != "" {
				pruned[key] = v
			}
		default:
			if value != nil {
				pruned[key] = value
			}
		}
	}

	id := strconv.FormatInt(time.Now().UnixNano(), 10)
	req := utaTradeRequest{
		Op:       "trade",
		ID:       id,
		Category: strings.ToLower(category),
		Topic:    topic,
		Args:     []any{pruned},
	}

	respBytes, err := c.sendRequest(id, req)
	if err != nil {
		return nil, err
	}

	var resp utaTradeResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	if !isWSSuccessCode(resp.Code) {
		return nil, fmt.Errorf("bitget private ws: %s failed: %s %s", topic, resp.Code, resp.Msg)
	}
	return &resp, nil
}

func firstTradeArg(resp *utaTradeResponse) struct {
	Symbol    string `json:"symbol"`
	OrderID   string `json:"orderId"`
	ClientOID string `json:"clientOid"`
	CTime     string `json:"cTime"`
} {
	if resp == nil || len(resp.Args) == 0 {
		return struct {
			Symbol    string `json:"symbol"`
			OrderID   string `json:"orderId"`
			ClientOID string `json:"clientOid"`
			CTime     string `json:"cTime"`
		}{}
	}
	return resp.Args[0]
}

func isWSSuccessCode(code NumberString) bool {
	switch strings.TrimSpace(string(code)) {
	case "", "0", "00000":
		return true
	default:
		return false
	}
}
