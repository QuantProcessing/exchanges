package exchanges_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

var _ exchanges.OptionExchange = (*stubOptionExchange)(nil)

type stubOptionExchange struct {
	stubExchange
}

func (s *stubOptionExchange) GetMarketType() exchanges.MarketType {
	return exchanges.MarketTypeOption
}

func (s *stubOptionExchange) ListOptionContracts(context.Context, string) ([]exchanges.OptionContract, error) {
	return nil, nil
}

func (s *stubOptionExchange) FetchOptionContract(context.Context, string) (*exchanges.OptionContract, error) {
	return nil, nil
}

func TestMarketTypeOptionAndOptionContractModel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "option", string(exchanges.MarketTypeOption))
	require.Equal(t, exchanges.OptionType("CALL"), exchanges.OptionTypeCall)
	require.Equal(t, exchanges.OptionType("PUT"), exchanges.OptionTypePut)

	contract := exchanges.OptionContract{
		Symbol:         "BTC-USDT-USDT-20260626-140000-C",
		ExchangeSymbol: "BTC-260626-140000-C",
		Underlying:     "BTC",
		BaseAsset:      "BTC",
		QuoteAsset:     "USDT",
		SettleAsset:    "USDT",
		Type:           exchanges.OptionTypeCall,
		StrikePrice:    decimal.RequireFromString("140000"),
		ExpiryTime:     1782460800000,
		ContractSize:   decimal.RequireFromString("0.01"),
		TickSize:       decimal.RequireFromString("0.1"),
		LotSize:        decimal.RequireFromString("0.001"),
		MinQuantity:    decimal.RequireFromString("0.001"),
		Status:         "TRADING",
	}

	require.Equal(t, "140000", contract.StrikePrice.String())
	require.Equal(t, "0.01", contract.ContractSize.String())
	require.Equal(t, "0.1", contract.TickSize.String())
	require.Equal(t, "0.001", contract.LotSize.String())
	require.Equal(t, "0.001", contract.MinQuantity.String())
}

func TestNewOptionSymbolIncludesQuoteAndSettleDimensions(t *testing.T) {
	t.Parallel()

	symbol := exchanges.NewOptionSymbol(
		"btc",
		"usdt",
		"usdt",
		time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC).UnixMilli(),
		decimal.RequireFromString("140000"),
		exchanges.OptionTypeCall,
	)

	require.Equal(t, "BTC-USDT-USDT-20260626-140000-C", symbol)

	parts, ok := exchanges.ParseOptionSymbol(symbol)
	require.True(t, ok)
	require.False(t, parts.Legacy)
	require.Equal(t, "BTC", parts.BaseAsset)
	require.Equal(t, "USDT", parts.QuoteAsset)
	require.Equal(t, "USDT", parts.SettleAsset)
	require.Equal(t, exchanges.OptionTypeCall, parts.Type)

	legacy, ok := exchanges.ParseOptionSymbol("BTC-20260626-140000-C")
	require.True(t, ok)
	require.True(t, legacy.Legacy)
	require.Equal(t, "BTC", legacy.BaseAsset)
	require.Equal(t, "", legacy.QuoteAsset)
}

func TestOptionTypeNormalizationRejectsUnknownValues(t *testing.T) {
	t.Parallel()

	require.Equal(t, exchanges.OptionTypeCall, exchanges.NormalizeOptionType("C"))
	require.Equal(t, exchanges.OptionTypeCall, exchanges.NormalizeOptionType("Call"))
	require.Equal(t, exchanges.OptionTypePut, exchanges.NormalizeOptionType("P"))
	require.Equal(t, exchanges.OptionTypePut, exchanges.NormalizeOptionType("Put"))
	require.Empty(t, exchanges.NormalizeOptionType(""))
	require.Empty(t, exchanges.NormalizeOptionType("sideways"))

	_, ok := exchanges.ParseOptionSymbol("BTC-USDT-USDT-20260626-140000-X")
	require.False(t, ok)
	require.Empty(t, exchanges.NewOptionSymbol("BTC", "USDT", "USDT", 1782460800000, decimal.RequireFromString("140000"), "sideways"))
}

func TestOptionCapabilitiesCanDescribePublicDataOnlyAdapters(t *testing.T) {
	name := fmt.Sprintf("OPTION_MODEL_TEST_%d", time.Now().UnixNano())

	exchanges.RegisterCapabilities(name, exchanges.MarketTypeOption, exchanges.Capabilities{
		FetchOptionContracts: true,
	})

	caps, ok := exchanges.LookupCapabilities(name, exchanges.MarketTypeOption)
	require.True(t, ok)
	require.True(t, caps.FetchOptionContracts)
	require.False(t, caps.PlaceOrder)
	require.False(t, caps.PlaceOrderWS)
	require.False(t, caps.WatchOrderBook)
	require.False(t, caps.TradingAccountReady)
}
