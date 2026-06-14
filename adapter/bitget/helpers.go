package bitget

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
	if strings.EqualFold(raw, "online") || strings.EqualFold(raw, "trading") {
		return model.InstrumentStatusTrading
	}
	return model.InstrumentStatusHalted
}

func toVenueSide(side model.OrderSide) string {
	if side == model.OrderSideSell {
		return "sell"
	}
	return "buy"
}

func fromVenueSide(side string) model.OrderSide {
	if strings.EqualFold(side, "sell") {
		return model.OrderSideSell
	}
	if strings.EqualFold(side, "buy") {
		return model.OrderSideBuy
	}
	return ""
}

func toVenueOrderType(orderType model.OrderType) string {
	if orderType == model.OrderTypeMarket {
		return "market"
	}
	return "limit"
}

func fromVenueOrderType(orderType string) model.OrderType {
	if strings.EqualFold(orderType, "market") {
		return model.OrderTypeMarket
	}
	if strings.EqualFold(orderType, "limit") {
		return model.OrderTypeLimit
	}
	return ""
}

func toVenueTIF(tif model.TimeInForce) string {
	switch tif {
	case model.TimeInForceIOC:
		return "ioc"
	case model.TimeInForceFOK:
		return "fok"
	default:
		return "gtc"
	}
}

func mapOrderStatus(status string) model.OrderStatus {
	switch strings.ToLower(status) {
	case "filled", "full_fill":
		return model.OrderStatusFilled
	case "partial_filled", "partially_filled":
		return model.OrderStatusPartiallyFilled
	case "cancelled", "canceled":
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

func bitgetAggressorSide(side string) model.AggressorSide {
	switch strings.ToLower(side) {
	case "buy":
		return model.AggressorSideBuyer
	case "sell":
		return model.AggressorSideSeller
	default:
		return model.AggressorSideNoAggressor
	}
}

func bitgetBarInterval(step time.Duration) string {
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
		return "1H"
	case 4 * time.Hour:
		return "4H"
	case 6 * time.Hour:
		return "6H"
	case 12 * time.Hour:
		return "12H"
	case 24 * time.Hour:
		return "1D"
	case 7 * 24 * time.Hour:
		return "1W"
	default:
		return "1m"
	}
}
