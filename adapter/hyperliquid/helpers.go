package hyperliquid

import (
	"strconv"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	hlsdk "github.com/QuantProcessing/exchanges/sdk/hyperliquid"
	"github.com/shopspring/decimal"
)

func decimalTick(decimals int) decimal.Decimal {
	if decimals < 0 {
		return decimal.RequireFromString("0.00000001")
	}
	return decimal.New(1, -int32(decimals))
}

func decimalOrFallback(value, fallback string) decimal.Decimal {
	if value == "" {
		value = fallback
	}
	return decimal.RequireFromString(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func float64OrZero(value decimal.Decimal) float64 {
	out, _ := value.Float64()
	return out
}

func toTIF(tif model.TimeInForce) hlsdk.Tif {
	switch tif {
	case model.TimeInForceIOC:
		return hlsdk.TifIoc
	case model.TimeInForceFOK:
		return hlsdk.TifFok
	default:
		return hlsdk.TifGtc
	}
}

func sideFromWire(side string) model.OrderSide {
	if side == string(hlsdk.SideAsk) || side == "sell" {
		return model.OrderSideSell
	}
	if side == string(hlsdk.SideBid) || side == "buy" {
		return model.OrderSideBuy
	}
	return ""
}

func mapHLOrderStatus(status hlsdk.OrderStatusValue) model.OrderStatus {
	switch status {
	case hlsdk.StatusFilled:
		return model.OrderStatusFilled
	case hlsdk.StatusTriggered:
		return model.OrderStatusTriggered
	case hlsdk.StatusRejected, hlsdk.StatusTickRejected, hlsdk.StatusMinTradeNtlRejected:
		return model.OrderStatusRejected
	case hlsdk.StatusScheduledCancel:
		return model.OrderStatusPendingCancel
	case hlsdk.StatusCanceled,
		hlsdk.StatusMarginCanceled,
		hlsdk.StatusVaultWithdrawalCanceled,
		hlsdk.StatusOpenInterestCapCanceled,
		hlsdk.StatusSelfTradeCanceled,
		hlsdk.StatusReduceOnlyCanceled,
		hlsdk.StatusSiblingFilledCanceled,
		hlsdk.StatusDelistedCanceled,
		hlsdk.StatusLiquidatedCanceled:
		return model.OrderStatusCanceled
	default:
		return model.OrderStatusAccepted
	}
}

func parseOrderID(raw model.OrderID) int64 {
	value, _ := strconv.ParseInt(string(raw), 10, 64)
	return value
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

type hlMarketTopic struct {
	kind     model.MarketDataType
	coin     string
	interval string
}

func hlTopicFor(raw string, sub model.SubscribeMarketData) hlMarketTopic {
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		return hlMarketTopic{kind: model.MarketDataTypeTicker, coin: raw}
	case model.MarketDataTypeOrderBook:
		return hlMarketTopic{kind: model.MarketDataTypeOrderBook, coin: raw}
	case model.MarketDataTypeTradeTick:
		return hlMarketTopic{kind: model.MarketDataTypeTradeTick, coin: raw}
	case model.MarketDataTypeBar:
		return hlMarketTopic{kind: model.MarketDataTypeBar, coin: raw, interval: hlBarInterval(sub.BarType.Canonical().Step)}
	default:
		return hlMarketTopic{}
	}
}

func hlBarInterval(step time.Duration) string {
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

func hlAggressorSide(side string) model.AggressorSide {
	switch sideFromWire(side) {
	case model.OrderSideBuy:
		return model.AggressorSideBuyer
	case model.OrderSideSell:
		return model.AggressorSideSeller
	default:
		return model.AggressorSideNoAggressor
	}
}

func parseHLTime(raw int64) time.Time {
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
