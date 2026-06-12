package okx

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/model"
	sdkokx "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

var _ venue.InstrumentProvider = (*instrumentProvider)(nil)

type okxInstrumentClient interface {
	GetInstruments(ctx context.Context, instType string) ([]sdkokx.Instrument, error)
}

type instrumentSeed struct {
	RawSymbol string
	Product   venue.ProductHint
	Base      model.Currency
	Quote     model.Currency
	Settle    model.Currency
	CtVal     decimal.Decimal
}

type instrumentProvider struct {
	mu         sync.RWMutex
	normalizer symbolNormalizer
	client     okxInstrumentClient
	seeds      []instrumentSeed
	cache      map[model.InstrumentID]model.Instrument
}

func newInstrumentProvider(client okxInstrumentClient) *instrumentProvider {
	return &instrumentProvider{
		client: client,
		cache:  make(map[model.InstrumentID]model.Instrument),
	}
}

func newInstrumentProviderForTest(seeds []instrumentSeed) *instrumentProvider {
	return &instrumentProvider{
		seeds: seeds,
		cache: make(map[model.InstrumentID]model.Instrument),
	}
}

func (p *instrumentProvider) LoadAll(ctx context.Context) error {
	instruments := make([]model.Instrument, 0)
	for _, seed := range p.seeds {
		inst, err := p.instrumentFromSeed(seed)
		if err != nil {
			return err
		}
		instruments = append(instruments, inst)
	}
	if p.client != nil {
		for _, instType := range []string{"SPOT", "SWAP"} {
			raw, err := p.client.GetInstruments(ctx, instType)
			if err != nil {
				return err
			}
			for _, s := range raw {
				inst, err := p.instrumentFromSDK(s, instType)
				if err != nil {
					return err
				}
				if inst.ID != (model.InstrumentID{}) {
					instruments = append(instruments, inst)
				}
			}
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, inst := range instruments {
		p.cache[inst.ID] = inst
	}
	return nil
}

func (p *instrumentProvider) Load(ctx context.Context, id model.InstrumentID) (model.Instrument, error) {
	if inst, ok := p.Get(id); ok {
		return inst, nil
	}
	if err := p.LoadAll(ctx); err != nil {
		return model.Instrument{}, err
	}
	if inst, ok := p.Get(id); ok {
		return inst, nil
	}
	return model.Instrument{}, fmt.Errorf("%w: %s", model.ErrInstrumentNotLoaded, id.String())
}

func (p *instrumentProvider) Find(ctx context.Context, q venue.InstrumentQuery) ([]model.Instrument, error) {
	if len(p.List()) == 0 {
		if err := p.LoadAll(ctx); err != nil {
			return nil, err
		}
	}
	out := make([]model.Instrument, 0)
	for _, inst := range p.List() {
		if q.Venue != "" && inst.ID.Venue != q.Venue {
			continue
		}
		if q.Type != "" && inst.Type != q.Type {
			continue
		}
		if q.Base != "" && inst.Base != q.Base {
			continue
		}
		if q.Quote != "" && inst.Quote != q.Quote {
			continue
		}
		if q.Settle != "" && inst.Settle != q.Settle {
			continue
		}
		out = append(out, inst)
	}
	return out, nil
}

func (p *instrumentProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	inst, ok := p.cache[id]
	return inst, ok
}

func (p *instrumentProvider) List() []model.Instrument {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]model.Instrument, 0, len(p.cache))
	for _, inst := range p.cache {
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID.String() < out[j].ID.String()
	})
	return out
}

func (p *instrumentProvider) instrumentFromSeed(seed instrumentSeed) (model.Instrument, error) {
	id, err := p.normalizer.ToInstrumentID(seed.RawSymbol, seed.Product)
	if err != nil {
		return model.Instrument{}, err
	}
	instType := model.InstrumentTypeCurrencyPair
	settle := seed.Quote
	multiplier := seed.CtVal
	if multiplier.IsZero() {
		multiplier = decimal.NewFromInt(1)
	}
	sizeStep := decimal.RequireFromString("0.001")
	minQty := decimal.RequireFromString("0.001")
	if seed.Product == venue.ProductHintPerp {
		instType = model.InstrumentTypeCryptoPerp
		sizeStep = multiplier
		minQty = multiplier
	}
	if seed.Settle != "" {
		settle = seed.Settle
	}
	inst := model.Instrument{
		ID:         id,
		RawSymbol:  seed.RawSymbol,
		Type:       instType,
		Base:       seed.Base,
		Quote:      seed.Quote,
		Settle:     settle,
		PriceStep:  decimal.RequireFromString("0.01"),
		SizeStep:   sizeStep,
		PricePrec:  2,
		SizePrec:   int32(exchanges.CountDecimalPlaces(sizeStep.String())),
		Multiplier: multiplier,
		MinQty:     minQty,
	}
	if err := inst.Validate(); err != nil {
		return model.Instrument{}, err
	}
	return inst, nil
}

func (p *instrumentProvider) instrumentFromSDK(s sdkokx.Instrument, fallbackType string) (model.Instrument, error) {
	if s.State != "" && s.State != "live" {
		return model.Instrument{}, nil
	}
	rawType := strings.ToUpper(strings.TrimSpace(s.InstType))
	if rawType == "" {
		rawType = fallbackType
	}
	product := venue.ProductHintSpot
	instType := model.InstrumentTypeCurrencyPair
	if rawType == "SWAP" {
		product = venue.ProductHintPerp
		instType = model.InstrumentTypeCryptoPerp
	}
	id, err := p.normalizer.ToInstrumentID(s.InstId, product)
	if err != nil {
		return model.Instrument{}, err
	}
	parts := strings.Split(id.Symbol, "-")
	base := model.Currency(firstNonEmpty(s.BaseCcy, partAt(parts, 0)))
	quote := model.Currency(firstNonEmpty(s.QuoteCcy, partAt(parts, 1)))
	settle := model.Currency(firstNonEmpty(s.SettCcy, s.SettleCcy, string(quote)))
	priceStep := parseOrDefault(s.TickSz, decimal.NewFromInt(1))
	lotStep := parseOrDefault(s.LotSz, decimal.NewFromInt(1))
	minSize := parseOrDefault(s.MinSz, decimal.Zero)
	multiplier := parseOrDefault(s.CtVal, decimal.NewFromInt(1))

	sizeStep := lotStep
	minQty := minSize
	if instType == model.InstrumentTypeCryptoPerp {
		sizeStep = lotStep.Mul(multiplier)
		minQty = minSize.Mul(multiplier)
	}
	inst := model.Instrument{
		ID:         id,
		RawSymbol:  s.InstId,
		Type:       instType,
		Base:       base,
		Quote:      quote,
		Settle:     settle,
		PriceStep:  priceStep,
		SizeStep:   sizeStep,
		PricePrec:  int32(exchanges.CountDecimalPlaces(priceStep.String())),
		SizePrec:   int32(exchanges.CountDecimalPlaces(sizeStep.String())),
		Multiplier: multiplier,
		MinQty:     minQty,
	}
	if err := inst.Validate(); err != nil {
		return model.Instrument{}, err
	}
	return inst, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.ToUpper(strings.TrimSpace(value))
		}
	}
	return ""
}

func partAt(parts []string, idx int) string {
	if idx >= 0 && idx < len(parts) {
		return parts[idx]
	}
	return ""
}

func parseOrDefault(raw string, fallback decimal.Decimal) decimal.Decimal {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value := parseString(raw)
	if value.IsZero() && !strings.EqualFold(strings.TrimSpace(raw), "0") && !strings.EqualFold(strings.TrimSpace(raw), "0.0") {
		return fallback
	}
	return value
}
