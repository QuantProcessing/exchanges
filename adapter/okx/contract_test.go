package okx

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	sdkokx "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestContractSuites(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-PERP.OKX")
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{
			RawSymbol: "BTC-USDT-SWAP",
			Product:   venue.ProductHintPerp,
			Base:      model.BTC,
			Quote:     model.USDT,
			Settle:    model.USDT,
			CtVal:     parseString("0.01"),
		},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	inst, ok := provider.Get(instID)
	require.True(t, ok)

	fake := &fakeOKXVenueClient{
		tickers: []sdkokx.Ticker{{
			InstId: "BTC-USDT-SWAP",
			BidPx:  "9999.5",
			AskPx:  "10000.5",
			Last:   "10000",
			Ts:     "1700000000000",
		}},
		books: []sdkokx.OrderBook{{
			Bids: [][]string{{"9999.5", "2"}},
			Asks: [][]string{{"10000.5", "3"}},
			Ts:   "1700000000001",
		}},
		balances: []sdkokx.Balance{{
			UTime: "1700000000002",
			Details: []sdkokx.BalanceDetail{{
				Ccy:       "USDT",
				Eq:        "100",
				FrozenBal: "10",
			}},
		}},
		orders: []sdkokx.Order{{
			InstId:    "BTC-USDT-SWAP",
			InstType:  "SWAP",
			OrdId:     "order-1",
			ClOrdId:   "client-1",
			State:     sdkokx.OrderStatusLive,
			Side:      sdkokx.SideBuy,
			OrdType:   sdkokx.OrderTypeLimit,
			Sz:        "5",
			AccFillSz: "1",
			AvgPx:     "10000",
			UTime:     "1700000000003",
		}},
		positions: []sdkokx.Position{{
			InstId:   "BTC-USDT-SWAP",
			InstType: "SWAP",
			PosId:    "pos-1",
			PosSide:  sdkokx.PosSideNet,
			Pos:      "2",
			AvgPx:    "10000",
			Upl:      "1.25",
			UTime:    "1700000000004",
		}},
	}

	testsuite.RunModelContractSuite(t, testsuite.ModelContractSuiteConfig{
		Instrument: inst,
		Account: model.AccountState{
			AccountID: "acct",
			Venue:     model.VenueOKX,
			Type:      model.AccountTypeMargin,
			Reported:  true,
		},
	})

	testsuite.RunVenueContractSuite(t, testsuite.VenueContractSuiteConfig{
		Provider:                 provider,
		MarketData:               newMarketDataClient(provider, fake),
		InstrumentID:             instID,
		ExpectTradesUnsupported:  true,
		ExpectStreamsUnsupported: true,
	})

	testsuite.RunAccountLifecycleSuite(t, testsuite.AccountLifecycleSuiteConfig{
		Execution:   newExecutionClient("acct", provider, fake),
		Instruments: []model.InstrumentID{instID},
	})
}

func TestNewVenueAdapterWiresVenueClients(t *testing.T) {
	adapter, err := NewVenueAdapter(context.Background(), VenueOptions{AccountID: "acct"})
	require.NoError(t, err)
	require.Equal(t, model.VenueOKX, adapter.Venue())
	require.NotNil(t, adapter.Instruments())
	require.NotNil(t, adapter.MarketData())
	require.NotNil(t, adapter.Execution())
	require.Equal(t, model.AccountID("acct"), adapter.Execution().AccountID())

	var _ venue.Adapter = adapter
}

func TestVenueRegistryOpensOKX(t *testing.T) {
	got, err := venue.Open(context.Background(), model.VenueOKX, map[string]string{
		"account_id": "acct",
	})
	require.NoError(t, err)
	require.Equal(t, model.VenueOKX, got.Venue())
	require.Equal(t, model.AccountID("acct"), got.Execution().AccountID())
	require.Equal(t, model.VenueOKX, got.Capabilities().Venue)
}

func TestDeclaredCapabilitiesDescribeCertifiedOKXSlice(t *testing.T) {
	caps := DeclaredCapabilities()
	require.Equal(t, model.VenueOKX, caps.Venue)
	require.Contains(t, caps.InstrumentTypes, model.InstrumentTypeCurrencyPair)
	require.Contains(t, caps.InstrumentTypes, model.InstrumentTypeCryptoPerp)
	require.True(t, caps.MarketData.Ticker)
	require.True(t, caps.MarketData.OrderBook)
	require.True(t, caps.Execution.Submit)
	require.True(t, caps.Execution.OrderReports)
	require.False(t, caps.Execution.FillReports)
	require.True(t, caps.AccountState.Snapshot)
	require.True(t, caps.Reconciliation.Startup)
}

func TestExecutionClientSubmitOrderConvertsPerpQuantityAndPreservesClientID(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-PERP.OKX")
	provider := newInstrumentProviderForTest([]instrumentSeed{{
		RawSymbol: "BTC-USDT-SWAP",
		Product:   venue.ProductHintPerp,
		Base:      model.BTC,
		Quote:     model.USDT,
		Settle:    model.USDT,
		CtVal:     parseString("0.01"),
	}})
	require.NoError(t, provider.LoadAll(context.Background()))

	fake := &fakeOKXVenueClient{}
	exec := newExecutionClient("acct", provider, fake)
	err := exec.SubmitOrder(context.Background(), model.SubmitOrder{
		InstrumentID: instID,
		Side:         model.OrderSideBuy,
		Type:         model.OrderTypeLimit,
		Quantity:     decimal.RequireFromString("0.05"),
		Price:        decimal.RequireFromString("10000"),
		ClientID:     model.ClientOrderID("MiXeD-client-1"),
	})
	require.NoError(t, err)
	require.NotNil(t, fake.lastPlaceOrder)
	require.Equal(t, "BTC-USDT-SWAP", fake.lastPlaceOrder.InstId)
	require.Equal(t, "5", fake.lastPlaceOrder.Sz)
	require.NotNil(t, fake.lastPlaceOrder.ClOrdId)
	require.Equal(t, "MiXeD-client-1", *fake.lastPlaceOrder.ClOrdId)

	select {
	case ev := <-exec.Events():
		require.NotNil(t, ev.Order)
		require.Equal(t, model.ClientOrderID("MiXeD-client-1"), ev.Order.ClientID)
	default:
		t.Fatal("expected order event")
	}
}

type fakeOKXVenueClient struct {
	instruments    []sdkokx.Instrument
	tickers        []sdkokx.Ticker
	books          []sdkokx.OrderBook
	balances       []sdkokx.Balance
	orders         []sdkokx.Order
	positions      []sdkokx.Position
	lastPlaceOrder *sdkokx.OrderRequest
}

func (f *fakeOKXVenueClient) GetInstruments(context.Context, string) ([]sdkokx.Instrument, error) {
	return append([]sdkokx.Instrument(nil), f.instruments...), nil
}

func (f *fakeOKXVenueClient) GetTicker(context.Context, string) ([]sdkokx.Ticker, error) {
	return append([]sdkokx.Ticker(nil), f.tickers...), nil
}

func (f *fakeOKXVenueClient) GetOrderBook(context.Context, string, *int) ([]sdkokx.OrderBook, error) {
	return append([]sdkokx.OrderBook(nil), f.books...), nil
}

func (f *fakeOKXVenueClient) PlaceOrder(_ context.Context, req *sdkokx.OrderRequest) ([]sdkokx.OrderId, error) {
	f.lastPlaceOrder = req
	clOrdID := ""
	if req.ClOrdId != nil {
		clOrdID = *req.ClOrdId
	}
	return []sdkokx.OrderId{{OrdId: "order-new", ClOrdId: clOrdID, SCode: "0", Ts: "1700000000005"}}, nil
}

func (f *fakeOKXVenueClient) ModifyOrder(context.Context, *sdkokx.ModifyOrderRequest) ([]sdkokx.OrderId, error) {
	return []sdkokx.OrderId{{OrdId: "order-new", SCode: "0", Ts: "1700000000006"}}, nil
}

func (f *fakeOKXVenueClient) CancelOrder(context.Context, string, string, string) ([]sdkokx.OrderId, error) {
	return []sdkokx.OrderId{{OrdId: "order-new", SCode: "0", Ts: "1700000000007"}}, nil
}

func (f *fakeOKXVenueClient) CancelOrders(context.Context, []sdkokx.CancelOrderRequest) ([]sdkokx.OrderId, error) {
	return []sdkokx.OrderId{{OrdId: "order-new", SCode: "0", Ts: "1700000000008"}}, nil
}

func (f *fakeOKXVenueClient) GetOrders(context.Context, *string, *string) ([]sdkokx.Order, error) {
	return append([]sdkokx.Order(nil), f.orders...), nil
}

func (f *fakeOKXVenueClient) GetAccountBalance(context.Context, *string) ([]sdkokx.Balance, error) {
	return append([]sdkokx.Balance(nil), f.balances...), nil
}

func (f *fakeOKXVenueClient) GetPositions(context.Context, *string, *string) ([]sdkokx.Position, error) {
	return append([]sdkokx.Position(nil), f.positions...), nil
}
