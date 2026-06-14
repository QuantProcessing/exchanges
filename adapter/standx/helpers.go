package standx

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	standxsdk "github.com/QuantProcessing/exchanges/sdk/standx"
	"github.com/shopspring/decimal"
)

func decimalTick(decimals int) decimal.Decimal {
	return decimal.New(1, -int32(decimals))
}

func decimalFromString(value, fallback string) (decimal.Decimal, error) {
	if value == "" {
		value = fallback
	}
	return decimal.NewFromString(value)
}

func marginFromLeverage(value string) (decimal.Decimal, error) {
	if value == "" {
		return decimal.Zero, nil
	}
	leverage, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero, err
	}
	if !leverage.IsPositive() {
		return decimal.Zero, nil
	}
	return decimal.NewFromInt(1).Div(leverage), nil
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

func lockedAmount(total, free string) string {
	totalDecimal, err := decimal.NewFromString(firstNonEmpty(total, "0"))
	if err != nil {
		return ""
	}
	freeDecimal, err := decimal.NewFromString(firstNonEmpty(free, "0"))
	if err != nil {
		return ""
	}
	locked := totalDecimal.Sub(freeDecimal)
	if locked.IsNegative() {
		return "0"
	}
	return locked.String()
}

func mapInstrumentStatus(enabled bool) model.InstrumentStatus {
	if enabled {
		return model.InstrumentStatusTrading
	}
	return model.InstrumentStatusHalted
}

func toVenueSide(side model.OrderSide) standxsdk.OrderSide {
	if side == model.OrderSideSell {
		return standxsdk.SideSell
	}
	return standxsdk.SideBuy
}

func fromVenueSide(side string) model.OrderSide {
	if strings.EqualFold(side, string(standxsdk.SideSell)) {
		return model.OrderSideSell
	}
	if strings.EqualFold(side, string(standxsdk.SideBuy)) {
		return model.OrderSideBuy
	}
	return ""
}

func toVenueOrderType(orderType model.OrderType) standxsdk.OrderType {
	if orderType == model.OrderTypeMarket {
		return standxsdk.OrderTypeMarket
	}
	return standxsdk.OrderTypeLimit
}

func fromVenueOrderType(orderType string) model.OrderType {
	if strings.EqualFold(orderType, string(standxsdk.OrderTypeMarket)) {
		return model.OrderTypeMarket
	}
	if strings.EqualFold(orderType, string(standxsdk.OrderTypeLimit)) {
		return model.OrderTypeLimit
	}
	return ""
}

func toVenueTIF(tif model.TimeInForce) standxsdk.TimeInForce {
	switch tif {
	case model.TimeInForceIOC:
		return standxsdk.TimeInForceIOC
	case model.TimeInForceFOK:
		return standxsdk.TimeInForceFOK
	default:
		return standxsdk.TimeInForceGTC
	}
}

func mapOrderStatus(status string) model.OrderStatus {
	switch strings.ToLower(status) {
	case standxsdk.OrderStatusFilled:
		return model.OrderStatusFilled
	case standxsdk.OrderStatusCancelled, standxsdk.OrderStatusCanceled:
		return model.OrderStatusCanceled
	case standxsdk.OrderStatusRejected:
		return model.OrderStatusRejected
	default:
		return model.OrderStatusAccepted
	}
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

func positionSide(quantity decimal.Decimal) model.PositionSide {
	if quantity.IsNegative() {
		return model.PositionSideShort
	}
	if quantity.IsPositive() {
		return model.PositionSideLong
	}
	return model.PositionSideFlat
}

func standxTopicFor(raw string, sub model.SubscribeMarketData) standxMarketTopic {
	switch sub.Type {
	case model.MarketDataTypeTicker:
		return standxMarketTopic{kind: model.MarketDataTypeTicker, symbol: raw}
	case model.MarketDataTypeOrderBook, model.MarketDataTypeQuoteTick:
		return standxMarketTopic{kind: model.MarketDataTypeOrderBook, symbol: raw}
	case model.MarketDataTypeTradeTick:
		return standxMarketTopic{kind: model.MarketDataTypeTradeTick, symbol: raw}
	default:
		return standxMarketTopic{}
	}
}

func standxAggressorSide(isBuyerTaker bool) model.AggressorSide {
	if isBuyerTaker {
		return model.AggressorSideBuyer
	}
	return model.AggressorSideSeller
}

func parseStandXTime(raw string) time.Time {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err == nil && value > 0 {
		switch {
		case value > 1_000_000_000_000_000:
			return time.UnixMicro(value)
		case value > 1_000_000_000_000:
			return time.UnixMilli(value)
		case value > 1_000_000_000:
			return time.Unix(value, 0)
		default:
			return time.UnixMilli(value)
		}
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts
		}
	}
	return time.Now()
}
