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
// The package is organized into several layers:
//
//   - Root package (exchanges): Unified interfaces, models, errors, and utilities
//   - Account package (account/): TradingAccount and OrderFlow runtime helpers
//   - Exchange packages (binance/, okx/, ...): Exchange-specific adapters and SDK clients
//   - Testsuite package: Adapter compliance test suite
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
