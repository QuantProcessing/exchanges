package okx

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantProcessing/exchanges/model"
	okxsdk "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/shopspring/decimal"
)

type sdkClient interface {
	GetInstruments(context.Context, string) ([]okxsdk.Instrument, error)
	GetTicker(context.Context, string) ([]okxsdk.Ticker, error)
	GetOrderBook(context.Context, string, *int) ([]okxsdk.OrderBook, error)
	GetAllFundingRates(context.Context) ([]okxsdk.FundingRate, error)
	GetAccountBalance(context.Context, *string) ([]okxsdk.Balance, error)
	GetPositions(context.Context, *string, *string) ([]okxsdk.Position, error)
	PlaceOrder(context.Context, *okxsdk.OrderRequest) ([]okxsdk.OrderId, error)
	CancelOrder(context.Context, string, string, string) ([]okxsdk.OrderId, error)
	GetOrders(context.Context, *string, *string) ([]okxsdk.Order, error)
}

type productProvider struct {
	sdk            sdkClient
	instType       string
	instrumentType model.InstrumentType
	symbolSuffix   string
	insts          map[model.InstrumentID]model.Instrument
	rawIndex       map[string]model.InstrumentID
}

func newSpotProvider(sdk sdkClient) *productProvider {
	return newProductProvider(sdk, "SPOT", model.InstrumentTypeSpot, "SPOT")
}

func newSwapProvider(sdk sdkClient) *productProvider {
	return newProductProvider(sdk, "SWAP", model.InstrumentTypePerp, "PERP")
}

func newProductProvider(sdk sdkClient, instType string, instrumentType model.InstrumentType, symbolSuffix string) *productProvider {
	return &productProvider{
		sdk:            sdk,
		instType:       instType,
		instrumentType: instrumentType,
		symbolSuffix:   symbolSuffix,
		insts:          make(map[model.InstrumentID]model.Instrument),
		rawIndex:       make(map[string]model.InstrumentID),
	}
}

func (p *productProvider) LoadAll(ctx context.Context) error {
	instruments, err := p.sdk.GetInstruments(ctx, p.instType)
	if err != nil {
		return err
	}
	p.insts = make(map[model.InstrumentID]model.Instrument)
	p.rawIndex = make(map[string]model.InstrumentID)
	for _, venueInst := range instruments {
		inst, err := p.mapInstrument(venueInst)
		if err != nil {
			return err
		}
		if err := inst.Validate(); err != nil {
			return err
		}
		p.insts[inst.ID] = inst
		p.rawIndex[inst.RawSymbol] = inst.ID
	}
	return nil
}

func (p *productProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	inst, ok := p.insts[id]
	return inst, ok
}

func (p *productProvider) List() []model.Instrument {
	out := make([]model.Instrument, 0, len(p.insts))
	for _, inst := range p.insts {
		out = append(out, inst)
	}
	return out
}

func (p *productProvider) rawSymbol(id model.InstrumentID) (string, error) {
	inst, ok := p.Get(id)
	if !ok {
		return "", fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	return inst.RawSymbol, nil
}

func (p *productProvider) instrumentIDByRaw(raw string) (model.InstrumentID, bool) {
	id, ok := p.rawIndex[raw]
	return id, ok
}

func (p *productProvider) mapInstrument(inst okxsdk.Instrument) (model.Instrument, error) {
	base := inst.BaseCcy
	quote := inst.QuoteCcy
	if base == "" || quote == "" {
		pair := defaultString(inst.Uly, defaultString(inst.InstFamily, strings.TrimSuffix(inst.InstId, "-SWAP")))
		var err error
		base, quote, err = splitPair(pair)
		if err != nil {
			return model.Instrument{}, err
		}
	}
	settle := ""
	if p.instrumentType != model.InstrumentTypeSpot {
		settle = defaultString(inst.SettleCcy, inst.SettCcy)
		if settle == "" {
			settle = quote
		}
	}
	mapped := model.Instrument{
		ID:        model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-%s", base, quote, p.symbolSuffix), Venue: Venue},
		RawSymbol: inst.InstId,
		Type:      p.instrumentType,
		Base:      model.Currency(base),
		Quote:     model.Currency(quote),
		Settle:    model.Currency(settle),
		PriceTick: decimal.RequireFromString(defaultString(inst.TickSz, "0.00000001")),
		SizeTick:  decimal.RequireFromString(defaultString(defaultString(inst.LotSz, inst.MinSz), "0.00000001")),
		Status:    mapInstrumentStatus(inst.State),
	}
	return mapped, nil
}
