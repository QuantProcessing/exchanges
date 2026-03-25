package decibel

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	decibelrest "github.com/QuantProcessing/exchanges/decibel/sdk/rest"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestDecibelMetadataBuildsDeterministicSymbolMap(t *testing.T) {
	markets := []decibelrest.Market{
		{
			MarketAddr: "0xeth",
			MarketName: "ETH/USDC PERP",
			Mode:       "perp",
			MinSize:    decimal.RequireFromString("0.010"),
			TickSize:   decimal.RequireFromString("0.05"),
			PxDecimals: 2,
			SzDecimals: 3,
		},
		{
			MarketAddr: "0xbtc",
			MarketName: "BTC-USDC-PERP",
			Mode:       "perp",
			MinSize:    decimal.RequireFromString("0.001"),
			TickSize:   decimal.RequireFromString("0.25"),
			PxDecimals: 2,
			SzDecimals: 4,
		},
	}

	cache, err := newMarketMetadataCache(markets)
	require.NoError(t, err)

	require.Equal(t, []string{"BTC", "ETH"}, cache.symbols())

	addr, err := cache.marketAddress("btc")
	require.NoError(t, err)
	require.Equal(t, "0xbtc", addr)

	symbol, err := cache.symbolForMarket("eth/usdc perp")
	require.NoError(t, err)
	require.Equal(t, "ETH", symbol)

	details, err := cache.symbolDetails("BTC")
	require.NoError(t, err)
	require.Equal(t, int32(2), details.PricePrecision)
	require.Equal(t, int32(4), details.QuantityPrecision)
	require.True(t, decimal.RequireFromString("0.001").Equal(details.MinQuantity))
}

func TestDecibelMetadataRejectsDuplicateBaseSymbolCollision(t *testing.T) {
	_, err := newMarketMetadataCache([]decibelrest.Market{
		{
			MarketAddr: "0xbtc-one",
			MarketName: "BTC-USDC-PERP",
			Mode:       "perp",
			MinSize:    decimal.RequireFromString("0.001"),
			TickSize:   decimal.RequireFromString("0.25"),
			PxDecimals: 2,
			SzDecimals: 4,
		},
		{
			MarketAddr: "0xbtc-two",
			MarketName: "BTC-USD-PERP",
			Mode:       "perp",
			MinSize:    decimal.RequireFromString("0.001"),
			TickSize:   decimal.RequireFromString("0.50"),
			PxDecimals: 1,
			SzDecimals: 3,
		},
	})

	require.ErrorContains(t, err, "duplicate base symbol")
	require.ErrorContains(t, err, "BTC")
}

func TestDecibelMetadataQuantizesAndConvertsChainUnitsWithDecimal(t *testing.T) {
	meta := marketMetadata{
		BaseSymbol:    "BTC",
		MarketAddr:    "0xbtc",
		MarketName:    "BTC-USDC-PERP",
		MinSize:       decimal.RequireFromString("0.001"),
		TickSize:      decimal.RequireFromString("0.125"),
		PriceDecimals: 3,
		SizeDecimals:  4,
	}

	price, err := meta.quantizePrice(decimal.RequireFromString("12.387"))
	require.NoError(t, err)
	require.True(t, decimal.RequireFromString("12.375").Equal(price))

	size, err := meta.quantizeSize(decimal.RequireFromString("1.23459"))
	require.NoError(t, err)
	require.True(t, decimal.RequireFromString("1.2345").Equal(size))

	priceUnits, err := meta.priceToChainUnits(price)
	require.NoError(t, err)
	require.IsType(t, decimal.Decimal{}, priceUnits)
	require.True(t, decimal.RequireFromString("12375").Equal(priceUnits))

	sizeUnits, err := meta.sizeToChainUnits(size)
	require.NoError(t, err)
	require.IsType(t, decimal.Decimal{}, sizeUnits)
	require.True(t, decimal.RequireFromString("12345").Equal(sizeUnits))

	_, err = meta.quantizeSize(decimal.RequireFromString("0.0009"))
	require.ErrorIs(t, err, exchanges.ErrMinQuantity)
}

func TestDecibelPrecisionRejectsInvalidValuesBeforeChainEncoding(t *testing.T) {
	meta := marketMetadata{
		BaseSymbol:    "BTC",
		MarketAddr:    "0xbtc",
		MarketName:    "BTC-USDC-PERP",
		LotSize:       decimal.RequireFromString("0.0005"),
		MinSize:       decimal.RequireFromString("0.0010"),
		TickSize:      decimal.RequireFromString("0.125"),
		PriceDecimals: 3,
		SizeDecimals:  4,
	}

	priceUnits, err := meta.encodePrice(decimal.RequireFromString("12.375"))
	require.NoError(t, err)
	require.Equal(t, uint64(12375), priceUnits)

	sizeUnits, err := meta.encodeSize(decimal.RequireFromString("1.2345"))
	require.NoError(t, err)
	require.Equal(t, uint64(12345), sizeUnits)

	_, err = meta.encodePrice(decimal.RequireFromString("12.387"))
	require.ErrorIs(t, err, exchanges.ErrInvalidPrecision)

	_, err = meta.encodeSize(decimal.RequireFromString("1.23459"))
	require.ErrorIs(t, err, exchanges.ErrInvalidPrecision)

	_, err = meta.encodeSize(decimal.RequireFromString("1.2346"))
	require.ErrorIs(t, err, exchanges.ErrInvalidPrecision)

	_, err = meta.encodeSize(decimal.RequireFromString("0.0009"))
	require.ErrorIs(t, err, exchanges.ErrMinQuantity)
}

func TestDecibelPrecisionQuantizeSizeUsesLotSizeStep(t *testing.T) {
	meta := marketMetadata{
		BaseSymbol:    "BTC",
		MarketAddr:    "0xbtc",
		MarketName:    "BTC-USDC-PERP",
		LotSize:       decimal.RequireFromString("0.0005"),
		MinSize:       decimal.RequireFromString("0.0010"),
		TickSize:      decimal.RequireFromString("0.125"),
		PriceDecimals: 3,
		SizeDecimals:  4,
	}

	size, err := meta.quantizeSize(decimal.RequireFromString("1.2346"))
	require.NoError(t, err)
	require.True(t, decimal.RequireFromString("1.2345").Equal(size))

	sizeUnits, err := meta.sizeToChainUnits(decimal.RequireFromString("1.2346"))
	require.NoError(t, err)
	require.True(t, decimal.RequireFromString("12345").Equal(sizeUnits))
}

func TestDecibelWSNormalizesOrderStatuses(t *testing.T) {
	require.Equal(t, exchanges.OrderStatusNew, normalizeOrderStatus("Placed"))
	require.Equal(t, exchanges.OrderStatusNew, normalizeOrderStatus("Open"))
	require.Equal(t, exchanges.OrderStatusPartiallyFilled, normalizeOrderStatus("PartiallyFilled"))
	require.Equal(t, exchanges.OrderStatusFilled, normalizeOrderStatus("Filled"))
	require.Equal(t, exchanges.OrderStatusCancelled, normalizeOrderStatus("Cancelled"))
	require.Equal(t, exchanges.OrderStatusCancelled, normalizeOrderStatus("Expired"))
	require.Equal(t, exchanges.OrderStatusRejected, normalizeOrderStatus("Rejected"))
	require.Equal(t, exchanges.OrderStatusUnknown, normalizeOrderStatus("mystery"))
}

func TestDecibelMetadataRecognizesLiveMarketModes(t *testing.T) {
	require.True(t, isPerpMode(""))
	require.True(t, isPerpMode("perp"))
	require.True(t, isPerpMode("Open"))
	require.True(t, isPerpMode("ReduceOnly"))
	require.True(t, isPerpMode("CloseOnly"))
	require.False(t, isPerpMode("spot"))
}

func TestDecibelMetadataScalesLiveIntegerPrecisionFields(t *testing.T) {
	meta, err := newMarketMetadata(decibelrest.Market{
		MarketAddr: "0xbtc",
		MarketName: "BTC/USD",
		Mode:       "Open",
		MinSize:    decimal.RequireFromString("2000"),
		LotSize:    decimal.RequireFromString("1000"),
		TickSize:   decimal.RequireFromString("5000"),
		PxDecimals: 6,
		SzDecimals: 8,
	})
	require.NoError(t, err)
	require.True(t, decimal.RequireFromString("0.00002").Equal(meta.MinSize))
	require.True(t, decimal.RequireFromString("0.00001").Equal(meta.LotSize))
	require.True(t, decimal.RequireFromString("0.005").Equal(meta.TickSize))
}
