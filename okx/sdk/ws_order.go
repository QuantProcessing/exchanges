package okx

import (
	"encoding/json"
	"fmt"
	"time"
)

// WsOrderRequest represents the structure for placing/cancelling orders via WS.
type WsOrderOp struct {
	Id   string        `json:"id"` // Request ID
	Op   string        `json:"op"` // "order", "batch-orders", "cancel-order", "batch-cancel-orders"
	Args []interface{} `json:"args"`
}

// PlaceOrderWS places an order via WebSocket.
func (c *WsClient) PlaceOrderWS(req *OrderRequest) (*OrderId, error) {
	// User should handle tracking via clOrdId.

	// Use int64 for internal tracking
	idInt := time.Now().UnixNano()
	idStr := fmt.Sprintf("%d", idInt)

	op := WsOrderOp{
		Id:   idStr,
		Op:   "order",
		Args: []interface{}{req},
	}

	// Create channel for response
	successCh, errorCh := c.AddPendingRequest(idInt)
	defer c.RemovePendingRequest(idInt)

	c.WriteMu.Lock()
	err := c.Conn.WriteJSON(op)
	c.WriteMu.Unlock()
	if err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case msg := <-successCh:
		// Parse result
		var resp struct {
			Code string    `json:"code"`
			Msg  string    `json:"msg"`
			Data []OrderId `json:"data"`
		}
		if err := json.Unmarshal(msg, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse WS response: %w", err)
		}
		// Double check code (though SuccessCh implies Success in handleMessage logic?
		// handleMessage checks code!=0. But let's be safe)
		if resp.Code != "0" {
			return nil, fmt.Errorf("okx ws error: code=%s msg=%s data=%v", resp.Code, resp.Msg, resp.Data)
		}
		if len(resp.Data) > 0 {
			return &resp.Data[0], nil
		}
		return nil, nil // No data but success?

	case msg := <-errorCh:
		var resp struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
		}
		json.Unmarshal(msg, &resp)
		return nil, fmt.Errorf("okx ws error: code=%s msg=%s", resp.Code, string(msg))

	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for order response")
	}
}

// CancelOrderWS cancels an order via WebSocket.
func (c *WsClient) CancelOrderWS(instId string, ordId, clOrdId *string) (*OrderId, error) {
	req := map[string]string{
		"instId": instId,
	}
	if ordId != nil {
		req["ordId"] = *ordId
	}
	if clOrdId != nil {
		req["clOrdId"] = *clOrdId
	}

	// Use int64 for internal tracking
	idInt := time.Now().UnixNano()
	idStr := fmt.Sprintf("%d", idInt)

	op := WsOrderOp{
		Id:   idStr,
		Op:   "cancel-order",
		Args: []interface{}{req},
	}

	// Create channel for response
	successCh, errorCh := c.AddPendingRequest(idInt)
	defer c.RemovePendingRequest(idInt)

	c.WriteMu.Lock()
	err := c.Conn.WriteJSON(op)
	c.WriteMu.Unlock()
	if err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case msg := <-successCh:
		// Parse result
		var resp struct {
			Code string    `json:"code"`
			Msg  string    `json:"msg"`
			Data []OrderId `json:"data"`
		}
		if err := json.Unmarshal(msg, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse WS response: %w", err)
		}
		if resp.Code != "0" {
			return nil, fmt.Errorf("okx ws error: code=%s msg=%s", resp.Code, resp.Msg)
		}
		if len(resp.Data) > 0 {
			return &resp.Data[0], nil
		}
		return nil, nil // Success

	case msg := <-errorCh:
		var resp struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
		}
		json.Unmarshal(msg, &resp)
		return nil, fmt.Errorf("okx ws error: code=%s msg=%s", resp.Code, resp.Msg)

	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for cancel response")
	}
}

// ModifyOrderWS amends an order via WebSocket.
func (c *WsClient) ModifyOrderWS(req *ModifyOrderRequest) (*OrderId, error) {
	// Use int64 for internal tracking
	idInt := time.Now().UnixNano()
	idStr := fmt.Sprintf("%d", idInt)

	op := WsOrderOp{
		Id:   idStr,
		Op:   "amend-order",
		Args: []interface{}{req},
	}

	// Create channel for response
	successCh, errorCh := c.AddPendingRequest(idInt)
	defer c.RemovePendingRequest(idInt)

	c.WriteMu.Lock()
	err := c.Conn.WriteJSON(op)
	c.WriteMu.Unlock()
	if err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case msg := <-successCh:
		// Parse result
		var resp struct {
			Code string    `json:"code"`
			Msg  string    `json:"msg"`
			Data []OrderId `json:"data"`
		}
		if err := json.Unmarshal(msg, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse WS response: %w", err)
		}
		if resp.Code != "0" {
			return nil, fmt.Errorf("okx ws error: code=%s msg=%s", resp.Code, resp.Msg)
		}
		if len(resp.Data) > 0 {
			return &resp.Data[0], nil
		}
		return nil, nil // Success but no data?

	case msg := <-errorCh:
		var resp struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
		}
		json.Unmarshal(msg, &resp)
		return nil, fmt.Errorf("okx ws error: code=%s msg=%s", resp.Code, resp.Msg)

	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for modify response")
	}
}

// CancelOrdersWS cancels a batch of orders via WebSocket.
func (c *WsClient) CancelOrdersWS(reqs []CancelOrderRequest) ([]OrderId, error) {
	// Use int64 for internal tracking
	idInt := time.Now().UnixNano()
	idStr := fmt.Sprintf("%d", idInt)

	op := WsOrderOp{
		Id:   idStr,
		Op:   "batch-cancel-orders",
		Args: make([]interface{}, len(reqs)),
	}
	for i, r := range reqs {
		op.Args[i] = r
	}

	// Create channel for response
	successCh, errorCh := c.AddPendingRequest(idInt)
	defer c.RemovePendingRequest(idInt)

	c.WriteMu.Lock()
	err := c.Conn.WriteJSON(op)
	c.WriteMu.Unlock()
	if err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case msg := <-successCh:
		// Parse result
		var resp struct {
			Code string    `json:"code"`
			Msg  string    `json:"msg"`
			Data []OrderId `json:"data"`
		}
		if err := json.Unmarshal(msg, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse WS response: %w", err)
		}
		if resp.Code != "0" {
			return nil, fmt.Errorf("okx ws error: code=%s msg=%s", resp.Code, resp.Msg)
		}
		return resp.Data, nil

	case msg := <-errorCh:
		var resp struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
		}
		json.Unmarshal(msg, &resp)
		return nil, fmt.Errorf("okx ws error: code=%s msg=%s", resp.Code, resp.Msg)

	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for batch cancel response")
	}
}
