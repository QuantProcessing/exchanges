package hyperliquid

import (
	"strconv"

	perpsdk "github.com/QuantProcessing/exchanges/hyperliquid/sdk/perp"
	spotsdk "github.com/QuantProcessing/exchanges/hyperliquid/sdk/spot"
)

func extractPerpOrderID(status *perpsdk.OrderStatus) string {
	if status == nil {
		return ""
	}
	if status.Resting != nil {
		return int64ToString(status.Resting.Oid)
	}
	if status.Filled != nil {
		return intToString(status.Filled.Oid)
	}
	return ""
}

func extractSpotOrderID(status *spotsdk.OrderStatus) string {
	if status == nil {
		return ""
	}
	if status.Resting != nil {
		return int64ToString(status.Resting.Oid)
	}
	if status.Filled != nil {
		return intToString(status.Filled.Oid)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func int64ToString(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

func intToString(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}
