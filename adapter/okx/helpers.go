package okx

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	okxsdk "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/shopspring/decimal"
)

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func zeroBlank(value decimal.Decimal) *string {
	if value.IsZero() {
		return nil
	}
	raw := value.String()
	return &raw
}

func decimalOrFallback(value, fallback string) decimal.Decimal {
	return decimal.RequireFromString(defaultString(value, fallback))
}

func okxAggressorSide(side string) model.AggressorSide {
	switch strings.ToLower(side) {
	case "buy":
		return model.AggressorSideBuyer
	case "sell":
		return model.AggressorSideSeller
	default:
		return model.AggressorSideNoAggressor
	}
}

func okxBarChannel(step time.Duration) string {
	switch step {
	case time.Minute:
		return "candle1m"
	case 3 * time.Minute:
		return "candle3m"
	case 5 * time.Minute:
		return "candle5m"
	case 15 * time.Minute:
		return "candle15m"
	case 30 * time.Minute:
		return "candle30m"
	case time.Hour:
		return "candle1H"
	case 2 * time.Hour:
		return "candle2H"
	case 4 * time.Hour:
		return "candle4H"
	case 6 * time.Hour:
		return "candle6H"
	case 12 * time.Hour:
		return "candle12H"
	case 24 * time.Hour:
		return "candle1D"
	case 7 * 24 * time.Hour:
		return "candle1W"
	default:
		return "candle1m"
	}
}

func mapInstrumentStatus(raw string) model.InstrumentStatus {
	if strings.EqualFold(raw, "live") {
		return model.InstrumentStatusTrading
	}
	return model.InstrumentStatusHalted
}

func splitPair(raw string) (string, string, error) {
	parts := strings.Split(raw, "-")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("%w: invalid OKX instrument pair %q", model.ErrInvalidInstrument, raw)
	}
	return parts[0], parts[1], nil
}

func toVenueSide(side model.OrderSide) string {
	if side == model.OrderSideSell {
		return string(okxsdk.SideSell)
	}
	return string(okxsdk.SideBuy)
}

func fromVenueSide(side okxsdk.Side) model.OrderSide {
	switch side {
	case okxsdk.SideSell:
		return model.OrderSideSell
	case okxsdk.SideBuy:
		return model.OrderSideBuy
	default:
		return ""
	}
}

func toVenueOrderType(orderType model.OrderType, tif model.TimeInForce) string {
	if orderType == model.OrderTypeMarket {
		return string(okxsdk.OrderTypeMarket)
	}
	switch tif {
	case model.TimeInForceIOC:
		return string(okxsdk.OrderTypeIoc)
	case model.TimeInForceFOK:
		return string(okxsdk.OrderTypeFok)
	default:
		return string(okxsdk.OrderTypeLimit)
	}
}

func fromVenueOrderType(orderType okxsdk.OrderType) model.OrderType {
	if orderType == okxsdk.OrderTypeMarket {
		return model.OrderTypeMarket
	}
	if orderType != "" {
		return model.OrderTypeLimit
	}
	return ""
}

func mapOrderStatus(status okxsdk.OrderStatus) model.OrderStatus {
	switch status {
	case okxsdk.OrderStatusFilled:
		return model.OrderStatusFilled
	case okxsdk.OrderStatusPartiallyFilled:
		return model.OrderStatusPartiallyFilled
	case okxsdk.OrderStatusCanceled, okxsdk.OrderStatusMmpCanceled:
		return model.OrderStatusCanceled
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

func parseUnixMillis(raw string) time.Time {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return time.Now()
	}
	return time.UnixMilli(value)
}
