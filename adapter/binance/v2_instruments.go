package binance

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

var _ venue.InstrumentProvider = (*v2InstrumentProvider)(nil)

type binanceSpotExchangeInfoClient interface {
	ExchangeInfo(ctx context.Context) (*spot.ExchangeInfoResponse, error)
}

type binancePerpExchangeInfoClient interface {
	ExchangeInfo(ctx context.Context) (*perp.ExchangeInfoResponse, error)
}

type v2InstrumentSeed struct {
	RawSymbol string
	Product   venue.ProductHint
	Base      model.Currency
	Quote     model.Currency
}

type v2InstrumentProvider struct {
	mu         sync.RWMutex
	normalizer v2SymbolNormalizer
	spot       binanceSpotExchangeInfoClient
	perp       binancePerpExchangeInfoClient
	seeds      []v2InstrumentSeed
	cache      map[model.InstrumentID]model.Instrument
}

func newV2InstrumentProvider(spotClient binanceSpotExchangeInfoClient, perpClient binancePerpExchangeInfoClient) *v2InstrumentProvider {
	return &v2InstrumentProvider{
		spot:  spotClient,
		perp:  perpClient,
		cache: make(map[model.InstrumentID]model.Instrument),
	}
}

func newV2InstrumentProviderForTest(seeds []v2InstrumentSeed) *v2InstrumentProvider {
	return &v2InstrumentProvider{
		seeds: seeds,
		cache: make(map[model.InstrumentID]model.Instrument),
	}
}

func (p *v2InstrumentProvider) LoadAll(ctx context.Context) error {
	instruments := make([]model.Instrument, 0)
	for _, seed := range p.seeds {
		inst, err := p.instrumentFromSeed(seed)
		if err != nil {
			return err
		}
		instruments = append(instruments, inst)
	}
	if p.spot != nil {
		info, err := p.spot.ExchangeInfo(ctx)
		if err != nil {
			return err
		}
		for _, s := range info.Symbols {
			inst, err := p.instrumentFromSpotSymbol(s)
			if err != nil {
				return err
			}
			if inst.ID != (model.InstrumentID{}) {
				instruments = append(instruments, inst)
			}
		}
	}
	if p.perp != nil {
		info, err := p.perp.ExchangeInfo(ctx)
		if err != nil {
			return err
		}
		for _, s := range info.Symbols {
			inst, err := p.instrumentFromPerpSymbol(s)
			if err != nil {
				return err
			}
			if inst.ID != (model.InstrumentID{}) {
				instruments = append(instruments, inst)
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

func (p *v2InstrumentProvider) Load(ctx context.Context, id model.InstrumentID) (model.Instrument, error) {
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

func (p *v2InstrumentProvider) Find(ctx context.Context, q venue.InstrumentQuery) ([]model.Instrument, error) {
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

func (p *v2InstrumentProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	inst, ok := p.cache[id]
	return inst, ok
}

func (p *v2InstrumentProvider) List() []model.Instrument {
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

func (p *v2InstrumentProvider) instrumentFromSeed(seed v2InstrumentSeed) (model.Instrument, error) {
	id, err := p.normalizer.ToInstrumentID(seed.RawSymbol, seed.Product)
	if err != nil {
		return model.Instrument{}, err
	}
	instType := model.InstrumentTypeCurrencyPair
	settle := seed.Quote
	if seed.Product == venue.ProductHintPerp {
		instType = model.InstrumentTypeCryptoPerp
	}
	inst := model.Instrument{
		ID:        id,
		RawSymbol: seed.RawSymbol,
		Type:      instType,
		Base:      seed.Base,
		Quote:     seed.Quote,
		Settle:    settle,
		PriceStep: decimal.RequireFromString("0.01"),
		SizeStep:  decimal.RequireFromString("0.001"),
		MinQty:    decimal.RequireFromString("0.001"),
	}
	if err := inst.Validate(); err != nil {
		return model.Instrument{}, err
	}
	return inst, nil
}

func (p *v2InstrumentProvider) instrumentFromSpotSymbol(s spot.SymbolInfo) (model.Instrument, error) {
	if s.Status != "" && s.Status != "TRADING" {
		return model.Instrument{}, nil
	}
	id, err := p.normalizer.ToInstrumentID(s.Symbol, venue.ProductHintSpot)
	if err != nil {
		return model.Instrument{}, err
	}
	priceStep := decimalFromFilter(s.Filters, "PRICE_FILTER", "tickSize", decimal.NewFromInt(1))
	sizeStep := decimalFromFilter(s.Filters, "LOT_SIZE", "stepSize", decimal.NewFromInt(1))
	inst := model.Instrument{
		ID:          id,
		RawSymbol:   s.Symbol,
		Type:        model.InstrumentTypeCurrencyPair,
		Base:        model.Currency(s.BaseAsset),
		Quote:       model.Currency(s.QuoteAsset),
		Settle:      model.Currency(s.QuoteAsset),
		PriceStep:   priceStep,
		SizeStep:    sizeStep,
		PricePrec:   countDecimalPlacesDecimal(priceStep),
		SizePrec:    countDecimalPlacesDecimal(sizeStep),
		MinQty:      decimalFromFilter(s.Filters, "LOT_SIZE", "minQty", decimal.Zero),
		MinNotional: model.Money{Amount: decimalFromFilter(s.Filters, "NOTIONAL", "minNotional", decimal.Zero), Currency: model.Currency(s.QuoteAsset)},
	}
	if err := inst.Validate(); err != nil {
		return model.Instrument{}, err
	}
	return inst, nil
}

func (p *v2InstrumentProvider) instrumentFromPerpSymbol(s perp.SymbolInfo) (model.Instrument, error) {
	if s.Status != "" && s.Status != "TRADING" {
		return model.Instrument{}, nil
	}
	if s.ContractType != "" && s.ContractType != "PERPETUAL" {
		return model.Instrument{}, nil
	}
	id, err := p.normalizer.ToInstrumentID(s.Symbol, venue.ProductHintPerp)
	if err != nil {
		return model.Instrument{}, err
	}
	priceStep := decimalFromFilter(s.Filters, "PRICE_FILTER", "tickSize", decimal.NewFromInt(1))
	sizeStep := decimalFromFilter(s.Filters, "LOT_SIZE", "stepSize", decimal.NewFromInt(1))
	settle := model.Currency(s.MarginAsset)
	if settle == "" {
		settle = model.Currency(s.QuoteAsset)
	}
	inst := model.Instrument{
		ID:          id,
		RawSymbol:   s.Symbol,
		Type:        model.InstrumentTypeCryptoPerp,
		Base:        model.Currency(s.BaseAsset),
		Quote:       model.Currency(s.QuoteAsset),
		Settle:      settle,
		PriceStep:   priceStep,
		SizeStep:    sizeStep,
		PricePrec:   int32(s.PricePrecision),
		SizePrec:    int32(s.QuantityPrecision),
		MinQty:      decimalFromFilter(s.Filters, "LOT_SIZE", "minQty", decimal.Zero),
		MinNotional: model.Money{Amount: decimalFromFilter(s.Filters, "MIN_NOTIONAL", "notional", decimal.Zero), Currency: settle},
		MarginInit:  parseDecimal(s.RequiredMarginPercent),
		MarginMaint: parseDecimal(s.MaintMarginPercent),
	}
	if err := inst.Validate(); err != nil {
		return model.Instrument{}, err
	}
	return inst, nil
}

func decimalFromFilter(filters []map[string]interface{}, filterType, field string, fallback decimal.Decimal) decimal.Decimal {
	for _, filter := range filters {
		if got, _ := filter["filterType"].(string); got != filterType {
			continue
		}
		switch v := filter[field].(type) {
		case string:
			return parseDecimal(v)
		case float64:
			return decimal.NewFromFloat(v)
		case int:
			return decimal.NewFromInt(int64(v))
		case int64:
			return decimal.NewFromInt(v)
		}
	}
	return fallback
}

func countDecimalPlacesDecimal(d decimal.Decimal) int32 {
	return int32(-d.Exponent())
}
