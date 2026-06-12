// Package exchanges provides a unified Go SDK for interacting with multiple
// cryptocurrency exchanges. It offers both low-level SDK clients (REST + WebSocket)
// and high-level adapters implementing a common [Exchange] interface.
//
// # Quick Start
//
//	adp, err := binance.NewAdapter(ctx, binance.Options{
//	    APIKey:    "your-api-key",
//	    SecretKey: "your-secret-key",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer adp.Close()
//
//	ticker, err := adp.FetchTicker(ctx, "BTC")
//
// # Architecture
//
// The module exposes several public entry layers:
//
//   - Root package (exchanges): normalized interfaces, models, errors, registry,
//     capabilities, and helpers.
//   - SDK packages (sdk/binance, sdk/okx, ...): venue-native REST and WebSocket
//     clients aligned with official exchange APIs.
//   - Adapter packages (adapter/binance, adapter/okx, ...): normalized
//     cross-exchange convenience implementations of the root interfaces.
//   - Account package (account): TradingAccount, OrderTracker, stream health,
//     and lifecycle state runtime.
//   - Testsuite package: adapter compliance test suite.
//
// # Logger
//
// All adapters accept an optional [Logger] interface for structured logging.
// The interface is compatible with *zap.SugaredLogger. If no logger is provided,
// [NopLogger] is used (all output discarded).
//
// # Symbol Convention
//
// All Exchange methods accept a base currency symbol (e.g. "BTC", "ETH").
// The adapter handles conversion to exchange-specific formats internally.
package exchanges
