package edgex

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	edgexperp "github.com/QuantProcessing/exchanges/sdk/edgex/perp"
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

func edgeXInitialMargin(contract edgexperp.Contract) (decimal.Decimal, error) {
	leverage := firstNonEmpty(firstRiskTierMaxLeverage(contract), contract.DisplayMaxLeverage, contract.DefaultLeverage)
	if leverage == "" {
		return decimal.Zero, nil
	}
	value, err := decimal.NewFromString(leverage)
	if err != nil {
		return decimal.Zero, err
	}
	if !value.IsPositive() {
		return decimal.Zero, nil
	}
	return decimal.NewFromInt(1).Div(value), nil
}

func edgeXMaintenanceMargin(contract edgexperp.Contract) (decimal.Decimal, error) {
	if len(contract.RiskTierList) == 0 || contract.RiskTierList[0].MaintenanceMarginRate == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(contract.RiskTierList[0].MaintenanceMarginRate)
}

func firstRiskTierMaxLeverage(contract edgexperp.Contract) string {
	if len(contract.RiskTierList) == 0 {
		return ""
	}
	return contract.RiskTierList[0].MaxLeverage
}

func mapInstrumentStatus(enabled bool) model.InstrumentStatus {
	if enabled {
		return model.InstrumentStatusTrading
	}
	return model.InstrumentStatusHalted
}

func toVenueSide(side model.OrderSide) string {
	if side == model.OrderSideSell {
		return string(edgexperp.SideSell)
	}
	return string(edgexperp.SideBuy)
}

func fromVenueSide(side edgexperp.Side) model.OrderSide {
	if side == edgexperp.SideSell {
		return model.OrderSideSell
	}
	if side == edgexperp.SideBuy {
		return model.OrderSideBuy
	}
	return ""
}

func toVenueOrderType(orderType model.OrderType) string {
	if orderType == model.OrderTypeMarket {
		return string(edgexperp.OrderTypeMarket)
	}
	return string(edgexperp.OrderTypeLimit)
}

func fromVenueOrderType(orderType edgexperp.OrderType) model.OrderType {
	if orderType == edgexperp.OrderTypeMarket {
		return model.OrderTypeMarket
	}
	if orderType == edgexperp.OrderTypeLimit {
		return model.OrderTypeLimit
	}
	return ""
}

func toVenueTIF(tif model.TimeInForce) string {
	switch tif {
	case model.TimeInForceIOC:
		return string(edgexperp.TimeInForceImmediateOrCancel)
	case model.TimeInForceFOK:
		return string(edgexperp.TimeInForceFillOrKill)
	default:
		return string(edgexperp.TimeInForceGoodTilCancel)
	}
}

func mapOrderStatus(status edgexperp.OrderStatus) model.OrderStatus {
	switch {
	case status == edgexperp.OrderStatusFilled:
		return model.OrderStatusFilled
	case status == edgexperp.OrderStatusCanceled:
		return model.OrderStatusCanceled
	case strings.EqualFold(string(status), "PARTIALLY_FILLED"):
		return model.OrderStatusPartiallyFilled
	case status == edgexperp.OrderStatusCanceling:
		return model.OrderStatusPendingCancel
	case status == edgexperp.OrderStatusUnknown || status == edgexperp.OrderStatusUnrecognized:
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

func edgeXBookDepth(depth int) edgexperp.OrderBookDepth {
	if depth <= int(edgexperp.OrderBookDepth15) {
		return edgexperp.OrderBookDepth15
	}
	return edgexperp.OrderBookDepth200
}

func edgeXTopicFor(contractID string, sub model.SubscribeMarketData) edgeXMarketTopic {
	switch sub.Type {
	case model.MarketDataTypeTicker:
		return edgeXMarketTopic{kind: model.MarketDataTypeTicker, contract: contractID}
	case model.MarketDataTypeOrderBook:
		return edgeXMarketTopic{kind: model.MarketDataTypeOrderBook, contract: contractID, depth: edgeXBookDepth(sub.Depth)}
	case model.MarketDataTypeQuoteTick:
		return edgeXMarketTopic{kind: model.MarketDataTypeOrderBook, contract: contractID, depth: edgexperp.OrderBookDepth15}
	case model.MarketDataTypeTradeTick:
		return edgeXMarketTopic{kind: model.MarketDataTypeTradeTick, contract: contractID}
	case model.MarketDataTypeBar:
		return edgeXMarketTopic{kind: model.MarketDataTypeBar, contract: contractID, priceType: edgexperp.PriceTypeLastPrice, interval: edgeXKlineInterval(sub.BarType.Canonical().Step)}
	default:
		return edgeXMarketTopic{}
	}
}

func edgeXKlineInterval(step time.Duration) edgexperp.KlineInterval {
	switch step {
	case time.Minute:
		return edgexperp.KlineInterval1m
	case 5 * time.Minute:
		return edgexperp.KlineInterval5m
	case 15 * time.Minute:
		return edgexperp.KlineInterval15m
	case 30 * time.Minute:
		return edgexperp.KlineInterval30m
	case time.Hour:
		return edgexperp.KlineInterval1h
	case 2 * time.Hour:
		return edgexperp.KlineInterval2h
	case 4 * time.Hour:
		return edgexperp.KlineInterval4h
	case 6 * time.Hour:
		return edgexperp.KlineInterval6h
	case 8 * time.Hour:
		return edgexperp.KlineInterval8h
	case 12 * time.Hour:
		return edgexperp.KlineInterval12h
	case 24 * time.Hour:
		return edgexperp.KlineInterval1d
	case 7 * 24 * time.Hour:
		return edgexperp.KlineInterval1w
	default:
		return edgexperp.KlineInterval1m
	}
}

func edgeXAggressorSide(isBuyerMaker bool) model.AggressorSide {
	if isBuyerMaker {
		return model.AggressorSideSeller
	}
	return model.AggressorSideBuyer
}

func parseEdgeXTime(raw string) time.Time {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
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
