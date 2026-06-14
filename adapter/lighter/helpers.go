package lighter

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	lightersdk "github.com/QuantProcessing/exchanges/sdk/lighter"
	"github.com/shopspring/decimal"
)

func decimalTick(decimals uint8) decimal.Decimal {
	return decimal.New(1, -int32(decimals))
}

func decimalOrFallback(value, fallback string) decimal.Decimal {
	if value == "" {
		value = fallback
	}
	return decimal.RequireFromString(value)
}

func decimalFromString(value, fallback string) (decimal.Decimal, error) {
	if value == "" {
		value = fallback
	}
	return decimal.NewFromString(value)
}

func lighterFraction(value int) decimal.Decimal {
	if value <= 0 {
		return decimal.Zero
	}
	return decimal.NewFromInt(int64(value)).Div(decimal.NewFromInt(10000))
}

func splitSymbol(raw string) (string, string) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == '-' || r == '/' || r == '_' })
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return raw, "USDC"
}

func mapInstrumentStatus(raw string) model.InstrumentStatus {
	if strings.EqualFold(raw, "active") || strings.EqualFold(raw, "open") {
		return model.InstrumentStatusTrading
	}
	return model.InstrumentStatusHalted
}

func scaleDecimal(value decimal.Decimal, decimals uint8) int64 {
	return value.Shift(int32(decimals)).IntPart()
}

func toSide(side model.OrderSide) uint32 {
	if side == model.OrderSideSell {
		return 1
	}
	return 0
}

func fromSide(side string, isAsk bool) model.OrderSide {
	if isAsk || strings.EqualFold(side, "sell") || strings.EqualFold(side, "ask") {
		return model.OrderSideSell
	}
	if strings.EqualFold(side, "buy") || strings.EqualFold(side, "bid") {
		return model.OrderSideBuy
	}
	return model.OrderSideBuy
}

func toOrderType(orderType model.OrderType) uint32 {
	if orderType == model.OrderTypeMarket {
		return lightersdk.OrderTypeMarket
	}
	return lightersdk.OrderTypeLimit
}

func fromOrderType(orderType lightersdk.OrderTypeResp) model.OrderType {
	if orderType == lightersdk.OrderTypeRespMarket {
		return model.OrderTypeMarket
	}
	return model.OrderTypeLimit
}

func toTIF(tif model.TimeInForce) uint32 {
	if tif == model.TimeInForceIOC || tif == model.TimeInForceFOK {
		return lightersdk.OrderTimeInForceImmediateOrCancel
	}
	return lightersdk.OrderTimeInForceGoodTillTime
}

func mapOrderStatus(status lightersdk.OrderStatus) model.OrderStatus {
	switch status {
	case lightersdk.OrderStatusFilled:
		return model.OrderStatusFilled
	case lightersdk.OrderStatusPartiallyFilled:
		return model.OrderStatusPartiallyFilled
	case lightersdk.OrderStatusCanceled:
		return model.OrderStatusCanceled
	case lightersdk.OrderStatusRejected:
		return model.OrderStatusRejected
	default:
		return model.OrderStatusAccepted
	}
}

func parseInt64(raw string) int64 {
	value, _ := strconv.ParseInt(raw, 10, 64)
	return value
}

func parseInt(raw string) int {
	value, _ := strconv.Atoi(raw)
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

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
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

func positionSide(quantity decimal.Decimal) model.PositionSide {
	if quantity.IsNegative() {
		return model.PositionSideShort
	}
	if quantity.IsPositive() {
		return model.PositionSideLong
	}
	return model.PositionSideFlat
}

func lighterTopicFor(marketID int, sub model.SubscribeMarketData) lighterMarketTopic {
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		return lighterMarketTopic{kind: model.MarketDataTypeTicker, marketID: marketID}
	case model.MarketDataTypeOrderBook:
		return lighterMarketTopic{kind: model.MarketDataTypeOrderBook, marketID: marketID}
	case model.MarketDataTypeTradeTick:
		return lighterMarketTopic{kind: model.MarketDataTypeTradeTick, marketID: marketID}
	default:
		return lighterMarketTopic{}
	}
}

func lighterAggressorSide(isMakerAsk bool) model.AggressorSide {
	if isMakerAsk {
		return model.AggressorSideBuyer
	}
	return model.AggressorSideSeller
}

func idQuote(id model.InstrumentID) model.Currency {
	parts := strings.Split(id.Symbol, "-")
	if len(parts) >= 2 {
		return model.Currency(parts[1])
	}
	return model.Currency("USDC")
}

func parseLighterUnix(value int64) time.Time {
	if value <= 0 {
		return time.Now()
	}
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
