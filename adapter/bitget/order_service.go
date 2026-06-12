package bitget

import (
	"context"

	exchanges "github.com/QuantProcessing/exchanges"
)

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	return a.private.PlaceOrder(ctx, params)
}

func (a *Adapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	return a.private.PlaceOrderWS(ctx, params)
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	return a.private.CancelOrder(ctx, orderID, symbol)
}

func (a *Adapter) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	return a.private.CancelOrderWS(ctx, orderID, symbol)
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
	return a.private.CancelAllOrders(ctx, symbol)
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	return a.private.FetchOrderByID(ctx, orderID, symbol)
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return a.private.FetchOrders(ctx, symbol)
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return a.private.FetchOpenOrders(ctx, symbol)
}

func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	return a.private.PlaceOrder(ctx, params)
}

func (a *SpotAdapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	return a.private.PlaceOrderWS(ctx, params)
}

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	return a.private.CancelOrder(ctx, orderID, symbol)
}

func (a *SpotAdapter) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	return a.private.CancelOrderWS(ctx, orderID, symbol)
}

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	return a.private.CancelAllOrders(ctx, symbol)
}

func (a *SpotAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	return a.private.FetchOrderByID(ctx, orderID, symbol)
}

func (a *SpotAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return a.private.FetchOrders(ctx, symbol)
}

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return a.private.FetchOpenOrders(ctx, symbol)
}
