package binance

import (
	"time"

	"github.com/QuantProcessing/exchanges/model"
)

func binanceAggressorSide(isBuyerMaker bool) model.AggressorSide {
	if isBuyerMaker {
		return model.AggressorSideSeller
	}
	return model.AggressorSideBuyer
}

func binanceBarInterval(step time.Duration) string {
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
	case 6 * time.Hour:
		return "6h"
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

func defaultInt64(value int64, fallback int64) int64 {
	if value != 0 {
		return value
	}
	return fallback
}
