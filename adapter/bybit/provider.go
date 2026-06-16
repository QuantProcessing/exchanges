package bybit

import (
	"context"
	"fmt"

	"github.com/QuantProcessing/exchanges/model"
	bybitsdk "github.com/QuantProcessing/exchanges/sdk/bybit"
	"github.com/shopspring/decimal"
)

type sdkClient interface {
	GetInstruments(context.Context, string) ([]bybitsdk.Instrument, error)
	GetTicker(context.Context, string, string) (*bybitsdk.Ticker, error)
	GetOrderBook(context.Context, string, string, int) (*bybitsdk.OrderBook, error)
	GetFundingHistory(context.Context, string, string, int64, int64, int) ([]bybitsdk.FundingHistoryEntry, error)
	GetWalletBalance(context.Context, string, string) (*bybitsdk.WalletBalanceResult, error)
	PlaceOrder(context.Context, bybitsdk.PlaceOrderRequest) (*bybitsdk.OrderActionResponse, error)
	CancelOrder(context.Context, bybitsdk.CancelOrderRequest) (*bybitsdk.OrderActionResponse, error)
	GetOpenOrders(context.Context, string, string) ([]bybitsdk.OrderRecord, error)
}

type productProvider struct {
	sdk            sdkClient
	category       string
	instrumentType model.InstrumentType
	symbolSuffix   string
	insts          map[model.InstrumentID]model.Instrument
}

func newSpotProvider(sdk sdkClient) *productProvider {
	return newProductProvider(sdk, "spot", model.InstrumentTypeSpot, "SPOT")
}

func newLinearProvider(sdk sdkClient) *productProvider {
	return newProductProvider(sdk, "linear", model.InstrumentTypePerp, "PERP")
}

func newProductProvider(sdk sdkClient, category string, instrumentType model.InstrumentType, symbolSuffix string) *productProvider {
	return &productProvider{
		sdk:            sdk,
		category:       category,
		instrumentType: instrumentType,
		symbolSuffix:   symbolSuffix,
		insts:          make(map[model.InstrumentID]model.Instrument),
	}
}

func (p *productProvider) LoadAll(ctx context.Context) error {
	instruments, err := p.sdk.GetInstruments(ctx, p.category)
	if err != nil {
		return err
	}
	p.insts = make(map[model.InstrumentID]model.Instrument)
	for _, venueInst := range instruments {
		inst := p.mapInstrument(venueInst)
		if err := inst.Validate(); err != nil {
			return err
		}
		p.insts[inst.ID] = inst
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
	for id, inst := range p.insts {
		if inst.RawSymbol == raw {
			return id, true
		}
	}
	return model.InstrumentID{}, false
}

func (p *productProvider) mapInstrument(inst bybitsdk.Instrument) model.Instrument {
	settle := ""
	if p.instrumentType != model.InstrumentTypeSpot {
		settle = defaultString(inst.SettleCoin, inst.QuoteCoin)
	}
	return model.Instrument{
		ID:        model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-%s", inst.BaseCoin, inst.QuoteCoin, p.symbolSuffix), Venue: Venue},
		RawSymbol: inst.Symbol,
		Type:      p.instrumentType,
		Base:      model.Currency(inst.BaseCoin),
		Quote:     model.Currency(inst.QuoteCoin),
		Settle:    model.Currency(settle),
		PriceTick: decimal.RequireFromString(defaultString(inst.PriceFilter.TickSize, "0.00000001")),
		SizeTick:  decimal.RequireFromString(defaultString(defaultString(inst.LotSizeFilter.QtyStep, inst.LotSizeFilter.BasePrecision), "0.00000001")),
		Status:    mapInstrumentStatus(inst.Status),
	}
}
