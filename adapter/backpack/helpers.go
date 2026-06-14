package backpack

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	backpacksdk "github.com/QuantProcessing/exchanges/sdk/backpack"
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

func isPerp(marketType string) bool {
	switch strings.ToUpper(marketType) {
	case "PERP", "PERPETUAL", "FUTURE", "FUTURES":
		return true
	default:
		return false
	}
}

func mapInstrumentStatus(visible bool) model.InstrumentStatus {
	if visible {
		return model.InstrumentStatusTrading
	}
	return model.InstrumentStatusHalted
}

func totalBalance(balance backpacksdk.CapitalBalance) string {
	total := decimal.Zero
	for _, raw := range []string{balance.Available, balance.Locked, balance.Staked} {
		if raw == "" {
			continue
		}
		value, err := decimal.NewFromString(raw)
		if err != nil {
			continue
		}
		total = total.Add(value)
	}
	return total.String()
}

func toVenueSide(side model.OrderSide) string {
	if side == model.OrderSideSell {
		return "Ask"
	}
	return "Bid"
}

func fromVenueSide(side string) model.OrderSide {
	switch strings.ToLower(side) {
	case "ask", "sell":
		return model.OrderSideSell
	case "bid", "buy":
		return model.OrderSideBuy
	default:
		return ""
	}
}

func toVenueOrderType(orderType model.OrderType) string {
	if orderType == model.OrderTypeMarket {
		return "Market"
	}
	return "Limit"
}

func fromVenueOrderType(orderType string) model.OrderType {
	switch strings.ToLower(orderType) {
	case "market":
		return model.OrderTypeMarket
	case "limit":
		return model.OrderTypeLimit
	default:
		return ""
	}
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
	case "partiallyfilled", "partially_filled", "partial":
		return model.OrderStatusPartiallyFilled
	case "cancelled", "canceled", "expired":
		return model.OrderStatusCanceled
	case "rejected":
		return model.OrderStatusRejected
	default:
		return model.OrderStatusAccepted
	}
}

func parseUint32(raw string) uint32 {
	value, _ := strconv.ParseUint(raw, 10, 32)
	return uint32(value)
}

func clientOrderID(raw uint32) model.ClientOrderID {
	if raw == 0 {
		return ""
	}
	return model.ClientOrderID(fmt.Sprintf("%d", raw))
}

func backpackAggressorSide(isBuyerMaker bool) model.AggressorSide {
	if isBuyerMaker {
		return model.AggressorSideSeller
	}
	return model.AggressorSideBuyer
}

func backpackBarInterval(step time.Duration) string {
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
	case 7 * 24 * time.Hour:
		return "1w"
	default:
		return "1m"
	}
}

func parseBackpackTime(raw string) time.Time {
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
