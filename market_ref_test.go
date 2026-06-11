package exchanges_test

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestParseMarketRefUsesDefaultQuoteForBaseOnlySymbols(t *testing.T) {
	t.Parallel()

	ref := exchanges.ParseMarketRef("btc", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypePerp)

	require.Equal(t, "BTC", ref.Base)
	require.Equal(t, exchanges.QuoteCurrencyUSDT, ref.Quote)
	require.Equal(t, exchanges.QuoteCurrencyUSDT, ref.Settle)
	require.Equal(t, exchanges.MarketTypePerp, ref.Type)
	require.Empty(t, ref.VenueSymbol)
	require.Equal(t, "BTC/USDT", ref.Symbol())
}

func TestParseMarketRefAcceptsExplicitQuote(t *testing.T) {
	t.Parallel()

	ref := exchanges.ParseMarketRef("eth/usdc", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypeSpot)

	require.Equal(t, "ETH", ref.Base)
	require.Equal(t, exchanges.QuoteCurrencyUSDC, ref.Quote)
	require.Equal(t, exchanges.QuoteCurrencyUSDC, ref.Settle)
	require.Equal(t, exchanges.MarketTypeSpot, ref.Type)
	require.Equal(t, "ETH/USDC", ref.Symbol())
}

func TestParseMarketRefAcceptsExplicitSettle(t *testing.T) {
	t.Parallel()

	ref := exchanges.ParseMarketRef("BTC/USDT:USDC", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypePerp)

	require.Equal(t, "BTC", ref.Base)
	require.Equal(t, exchanges.QuoteCurrencyUSDT, ref.Quote)
	require.Equal(t, exchanges.QuoteCurrencyUSDC, ref.Settle)
	require.Equal(t, "BTC/USDT:USDC", ref.String())
}

func TestParseMarketRefAcceptsOKXStyleSymbols(t *testing.T) {
	t.Parallel()

	ref := exchanges.ParseMarketRef("BTC-USDC-SWAP", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypePerp)

	require.Equal(t, "BTC", ref.Base)
	require.Equal(t, exchanges.QuoteCurrencyUSDC, ref.Quote)
	require.Equal(t, exchanges.QuoteCurrencyUSDC, ref.Settle)
	require.Equal(t, exchanges.MarketTypePerp, ref.Type)
	require.Equal(t, "BTC/USDC", ref.Symbol())
}

func TestMarketRefKeyIncludesTypeQuoteAndSettle(t *testing.T) {
	t.Parallel()

	usdt := exchanges.ParseMarketRef("BTC/USDT", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypePerp)
	usdc := exchanges.ParseMarketRef("BTC/USDC", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypePerp)

	require.NotEqual(t, usdt.Key(), usdc.Key())
	require.Equal(t, "perp:BTC/USDT:USDT", usdt.Key())
	require.Equal(t, "perp:BTC/USDC:USDC", usdc.Key())
}
