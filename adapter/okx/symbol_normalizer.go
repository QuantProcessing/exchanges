package okx

import (
	"fmt"
	"strings"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

type symbolNormalizer struct{}

func (symbolNormalizer) ToInstrumentID(raw string, hint venue.ProductHint) (model.InstrumentID, error) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	parts := strings.Split(raw, "-")
	if len(parts) < 2 {
		return model.InstrumentID{}, fmt.Errorf("%w: %s", model.ErrInvalidInstrumentID, raw)
	}

	product := "SPOT"
	switch {
	case len(parts) >= 3 && parts[2] == "SWAP":
		product = "PERP"
	case hint == venue.ProductHintPerp:
		product = "PERP"
	case hint == venue.ProductHintSpot || hint == "":
		product = "SPOT"
	case hint == venue.ProductHintOption:
		product = "OPTION"
	default:
		return model.InstrumentID{}, fmt.Errorf("%w: unsupported okx product hint %q", model.ErrInvalidInstrumentID, hint)
	}

	return model.ParseInstrumentID(parts[0] + "-" + parts[1] + "-" + product + ".OKX")
}

func (symbolNormalizer) ToVenueSymbol(id model.InstrumentID) (string, error) {
	if id.Venue != model.VenueOKX {
		return "", fmt.Errorf("%w: %s", model.ErrInvalidInstrumentID, id.String())
	}
	parts := strings.Split(id.Symbol, "-")
	if len(parts) < 3 {
		return "", fmt.Errorf("%w: %s", model.ErrInvalidInstrumentID, id.String())
	}
	switch parts[2] {
	case "SPOT":
		return parts[0] + "-" + parts[1], nil
	case "PERP", "SWAP":
		return parts[0] + "-" + parts[1] + "-SWAP", nil
	default:
		return "", fmt.Errorf("%w: unsupported okx product %q", model.ErrInvalidInstrumentID, parts[2])
	}
}
