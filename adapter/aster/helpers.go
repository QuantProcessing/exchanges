package aster

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

func decimalFromFilter(filters []map[string]interface{}, filterType, key, fallback string) decimal.Decimal {
	for _, filter := range filters {
		if fmt.Sprint(filter["filterType"]) == filterType {
			return decimal.RequireFromString(fmt.Sprint(filter[key]))
		}
	}
	return decimal.RequireFromString(fallback)
}

func mapTradingStatus(raw string) model.InstrumentStatus {
	if strings.EqualFold(raw, "TRADING") {
		return model.InstrumentStatusTrading
	}
	return model.InstrumentStatusHalted
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func zeroBlank(value decimal.Decimal) string {
	if value.IsZero() {
		return ""
	}
	return value.String()
}

func toVenueSide(side model.OrderSide) string {
	if side == model.OrderSideSell {
		return "SELL"
	}
	return "BUY"
}

func fromVenueSide(side string) model.OrderSide {
	if strings.EqualFold(side, "SELL") {
		return model.OrderSideSell
	}
	if strings.EqualFold(side, "BUY") {
		return model.OrderSideBuy
	}
	return ""
}

func toVenueType(orderType model.OrderType) string {
	if orderType == model.OrderTypeMarket {
		return "MARKET"
	}
	return "LIMIT"
}

func fromVenueType(orderType string) model.OrderType {
	if strings.EqualFold(orderType, "MARKET") {
		return model.OrderTypeMarket
	}
	if strings.EqualFold(orderType, "LIMIT") {
		return model.OrderTypeLimit
	}
	return ""
}

func toVenueTIF(tif model.TimeInForce) string {
	switch tif {
	case model.TimeInForceIOC:
		return "IOC"
	case model.TimeInForceFOK:
		return "FOK"
	default:
		return "GTC"
	}
}

func mapOrderStatus(status string) model.OrderStatus {
	switch strings.ToUpper(status) {
	case "FILLED":
		return model.OrderStatusFilled
	case "PARTIALLY_FILLED":
		return model.OrderStatusPartiallyFilled
	case "CANCELED":
		return model.OrderStatusCanceled
	case "REJECTED", "EXPIRED":
		return model.OrderStatusRejected
	default:
		return model.OrderStatusAccepted
	}
}

func parseBookLevel(raw []string) model.OrderBookLevel {
	return model.OrderBookLevel{
		Price: decimal.RequireFromString(raw[0]),
		Size:  decimal.RequireFromString(raw[1]),
	}
}

func parseBookLevelAny(raw []interface{}) model.OrderBookLevel {
	price := "0"
	size := "0"
	if len(raw) > 0 {
		price = fmt.Sprint(raw[0])
	}
	if len(raw) > 1 {
		size = fmt.Sprint(raw[1])
	}
	return model.OrderBookLevel{
		Price: decimal.RequireFromString(price),
		Size:  decimal.RequireFromString(size),
	}
}

func parseAsterTime(raw int64) time.Time {
	if raw <= 0 {
		return time.Now()
	}
	switch {
	case raw > 1_000_000_000_000_000:
		return time.UnixMicro(raw)
	case raw > 1_000_000_000_000:
		return time.UnixMilli(raw)
	case raw > 1_000_000_000:
		return time.Unix(raw, 0)
	default:
		return time.UnixMilli(raw)
	}
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func leavesQuantity(quantity, filled decimal.Decimal) decimal.Decimal {
	if quantity.IsZero() {
		return decimal.Zero
	}
	leaves := quantity.Sub(filled)
	if leaves.IsNegative() {
		return decimal.Zero
	}
	return leaves
}

func filledQuantity(quantity, leaves decimal.Decimal) decimal.Decimal {
	if quantity.IsZero() {
		return decimal.Zero
	}
	filled := quantity.Sub(leaves)
	if filled.IsNegative() {
		return decimal.Zero
	}
	return filled
}

func positionSide(quantity decimal.Decimal) model.PositionSide {
	if quantity.IsNegative() {
		return model.PositionSideShort
	}
	if quantity.IsPositive() {
		return model.PositionSideLong
	}
	return model.PositionSideFlat
}

func asterAggressorSide(isBuyerMaker bool) model.AggressorSide {
	if isBuyerMaker {
		return model.AggressorSideSeller
	}
	return model.AggressorSideBuyer
}

func asterBarInterval(step time.Duration) string {
	switch step {
	case time.Minute:
		return "1m"
	case 3 * time.Minute:
		return "3m"
	case 5 * time.Minute:
		return "5m"
	case 15 * time.Minute:
		return "15m"
	case 30 * time.Minute:
		return "30m"
	case time.Hour:
		return "1h"
	case 2 * time.Hour:
		return "2h"
	case 4 * time.Hour:
		return "4h"
	case 6 * time.Hour:
		return "6h"
	case 8 * time.Hour:
		return "8h"
	case 12 * time.Hour:
		return "12h"
	case 24 * time.Hour:
		return "1d"
	case 3 * 24 * time.Hour:
		return "3d"
	case 7 * 24 * time.Hour:
		return "1w"
	default:
		return "1m"
	}
}

func asterBookDepth(depth int) int {
	switch {
	case depth <= 5:
		return 5
	case depth <= 10:
		return 10
	default:
		return 20
	}
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}
