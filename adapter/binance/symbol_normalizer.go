package binance

import (
	"fmt"
	"strings"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

type symbolNormalizer struct{}

func (symbolNormalizer) ToInstrumentID(raw string, hint venue.ProductHint) (model.InstrumentID, error) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	base, quote, ok := splitBinanceSymbol(raw)
	if !ok {
		return model.InstrumentID{}, fmt.Errorf("%w: %s", model.ErrInvalidInstrumentID, raw)
	}

	product := "SPOT"
	switch hint {
	case venue.ProductHintSpot, "":
		product = "SPOT"
	case venue.ProductHintPerp:
		product = "PERP"
	case venue.ProductHintOption:
		product = "OPTION"
	default:
		return model.InstrumentID{}, fmt.Errorf("%w: unsupported binance product hint %q", model.ErrInvalidInstrumentID, hint)
	}

	return model.ParseInstrumentID(base + "-" + quote + "-" + product + ".BINANCE")
}

func (symbolNormalizer) ToVenueSymbol(id model.InstrumentID) (string, error) {
	if id.Venue != model.VenueBinance {
		return "", fmt.Errorf("%w: %s", model.ErrInvalidInstrumentID, id.String())
	}
	parts := strings.Split(id.Symbol, "-")
	if len(parts) < 3 {
		return "", fmt.Errorf("%w: %s", model.ErrInvalidInstrumentID, id.String())
	}
	return parts[0] + parts[1], nil
}

func splitBinanceSymbol(raw string) (base, quote string, ok bool) {
	for _, q := range []string{"USDT", "USDC", "BUSD", "FDUSD", "BTC", "ETH"} {
		if strings.HasSuffix(raw, q) && len(raw) > len(q) {
			return raw[:len(raw)-len(q)], q, true
		}
	}
	return "", "", false
}
