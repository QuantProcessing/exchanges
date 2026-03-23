package bitget

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/shopspring/decimal"
)

type utaPerpProfile struct {
	adp *Adapter
}

type utaSpotProfile struct {
	adp *SpotAdapter
}

func (p *utaPerpProfile) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	if err := p.adp.BaseAdapter.ApplySlippage(ctx, params, p.adp.FetchTicker); err != nil {
		return nil, err
	}
	if err := p.adp.BaseAdapter.ValidateOrder(params); err != nil {
		return nil, err
	}
	req, err := toPlaceOrderRequest(ctx, p.adp, p.adp.perpCategory, params)
	if err != nil {
		return nil, err
	}
	var raw *sdk.PlaceOrderResponse
	if p.adp.IsRESTMode() {
		raw, err = p.adp.client.PlaceOrder(ctx, req)
	} else {
		if err := p.adp.WsOrderConnected(ctx); err != nil {
			return nil, err
		}
		raw, err = p.adp.privateWS.PlaceOrderUTAWS(req)
	}
	if err != nil {
		return nil, err
	}
	return &exchanges.Order{
		OrderID:       raw.OrderID,
		ClientOrderID: firstNonEmpty(raw.ClientOID, req.ClientOID),
		Symbol:        strings.ToUpper(params.Symbol),
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusNew,
		Timestamp:     time.Now().UnixMilli(),
		ReduceOnly:    params.ReduceOnly,
		TimeInForce:   params.TimeInForce,
	}, nil
}

func (p *utaPerpProfile) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	req := &sdk.CancelOrderRequest{
		Category: p.adp.perpCategory,
		Symbol:   p.adp.FormatSymbol(symbol),
		OrderID:  orderID,
	}
	if p.adp.IsRESTMode() {
		_, err := p.adp.client.CancelOrder(ctx, req)
		return err
	}
	if err := p.adp.WsOrderConnected(ctx); err != nil {
		return err
	}
	_, err := p.adp.privateWS.CancelOrderUTAWS(req)
	return err
}

func (p *utaPerpProfile) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	return p.adp.client.CancelAllOrders(ctx, &sdk.CancelAllOrdersRequest{
		Category: p.adp.perpCategory,
		Symbol:   p.adp.FormatSymbol(symbol),
	})
}

func (p *utaPerpProfile) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetOrder(ctx, p.adp.perpCategory, p.adp.FormatSymbol(symbol), orderID, "")
	if err != nil {
		if isBitgetOrderNotFound(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	return mapOrder(strings.ToUpper(symbol), *raw), nil
}

func (p *utaPerpProfile) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetOrderHistory(ctx, p.adp.perpCategory, p.adp.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	orders := make([]exchanges.Order, 0, len(raw))
	for _, order := range raw {
		orders = append(orders, *mapOrder(strings.ToUpper(symbol), order))
	}
	return orders, nil
}

func (p *utaPerpProfile) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetOpenOrders(ctx, p.adp.perpCategory, p.adp.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	orders := make([]exchanges.Order, 0, len(raw))
	for _, order := range raw {
		orders = append(orders, *mapOrder(strings.ToUpper(symbol), order))
	}
	return orders, nil
}

func (p *utaPerpProfile) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	assets, err := p.adp.client.GetAccountAssets(ctx)
	if err != nil {
		return nil, err
	}
	orders, err := p.FetchOpenOrders(ctx, "")
	if err != nil {
		return nil, err
	}
	positions, err := p.FetchPositions(ctx)
	if err != nil {
		return nil, err
	}
	account := &exchanges.Account{
		Positions: positions,
		Orders:    orders,
	}
	for _, asset := range assets.Assets {
		if strings.EqualFold(asset.Coin, string(p.adp.quote)) {
			account.TotalBalance = parseDecimal(firstNonEmpty(asset.Equity, asset.Available))
			account.AvailableBalance = parseDecimal(rawAvailable(asset))
			break
		}
	}
	for _, position := range positions {
		account.UnrealizedPnL = account.UnrealizedPnL.Add(position.UnrealizedPnL)
		account.RealizedPnL = account.RealizedPnL.Add(position.RealizedPnL)
	}
	return account, nil
}

func (p *utaPerpProfile) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	account, err := p.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return account.AvailableBalance, nil
}

func (p *utaPerpProfile) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	p.adp.mu.RLock()
	inst, ok := p.adp.markets.perpByBase[strings.ToUpper(symbol)]
	p.adp.mu.RUnlock()
	if !ok {
		return nil, exchanges.ErrSymbolNotFound
	}
	return &exchanges.FeeRate{
		Maker: parseDecimal(inst.MakerFeeRate),
		Taker: parseDecimal(inst.TakerFeeRate),
	}, nil
}

func (p *utaPerpProfile) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetCurrentPositions(ctx, p.adp.perpCategory, "")
	if err != nil {
		return nil, err
	}
	positions := make([]exchanges.Position, 0, len(raw))
	for _, position := range raw {
		update := mapPosition(position)
		update.Symbol = p.adp.ExtractSymbol(position.Symbol)
		positions = append(positions, update)
	}
	return positions, nil
}

func (p *utaPerpProfile) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	return p.adp.client.SetLeverage(ctx, &sdk.SetLeverageRequest{
		Symbol:   p.adp.FormatSymbol(symbol),
		Category: p.adp.perpCategory,
		Leverage: strconv.Itoa(leverage),
	})
}

func (p *utaPerpProfile) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	req := toModifyOrderRequest(p.adp, p.adp.perpCategory, orderID, symbol, params)
	var raw *sdk.CancelOrderResponse
	var err error
	if p.adp.IsRESTMode() {
		raw, err = p.adp.client.ModifyOrder(ctx, req)
	} else {
		if err := p.adp.WsOrderConnected(ctx); err != nil {
			return nil, err
		}
		raw, err = p.adp.privateWS.ModifyOrderUTAWS(req)
	}
	if err != nil {
		return nil, err
	}
	return p.FetchOrderByID(ctx, firstNonEmpty(raw.OrderID, orderID), symbol)
}

func (p *utaPerpProfile) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	err := p.adp.privateWS.Subscribe(ctx, sdk.WSArg{
		InstType: "UTA",
		Topic:    "order",
	}, func(payload json.RawMessage) {
		msg, err := sdk.DecodeOrderMessage(payload)
		if err != nil {
			return
		}
		for _, order := range msg.Data {
			if !strings.EqualFold(order.Category, p.adp.perpCategory) {
				continue
			}
			if cb != nil {
				cb(mapOrder(p.adp.ExtractSymbol(order.Symbol), order))
			}
		}
	})
	if err == nil {
		p.adp.MarkOrderConnected()
	}
	return err
}

func (p *utaPerpProfile) StopWatchOrders(ctx context.Context) error {
	return p.adp.privateWS.Unsubscribe(ctx, sdk.WSArg{
		InstType: "UTA",
		Topic:    "order",
	})
}

func (p *utaPerpProfile) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	err := p.adp.privateWS.Subscribe(ctx, sdk.WSArg{
		InstType: "UTA",
		Topic:    "position",
	}, func(payload json.RawMessage) {
		msg, err := sdk.DecodePositionMessage(payload)
		if err != nil {
			return
		}
		for _, position := range msg.Data {
			if !strings.EqualFold(position.Category, p.adp.perpCategory) {
				continue
			}
			update := mapPosition(position)
			update.Symbol = p.adp.ExtractSymbol(position.Symbol)
			if cb != nil {
				cb(&update)
			}
		}
	})
	if err == nil {
		p.adp.MarkAccountConnected()
	}
	return err
}

func (p *utaPerpProfile) StopWatchPositions(ctx context.Context) error {
	return p.adp.privateWS.Unsubscribe(ctx, sdk.WSArg{
		InstType: "UTA",
		Topic:    "position",
	})
}

func (p *utaSpotProfile) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	if err := p.adp.BaseAdapter.ApplySlippage(ctx, params, p.adp.FetchTicker); err != nil {
		return nil, err
	}
	if err := p.adp.BaseAdapter.ValidateOrder(params); err != nil {
		return nil, err
	}
	req, err := toPlaceOrderRequest(ctx, p.adp, categorySpot, params)
	if err != nil {
		return nil, err
	}
	var raw *sdk.PlaceOrderResponse
	if p.adp.IsRESTMode() {
		raw, err = p.adp.client.PlaceOrder(ctx, req)
	} else {
		if err := p.adp.WsOrderConnected(ctx); err != nil {
			return nil, err
		}
		raw, err = p.adp.privateWS.PlaceOrderUTAWS(req)
	}
	if err != nil {
		return nil, err
	}
	return &exchanges.Order{
		OrderID:       raw.OrderID,
		ClientOrderID: firstNonEmpty(raw.ClientOID, req.ClientOID),
		Symbol:        strings.ToUpper(params.Symbol),
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusNew,
		Timestamp:     time.Now().UnixMilli(),
		TimeInForce:   params.TimeInForce,
	}, nil
}

func (p *utaSpotProfile) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	req := &sdk.CancelOrderRequest{
		Category: categorySpot,
		Symbol:   p.adp.FormatSymbol(symbol),
		OrderID:  orderID,
	}
	if p.adp.IsRESTMode() {
		_, err := p.adp.client.CancelOrder(ctx, req)
		return err
	}
	if err := p.adp.WsOrderConnected(ctx); err != nil {
		return err
	}
	_, err := p.adp.privateWS.CancelOrderUTAWS(req)
	return err
}

func (p *utaSpotProfile) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	return p.adp.client.CancelAllOrders(ctx, &sdk.CancelAllOrdersRequest{
		Category: categorySpot,
		Symbol:   p.adp.FormatSymbol(symbol),
	})
}

func (p *utaSpotProfile) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetOrder(ctx, categorySpot, p.adp.FormatSymbol(symbol), orderID, "")
	if err != nil {
		if isBitgetOrderNotFound(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	return mapOrder(strings.ToUpper(symbol), *raw), nil
}

func (p *utaSpotProfile) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetOrderHistory(ctx, categorySpot, p.adp.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	orders := make([]exchanges.Order, 0, len(raw))
	for _, order := range raw {
		orders = append(orders, *mapOrder(strings.ToUpper(symbol), order))
	}
	return orders, nil
}

func (p *utaSpotProfile) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetOpenOrders(ctx, categorySpot, p.adp.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	orders := make([]exchanges.Order, 0, len(raw))
	for _, order := range raw {
		orders = append(orders, *mapOrder(strings.ToUpper(symbol), order))
	}
	return orders, nil
}

func (p *utaSpotProfile) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	assets, err := p.adp.client.GetAccountAssets(ctx)
	if err != nil {
		return nil, err
	}
	orders, err := p.FetchOpenOrders(ctx, "")
	if err != nil {
		return nil, err
	}
	account := &exchanges.Account{
		Orders: orders,
	}
	for _, asset := range assets.Assets {
		if strings.EqualFold(asset.Coin, string(p.adp.quote)) {
			account.TotalBalance = parseDecimal(firstNonEmpty(asset.Equity, asset.Available))
			account.AvailableBalance = parseDecimal(rawAvailable(asset))
			break
		}
	}
	return account, nil
}

func (p *utaSpotProfile) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	account, err := p.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return account.AvailableBalance, nil
}

func (p *utaSpotProfile) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	p.adp.mu.RLock()
	inst, ok := p.adp.markets.spotByBase[strings.ToUpper(symbol)]
	p.adp.mu.RUnlock()
	if !ok {
		return nil, exchanges.ErrSymbolNotFound
	}
	return &exchanges.FeeRate{
		Maker: parseDecimal(inst.MakerFeeRate),
		Taker: parseDecimal(inst.TakerFeeRate),
	}, nil
}

func (p *utaSpotProfile) FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	assets, err := p.adp.client.GetAccountAssets(ctx)
	if err != nil {
		return nil, err
	}
	return mapSpotBalances(assets.Assets), nil
}

func (p *utaSpotProfile) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	err := p.adp.privateWS.Subscribe(ctx, sdk.WSArg{
		InstType: "UTA",
		Topic:    "order",
	}, func(payload json.RawMessage) {
		msg, err := sdk.DecodeOrderMessage(payload)
		if err != nil {
			return
		}
		for _, order := range msg.Data {
			if !strings.EqualFold(order.Category, categorySpot) {
				continue
			}
			if cb != nil {
				cb(mapOrder(p.adp.ExtractSymbol(order.Symbol), order))
			}
		}
	})
	if err == nil {
		p.adp.MarkOrderConnected()
	}
	return err
}

func (p *utaSpotProfile) StopWatchOrders(ctx context.Context) error {
	return p.adp.privateWS.Unsubscribe(ctx, sdk.WSArg{
		InstType: "UTA",
		Topic:    "order",
	})
}
