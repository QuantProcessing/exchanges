# Exchange Adapter Layout Convergence Plan

## Scope

Implement the file-layout convergence defined in `2026-03-23-exchange-adapter-layout-convergence-design.md`.

Packages:

- `backpack`
- `aster`
- `binance`

## Plan

1. Converge Backpack stream files
   - create `backpack/streams.go`
   - move stream methods from `perp_streams.go` and `spot_streams.go`
   - delete the old market-specific stream files

2. Converge Aster orderbook files
   - create `aster/orderbook.go`
   - move orderbook types and methods from `perp_orderbook.go` and `spot_orderbook.go`
   - delete the old market-specific orderbook files

3. Converge Binance orderbook files
   - create `binance/orderbook.go`
   - move orderbook types and methods from `perp_orderbook.go` and `spot_orderbook.go`
   - delete the old market-specific orderbook files

4. Verify
   - run `go test -short ./...`
