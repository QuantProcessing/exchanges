package backpack

import (
	"fmt"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/shopspring/decimal"
)

func mapOrder(raw sdk.Order) *exchanges.Order {
	return &exchanges.Order{
		OrderID:        raw.ID,
		ClientOrderID:  fmt.Sprintf("%d", raw.ClientID),
		Symbol:         extractBaseSymbol(raw.Symbol),
		Side:           mapOrderSide(raw.Side),
		Type:           mapOrderType(raw.OrderType),
		Quantity:       parseDecimal(raw.Quantity),
		Price:          parseDecimal(raw.Price),
		FilledQuantity: parseDecimal(raw.ExecutedQuantity),
		Status:         mapOrderStatus(raw.Status),
		Timestamp:      microsToMillis(raw.CreatedAt),
		ReduceOnly:     raw.ReduceOnly,
		TimeInForce:    mapTimeInForce(raw.TimeInForce),
	}
}

func mapOrderUpdate(raw sdk.OrderUpdateEvent) *exchanges.Order {
	ts := raw.EngineTimestamp
	if ts == 0 {
		ts = raw.EventTime
	}
	status := mapOrderStatus(raw.OrderState)
	if status == exchanges.OrderStatusUnknown {
		status = mapOrderStatus(raw.EventType)
	}
	quantity := parseDecimal(raw.Quantity)
	filledQuantity := parseDecimal(raw.ExecutedQuantity)
	if status == exchanges.OrderStatusFilled && quantity.IsPositive() && filledQuantity.GreaterThan(decimal.Zero) && filledQuantity.LessThan(quantity) {
		status = exchanges.OrderStatusPartiallyFilled
	}

	return &exchanges.Order{
		OrderID:        raw.OrderID,
		ClientOrderID:  raw.ClientID.String(),
		Symbol:         extractBaseSymbol(raw.Symbol),
		Side:           mapOrderSide(raw.Side),
		Type:           mapOrderType(raw.OrderType),
		Quantity:       quantity,
		Price:          parseDecimal(raw.Price),
		OrderPrice:     parseDecimal(raw.Price),
		Status:         status,
		FilledQuantity: filledQuantity,
		Timestamp:      microsToMillis(ts),
		TimeInForce:    mapTimeInForce(raw.TimeInForce),
	}
}

func mapOrderFill(raw sdk.OrderUpdateEvent) *exchanges.Fill {
	qty := parseDecimal(raw.FillQuantity)
	if qty.IsZero() {
		return nil
	}

	ts := raw.EngineTimestamp
	if ts == 0 {
		ts = raw.EventTime
	}

	return &exchanges.Fill{
		TradeID:       raw.TradeID.String(),
		OrderID:       raw.OrderID,
		ClientOrderID: raw.ClientID.String(),
		Symbol:        extractBaseSymbol(raw.Symbol),
		Side:          mapOrderSide(raw.Side),
		Price:         parseDecimal(raw.FillPrice),
		Quantity:      qty,
		Fee:           parseDecimal(raw.Fee),
		FeeAsset:      raw.FeeSymbol,
		IsMaker:       raw.IsMaker,
		Timestamp:     microsToMillis(ts),
	}
}

func mapSpotBalances(raw map[string]sdk.CapitalBalance) []exchanges.SpotBalance {
	balances := make([]exchanges.SpotBalance, 0, len(raw))
	for asset, balance := range raw {
		free := parseDecimal(balance.Available)
		locked := parseDecimal(balance.Locked)
		staked := parseDecimal(balance.Staked)
		balances = append(balances, exchanges.SpotBalance{
			Asset:  strings.ToUpper(asset),
			Free:   free,
			Locked: locked.Add(staked),
			Total:  free.Add(locked).Add(staked),
		})
	}
	return balances
}

func mapPosition(raw sdk.Position) exchanges.Position {
	qty := parseDecimal(raw.NetQuantity)
	side := exchanges.PositionSideLong
	if qty.IsNegative() {
		side = exchanges.PositionSideShort
		qty = qty.Abs()
	}
	return exchanges.Position{
		Symbol:           extractBaseSymbol(raw.Symbol),
		Side:             side,
		Quantity:         qty,
		EntryPrice:       parseDecimal(raw.EntryPrice),
		UnrealizedPnL:    parseDecimal(raw.PnlUnrealized),
		RealizedPnL:      parseDecimal(raw.PnlRealized),
		LiquidationPrice: parseDecimal(raw.EstLiquidationPrice),
	}
}

func mapPositionUpdate(raw sdk.PositionUpdateEvent) *exchanges.Position {
	qty := parseDecimal(raw.NetQuantity.String())
	side := exchanges.PositionSideLong
	if qty.IsNegative() {
		side = exchanges.PositionSideShort
		qty = qty.Abs()
	}

	return &exchanges.Position{
		Symbol:        extractBaseSymbol(raw.Symbol),
		Side:          side,
		Quantity:      qty,
		EntryPrice:    parseDecimal(raw.EntryPrice.String()),
		UnrealizedPnL: parseDecimal(raw.PnlUnrealized.String()),
		RealizedPnL:   parseDecimal(raw.PnlRealized.String()),
	}
}

func extractBaseSymbol(symbol string) string {
	upper := strings.ToUpper(symbol)
	upper = strings.TrimSuffix(upper, "_PERP")
	parts := strings.Split(upper, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return upper
}

func mapOrderSide(raw string) exchanges.OrderSide {
	switch strings.ToUpper(raw) {
	case "ASK", "SELL":
		return exchanges.OrderSideSell
	default:
		return exchanges.OrderSideBuy
	}
}

func mapOrderType(raw string) exchanges.OrderType {
	switch strings.ToUpper(raw) {
	case "LIMIT":
		return exchanges.OrderTypeLimit
	case "MARKET":
		return exchanges.OrderTypeMarket
	default:
		return exchanges.OrderTypeUnknown
	}
}

func mapOrderStatus(raw string) exchanges.OrderStatus {
	switch strings.ToUpper(raw) {
	case "NEW", "OPEN", "ACCEPTED", "TRIGGERED", "ORDERACCEPTED", "ORDERMODIFIED", "TRIGGERPLACED":
		return exchanges.OrderStatusNew
	case "FILLED", "ORDERFILL":
		return exchanges.OrderStatusFilled
	case "CANCELLED", "CANCELED", "EXPIRED", "ORDERCANCELLED", "ORDEREXPIRED":
		return exchanges.OrderStatusCancelled
	case "REJECTED", "TRIGGERFAILED":
		return exchanges.OrderStatusRejected
	case "PARTIALLYFILLED", "PARTIALLY_FILLED":
		return exchanges.OrderStatusPartiallyFilled
	default:
		return exchanges.OrderStatusUnknown
	}
}

func mapTimeInForce(raw string) exchanges.TimeInForce {
	switch strings.ToUpper(raw) {
	case "IOC":
		return exchanges.TimeInForceIOC
	case "FOK":
		return exchanges.TimeInForceFOK
	case "POSTONLY", "PO":
		return exchanges.TimeInForcePO
	default:
		return exchanges.TimeInForceGTC
	}
}
