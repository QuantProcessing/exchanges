# Decibel Perp Adapter Handoff

## Status

Completed and merged to `main`.

## Final Commit

- `b8c8dad` `Add decibel perp adapter`

## Source Documents

- Spec: `docs/superpowers/specs/2026-03-24-decibel-perp-adapter-design.md`
- Plan: `docs/superpowers/plans/2026-03-24-decibel-perp-adapter.md`

## Final State

All planned tasks were completed and the adapter was merged after review plus live verification.

Key landed behavior:

- constructor config is `DECIBEL_API_KEY + DECIBEL_PRIVATE_KEY + DECIBEL_SUBACCOUNT_ADDR`
- `DECIBEL_PRIVATE_KEY` is the API wallet private key
- API wallet address is derived internally and used for private WebSocket topics
- account/order REST reads use the configured subaccount address
- `FetchOrders` uses order history rather than open orders
- single-order reconciliation uses `GET /api/v1/orders`
- private order updates use wallet-address `order_updates` plus `user_order_history`
- timeout fallback no longer exposes Aptos tx hashes as stable public order IDs

## Final Verification

- `go test -mod=mod ./decibel/... -count=1`
- `GOCACHE=/tmp/exchanges-gocache RUN_FULL=1 go test -mod=mod ./decibel -run "TestPerpAdapter_(Compliance|Orders|Lifecycle)$" -count=1 -v`

## Recommended Next Step

No adapter implementation follow-up is required before merge; this handoff remains only as historical record.
