package bybit

import (
	"context"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/shopspring/decimal"
)

func toPlaceOrderRequest(ctx context.Context, adp exchanges.Exchange, category string, params *exchanges.OrderParams) (*sdk.PlaceOrderRequest, error) {
	_ = ctx

	clientID := params.ClientID
	if clientID == "" {
		clientID = exchanges.GenerateID()
	}

	req := &sdk.PlaceOrderRequest{
		Category:    category,
		Symbol:      adp.FormatSymbol(params.Symbol),
		Side:        toBybitOrderSide(params.Side),
		OrderType:   toBybitOrderType(params.Type),
		Qty:         params.Quantity.String(),
		OrderLinkID: clientID,
		ReduceOnly:  params.ReduceOnly,
	}
	if category == categorySpot && params.Type == exchanges.OrderTypeMarket && params.Side == exchanges.OrderSideBuy {
		req.MarketUnit = "baseCoin"
	}
	if params.Type == exchanges.OrderTypeMarket && params.Slippage.IsPositive() {
		req.SlippageToleranceType = "Percent"
		req.SlippageTolerance = params.Slippage.Mul(decimal.NewFromInt(100)).String()
	}
	if params.Type == exchanges.OrderTypeLimit || params.Type == exchanges.OrderTypePostOnly {
		req.Price = params.Price.String()
	}
	if tif := toBybitTimeInForce(params); tif != "" {
		req.TimeInForce = tif
	}
	return req, nil
}

func toBybitTimeInForce(params *exchanges.OrderParams) string {
	if params.Type == exchanges.OrderTypePostOnly {
		return "PostOnly"
	}
	switch params.TimeInForce {
	case exchanges.TimeInForceIOC:
		return "IOC"
	case exchanges.TimeInForceFOK:
		return "FOK"
	case exchanges.TimeInForcePO:
		return "PostOnly"
	case "", exchanges.TimeInForceGTC:
		if params.Type == exchanges.OrderTypeLimit {
			return "GTC"
		}
	}
	return ""
}

func toBybitOrderSide(side exchanges.OrderSide) string {
	if side == exchanges.OrderSideSell {
		return "Sell"
	}
	return "Buy"
}

func toBybitOrderType(orderType exchanges.OrderType) string {
	if orderType == exchanges.OrderTypeMarket {
		return "Market"
	}
	return "Limit"
}

func mapOrder(symbol string, raw sdk.OrderRecord) *exchanges.Order {
	ts := parseMillis(firstNonEmpty(raw.UpdatedTime, raw.CreatedTime))
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	return &exchanges.Order{
		OrderID:          raw.OrderID,
		ClientOrderID:    raw.OrderLinkID,
		Symbol:           strings.ToUpper(symbol),
		Side:             mapOrderSide(raw.Side),
		Type:             mapOrderType(raw.OrderType, raw.TimeInForce),
		Quantity:         parseDecimal(raw.Qty),
		Price:            parseDecimal(firstNonEmpty(raw.Price, raw.AvgPrice)),
		OrderPrice:       parseDecimal(raw.Price),
		AverageFillPrice: parseDecimal(raw.AvgPrice),
		Status:           mapOrderStatus(raw.OrderStatus),
		FilledQuantity:   parseDecimal(raw.CumExecQty),
		Timestamp:        ts,
		Fee:              parseDecimal(raw.CumExecFee),
		ReduceOnly:       raw.ReduceOnly,
		TimeInForce:      mapTimeInForce(raw.TimeInForce),
	}
}

func mapOrderSide(side string) exchanges.OrderSide {
	if strings.EqualFold(side, "sell") {
		return exchanges.OrderSideSell
	}
	return exchanges.OrderSideBuy
}

func mapOrderType(orderType, tif string) exchanges.OrderType {
	switch strings.ToLower(orderType) {
	case "market":
		return exchanges.OrderTypeMarket
	case "limit":
		if strings.EqualFold(tif, "PostOnly") {
			return exchanges.OrderTypePostOnly
		}
		return exchanges.OrderTypeLimit
	default:
		return exchanges.OrderTypeUnknown
	}
}

func mapTimeInForce(tif string) exchanges.TimeInForce {
	switch strings.ToUpper(tif) {
	case "IOC":
		return exchanges.TimeInForceIOC
	case "FOK":
		return exchanges.TimeInForceFOK
	case "POSTONLY":
		return exchanges.TimeInForcePO
	case "GTC":
		return exchanges.TimeInForceGTC
	default:
		return ""
	}
}

func mapOrderStatus(status string) exchanges.OrderStatus {
	switch strings.ToLower(status) {
	case "new", "created", "untriggered", "partiallyfilled":
		if strings.EqualFold(status, "PartiallyFilled") {
			return exchanges.OrderStatusPartiallyFilled
		}
		return exchanges.OrderStatusNew
	case "filled", "partiallyfilledcanceled", "cancelled", "deactivated":
		if strings.EqualFold(status, "Filled") {
			return exchanges.OrderStatusFilled
		}
		return exchanges.OrderStatusCancelled
	case "rejected":
		return exchanges.OrderStatusRejected
	default:
		return exchanges.OrderStatusUnknown
	}
}

func containsActiveOrder(order sdk.OrderRecord) bool {
	status := mapOrderStatus(order.OrderStatus)
	return status == exchanges.OrderStatusNew || status == exchanges.OrderStatusPartiallyFilled
}

func dedupeOrders(orders []exchanges.Order) []exchanges.Order {
	indexByID := make(map[string]int, len(orders))
	out := make([]exchanges.Order, 0, len(orders))
	for _, order := range orders {
		if order.OrderID == "" {
			continue
		}
		if idx, ok := indexByID[order.OrderID]; ok {
			out[idx] = order
			continue
		}
		indexByID[order.OrderID] = len(out)
		out = append(out, order)
	}
	return out
}
