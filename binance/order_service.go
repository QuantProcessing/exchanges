package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/binance/sdk/perp"
	"github.com/QuantProcessing/exchanges/binance/sdk/spot"
)

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (_ *exchanges.Order, retErr error) {
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return nil, err
		}
	}

	p := perp.PlaceOrderParams{
		Symbol:   a.FormatSymbol(params.Symbol),
		Side:     string(params.Side),
		Type:     a.mapOrderType(params.Type),
		Quantity: params.Quantity.String(),
	}
	if params.Price.IsPositive() {
		p.Price = params.Price.String()
	}
	p.TimeInForce = a.mapTimeInForce(params)
	if params.Type == exchanges.OrderTypePostOnly {
		p.Type = "LIMIT"
	}
	if params.ClientID != "" {
		p.NewClientOrderID = params.ClientID
	}

	resp, err := a.client.PlaceOrder(ctx, p)
	if err != nil {
		return nil, err
	}
	return a.normalizeOrderResponse(resp)
}

func (a *Adapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	if strings.TrimSpace(params.ClientID) == "" {
		return fmt.Errorf("client id required for PlaceOrderWS")
	}
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return err
	}
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return err
		}
	}

	p := perp.PlaceOrderParams{
		Symbol:           a.FormatSymbol(params.Symbol),
		Side:             string(params.Side),
		Type:             a.mapOrderType(params.Type),
		Quantity:         params.Quantity.String(),
		NewClientOrderID: params.ClientID,
	}
	if params.Price.IsPositive() {
		p.Price = params.Price.String()
	}
	p.TimeInForce = a.mapTimeInForce(params)
	if params.Type == exchanges.OrderTypePostOnly {
		p.Type = "LIMIT"
	}

	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	_, err = a.wsAPI.PlaceOrderWS(a.apiKey, a.secretKey, p, reqID)
	return err
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) (retErr error) {
	p := perp.CancelOrderParams{
		Symbol:  a.FormatSymbol(symbol),
		OrderID: orderID,
	}
	_, err := a.client.CancelOrder(ctx, p)
	return err
}

func (a *Adapter) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	p := perp.CancelOrderParams{
		Symbol:  a.FormatSymbol(symbol),
		OrderID: orderID,
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	_, err := a.wsAPI.CancelOrderWS(a.apiKey, a.secretKey, p, reqID)
	return err
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) (retErr error) {
	return a.client.CancelAllOpenOrders(ctx, perp.CancelAllOrdersParams{Symbol: a.FormatSymbol(symbol)})
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (_ *exchanges.Order, retErr error) {
	oid, _ := strconv.ParseInt(orderID, 10, 64)
	p := perp.ModifyOrderParams{
		Symbol:   a.FormatSymbol(symbol),
		OrderID:  oid,
		Quantity: params.Quantity.String(),
		Price:    params.Price.String(),
	}

	resp, err := a.client.ModifyOrder(ctx, p)
	if err != nil {
		return nil, err
	}
	return a.normalizeOrderResponse(resp)
}

func (a *Adapter) ModifyOrderWS(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}

	oid, _ := strconv.ParseInt(orderID, 10, 64)
	p := perp.ModifyOrderParams{
		Symbol:   a.FormatSymbol(symbol),
		OrderID:  oid,
		Quantity: params.Quantity.String(),
		Price:    params.Price.String(),
	}

	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	_, err := a.wsAPI.ModifyOrderWS(a.apiKey, a.secretKey, p, reqID)
	return err
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (_ *exchanges.Order, retErr error) {
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %w", err)
	}

	res, err := a.client.GetOrder(ctx, a.FormatSymbol(symbol), oid, "")
	if err != nil {
		if isBinanceOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}

	return a.normalizeOrderResponse(res)
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) (_ []exchanges.Order, retErr error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) (_ []exchanges.Order, retErr error) {
	res, err := a.client.GetOpenOrders(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}

	orders := make([]exchanges.Order, 0, len(res))
	for _, r := range res {
		o, err := a.normalizeOrderResponse(&r)
		if err != nil {
			continue
		}
		orders = append(orders, *o)
	}
	return orders, nil
}

func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}

	side := "BUY"
	if params.Side == exchanges.OrderSideSell {
		side = "SELL"
	}
	orderType := strings.ToUpper(string(params.Type))
	p := spot.PlaceOrderParams{
		Symbol:           strings.ToUpper(a.FormatSymbol(params.Symbol)),
		Side:             side,
		Type:             orderType,
		Quantity:         params.Quantity.String(),
		NewClientOrderID: params.ClientID,
	}
	if params.Type == exchanges.OrderTypeLimit || params.Type == exchanges.OrderTypePostOnly {
		p.Price = params.Price.String()
		p.TimeInForce = "GTC"
	}

	resp, err := a.client.PlaceOrder(ctx, p)
	if err != nil {
		return nil, err
	}
	return a.normalizeOrderResponse(resp)
}

func (a *SpotAdapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	if strings.TrimSpace(params.ClientID) == "" {
		return fmt.Errorf("client id required for PlaceOrderWS")
	}
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return err
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}

	side := "BUY"
	if params.Side == exchanges.OrderSideSell {
		side = "SELL"
	}
	p := spot.PlaceOrderParams{
		Symbol:           strings.ToUpper(a.FormatSymbol(params.Symbol)),
		Side:             side,
		Type:             strings.ToUpper(string(params.Type)),
		Quantity:         params.Quantity.String(),
		NewClientOrderID: params.ClientID,
	}
	if params.Type == exchanges.OrderTypeLimit || params.Type == exchanges.OrderTypePostOnly {
		p.Price = params.Price.String()
		p.TimeInForce = "GTC"
	}

	reqID := fmt.Sprintf("order-%d", time.Now().UnixNano())
	_, err := a.wsAPI.PlaceOrderWS(a.apiKey, a.secretKey, p, reqID)
	return err
}

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order id: %w", err)
	}
	_, err = a.client.CancelOrder(ctx, strings.ToUpper(a.FormatSymbol(symbol)), oid, "")
	return err
}

func (a *SpotAdapter) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}

	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order id: %w", err)
	}
	reqID := fmt.Sprintf("cancel-%d", time.Now().UnixNano())
	_, err = a.wsAPI.CancelOrderWS(a.apiKey, a.secretKey, strings.ToUpper(a.FormatSymbol(symbol)), oid, "", reqID)
	return err
}

func (a *SpotAdapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %w", err)
	}

	existingOrder, err := a.FetchOrderByID(ctx, orderID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing order: %w", err)
	}

	side := "BUY"
	if existingOrder.Side == exchanges.OrderSideSell {
		side = "SELL"
	}

	resp, err := a.client.ModifyOrder(ctx, spot.CancelReplaceOrderParams{
		Symbol:            a.FormatSymbol(symbol),
		Side:              side,
		Type:              "LIMIT",
		CancelReplaceMode: "STOP_ON_FAILURE",
		TimeInForce:       "GTC",
		Quantity:          params.Quantity.String(),
		Price:             params.Price.String(),
		CancelOrderID:     oid,
	})
	if err != nil {
		return nil, err
	}
	if resp.NewOrderResponse == nil {
		return nil, fmt.Errorf("modify order failed: %s", resp.NewOrderStatus)
	}
	return a.normalizeOrderResponse(resp.NewOrderResponse)
}

func (a *SpotAdapter) ModifyOrderWS(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}

	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order id: %w", err)
	}
	existingOrder, err := a.FetchOrderByID(ctx, orderID, symbol)
	if err != nil {
		return fmt.Errorf("failed to get existing order: %w", err)
	}

	side := "BUY"
	if existingOrder.Side == exchanges.OrderSideSell {
		side = "SELL"
	}

	p := spot.CancelReplaceOrderParams{
		Symbol:            a.FormatSymbol(symbol),
		Side:              side,
		Type:              "LIMIT",
		CancelReplaceMode: "STOP_ON_FAILURE",
		TimeInForce:       "GTC",
		Quantity:          params.Quantity.String(),
		Price:             params.Price.String(),
		CancelOrderID:     oid,
	}
	reqID := fmt.Sprintf("modify-%d", time.Now().UnixNano())
	_, err = a.wsAPI.ModifyOrderWS(a.apiKey, a.secretKey, p, reqID)
	return err
}

func (a *SpotAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %w", err)
	}

	resp, err := a.client.GetOrder(ctx, strings.ToUpper(a.FormatSymbol(symbol)), oid, "")
	if err != nil {
		if isBinanceOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	return a.normalizeOrderResponse(resp)
}

func (a *SpotAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	resp, err := a.client.GetOpenOrders(ctx, strings.ToUpper(a.FormatSymbol(symbol)))
	if err != nil {
		return nil, err
	}

	orders := make([]exchanges.Order, 0, len(resp))
	for _, r := range resp {
		o, err := a.normalizeOrderResponse(&r)
		if err != nil {
			continue
		}
		orders = append(orders, *o)
	}
	return orders, nil
}

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	orders, err := a.FetchOpenOrders(ctx, symbol)
	if err != nil {
		return err
	}
	for _, order := range orders {
		if err := a.CancelOrder(ctx, order.OrderID, symbol); err != nil {
			a.Logger.Warnw("Failed to cancel order", "orderID", order.OrderID, "error", err)
		}
	}
	return nil
}
