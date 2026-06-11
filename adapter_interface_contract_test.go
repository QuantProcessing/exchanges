package exchanges_test

import (
	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/binance"
	"github.com/QuantProcessing/exchanges/bitget"
	"github.com/QuantProcessing/exchanges/bybit"
	"github.com/QuantProcessing/exchanges/hyperliquid"
	"github.com/QuantProcessing/exchanges/lighter"
	"github.com/QuantProcessing/exchanges/okx"
)

var (
	_ exchanges.MarketDataExchange      = (*binance.Adapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*binance.Adapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*binance.Adapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*binance.Adapter)(nil)
	_ exchanges.Streamable              = (*binance.Adapter)(nil)
	_ exchanges.PerpRiskExchange        = (*binance.Adapter)(nil)
	_ exchanges.PerpMarketAnalytics     = (*binance.Adapter)(nil)
	_ exchanges.InstrumentExchange      = (*binance.Adapter)(nil)

	_ exchanges.MarketDataExchange      = (*binance.SpotAdapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*binance.SpotAdapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*binance.SpotAdapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*binance.SpotAdapter)(nil)
	_ exchanges.Streamable              = (*binance.SpotAdapter)(nil)
	_ exchanges.SpotBalanceExchange     = (*binance.SpotAdapter)(nil)
	_ exchanges.AssetTransferExchange   = (*binance.SpotAdapter)(nil)
	_ exchanges.InstrumentExchange      = (*binance.SpotAdapter)(nil)

	_ exchanges.MarketDataExchange      = (*okx.Adapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*okx.Adapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*okx.Adapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*okx.Adapter)(nil)
	_ exchanges.Streamable              = (*okx.Adapter)(nil)
	_ exchanges.PerpRiskExchange        = (*okx.Adapter)(nil)
	_ exchanges.PerpMarketAnalytics     = (*okx.Adapter)(nil)
	_ exchanges.InstrumentExchange      = (*okx.Adapter)(nil)

	_ exchanges.MarketDataExchange      = (*okx.SpotAdapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*okx.SpotAdapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*okx.SpotAdapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*okx.SpotAdapter)(nil)
	_ exchanges.Streamable              = (*okx.SpotAdapter)(nil)
	_ exchanges.SpotBalanceExchange     = (*okx.SpotAdapter)(nil)
	_ exchanges.AssetTransferExchange   = (*okx.SpotAdapter)(nil)
	_ exchanges.InstrumentExchange      = (*okx.SpotAdapter)(nil)

	_ exchanges.MarketDataExchange      = (*bybit.Adapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*bybit.Adapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*bybit.Adapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*bybit.Adapter)(nil)
	_ exchanges.Streamable              = (*bybit.Adapter)(nil)
	_ exchanges.PerpRiskExchange        = (*bybit.Adapter)(nil)
	_ exchanges.PerpMarketAnalytics     = (*bybit.Adapter)(nil)
	_ exchanges.InstrumentExchange      = (*bybit.Adapter)(nil)

	_ exchanges.MarketDataExchange      = (*bybit.SpotAdapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*bybit.SpotAdapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*bybit.SpotAdapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*bybit.SpotAdapter)(nil)
	_ exchanges.Streamable              = (*bybit.SpotAdapter)(nil)
	_ exchanges.SpotBalanceExchange     = (*bybit.SpotAdapter)(nil)
	_ exchanges.AssetTransferExchange   = (*bybit.SpotAdapter)(nil)
	_ exchanges.InstrumentExchange      = (*bybit.SpotAdapter)(nil)

	_ exchanges.MarketDataExchange      = (*bitget.Adapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*bitget.Adapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*bitget.Adapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*bitget.Adapter)(nil)
	_ exchanges.Streamable              = (*bitget.Adapter)(nil)
	_ exchanges.PerpRiskExchange        = (*bitget.Adapter)(nil)
	_ exchanges.PerpMarketAnalytics     = (*bitget.Adapter)(nil)
	_ exchanges.InstrumentExchange      = (*bitget.Adapter)(nil)

	_ exchanges.MarketDataExchange      = (*bitget.SpotAdapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*bitget.SpotAdapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*bitget.SpotAdapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*bitget.SpotAdapter)(nil)
	_ exchanges.Streamable              = (*bitget.SpotAdapter)(nil)
	_ exchanges.SpotBalanceExchange     = (*bitget.SpotAdapter)(nil)
	_ exchanges.AssetTransferExchange   = (*bitget.SpotAdapter)(nil)
	_ exchanges.InstrumentExchange      = (*bitget.SpotAdapter)(nil)

	_ exchanges.MarketDataExchange      = (*hyperliquid.Adapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*hyperliquid.Adapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*hyperliquid.Adapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*hyperliquid.Adapter)(nil)
	_ exchanges.Streamable              = (*hyperliquid.Adapter)(nil)
	_ exchanges.PerpRiskExchange        = (*hyperliquid.Adapter)(nil)
	_ exchanges.PerpMarketAnalytics     = (*hyperliquid.Adapter)(nil)

	_ exchanges.MarketDataExchange      = (*hyperliquid.SpotAdapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*hyperliquid.SpotAdapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*hyperliquid.SpotAdapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*hyperliquid.SpotAdapter)(nil)
	_ exchanges.Streamable              = (*hyperliquid.SpotAdapter)(nil)
	_ exchanges.SpotBalanceExchange     = (*hyperliquid.SpotAdapter)(nil)
	_ exchanges.AssetTransferExchange   = (*hyperliquid.SpotAdapter)(nil)

	_ exchanges.MarketDataExchange      = (*lighter.Adapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*lighter.Adapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*lighter.Adapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*lighter.Adapter)(nil)
	_ exchanges.Streamable              = (*lighter.Adapter)(nil)
	_ exchanges.PerpRiskExchange        = (*lighter.Adapter)(nil)
	_ exchanges.PerpMarketAnalytics     = (*lighter.Adapter)(nil)

	_ exchanges.MarketDataExchange      = (*lighter.SpotAdapter)(nil)
	_ exchanges.OrderExecutionExchange  = (*lighter.SpotAdapter)(nil)
	_ exchanges.AccountSnapshotExchange = (*lighter.SpotAdapter)(nil)
	_ exchanges.LocalOrderBookExchange  = (*lighter.SpotAdapter)(nil)
	_ exchanges.Streamable              = (*lighter.SpotAdapter)(nil)
)
