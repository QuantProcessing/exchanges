package grvt

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	grvtsdk "github.com/QuantProcessing/exchanges/sdk/grvt"
	"github.com/shopspring/decimal"
)

func decimalFromString(value, fallback string) (decimal.Decimal, error) {
	if value == "" {
		value = fallback
	}
	return decimal.NewFromString(value)
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

func isPerp(inst grvtsdk.Instrument) bool {
	kind := strings.ToUpper(inst.Kind)
	return kind == "" || kind == "PERPETUAL" || kind == "PERP"
}

func toVenueTIF(tif model.TimeInForce) grvtsdk.TimeInForce {
	switch tif {
	case model.TimeInForceIOC:
		return grvtsdk.IOC
	case model.TimeInForceFOK:
		return grvtsdk.FOK
	default:
		return grvtsdk.GTT
	}
}

func fromVenueSide(isBuy bool) model.OrderSide {
	if isBuy {
		return model.OrderSideBuy
	}
	return model.OrderSideSell
}

func fromVenueOrderType(isMarket bool) model.OrderType {
	if isMarket {
		return model.OrderTypeMarket
	}
	return model.OrderTypeLimit
}

func mapOrderStatus(status grvtsdk.OrderStatus) model.OrderStatus {
	switch status {
	case grvtsdk.OrderStatusFilled:
		return model.OrderStatusFilled
	case grvtsdk.OrderStatusCancelled:
		return model.OrderStatusCanceled
	case grvtsdk.OrderStatusRejected:
		return model.OrderStatusRejected
	default:
		return model.OrderStatusAccepted
	}
}

func parseUint64(raw string) uint64 {
	value, _ := strconv.ParseUint(raw, 10, 64)
	return value
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

func grvtBookDepth(depth int) grvtsdk.OrderBookSnapDepth {
	switch {
	case depth <= int(grvtsdk.OrderBookSnapDepth10):
		return grvtsdk.OrderBookSnapDepth10
	case depth <= int(grvtsdk.OrderBookSnapDepth50):
		return grvtsdk.OrderBookSnapDepth50
	case depth <= int(grvtsdk.OrderBookSnapDepth100):
		return grvtsdk.OrderBookSnapDepth100
	default:
		return grvtsdk.OrderBookSnapDepth500
	}
}

func grvtTopicFor(raw string, sub model.SubscribeMarketData) grvtMarketTopic {
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		return grvtMarketTopic{kind: model.MarketDataTypeTicker, instrument: raw, tickerInterval: grvtsdk.TickerSnapRate1000}
	case model.MarketDataTypeOrderBook:
		return grvtMarketTopic{kind: model.MarketDataTypeOrderBook, instrument: raw, bookInterval: grvtsdk.OrderBookSnapRate1000, bookDepth: grvtBookDepth(sub.Depth)}
	case model.MarketDataTypeTradeTick:
		return grvtMarketTopic{kind: model.MarketDataTypeTradeTick, instrument: raw, tradeLimit: grvtsdk.TradeLimit50}
	case model.MarketDataTypeBar:
		return grvtMarketTopic{kind: model.MarketDataTypeBar, instrument: raw, klineInterval: grvtKlineInterval(sub.BarType.Canonical().Step), klineType: grvtsdk.KlineTypeTrade}
	default:
		return grvtMarketTopic{}
	}
}

func grvtKlineInterval(step time.Duration) grvtsdk.KlineInterval {
	switch step {
	case time.Minute:
		return grvtsdk.KlineInterval1m
	case 3 * time.Minute:
		return grvtsdk.KlineInterval3m
	case 5 * time.Minute:
		return grvtsdk.KlineInterval5m
	case 15 * time.Minute:
		return grvtsdk.KlineInterval15m
	case 30 * time.Minute:
		return grvtsdk.KlineInterval30m
	case time.Hour:
		return grvtsdk.KlineInterval1h
	case 2 * time.Hour:
		return grvtsdk.KlineInterval2h
	case 4 * time.Hour:
		return grvtsdk.KlineInterval4h
	case 8 * time.Hour:
		return grvtsdk.KlineInterval8h
	case 12 * time.Hour:
		return grvtsdk.KlineInterval12h
	case 24 * time.Hour:
		return grvtsdk.KlineInterval1d
	case 3 * 24 * time.Hour:
		return grvtsdk.KlineInterval3d
	case 5 * 24 * time.Hour:
		return grvtsdk.KlineInterval5d
	case 7 * 24 * time.Hour:
		return grvtsdk.KlineInterval1w
	case 14 * 24 * time.Hour:
		return grvtsdk.KlineInterval2w
	case 21 * 24 * time.Hour:
		return grvtsdk.KlineInterval3w
	case 28 * 24 * time.Hour:
		return grvtsdk.KlineInterval4w
	default:
		return grvtsdk.KlineInterval1m
	}
}

func grvtAggressorSide(isTakerBuyer bool) model.AggressorSide {
	if isTakerBuyer {
		return model.AggressorSideBuyer
	}
	return model.AggressorSideSeller
}

func parseGRVTTime(raw string) time.Time {
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
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts
	}
	return time.Now()
}
