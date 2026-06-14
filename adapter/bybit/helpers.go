package bybit

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func zeroBlank(value decimal.Decimal) string {
	if value.IsZero() {
		return ""
	}
	return value.String()
}

func decimalOrFallback(value, fallback string) decimal.Decimal {
	return decimal.RequireFromString(defaultString(value, fallback))
}

func mapInstrumentStatus(raw string) model.InstrumentStatus {
	if strings.EqualFold(raw, "Trading") {
		return model.InstrumentStatusTrading
	}
	return model.InstrumentStatusHalted
}

func toVenueSide(side model.OrderSide) string {
	if side == model.OrderSideSell {
		return "Sell"
	}
	return "Buy"
}

func fromVenueSide(side string) model.OrderSide {
	if strings.EqualFold(side, "Sell") {
		return model.OrderSideSell
	}
	if strings.EqualFold(side, "Buy") {
		return model.OrderSideBuy
	}
	return ""
}

func toVenueOrderType(orderType model.OrderType) string {
	if orderType == model.OrderTypeMarket {
		return "Market"
	}
	return "Limit"
}

func fromVenueOrderType(orderType string) model.OrderType {
	if strings.EqualFold(orderType, "Market") {
		return model.OrderTypeMarket
	}
	if strings.EqualFold(orderType, "Limit") {
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
	switch strings.ToLower(status) {
	case "filled":
		return model.OrderStatusFilled
	case "partiallyfilled", "partially_filled":
		return model.OrderStatusPartiallyFilled
	case "cancelled", "canceled", "deactivated":
		return model.OrderStatusCanceled
	case "rejected":
		return model.OrderStatusRejected
	default:
		return model.OrderStatusAccepted
	}
}

func parseUnixMillis(raw string) time.Time {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return time.Now()
	}
	return time.UnixMilli(value)
}

func parseUnixMillisInt(value int64) time.Time {
	if value <= 0 {
		return time.Now()
	}
	return time.UnixMilli(value)
}

func defaultInt64(value int64, fallback int64) int64 {
	if value != 0 {
		return value
	}
	return fallback
}

func bybitAggressorSide(side string) model.AggressorSide {
	switch strings.ToLower(side) {
	case "buy":
		return model.AggressorSideBuyer
	case "sell":
		return model.AggressorSideSeller
	default:
		return model.AggressorSideNoAggressor
	}
}

func bybitBarInterval(step time.Duration) string {
	switch step {
	case time.Minute:
		return "1"
	case 3 * time.Minute:
		return "3"
	case 5 * time.Minute:
		return "5"
	case 15 * time.Minute:
		return "15"
	case 30 * time.Minute:
		return "30"
	case time.Hour:
		return "60"
	case 2 * time.Hour:
		return "120"
	case 4 * time.Hour:
		return "240"
	case 6 * time.Hour:
		return "360"
	case 12 * time.Hour:
		return "720"
	case 24 * time.Hour:
		return "D"
	case 7 * 24 * time.Hour:
		return "W"
	default:
		return "1"
	}
}
