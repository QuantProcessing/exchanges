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

// classicPerpProfile owns Bitget's current hybrid transport subset:
// place/cancel honor OrderModeWS, while the rest of private trading remains REST-backed.
type classicPerpProfile struct {
	adp *Adapter
}

// classicSpotProfile mirrors the classic perp hybrid split for spot place/cancel flows.
type classicSpotProfile struct {
	adp *SpotAdapter
}

func (p *classicPerpProfile) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
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
	marginMode, tradeSide, err := p.orderContext(ctx, params.Symbol, params.ReduceOnly)
	if err != nil {
		return nil, err
	}
	req.MarginMode = marginMode
	req.TradeSide = tradeSide
	if tradeSide == "close" {
		req.Side = oppositeBitgetSide(req.Side)
	}
	var raw *sdk.PlaceOrderResponse
	if p.adp.IsRESTMode() {
		raw, err = p.adp.client.PlaceClassicMixOrder(ctx, req, p.adp.perpCategory, p.marginCoin())
	} else {
		if err := p.adp.WsOrderConnected(ctx); err != nil {
			return nil, err
		}
		raw, err = p.adp.privateWS.PlaceClassicPerpOrderWS(req, p.adp.perpCategory, p.marginCoin())
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

func (p *classicPerpProfile) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	if p.adp.IsRESTMode() {
		_, err := p.adp.client.CancelClassicMixOrder(ctx, p.adp.FormatSymbol(symbol), p.adp.perpCategory, p.marginCoin(), orderID, "")
		return err
	}
	if err := p.adp.WsOrderConnected(ctx); err != nil {
		return err
	}
	_, err := p.adp.privateWS.CancelClassicPerpOrderWS(p.adp.FormatSymbol(symbol), p.adp.perpCategory, p.marginCoin(), orderID, "")
	return err
}

func (p *classicPerpProfile) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	return p.adp.client.CancelAllClassicMixOrders(ctx, p.adp.perpCategory, p.adp.FormatSymbol(symbol), p.marginCoin())
}

func (p *classicPerpProfile) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetClassicMixOrder(ctx, p.adp.FormatSymbol(symbol), p.adp.perpCategory, orderID, "")
	if err != nil {
		if isBitgetOrderNotFound(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	return mapClassicMixOrder(p.adp.ExtractSymbol(firstNonEmpty(raw.InstID, raw.Symbol)), *raw), nil
}

func (p *classicPerpProfile) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetClassicMixOrderHistory(ctx, p.adp.perpCategory, p.adp.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	open, err := p.adp.client.GetClassicMixOpenOrders(ctx, p.adp.perpCategory, p.adp.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Order, 0, len(raw)+len(open))
	for _, order := range raw {
		out = append(out, *mapClassicMixOrder(p.adp.ExtractSymbol(firstNonEmpty(order.InstID, order.Symbol)), order))
	}
	for _, order := range open {
		out = append(out, *mapClassicMixOrder(p.adp.ExtractSymbol(firstNonEmpty(order.InstID, order.Symbol)), order))
	}
	return dedupeOrders(out), nil
}

func (p *classicPerpProfile) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetClassicMixOpenOrders(ctx, p.adp.perpCategory, p.adp.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Order, 0, len(raw))
	for _, order := range raw {
		out = append(out, *mapClassicMixOrder(p.adp.ExtractSymbol(firstNonEmpty(order.InstID, order.Symbol)), order))
	}
	return out, nil
}

func (p *classicPerpProfile) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	accountRaw, err := p.adp.client.GetClassicMixAccount(ctx, p.defaultSymbol(), p.adp.perpCategory, p.marginCoin())
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
		Positions:        positions,
		Orders:           orders,
		TotalBalance:     parseDecimal(firstNonEmpty(accountRaw.AccountEquity, accountRaw.UsdtEquity)),
		AvailableBalance: parseDecimal(firstNonEmpty(accountRaw.Available, accountRaw.CrossedMaxAvailable, accountRaw.IsolatedMaxAvailable)),
		UnrealizedPnL:    parseDecimal(firstNonEmpty(accountRaw.UnrealizedPL, accountRaw.CrossedUnrealizedPL, accountRaw.IsolatedUnrealizedPL)),
	}
	for _, position := range positions {
		account.RealizedPnL = account.RealizedPnL.Add(position.RealizedPnL)
	}
	return account, nil
}

func (p *classicPerpProfile) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	account, err := p.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return account.AvailableBalance, nil
}

func (p *classicPerpProfile) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
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

func (p *classicPerpProfile) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetClassicMixPositions(ctx, p.adp.perpCategory, p.marginCoin())
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Position, 0, len(raw))
	for _, position := range raw {
		update := mapClassicMixPosition(position)
		update.Symbol = p.adp.ExtractSymbol(firstNonEmpty(position.InstID, position.Symbol))
		out = append(out, update)
	}
	return out, nil
}

func (p *classicPerpProfile) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	return p.adp.client.SetClassicMixLeverage(ctx, p.adp.FormatSymbol(symbol), p.adp.perpCategory, p.marginCoin(), strconv.Itoa(leverage))
}

func (p *classicPerpProfile) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	req := toModifyOrderRequest(p.adp, p.adp.perpCategory, orderID, symbol, params)
	raw, err := p.adp.client.ModifyClassicMixOrder(ctx, req, p.adp.perpCategory, p.marginCoin())
	if err != nil {
		return nil, err
	}
	return p.FetchOrderByID(ctx, firstNonEmpty(raw.OrderID, orderID), symbol)
}

func (p *classicPerpProfile) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	err := p.adp.privateWS.Subscribe(ctx, sdk.WSArg{
		InstType: p.adp.perpCategory,
		Channel:  "orders",
		InstID:   "default",
	}, func(payload json.RawMessage) {
		msg, err := sdk.DecodeClassicWSOrderMessage(payload)
		if err != nil {
			return
		}
		for _, order := range msg.Data {
			if cb != nil {
				cb(mapClassicMixOrderStream(p.adp.ExtractSymbol(firstNonEmpty(order.InstID, order.Symbol)), order))
			}
		}
	})
	if err == nil {
		p.adp.MarkOrderConnected()
	}
	return err
}

func (p *classicPerpProfile) StopWatchOrders(ctx context.Context) error {
	return p.adp.privateWS.Unsubscribe(ctx, sdk.WSArg{
		InstType: p.adp.perpCategory,
		Channel:  "orders",
		InstID:   "default",
	})
}

func (p *classicPerpProfile) WatchFills(ctx context.Context, cb exchanges.FillCallback) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	err := p.adp.privateWS.Subscribe(ctx, sdk.WSArg{
		InstType: p.adp.perpCategory,
		Channel:  "fill",
		InstID:   "default",
	}, func(payload json.RawMessage) {
		msg, err := sdk.DecodeClassicWSFillMessage(payload)
		if err != nil {
			return
		}
		for _, fill := range msg.Data {
			if cb != nil {
				cb(mapClassicMixFill(p.adp.ExtractSymbol(firstNonEmpty(fill.InstID, fill.Symbol)), fill))
			}
		}
	})
	if err == nil {
		p.adp.MarkOrderConnected()
	}
	return err
}

func (p *classicPerpProfile) StopWatchFills(ctx context.Context) error {
	return p.adp.privateWS.Unsubscribe(ctx, sdk.WSArg{
		InstType: p.adp.perpCategory,
		Channel:  "fill",
		InstID:   "default",
	})
}

func (p *classicPerpProfile) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	err := p.adp.privateWS.Subscribe(ctx, sdk.WSArg{
		InstType: p.adp.perpCategory,
		Channel:  "positions",
		InstID:   "default",
	}, func(payload json.RawMessage) {
		msg, err := sdk.DecodeClassicWSPositionMessage(payload)
		if err != nil {
			return
		}
		for _, position := range msg.Data {
			update := mapClassicMixPosition(position)
			update.Symbol = p.adp.ExtractSymbol(firstNonEmpty(position.InstID, position.Symbol))
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

func (p *classicPerpProfile) StopWatchPositions(ctx context.Context) error {
	return p.adp.privateWS.Unsubscribe(ctx, sdk.WSArg{
		InstType: p.adp.perpCategory,
		Channel:  "positions",
		InstID:   "default",
	})
}

func (p *classicPerpProfile) marginCoin() string {
	return strings.ToUpper(string(p.adp.quote))
}

func (p *classicPerpProfile) defaultSymbol() string {
	p.adp.mu.RLock()
	defer p.adp.mu.RUnlock()
	for _, inst := range p.adp.markets.perpByBase {
		return inst.Symbol
	}
	return ""
}

func (p *classicPerpProfile) orderContext(ctx context.Context, symbol string, reduceOnly bool) (string, string, error) {
	account, err := p.adp.client.GetClassicMixAccount(ctx, p.adp.FormatSymbol(symbol), p.adp.perpCategory, p.marginCoin())
	if err != nil {
		return "", "", err
	}
	marginMode := firstNonEmpty(account.MarginMode, "crossed")
	tradeSide := ""
	if strings.EqualFold(account.PosMode, "hedge_mode") {
		if reduceOnly {
			tradeSide = "close"
		} else {
			tradeSide = "open"
		}
	}
	return marginMode, tradeSide, nil
}

func (p *classicSpotProfile) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
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
		raw, err = p.adp.client.PlaceClassicSpotOrder(ctx, req)
	} else {
		if err := p.adp.WsOrderConnected(ctx); err != nil {
			return nil, err
		}
		raw, err = p.adp.privateWS.PlaceClassicSpotOrderWS(req)
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

func (p *classicSpotProfile) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	if p.adp.IsRESTMode() {
		_, err := p.adp.client.CancelClassicSpotOrder(ctx, p.adp.FormatSymbol(symbol), orderID, "")
		return err
	}
	if err := p.adp.WsOrderConnected(ctx); err != nil {
		return err
	}
	_, err := p.adp.privateWS.CancelClassicSpotOrderWS(p.adp.FormatSymbol(symbol), orderID, "")
	return err
}

func (p *classicSpotProfile) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	return p.adp.client.CancelAllClassicSpotOrders(ctx, p.adp.FormatSymbol(symbol))
}

func (p *classicSpotProfile) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetClassicSpotOrder(ctx, orderID, "")
	if err != nil {
		if isBitgetOrderNotFound(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	return mapClassicSpotOrder(p.adp.ExtractSymbol(firstNonEmpty(raw.InstID, raw.Symbol)), *raw), nil
}

func (p *classicSpotProfile) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetClassicSpotOrderHistory(ctx, p.adp.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	open, err := p.adp.client.GetClassicSpotOpenOrders(ctx, p.adp.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Order, 0, len(raw)+len(open))
	for _, order := range raw {
		out = append(out, *mapClassicSpotOrder(p.adp.ExtractSymbol(firstNonEmpty(order.InstID, order.Symbol)), order))
	}
	for _, order := range open {
		out = append(out, *mapClassicSpotOrder(p.adp.ExtractSymbol(firstNonEmpty(order.InstID, order.Symbol)), order))
	}
	return dedupeOrders(out), nil
}

func (p *classicSpotProfile) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	raw, err := p.adp.client.GetClassicSpotOpenOrders(ctx, p.adp.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Order, 0, len(raw))
	for _, order := range raw {
		out = append(out, *mapClassicSpotOrder(p.adp.ExtractSymbol(firstNonEmpty(order.InstID, order.Symbol)), order))
	}
	return out, nil
}

func (p *classicSpotProfile) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	assets, err := p.adp.client.GetClassicSpotAssets(ctx, "")
	if err != nil {
		return nil, err
	}
	orders, err := p.FetchOpenOrders(ctx, "")
	if err != nil {
		return nil, err
	}
	account := &exchanges.Account{Orders: orders}
	for _, asset := range assets {
		if strings.EqualFold(asset.Coin, string(p.adp.quote)) {
			account.TotalBalance = parseDecimal(firstNonEmpty(asset.Available, asset.LimitAvailable))
			account.AvailableBalance = parseDecimal(asset.Available)
			break
		}
	}
	return account, nil
}

func (p *classicSpotProfile) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	account, err := p.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return account.AvailableBalance, nil
}

func (p *classicSpotProfile) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
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

func (p *classicSpotProfile) FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error) {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return nil, err
	}
	assets, err := p.adp.client.GetClassicSpotAssets(ctx, "")
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.SpotBalance, 0, len(assets))
	for _, asset := range assets {
		free := parseDecimal(asset.Available)
		locked := parseDecimal(firstNonEmpty(asset.Frozen, asset.Locked))
		out = append(out, exchanges.SpotBalance{
			Asset:  strings.ToUpper(asset.Coin),
			Free:   free,
			Locked: locked,
			Total:  free.Add(locked),
		})
	}
	return out, nil
}

func (p *classicSpotProfile) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	err := p.adp.privateWS.Subscribe(ctx, sdk.WSArg{
		InstType: "SPOT",
		Channel:  "orders",
		InstID:   "default",
	}, func(payload json.RawMessage) {
		msg, err := sdk.DecodeClassicWSSpotOrderMessage(payload)
		if err != nil {
			return
		}
		for _, order := range msg.Data {
			if cb != nil {
				cb(mapClassicSpotOrderStream(p.adp.ExtractSymbol(firstNonEmpty(order.InstID, order.Symbol)), order))
			}
		}
	})
	if err == nil {
		p.adp.MarkOrderConnected()
	}
	return err
}

func (p *classicSpotProfile) StopWatchOrders(ctx context.Context) error {
	return p.adp.privateWS.Unsubscribe(ctx, sdk.WSArg{
		InstType: "SPOT",
		Channel:  "orders",
		InstID:   "default",
	})
}

func (p *classicSpotProfile) WatchFills(ctx context.Context, cb exchanges.FillCallback) error {
	if err := requirePrivateAccess(p.adp.client); err != nil {
		return err
	}
	err := p.adp.privateWS.Subscribe(ctx, sdk.WSArg{
		InstType: "SPOT",
		Channel:  "fill",
		InstID:   "default",
	}, func(payload json.RawMessage) {
		msg, err := sdk.DecodeClassicWSSpotFillMessage(payload)
		if err != nil {
			return
		}
		for _, fill := range msg.Data {
			if cb != nil {
				cb(mapClassicSpotFill(p.adp.ExtractSymbol(firstNonEmpty(fill.InstID, fill.Symbol)), fill))
			}
		}
	})
	if err == nil {
		p.adp.MarkOrderConnected()
	}
	return err
}

func (p *classicSpotProfile) StopWatchFills(ctx context.Context) error {
	return p.adp.privateWS.Unsubscribe(ctx, sdk.WSArg{
		InstType: "SPOT",
		Channel:  "fill",
		InstID:   "default",
	})
}

func mapClassicSpotOrder(symbol string, raw sdk.ClassicSpotOrderRecord) *exchanges.Order {
	ts := parseMillis(firstNonEmpty(raw.UTime, raw.CTime))
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	return &exchanges.Order{
		OrderID:          raw.OrderID,
		ClientOrderID:    raw.ClientOID,
		Symbol:           symbol,
		Side:             mapOrderSide(raw.Side),
		Type:             mapOrderType(raw.OrderType, raw.Force),
		Quantity:         parseDecimal(firstNonEmpty(raw.NewSize, raw.Size)),
		Price:            parseDecimal(firstNonEmpty(raw.Price, raw.FillPrice, raw.PriceAvg)),
		OrderPrice:       parseDecimal(raw.Price),
		AverageFillPrice: parseDecimal(raw.PriceAvg),
		LastFillPrice:    parseDecimal(raw.FillPrice),
		Status:           mapOrderStatus(raw.Status),
		FilledQuantity:   parseDecimal(firstNonEmpty(raw.AccBaseVolume, raw.BaseVolume)),
		Timestamp:        ts,
		Fee:              parseDecimal(firstNonEmpty(raw.FillFee, feeFromFlexibleDetails(raw.FeeDetail))),
		TimeInForce:      mapTimeInForce(raw.Force),
	}
}

func mapClassicSpotOrderStream(symbol string, raw sdk.ClassicSpotOrderRecord) *exchanges.Order {
	order := mapClassicSpotOrder(symbol, raw)
	order.Price = order.OrderPrice
	order.AverageFillPrice = decimal.Zero
	order.LastFillPrice = decimal.Zero
	order.LastFillQuantity = decimal.Zero
	order.Fee = decimal.Zero
	return order
}

func mapClassicMixOrder(symbol string, raw sdk.ClassicMixOrderRecord) *exchanges.Order {
	ts := parseMillis(firstNonEmpty(raw.UTime, raw.CTime))
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	return &exchanges.Order{
		OrderID:          raw.OrderID,
		ClientOrderID:    raw.ClientOID,
		Symbol:           symbol,
		Side:             mapOrderSide(raw.Side),
		Type:             mapOrderType(raw.OrderType, raw.Force),
		Quantity:         parseDecimal(raw.Size),
		Price:            parseDecimal(firstNonEmpty(raw.Price, raw.PriceAvg)),
		OrderPrice:       parseDecimal(raw.Price),
		AverageFillPrice: parseDecimal(raw.PriceAvg),
		Status:           mapOrderStatus(raw.Status),
		FilledQuantity:   parseDecimal(firstNonEmpty(raw.BaseVolume, raw.AccBaseVolume)),
		Timestamp:        ts,
		Fee:              parseDecimal(firstNonEmpty(raw.Fee, feeFromDetails(raw.FeeDetail))),
		ReduceOnly:       strings.EqualFold(raw.ReduceOnly, "yes"),
		TimeInForce:      mapTimeInForce(raw.Force),
	}
}

func mapClassicMixOrderStream(symbol string, raw sdk.ClassicMixOrderRecord) *exchanges.Order {
	order := mapClassicMixOrder(symbol, raw)
	order.Price = order.OrderPrice
	order.AverageFillPrice = decimal.Zero
	order.LastFillPrice = decimal.Zero
	order.LastFillQuantity = decimal.Zero
	order.Fee = decimal.Zero
	return order
}

func mapClassicMixPosition(raw sdk.ClassicMixPositionRecord) exchanges.Position {
	return exchanges.Position{
		Symbol:           strings.ToUpper(firstNonEmpty(raw.InstID, raw.Symbol)),
		Side:             mapPositionSide(raw.HoldSide),
		Quantity:         parseDecimal(raw.Total),
		EntryPrice:       parseDecimal(raw.OpenPriceAvg),
		UnrealizedPnL:    parseDecimal(raw.UnrealizedPL),
		RealizedPnL:      parseDecimal(raw.AchievedProfits),
		LiquidationPrice: parseDecimal(raw.LiquidationPrice),
		Leverage:         parseDecimal(raw.Leverage),
		MarginType:       strings.ToUpper(raw.MarginMode),
	}
}

func mapClassicMixFill(symbol string, raw sdk.ClassicMixFillRecord) *exchanges.Fill {
	ts := parseMillis(firstNonEmpty(raw.UTime, raw.CTime))
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	fee, feeAsset := classicFillFee(raw.FeeDetail)
	return &exchanges.Fill{
		TradeID:       raw.TradeID,
		OrderID:       raw.OrderID,
		ClientOrderID: raw.ClientOID,
		Symbol:        symbol,
		Side:          mapOrderSide(raw.Side),
		Price:         parseDecimal(raw.Price),
		Quantity:      parseDecimal(raw.BaseVolume),
		Fee:           fee,
		FeeAsset:      feeAsset,
		IsMaker:       classicTradeScopeIsMaker(raw.TradeScope),
		Timestamp:     ts,
	}
}

func mapClassicSpotFill(symbol string, raw sdk.ClassicSpotFillRecord) *exchanges.Fill {
	ts := parseMillis(firstNonEmpty(raw.UTime, raw.CTime))
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	fee, feeAsset := classicFillFee(raw.FeeDetail)
	return &exchanges.Fill{
		TradeID:       raw.TradeID,
		OrderID:       raw.OrderID,
		ClientOrderID: raw.ClientOID,
		Symbol:        symbol,
		Side:          mapOrderSide(raw.Side),
		Price:         parseDecimal(raw.PriceAvg),
		Quantity:      parseDecimal(raw.Size),
		Fee:           fee,
		FeeAsset:      feeAsset,
		IsMaker:       classicTradeScopeIsMaker(raw.TradeScope),
		Timestamp:     ts,
	}
}

func oppositeBitgetSide(side string) string {
	if strings.EqualFold(side, "buy") {
		return "sell"
	}
	return "buy"
}

func dedupeOrders(orders []exchanges.Order) []exchanges.Order {
	if len(orders) == 0 {
		return orders
	}
	seen := make(map[string]int, len(orders))
	out := make([]exchanges.Order, 0, len(orders))
	for _, order := range orders {
		if idx, ok := seen[order.OrderID]; ok {
			out[idx] = order
			continue
		}
		seen[order.OrderID] = len(out)
		out = append(out, order)
	}
	return out
}

func feeFromFlexibleDetails(fees sdk.FlexibleFeeDetails) string {
	if len(fees) == 0 {
		return ""
	}
	return fees[0].Fee
}

func classicFillFee(fees []sdk.ClassicFillFeeDetail) (decimal.Decimal, string) {
	if len(fees) == 0 {
		return decimal.Zero, ""
	}
	fee := parseDecimal(firstNonEmpty(fees[0].TotalFee, fees[0].TotalDeductionFee))
	return fee, fees[0].FeeCoin
}

func classicTradeScopeIsMaker(scope string) bool {
	scope = strings.ToLower(strings.TrimSpace(scope))
	return strings.HasPrefix(scope, "mak") || strings.HasPrefix(scope, "mar")
}
